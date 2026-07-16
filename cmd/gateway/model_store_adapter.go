package main

import (
	"context"

	"github.com/HyperToken-dev/fabric/internal/storage/postgres"
)

type modelStoreAdapter struct {
	store *postgres.ProxyStore
}

func newModelStoreAdapter(store *postgres.ProxyStore) modelStoreAdapter {
	return modelStoreAdapter{store: store}
}

func (a modelStoreAdapter) ResolveModel(ctx context.Context, channelID int32, modelName string) (*ModelInfo, error) {
	model, err := a.store.ResolveModel(ctx, channelID, modelName)
	if err != nil {
		return nil, err
	}
	if model == nil {
		return nil, nil
	}

	status := ModelStatusUnknown
	switch model.Status {
	case postgres.ModelStatusActive:
		status = ModelStatusActive
	case postgres.ModelStatusBanned:
		status = ModelStatusBanned
	}
	return &ModelInfo{ID: model.ID, Status: status}, nil
}
