package main

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	sensitiveopenai "github.com/HyperToken-dev/fabric/business/sensitive/openai"
	coreopenai "github.com/HyperToken-dev/fabric/core/providers/openai"
	coreproxy "github.com/HyperToken-dev/fabric/core/proxy"

	"go.uber.org/zap"
)

type OpenAIProxy struct {
	coreProxy    *coreproxy.Proxy
	modelStore   ModelStore
	usageHandler UsageHandler
	integralLogs IntegralLogHandler
	textPolicy   TextPolicy
}

type OpenAIProxyOptions struct {
	ModelStore         ModelStore
	UsageHandler       UsageHandler
	IntegralLogHandler IntegralLogHandler
	TextPolicy         TextPolicy
}

func NewOpenAIProxy(opts OpenAIProxyOptions) (*OpenAIProxy, error) {
	if opts.ModelStore == nil {
		opts.ModelStore = NoopModelStore{}
	}
	if opts.UsageHandler == nil {
		opts.UsageHandler = NoopUsageHandler{}
	}
	if opts.IntegralLogHandler == nil {
		opts.IntegralLogHandler = NoopIntegralLogHandler{}
	}
	if opts.TextPolicy == nil {
		opts.TextPolicy = NoopTextPolicy{}
	}
	p := &OpenAIProxy{
		modelStore:   opts.ModelStore,
		usageHandler: opts.UsageHandler,
		integralLogs: opts.IntegralLogHandler,
		textPolicy:   opts.TextPolicy,
	}
	coreProxy := coreopenai.New(coreproxy.Options{
		OnComplete:      p.onComplete,
		StreamTransform: p.openAIStreamTransform,
	})
	p.coreProxy = coreProxy
	return p, nil
}

func (p *OpenAIProxy) onComplete(resp *http.Response, decodedBody []byte) {
	keyID := getContextInt32(resp.Request, ctxKeyID)
	channelID := getContextInt32(resp.Request, ctxChannelID)
	model := getContextString(resp.Request, ctxModel)
	modelID := getContextInt32(resp.Request, ctxModelID)
	contentType := resp.Header.Get("Content-Type")
	contentEncoding := resp.Header.Get("Content-Encoding")
	zap.L().Info("openai upstream response received",
		zap.Int("status_code", resp.StatusCode),
		zap.String("content_type", contentType),
		zap.String("content_encoding", contentEncoding),
		zap.Int32("key_id", keyID),
		zap.Int32("channel_id", channelID),
		zap.Int32("model_id", modelID),
		zap.String("model", model),
		zap.Int("decoded_body_bytes", len(decodedBody)),
	)

	info := integralLogInfo{
		Provider:                ProviderOpenAI,
		APIFormat:               1,
		KeyID:                   keyID,
		ChannelID:               channelID,
		ModelID:                 modelID,
		Model:                   model,
		Outcome:                 responseOutcome(resp.StatusCode),
		ResponseStatus:          resp.StatusCode,
		ResponseContentType:     contentType,
		ResponseContentEncoding: contentEncoding,
		DecodeOK:                decodedBody != nil,
	}
	integralResponseBody := decodedBody
	if decodedBody == nil {
		integralResponseBody = nil
	}
	upstreamSuccess := resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices
	isStream := isOpenAIUsageStream(resp.Request, contentType)

	if decodedBody != nil && upstreamSuccess && !isStream {
		outputTexts, err := sensitiveopenai.ExtractOutputTexts(resp.Request, decodedBody)
		if err != nil {
			zap.L().Error("extract openai output texts failed", zap.Error(err), zap.String("content_type", contentType), zap.String("decoded_body_prefix", bodyPrefix(decodedBody, 128)))
		} else if detectPrompts(resp.Request.Context(), model, TextDirectionOutput, outputTexts, p.textPolicy) {
			integralResponseBody = rejectOpenAIOutputResponse(resp)
			info.Outcome = integralOutcomeRejected
			info.RejectionStage = rejectionStageOutput
			info.RejectionReason = rejectionReasonSensitive
			info.ResponseStatus = resp.StatusCode
			info.ResponseContentType = resp.Header.Get("Content-Type")
			info.ResponseContentEncoding = resp.Header.Get("Content-Encoding")
		}
	}

	if decodedBody != nil && upstreamSuccess {
		if isStream {
			go p.processStreamingUsageAsync(resp.Request, decodedBody, keyID, channelID, modelID, model)
		} else {
			go p.processUsageAsync(resp.Request, decodedBody, "identity", contentType, keyID, channelID, modelID, model)
		}
	}
	go processIntegralLogAsync(p.integralLogs, resp.Request, info, integralResponseBody)
}

func responseOutcome(statusCode int) string {
	if statusCode >= http.StatusOK && statusCode < http.StatusMultipleChoices {
		return integralOutcomeOK
	}
	return integralOutcomeError
}

func rejectOpenAIOutputResponse(resp *http.Response) []byte {
	errorBody := []byte(`{"error":"model output rejected, please change your prompt"}`)
	resp.StatusCode = http.StatusUnprocessableEntity
	resp.Status = strconv.Itoa(http.StatusUnprocessableEntity) + " " + http.StatusText(http.StatusUnprocessableEntity)
	resp.Header.Set("Content-Type", "application/json")
	resp.Header.Del("Content-Encoding")
	resp.Header.Set("Content-Length", strconv.Itoa(len(errorBody)))
	resp.Body = io.NopCloser(bytes.NewReader(errorBody))
	resp.ContentLength = int64(len(errorBody))
	return errorBody
}

// determine whether the data is SSE stram
func isOpenAIUsageStream(req *http.Request, contentType string) bool {
	isOpenAIUsagePath := strings.Contains(req.URL.Path, "/v1/responses") || strings.Contains(req.URL.Path, "/v1/chat/completions")
	return isOpenAIUsagePath && strings.Contains(strings.ToLower(contentType), "text/event-stream")
}

// async usage processing
func (p *OpenAIProxy) processUsageAsync(req *http.Request, rawBody []byte, contentEncoding string, contentType string, keyID int32, channelID int32, modelID int32, model string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if modelID == 0 {
		zap.L().Error("missing resolved model id for non-streaming usage", zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.String("model", model))
		return
	}

	if err := p.usageHandler.ProcessNonStreamingResponse(ctx, req, rawBody, contentEncoding, contentType, UsageContext{
		KeyID:     keyID,
		ChannelID: channelID,
		ModelID:   modelID,
		Model:     model,
	}); err != nil {
		zap.L().Error("process token usage failed", zap.Error(err), zap.String("content_type", contentType), zap.String("content_encoding", contentEncoding), zap.String("raw_body_prefix", bodyPrefix(rawBody, 128)), zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.Int32("model_id", modelID), zap.String("model", model))
		return
	}
	zap.L().Info("non-streaming usage processed", zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.Int32("model_id", modelID), zap.String("model", model))
}

func (p *OpenAIProxy) processStreamingUsageAsync(req *http.Request, decodedBody []byte, keyID int32, channelID int32, modelID int32, model string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if modelID == 0 {
		zap.L().Error("missing resolved model id for streaming usage", zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.String("model", model))
		return
	}

	if err := p.usageHandler.ProcessStreamingResponse(ctx, req, decodedBody, UsageContext{
		KeyID:     keyID,
		ChannelID: channelID,
		ModelID:   modelID,
		Model:     model,
	}); err != nil {
		zap.L().Error("process streaming usage failed", zap.Error(err), zap.String("raw_body_prefix", bodyPrefix(decodedBody, 128)), zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.Int32("model_id", modelID), zap.String("model", model))
		return
	}
	zap.L().Info("streaming usage processed", zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.Int32("model_id", modelID), zap.String("model", model))
}

func bodyPrefix(body []byte, maxLen int) string {
	if len(body) > maxLen {
		body = body[:maxLen]
	}
	return string(body)
}

func (p *OpenAIProxy) ServeHTTP(w http.ResponseWriter, r *http.Request, keyID int32, channelID int32, baseURL string, providerKey string) {
	if strings.TrimSpace(providerKey) == "" {
		zap.L().Error("missing provider key", zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.String("method", r.Method), zap.String("path", r.URL.Path))
		http.Error(w, "missing provider key", http.StatusBadGateway)
		return
	}

	// parse request and apply text policy
	parsedReq, err := sensitiveopenai.ExtractPromptRequest(r)
	if err != nil {
		zap.L().Error("parse openai prompt request failed", zap.Error(err), zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.String("method", r.Method), zap.String("path", r.URL.Path))
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	modelName := strings.TrimSpace(parsedReq.Model)
	if modelName == "" {
		zap.L().Warn("missing openai model", zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.String("method", r.Method), zap.String("path", r.URL.Path))
		http.Error(w, "missing model", http.StatusBadRequest)
		return
	}
	zap.L().Info("openai proxy request received",
		zap.String("method", r.Method),
		zap.String("path", r.URL.Path),
		zap.Int32("key_id", keyID),
		zap.Int32("channel_id", channelID),
		zap.String("model", modelName),
		zap.Bool("stream", parsedReq.Stream),
	)
	if detectPrompts(r.Context(), modelName, TextDirectionInput, parsedReq.Prompts, p.textPolicy) {
		zap.L().Warn("openai prompt rejected", zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.String("model", modelName))
		responseBody := []byte("prompt rejected\n")
		http.Error(w, "prompt rejected", http.StatusForbidden)
		go processIntegralLogAsync(p.integralLogs, r, integralLogInfo{
			Provider:            ProviderOpenAI,
			APIFormat:           1,
			KeyID:               keyID,
			ChannelID:           channelID,
			Model:               modelName,
			Outcome:             integralOutcomeRejected,
			RejectionStage:      rejectionStageInput,
			RejectionReason:     rejectionReasonSensitive,
			ResponseStatus:      http.StatusForbidden,
			ResponseContentType: "text/plain; charset=utf-8",
			DecodeOK:            true,
		}, responseBody)
		return
	}
	modelID, err := p.resolveModel(r.Context(), channelID, modelName)
	if err != nil {
		if err == errModelDisabled {
			zap.L().Warn("openai model disabled", zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.String("model", modelName))
			http.Error(w, "model disabled", http.StatusForbidden)
			return
		}
		if err == errModelUnsupported {
			zap.L().Warn("openai model unsupported", zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.String("model", modelName))
			http.Error(w, "unsupported model", http.StatusBadRequest)
			return
		}
		zap.L().Error("resolve model failed", zap.Error(err), zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.String("model", modelName))
		http.Error(w, "model lookup failed", http.StatusInternalServerError)
		return
	}
	zap.L().Info("openai model resolved", zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.String("model", modelName), zap.Int32("model_id", modelID))

	r = setContextInt32(r, ctxKeyID, keyID)
	r = setContextInt32(r, ctxChannelID, channelID)
	r = setContextString(r, ctxModel, modelName)
	r = setContextInt32(r, ctxModelID, modelID)
	p.coreProxy.ServeHTTP(w, r, coreproxy.Upstream{BaseURL: baseURL, APIKey: providerKey})
}
