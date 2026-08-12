package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
)

type Provider string

const (
	ProviderOpenAI   Provider = "openai"
	ProviderAlibaba  Provider = "alibaba"
	ProviderSeedance Provider = "seedance"
	ProviderGoogle   Provider = "google"
	ProviderExtrotec Provider = "extrotec"
)

type contextKey string

const (
	ctxKeyID     contextKey = "key_id"
	ctxChannelID contextKey = "channel_id"
	ctxModel     contextKey = "model"
	ctxModelID   contextKey = "model_id"
	ctxProvider  contextKey = "provider"
)

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

var errModelDisabled = errors.New("model disabled")
var errModelUnsupported = errors.New("model unsupported")
var errMissingModel = errors.New("missing model")
var errInvalidRequestBody = errors.New("invalid request body")

func (p *OpenAIProxy) resolveModel(ctx context.Context, channelID int32, modelName string) (int32, error) {
	return resolveModelFromStore(ctx, p.modelStore, channelID, modelName)
}

func resolveModelFromStore(ctx context.Context, store ModelStore, channelID int32, modelName string) (int32, error) {
	model, err := store.ResolveModel(ctx, channelID, modelName)
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

func restoreRequestBody(req *http.Request, body []byte) {
	req.Body = io.NopCloser(bytes.NewReader(body))
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(body)), nil
	}
	req.ContentLength = int64(len(body))
	req.Header.Set("Content-Length", strconv.Itoa(len(body)))
}
