package proxy

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"hyper-token/internal/repository"
)

type Provider string

const (
	ProviderOpenAI Provider = "openai"
)

const (
	modelStatusActive  int16 = 1
	modelStatusBanned  int16 = 2
	modelStatusPending int16 = 3
	modelTypeText      int32 = 1
)

type contextKey string

const (
	ctxKeyID     contextKey = "key_id"
	ctxChannelID contextKey = "channel_id"
	ctxModel     contextKey = "model"
	ctxModelID   contextKey = "model_id"
	ctxProvider  contextKey = "provider"
	ctxUpstream  contextKey = "upstream"
	ctxAPIKey    contextKey = "api_key"
)

type usageLog struct {
	PromptTokens   int `json:"prompt_tokens"`
	CompleteTokens int `json:"complete_tokens"`
}

func setContextInt32(r *http.Request, key contextKey, val int32) *http.Request {
	ctx := context.WithValue(r.Context(), key, val)
	return r.WithContext(ctx)
}

func setContextString(r *http.Request, key contextKey, val string) *http.Request {
	ctx := context.WithValue(r.Context(), key, val)
	return r.WithContext(ctx)
}

func getContextInt32(r *http.Request, key contextKey) int32 {
	v, _ := r.Context().Value(key).(int32)
	return v
}

func getContextString(r *http.Request, key contextKey) string {
	v, _ := r.Context().Value(key).(string)
	return v
}

func processNonStreaming(ctx context.Context, body []byte, keyID, channelID, modelID int32, provider Provider, queries *repository.Queries) error {
	usagelog, err := extractTokenUsage(body, provider)
	if err != nil {
		return err
	}
	return insertUsageLog(ctx, queries, keyID, channelID, modelID, usagelog)
}

func insertUsageLog(ctx context.Context, queries *repository.Queries, keyID, channelID, modelID int32, usagelog *usageLog) error {
	_, err := queries.InsertUsageLog(ctx, repository.InsertUsageLogParams{
		KeyID:            keyID,
		ChannelID:        channelID,
		ModelID:          modelID,
		PromptTokens:     int64(usagelog.PromptTokens),
		CompletionTokens: int64(usagelog.CompleteTokens),
	})
	return err
}

func extractTokenUsage(body []byte, provider Provider) (*usageLog, error) {
	var usagelog *usageLog = new(usageLog)
	switch provider {
	case ProviderOpenAI:
		openaiUsage, err := extractOpenAITokenUsage(body)
		if err != nil {
			return nil, err
		}
		usagelog.PromptTokens = openaiUsage.Usage.InputTokens
		usagelog.CompleteTokens = openaiUsage.Usage.OutputTokens
		return usagelog, nil
	default:
		return nil, errors.New("Not supported provider.")
	}
}

func extractOpenAITokenUsage(body []byte) (*OpenAIUsage, error) {
	var openaiUsage *OpenAIUsage = new(OpenAIUsage)
	err := json.Unmarshal(body, &openaiUsage)
	if err != nil {
		return nil, err
	}
	return openaiUsage, nil
}
