package proxy

import (
	"context"
	"database/sql"
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
	ctxStreamKey contextKey = "stream"
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

func setContextBool(r *http.Request, key contextKey, val bool) *http.Request {
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

func getContextBool(r *http.Request, key contextKey) bool {
	v, _ := r.Context().Value(key).(bool)
	return v
}

var errModelDisabled = errors.New("model disabled")

func (p *OpenAIProxy) resolveModel(ctx context.Context, channelID int32, modelName string) (int32, error) {
	model, err := p.queries.GetModelByChannelAndName(ctx, repository.GetModelByChannelAndNameParams{
		ChannelID: channelID,
		ModelName: modelName,
	})
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	switch model.Status {
	case modelStatusActive:
		return model.ID, nil
	case modelStatusBanned:
		return 0, errModelDisabled
	default:
		return 0, nil
	}
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
	switch provider {
	case ProviderOpenAI:
		return extractOpenAITokenUsage(body)
	default:
		return nil, errors.New("Not supported provider.")
	}
}

func extractOpenAITokenUsage(body []byte) (*usageLog, error) {
	var resp struct {
		Usage struct {
			InputTokens      int `json:"input_tokens"`
			OutputTokens     int `json:"output_tokens"`
			PromptTokens     int `json:"prompt_tokens"`
			CompletionTokens int `json:"completion_tokens"`
		} `json:"usage"`
	}
	err := json.Unmarshal(body, &resp)
	if err != nil {
		return nil, err
	}
	switch {
	case resp.Usage.InputTokens != 0 || resp.Usage.OutputTokens != 0:
		return &usageLog{
			PromptTokens:   resp.Usage.InputTokens,
			CompleteTokens: resp.Usage.OutputTokens,
		}, nil
	case resp.Usage.PromptTokens != 0 || resp.Usage.CompletionTokens != 0:
		return &usageLog{
			PromptTokens:   resp.Usage.PromptTokens,
			CompleteTokens: resp.Usage.CompletionTokens,
		}, nil
	default:
		return nil, errors.New("missing openai usage")
	}
}
