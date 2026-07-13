package proxy

import (
	"context"
	"io"
)

type ModelStatus int

const (
	ModelStatusUnknown ModelStatus = iota
	ModelStatusActive
	ModelStatusBanned
)

type ModelInfo struct {
	ID     int32
	Status ModelStatus
}

type ModelStore interface {
	ResolveModel(ctx context.Context, channelID int32, modelName string) (*ModelInfo, error)
}

type UsageContext struct {
	KeyID     int32
	ChannelID int32
	ModelID   int32
	Model     string
}

type UsageHandler interface {
	WrapStreamingResponse(body io.ReadCloser, contentEncoding string, info UsageContext) io.ReadCloser
	ProcessNonStreamingResponse(ctx context.Context, rawBody []byte, contentEncoding string, contentType string, info UsageContext) error
}

type NoopModelStore struct{}

func (NoopModelStore) ResolveModel(ctx context.Context, channelID int32, modelName string) (*ModelInfo, error) {
	return &ModelInfo{Status: ModelStatusActive}, nil
}

type NoopUsageHandler struct{}

func (NoopUsageHandler) WrapStreamingResponse(body io.ReadCloser, contentEncoding string, info UsageContext) io.ReadCloser {
	return body
}

func (NoopUsageHandler) ProcessNonStreamingResponse(ctx context.Context, rawBody []byte, contentEncoding string, contentType string, info UsageContext) error {
	return nil
}
