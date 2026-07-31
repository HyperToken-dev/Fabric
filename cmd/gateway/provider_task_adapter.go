package main

import (
	"context"

	"github.com/HyperToken-dev/fabric/internal/storage/postgres"
)

type providerTaskStoreAdapter struct {
	store *postgres.ProxyStore
}

func newProviderTaskStoreAdapter(store *postgres.ProxyStore) providerTaskStoreAdapter {
	return providerTaskStoreAdapter{store: store}
}

func (a providerTaskStoreAdapter) CreateProviderTask(ctx context.Context, task ProviderTaskInfo) error {
	return a.store.CreateProviderTask(ctx, postgres.ProviderTaskInfo{
		Provider:       string(task.Provider),
		KeyID:          task.KeyID,
		ChannelID:      task.ChannelID,
		ModelID:        task.ModelID,
		ProviderTaskID: task.ProviderTaskID,
		Status:         postgres.ProviderTaskStatus(task.Status),
		Request:        task.Request,
		Response:       task.Response,
	})
}

func (a providerTaskStoreAdapter) CompleteProviderTask(ctx context.Context, completion ProviderTaskCompletion) (bool, error) {
	return a.store.CompleteProviderTask(ctx, postgres.ProviderTaskCompletion{
		Provider:         string(completion.Provider),
		ProviderTaskID:   completion.ProviderTaskID,
		Status:           postgres.ProviderTaskStatus(completion.Status),
		Response:         completion.Response,
		CompletionTokens: completion.CompletionTokens,
	})
}
