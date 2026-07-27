package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/HyperToken-dev/fabric/business/usage"
	openaiusage "github.com/HyperToken-dev/fabric/business/usage/openai"
	"github.com/HyperToken-dev/fabric/internal/storage/postgres"

	"go.uber.org/zap"
)

type openAIUsageAdapter struct {
	store *postgres.ProxyStore
}

func newOpenAIUsageAdapter(store *postgres.ProxyStore) openAIUsageAdapter {
	return openAIUsageAdapter{store: store}
}

func (a openAIUsageAdapter) WrapStreamingResponse(req *http.Request, body io.ReadCloser, contentEncoding string, info UsageContext, onComplete func([]byte)) io.ReadCloser {
	onError := func(err error) {
		zap.L().Error("openai streaming usage processing failed",
			zap.Error(err),
			zap.Int32("key_id", info.KeyID),
			zap.Int32("channel_id", info.ChannelID),
			zap.Int32("model_id", info.ModelID),
			zap.String("model", info.Model),
			zap.String("content_encoding", contentEncoding),
		)
	}
	return openaiusage.NewTrackingReaderWithFallbackAndErrors(req, body, contentEncoding, info.Model, func(parsedUsage *usage.Usage) {
		if info.ModelID == 0 {
			zap.L().Error("missing resolved model id for streaming usage",
				zap.Int32("key_id", info.KeyID),
				zap.Int32("channel_id", info.ChannelID),
				zap.String("model", info.Model),
			)
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := a.store.InsertUsage(ctx, info.KeyID, info.ChannelID, info.ModelID, parsedUsage.PromptTokens, parsedUsage.CompletionTokens); err != nil {
			zap.L().Error("insert streaming usage log failed",
				zap.Error(err),
				zap.Int32("key_id", info.KeyID),
				zap.Int32("channel_id", info.ChannelID),
				zap.Int32("model_id", info.ModelID),
				zap.String("model", info.Model),
				zap.Int64("prompt_tokens", parsedUsage.PromptTokens),
				zap.Int64("completion_tokens", parsedUsage.CompletionTokens),
			)
			return
		}
		zap.L().Info("streaming usage log inserted",
			zap.Int32("key_id", info.KeyID),
			zap.Int32("channel_id", info.ChannelID),
			zap.Int32("model_id", info.ModelID),
			zap.String("model", info.Model),
			zap.Int64("prompt_tokens", parsedUsage.PromptTokens),
			zap.Int64("completion_tokens", parsedUsage.CompletionTokens),
		)
	}, onComplete, onError)
}

func (a openAIUsageAdapter) ProcessNonStreamingResponse(ctx context.Context, req *http.Request, rawBody []byte, contentEncoding string, contentType string, info UsageContext) error {
	if info.ModelID == 0 {
		return fmt.Errorf("missing resolved model id for non-streaming usage: key_id=%d, channel_id=%d, model=%q", info.KeyID, info.ChannelID, info.Model)
	}

	parsedUsage, err := openaiusage.ExtractNonStreamingWithFallback(req, rawBody, contentEncoding, info.Model)
	if err != nil {
		return err
	}
	if err := a.store.InsertUsage(ctx, info.KeyID, info.ChannelID, info.ModelID, parsedUsage.PromptTokens, parsedUsage.CompletionTokens); err != nil {
		return err
	}
	zap.L().Info("non-streaming usage log inserted",
		zap.Int32("key_id", info.KeyID),
		zap.Int32("channel_id", info.ChannelID),
		zap.Int32("model_id", info.ModelID),
		zap.String("model", info.Model),
		zap.Int64("prompt_tokens", parsedUsage.PromptTokens),
		zap.Int64("completion_tokens", parsedUsage.CompletionTokens),
	)
	return nil
}
