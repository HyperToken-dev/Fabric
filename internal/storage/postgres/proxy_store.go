package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"fabric/business/usage"
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
	fmt.Println(channelID, modelName)
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
	fmt.Println(model.ID, model.ModelName)
	return &proxy.ModelInfo{ID: model.ID, Status: status}, nil
}

func (s *ProxyStore) RecordUsage(ctx context.Context, record usage.Record) error {
	_, err := s.queries.InsertUsageLog(ctx, repository.InsertUsageLogParams{
		KeyID:            record.KeyID,
		ChannelID:        record.ChannelID,
		ModelID:          record.ModelID,
		PromptTokens:     record.PromptTokens,
		CompletionTokens: record.CompletionTokens,
	})
	return err
}
