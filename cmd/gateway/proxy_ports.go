package main

import (
	"context"
	"net/http"
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
	ProcessNonStreamingResponse(ctx context.Context, req *http.Request, rawBody []byte, contentEncoding string, contentType string, info UsageContext) error
	ProcessStreamingResponse(ctx context.Context, req *http.Request, decodedBody []byte, info UsageContext) error
}

type IntegralLogHandler interface {
	InsertIntegralLog(ctx context.Context, keyID int32, context string, response string) error
}

type NoopModelStore struct{}

func (NoopModelStore) ResolveModel(ctx context.Context, channelID int32, modelName string) (*ModelInfo, error) {
	return &ModelInfo{Status: ModelStatusActive}, nil
}

type NoopUsageHandler struct{}

func (NoopUsageHandler) ProcessNonStreamingResponse(ctx context.Context, req *http.Request, rawBody []byte, contentEncoding string, contentType string, info UsageContext) error {
	return nil
}

func (NoopUsageHandler) ProcessStreamingResponse(ctx context.Context, req *http.Request, decodedBody []byte, info UsageContext) error {
	return nil
}

type NoopIntegralLogHandler struct{}

func (NoopIntegralLogHandler) InsertIntegralLog(ctx context.Context, keyID int32, context string, response string) error {
	return nil
}
