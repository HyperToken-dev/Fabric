package common

import (
	"context"
	"net/http"
)

// ModelStatus is the gateway-facing status used by provider packages during model checks.
type ModelStatus int

const (
	ModelStatusUnknown ModelStatus = iota
	ModelStatusActive
	ModelStatusBanned
)

// ModelInfo is the minimal model metadata needed by proxy request validation.
type ModelInfo struct {
	ID     int32
	Status ModelStatus
}

// ModelStore resolves provider request model names within a gateway channel.
type ModelStore interface {
	ResolveModel(ctx context.Context, channelID int32, modelName string) (*ModelInfo, error)
}

// UsageContext carries resolved gateway identities for asynchronous usage writes.
type UsageContext struct {
	KeyID     int32
	ChannelID int32
	ModelID   int32
	Model     string
}

// UsageHandler records OpenAI-compatible token usage extracted from upstream responses.
type UsageHandler interface {
	ProcessNonStreamingResponse(ctx context.Context, req *http.Request, rawBody []byte, contentEncoding string, contentType string, info UsageContext) error
	ProcessStreamingResponse(ctx context.Context, req *http.Request, decodedBody []byte, info UsageContext) error
}

// GoogleUsageHandler records Google interaction usage extracted from upstream responses.
type GoogleUsageHandler interface {
	ProcessInteractionResponse(ctx context.Context, rawBody []byte, info UsageContext) error
	ProcessInteractionStreamingResponse(ctx context.Context, decodedBody []byte, info UsageContext) error
}

// IntegralLogHandler writes provider request and response audit data.
type IntegralLogHandler interface {
	InsertIntegralLog(ctx context.Context, keyID int32, context string, response string) error
}

// NoopModelStore allows provider tests to skip persistent model resolution.
type NoopModelStore struct{}

func (NoopModelStore) ResolveModel(ctx context.Context, channelID int32, modelName string) (*ModelInfo, error) {
	return &ModelInfo{Status: ModelStatusActive}, nil
}

// NoopUsageHandler disables usage side effects when a provider is tested in isolation.
type NoopUsageHandler struct{}

func (NoopUsageHandler) ProcessNonStreamingResponse(ctx context.Context, req *http.Request, rawBody []byte, contentEncoding string, contentType string, info UsageContext) error {
	return nil
}

func (NoopUsageHandler) ProcessStreamingResponse(ctx context.Context, req *http.Request, decodedBody []byte, info UsageContext) error {
	return nil
}

// NoopGoogleUsageHandler disables Google usage side effects for isolated proxy tests.
type NoopGoogleUsageHandler struct{}

func (NoopGoogleUsageHandler) ProcessInteractionResponse(ctx context.Context, rawBody []byte, info UsageContext) error {
	return nil
}

func (NoopGoogleUsageHandler) ProcessInteractionStreamingResponse(ctx context.Context, decodedBody []byte, info UsageContext) error {
	return nil
}

// NoopIntegralLogHandler disables audit side effects when none are configured.
type NoopIntegralLogHandler struct{}

func (NoopIntegralLogHandler) InsertIntegralLog(ctx context.Context, keyID int32, context string, response string) error {
	return nil
}
