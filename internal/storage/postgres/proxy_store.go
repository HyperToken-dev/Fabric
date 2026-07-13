package postgres

import (
	"context"
	"database/sql"
	"strings"

	"fabric/internal/proxy"
	"fabric/internal/repository"
)

const (
	modelStatusActive int16 = 1
	modelStatusBanned int16 = 2
)

type ProxyStore struct {
	queries *repository.Queries
}

func NewProxyStore(queries *repository.Queries) *ProxyStore {
	return &ProxyStore{queries: queries}
}

func (s *ProxyStore) ResolveModel(ctx context.Context, channelID int32, modelName string) (*proxy.ModelInfo, error) {
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

	status := proxy.ModelStatusUnknown
	switch model.Status {
	case modelStatusActive:
		status = proxy.ModelStatusActive
	case modelStatusBanned:
		status = proxy.ModelStatusBanned
	}
	return &proxy.ModelInfo{ID: model.ID, Status: status}, nil
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
