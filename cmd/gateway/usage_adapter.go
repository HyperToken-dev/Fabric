package main

import (
	"context"
	"fmt"
	"io"
	"time"

	"fabric/business/usage"
	openaiusage "fabric/business/usage/openai"
	"fabric/internal/proxy"
	"fabric/internal/storage/postgres"

	"go.uber.org/zap"
)

type openAIUsageAdapter struct {
	store *postgres.ProxyStore
}

func newOpenAIUsageAdapter(store *postgres.ProxyStore) openAIUsageAdapter {
	return openAIUsageAdapter{store: store}
}

func (a openAIUsageAdapter) WrapStreamingResponse(body io.ReadCloser, contentEncoding string, info proxy.UsageContext) io.ReadCloser {
	return openaiusage.NewTrackingReader(body, contentEncoding, func(parsedUsage *usage.Usage) {
		if info.ModelID == 0 {
			zap.S().Errorf("Error catched: missing resolved model id for responses stream usage: key_id=%d, channel_id=%d, model=%q", info.KeyID, info.ChannelID, info.Model)
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := a.store.InsertUsage(ctx, info.KeyID, info.ChannelID, info.ModelID, parsedUsage.PromptTokens, parsedUsage.CompletionTokens); err != nil {
			zap.S().Errorf("Error catched: insert responses stream usage log error: %v", err)
		}
	})
}

func (a openAIUsageAdapter) ProcessNonStreamingResponse(ctx context.Context, rawBody []byte, contentEncoding string, contentType string, info proxy.UsageContext) error {
	if info.ModelID == 0 {
		return fmt.Errorf("missing resolved model id for non-streaming usage: key_id=%d, channel_id=%d, model=%q", info.KeyID, info.ChannelID, info.Model)
	}

	parsedUsage, err := openaiusage.ExtractNonStreaming(rawBody, contentEncoding)
	if err != nil {
		return err
	}
	return a.store.InsertUsage(ctx, info.KeyID, info.ChannelID, info.ModelID, parsedUsage.PromptTokens, parsedUsage.CompletionTokens)
}
