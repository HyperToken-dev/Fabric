package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"

	"github.com/HyperToken-dev/fabric/internal/repository"
)

type ModelStatus int

const (
	ModelStatusUnknown ModelStatus = iota
	ModelStatusActive
	ModelStatusBanned
)

const (
	modelStatusActive int16 = 1
	modelStatusBanned int16 = 2
)

type ModelInfo struct {
	ID     int32
	Status ModelStatus
}

type ProxyStore struct {
	db      *sql.DB
	queries *repository.Queries
}

func NewProxyStore(db *sql.DB) *ProxyStore {
	return &ProxyStore{db: db, queries: repository.New(db)}
}

func (s *ProxyStore) ResolveModel(ctx context.Context, channelID int32, modelName string) (*ModelInfo, error) {
	model, err := s.queries.GetModelByChannelAndName(ctx, repository.GetModelByChannelAndNameParams{
		ChannelID: channelID,
		ModelName: strings.TrimSpace(modelName),
	})
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	status := ModelStatusUnknown
	switch model.Status {
	case modelStatusActive:
		status = ModelStatusActive
	case modelStatusBanned:
		status = ModelStatusBanned
	}
	return &ModelInfo{ID: model.ID, Status: status}, nil
}

func (s *ProxyStore) InsertUsage(ctx context.Context, keyID, channelID, modelID int32, promptTokens, completionTokens int64) error {
	key, err := s.queries.GetApiKeyById(ctx, keyID)
	if err != nil {
		return err
	}
	_, err = s.queries.InsertUsageLog(ctx, repository.InsertUsageLogParams{
		KeyID:            keyID,
		ChannelID:        channelID,
		ModelID:          modelID,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
		OwnerOpenid:      key.OwnerOpenid,
	})
	return err
}

func (s *ProxyStore) InsertIntegratedLog(ctx context.Context, keyID int32, context, response string) error {
	key, err := s.queries.GetApiKeyById(ctx, keyID)
	if err != nil {
		return err
	}
	_, err = s.queries.CreateIntegralLog(ctx, repository.CreateIntegralLogParams{
		Context:     json.RawMessage(context),
		Response:    sql.NullString{String: response, Valid: true},
		KeyID:       keyID,
		OwnerOpenid: key.OwnerOpenid,
	})
	return err
}

type ProviderTaskStatus int16

const (
	ProviderTaskStatusPending ProviderTaskStatus = 1
	ProviderTaskStatusSuccess ProviderTaskStatus = 2
	ProviderTaskStatusFail    ProviderTaskStatus = 3
)

type ProviderTaskInfo struct {
	Provider       string
	KeyID          int32
	ChannelID      int32
	ModelID        int32
	ProviderTaskID string
	Status         ProviderTaskStatus
	Request        json.RawMessage
	Response       json.RawMessage
}

type ProviderTaskCompletion struct {
	Provider         string
	ProviderTaskID   string
	Status           ProviderTaskStatus
	Response         json.RawMessage
	CompletionTokens int64
}

func (s *ProxyStore) CreateProviderTask(ctx context.Context, task ProviderTaskInfo) error {
	_, err := s.queries.CreateProviderTask(ctx, repository.CreateProviderTaskParams{
		Provider:       task.Provider,
		KeyID:          task.KeyID,
		ChannelID:      task.ChannelID,
		ModelID:        task.ModelID,
		ProviderTaskID: task.ProviderTaskID,
		Status:         int16(task.Status),
		Request:        append(json.RawMessage(nil), task.Request...),
		Response:       cloneRawMessage(task.Response),
	})
	if errors.Is(err, sql.ErrNoRows) {
		return nil
	}
	return err
}

func (s *ProxyStore) CompleteProviderTask(ctx context.Context, completion ProviderTaskCompletion) (bool, error) {
	if s.db == nil {
		return false, errors.New("provider task completion requires database handle")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer func() {
		if tx != nil {
			_ = tx.Rollback()
		}
	}()

	qtx := s.queries.WithTx(tx)
	task, err := qtx.GetProviderTaskForUpdate(ctx, repository.GetProviderTaskForUpdateParams{
		Provider:       completion.Provider,
		ProviderTaskID: completion.ProviderTaskID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		if err := tx.Commit(); err != nil {
			return false, err
		}
		tx = nil
		return false, nil
	}
	if err != nil {
		return false, err
	}

	if task.Status == int16(ProviderTaskStatusPending) {
		if isTerminalProviderTaskStatus(completion.Status) {
			task, err = qtx.UpdateProviderTaskTerminalResponse(ctx, repository.UpdateProviderTaskTerminalResponseParams{
				Provider:       completion.Provider,
				ProviderTaskID: completion.ProviderTaskID,
				Status:         int16(completion.Status),
				Response:       cloneRawMessage(completion.Response),
			})
		} else {
			task, err = qtx.UpdateProviderTaskPendingResponse(ctx, repository.UpdateProviderTaskPendingResponseParams{
				Provider:       completion.Provider,
				ProviderTaskID: completion.ProviderTaskID,
				Status:         int16(ProviderTaskStatusPending),
				Response:       cloneRawMessage(completion.Response),
			})
		}
		if err != nil {
			return false, err
		}
	}

	insertedUsage := false
	if task.Status == int16(ProviderTaskStatusSuccess) && !task.UsageRecorded && completion.CompletionTokens > 0 {
		key, err := qtx.GetApiKeyById(ctx, task.KeyID)
		if err != nil {
			return false, err
		}
		if _, err := qtx.InsertUsageLog(ctx, repository.InsertUsageLogParams{
			KeyID:            task.KeyID,
			ChannelID:        task.ChannelID,
			ModelID:          task.ModelID,
			PromptTokens:     0,
			CompletionTokens: completion.CompletionTokens,
			OwnerOpenid:      key.OwnerOpenid,
		}); err != nil {
			return false, err
		}
		if _, err := qtx.MarkProviderTaskUsageRecorded(ctx, repository.MarkProviderTaskUsageRecordedParams{
			Provider:       completion.Provider,
			ProviderTaskID: completion.ProviderTaskID,
		}); err != nil {
			return false, err
		}
		insertedUsage = true
	}

	if err := tx.Commit(); err != nil {
		return false, err
	}
	tx = nil
	return insertedUsage, nil
}

func cloneRawMessage(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage(`null`)
	}
	return append(json.RawMessage(nil), raw...)
}

func isTerminalProviderTaskStatus(status ProviderTaskStatus) bool {
	return status == ProviderTaskStatusSuccess || status == ProviderTaskStatusFail
}
