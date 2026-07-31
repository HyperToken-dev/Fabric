package main

import (
	"context"
	"encoding/json"
	"strings"
)

type ProviderTaskStatus int16

const (
	ProviderTaskStatusPending ProviderTaskStatus = 1
	ProviderTaskStatusSuccess ProviderTaskStatus = 2
	ProviderTaskStatusFail    ProviderTaskStatus = 3
)

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

type ProviderTaskCompletion struct {
	Provider         Provider
	ProviderTaskID   string
	Status           ProviderTaskStatus
	Response         json.RawMessage
	CompletionTokens int64
}

type ProviderTaskStore interface {
	CreateProviderTask(ctx context.Context, task ProviderTaskInfo) error
	CompleteProviderTask(ctx context.Context, completion ProviderTaskCompletion) (bool, error)
}

type NoopProviderTaskStore struct{}

func (NoopProviderTaskStore) CreateProviderTask(ctx context.Context, task ProviderTaskInfo) error {
	return nil
}

func (NoopProviderTaskStore) CompleteProviderTask(ctx context.Context, completion ProviderTaskCompletion) (bool, error) {
	return false, nil
}

func normalizeProviderTaskStatus(status string) ProviderTaskStatus {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "success", "succeeded", "completed", "complete":
		return ProviderTaskStatusSuccess
	case "fail", "failed", "failure", "error", "cancelled", "canceled":
		return ProviderTaskStatusFail
	default:
		return ProviderTaskStatusPending
	}
}

func isTerminalProviderTaskStatus(status ProviderTaskStatus) bool {
	return status == ProviderTaskStatusSuccess || status == ProviderTaskStatusFail
}
