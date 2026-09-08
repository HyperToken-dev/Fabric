package common

import (
	"context"

	"github.com/HyperToken-dev/fabric/internal/storage/postgres"
)

type integralLogAdapter struct {
	store *postgres.ProxyStore
}

// NewIntegralLogAdapter exposes PostgreSQL-backed audit logging to provider packages.
func NewIntegralLogAdapter(store *postgres.ProxyStore) integralLogAdapter {
	return integralLogAdapter{store: store}
}

func (a integralLogAdapter) InsertIntegralLog(ctx context.Context, keyID int32, context string, response string) error {
	return a.store.InsertIntegratedLog(ctx, keyID, context, response)
}
