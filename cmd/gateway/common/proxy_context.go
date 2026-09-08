package common

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"strconv"
)

// Provider names the upstream provider represented in audit and task metadata.
type Provider string

const (
	ProviderOpenAI   Provider = "openai"
	ProviderAlibaba  Provider = "alibaba"
	ProviderSeedance Provider = "seedance"
	ProviderGoogle   Provider = "google"
	ProviderExtrotec Provider = "extrotec"
)

// ContextKey scopes per-request proxy metadata to gateway-owned context values.
type ContextKey string

const (
	ContextKeyID        ContextKey = "key_id"
	ContextKeyChannelID ContextKey = "channel_id"
	ContextKeyModel     ContextKey = "model"
	ContextKeyModelID   ContextKey = "model_id"
	ContextKeyProvider  ContextKey = "provider"
)

// SetContextInt32 stores integer metadata on a cloned request context.
func SetContextInt32(r *http.Request, key ContextKey, val int32) *http.Request {
	ctx := context.WithValue(r.Context(), key, val)
	return r.WithContext(ctx)
}

// SetContextString stores string metadata on a cloned request context.
func SetContextString(r *http.Request, key ContextKey, val string) *http.Request {
	ctx := context.WithValue(r.Context(), key, val)
	return r.WithContext(ctx)
}

// GetContextInt32 reads gateway metadata and returns zero when absent or malformed.
func GetContextInt32(r *http.Request, key ContextKey) int32 {
	v, _ := r.Context().Value(key).(int32)
	return v
}

// GetContextString reads gateway metadata and returns an empty string when absent or malformed.
func GetContextString(r *http.Request, key ContextKey) string {
	v, _ := r.Context().Value(key).(string)
	return v
}

// ErrModelDisabled reports that a configured model exists but cannot be used.
var ErrModelDisabled = errors.New("model disabled")

// ErrModelUnsupported reports that the channel has no active mapping for the requested model.
var ErrModelUnsupported = errors.New("model unsupported")

// ErrMissingModel reports provider requests that omit their model selector.
var ErrMissingModel = errors.New("missing model")

// ErrInvalidRequestBody reports requests that cannot be parsed according to provider format.
var ErrInvalidRequestBody = errors.New("invalid request body")

// ErrPromptRejected reports input text rejected by the sensitive-word policy.
var ErrPromptRejected = errors.New("prompt rejected")

// ResolveModelFromStore returns the gateway model id only when the channel model is active.
func ResolveModelFromStore(ctx context.Context, store ModelStore, channelID int32, modelName string) (int32, error) {
	model, err := store.ResolveModel(ctx, channelID, modelName)
	if err != nil {
		return 0, err
	}
	if model == nil {
		return 0, ErrModelUnsupported
	}
	switch model.Status {
	case ModelStatusActive:
		return model.ID, nil
	case ModelStatusBanned:
		return 0, ErrModelDisabled
	default:
		return 0, ErrModelUnsupported
	}
}

// RestoreRequestBody makes a consumed request body readable again for upstream forwarding and audits.
func RestoreRequestBody(req *http.Request, body []byte) {
	req.Body = io.NopCloser(bytes.NewReader(body))
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(body)), nil
	}
	req.ContentLength = int64(len(body))
	req.Header.Set("Content-Length", strconv.Itoa(len(body)))
}

// BodyPrefix returns a bounded payload preview for logs.
func BodyPrefix(body []byte, maxLen int) string {
	if len(body) > maxLen {
		body = body[:maxLen]
	}
	return string(body)
}
