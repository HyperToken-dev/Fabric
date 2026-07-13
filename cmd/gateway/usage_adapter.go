package main

import (
	"context"
	"io"

	"fabric/business/usage"
	openaiusage "fabric/business/usage/openai"
	"fabric/internal/proxy"
)

type openAIUsageAdapter struct {
	handler *openaiusage.Handler
}

func newOpenAIUsageAdapter(handler *openaiusage.Handler) openAIUsageAdapter {
	return openAIUsageAdapter{handler: handler}
}

func (a openAIUsageAdapter) WrapStreamingResponse(body io.ReadCloser, contentEncoding string, info proxy.UsageContext) io.ReadCloser {
	return a.handler.WrapStreamingResponse(body, contentEncoding, usage.Context{
		KeyID:     info.KeyID,
		ChannelID: info.ChannelID,
		ModelID:   info.ModelID,
		Model:     info.Model,
	})
}

func (a openAIUsageAdapter) ProcessNonStreamingResponse(ctx context.Context, rawBody []byte, contentEncoding string, contentType string, info proxy.UsageContext) error {
	return a.handler.ProcessNonStreamingResponse(ctx, rawBody, contentEncoding, contentType, usage.Context{
		KeyID:     info.KeyID,
		ChannelID: info.ChannelID,
		ModelID:   info.ModelID,
		Model:     info.Model,
	})
}
