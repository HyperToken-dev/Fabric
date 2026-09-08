package common

import (
	"context"
	"encoding/json"
	"strings"
)

// ProviderTaskStatus normalizes asynchronous provider lifecycle states for storage.
type ProviderTaskStatus int16

const (
	ProviderTaskStatusPending ProviderTaskStatus = 1
	ProviderTaskStatusSuccess ProviderTaskStatus = 2
	ProviderTaskStatusFail    ProviderTaskStatus = 3
)

// ProviderTaskInfo records a Fabric-created asynchronous provider task.
type ProviderTaskInfo struct {
	Provider       Provider
	KeyID          int32
	ChannelID      int32
	ModelID        int32
	ProviderTaskID string
	Status         ProviderTaskStatus
	Request        json.RawMessage
	Response       json.RawMessage
}

// ProviderTaskCompletion records terminal or latest observed provider task state.
type ProviderTaskCompletion struct {
	Provider         Provider
	ProviderTaskID   string
	Status           ProviderTaskStatus
	Response         json.RawMessage
	CompletionTokens int64
}

// ProviderTaskStore persists task state and coordinates idempotent usage recording.
type ProviderTaskStore interface {
	CreateProviderTask(ctx context.Context, task ProviderTaskInfo) error
	CompleteProviderTask(ctx context.Context, completion ProviderTaskCompletion) (bool, error)
}

// NoopProviderTaskStore disables task persistence for tests and optional wiring.
type NoopProviderTaskStore struct{}

func (NoopProviderTaskStore) CreateProviderTask(ctx context.Context, task ProviderTaskInfo) error {
	return nil
}

func (NoopProviderTaskStore) CompleteProviderTask(ctx context.Context, completion ProviderTaskCompletion) (bool, error) {
	return false, nil
}

// NormalizeProviderTaskStatus converts provider status strings into Fabric task states.
func NormalizeProviderTaskStatus(status string) ProviderTaskStatus {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "success", "succeeded", "completed", "complete":
		return ProviderTaskStatusSuccess
	case "fail", "failed", "failure", "error", "cancelled", "canceled":
		return ProviderTaskStatusFail
	default:
		return ProviderTaskStatusPending
	}
}

// IsTerminalProviderTaskStatus reports whether no further provider lifecycle updates are expected.
func IsTerminalProviderTaskStatus(status ProviderTaskStatus) bool {
	return status == ProviderTaskStatusSuccess || status == ProviderTaskStatusFail
}
