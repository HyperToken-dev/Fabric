package main

import (
	"context"
	"errors"
	"net/http"
)

type Provider string

const (
	ProviderOpenAI Provider = "openai"
)

type contextKey string

const (
	ctxKeyID     contextKey = "key_id"
	ctxChannelID contextKey = "channel_id"
	ctxModel     contextKey = "model"
	ctxModelID   contextKey = "model_id"
	ctxProvider  contextKey = "provider"
	ctxStreamKey contextKey = "stream"
)

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
var errModelUnsupported = errors.New("model unsupported")

func (p *OpenAIProxy) resolveModel(ctx context.Context, channelID int32, modelName string) (int32, error) {
	model, err := p.modelStore.ResolveModel(ctx, channelID, modelName)
	if err != nil {
		return 0, err
	}
	if model == nil {
		return 0, errModelUnsupported
	}
	switch model.Status {
	case ModelStatusActive:
		return model.ID, nil
	case ModelStatusBanned:
		return 0, errModelDisabled
	default:
		return 0, errModelUnsupported
	}
}
