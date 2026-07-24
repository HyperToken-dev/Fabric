package main

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
	WrapStreamingResponse(body io.ReadCloser, contentEncoding string, info UsageContext, onComplete func([]byte)) io.ReadCloser
	ProcessNonStreamingResponse(ctx context.Context, rawBody []byte, contentEncoding string, contentType string, info UsageContext) error
}

type IntegralLogHandler interface {
	InsertIntegralLog(ctx context.Context, keyID int32, context string, response string) error
}

type NoopModelStore struct{}

func (NoopModelStore) ResolveModel(ctx context.Context, channelID int32, modelName string) (*ModelInfo, error) {
	return &ModelInfo{Status: ModelStatusActive}, nil
}

type NoopUsageHandler struct{}

func (NoopUsageHandler) WrapStreamingResponse(body io.ReadCloser, contentEncoding string, info UsageContext, onComplete func([]byte)) io.ReadCloser {
	return body
}

func (NoopUsageHandler) ProcessNonStreamingResponse(ctx context.Context, rawBody []byte, contentEncoding string, contentType string, info UsageContext) error {
	return nil
}

type NoopIntegralLogHandler struct{}

func (NoopIntegralLogHandler) InsertIntegralLog(ctx context.Context, keyID int32, context string, response string) error {
	return nil
}
