package main

import (
	"context"

	"github.com/HyperToken-dev/fabric/internal/storage/postgres"
)

type openAIIntegralLogAdapter struct {
	store *postgres.ProxyStore
}

func newOpenAIIntegralLogAdapter(store *postgres.ProxyStore) openAIIntegralLogAdapter {
	return openAIIntegralLogAdapter{store: store}
}

func (a openAIIntegralLogAdapter) InsertIntegralLog(ctx context.Context, keyID int32, context string, response string) error {
	return a.store.InsertIntegratedLog(ctx, keyID, context, response)
}
