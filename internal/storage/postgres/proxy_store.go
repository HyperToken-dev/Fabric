package postgres

import (
	"context"
	"database/sql"
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
	queries *repository.Queries
}

func NewProxyStore(queries *repository.Queries) *ProxyStore {
	return &ProxyStore{queries: queries}
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
	_, err := s.queries.InsertUsageLog(ctx, repository.InsertUsageLogParams{
		KeyID:            keyID,
		ChannelID:        channelID,
		ModelID:          modelID,
		PromptTokens:     promptTokens,
		CompletionTokens: completionTokens,
	})
	return err
}
