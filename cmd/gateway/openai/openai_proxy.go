package openai

import (
	"bytes"
	"context"
	"github.com/HyperToken-dev/fabric/cmd/gateway/common"
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

// OpenAIProxy validates and audits OpenAI-compatible proxy requests.
type OpenAIProxy struct {
	coreProxy    *coreproxy.Proxy
	modelStore   common.ModelStore
	usageHandler common.UsageHandler
	integralLogs common.IntegralLogHandler
	textPolicy   common.TextPolicy
}

// OpenAIProxyOptions wires optional stores, usage, audit, and text policy dependencies.
type OpenAIProxyOptions struct {
	ModelStore         common.ModelStore
	UsageHandler       common.UsageHandler
	IntegralLogHandler common.IntegralLogHandler
	TextPolicy         common.TextPolicy
}

// NewOpenAIProxy creates an OpenAI-compatible proxy with noop dependencies for nil options.
func NewOpenAIProxy(opts OpenAIProxyOptions) (*OpenAIProxy, error) {
	if opts.ModelStore == nil {
		opts.ModelStore = common.NoopModelStore{}
	}
	if opts.UsageHandler == nil {
		opts.UsageHandler = common.NoopUsageHandler{}
	}
	if opts.IntegralLogHandler == nil {
		opts.IntegralLogHandler = common.NoopIntegralLogHandler{}
	}
	if opts.TextPolicy == nil {
		opts.TextPolicy = common.NoopTextPolicy{}
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
	keyID := common.GetContextInt32(resp.Request, common.ContextKeyID)
	channelID := common.GetContextInt32(resp.Request, common.ContextKeyChannelID)
	model := common.GetContextString(resp.Request, common.ContextKeyModel)
	modelID := common.GetContextInt32(resp.Request, common.ContextKeyModelID)
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

	info := common.IntegralLogInfo{
		Provider:                common.ProviderOpenAI,
		APIFormat:               1,
		KeyID:                   keyID,
		ChannelID:               channelID,
		ModelID:                 modelID,
		Model:                   model,
		Outcome:                 common.ResponseOutcome(resp.StatusCode),
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
			zap.L().Error("extract openai output texts failed", zap.Error(err), zap.String("content_type", contentType), zap.String("decoded_body_prefix", common.BodyPrefix(decodedBody, 128)))
		} else if common.DetectPrompts(resp.Request.Context(), model, common.TextDirectionOutput, outputTexts, p.textPolicy) {
			integralResponseBody = rejectOpenAIOutputResponse(resp)
			info.Outcome = common.IntegralOutcomeRejected
			info.RejectionStage = common.RejectionStageOutput
			info.RejectionReason = common.RejectionReasonSensitive
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
	go common.ProcessIntegralLogAsync(p.integralLogs, resp.Request, info, integralResponseBody)
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

	if err := p.usageHandler.ProcessNonStreamingResponse(ctx, req, rawBody, contentEncoding, contentType, common.UsageContext{
		KeyID:     keyID,
		ChannelID: channelID,
		ModelID:   modelID,
		Model:     model,
	}); err != nil {
		zap.L().Error("process token usage failed", zap.Error(err), zap.String("content_type", contentType), zap.String("content_encoding", contentEncoding), zap.String("raw_body_prefix", common.BodyPrefix(rawBody, 128)), zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.Int32("model_id", modelID), zap.String("model", model))
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

	if err := p.usageHandler.ProcessStreamingResponse(ctx, req, decodedBody, common.UsageContext{
		KeyID:     keyID,
		ChannelID: channelID,
		ModelID:   modelID,
		Model:     model,
	}); err != nil {
		zap.L().Error("process streaming usage failed", zap.Error(err), zap.String("raw_body_prefix", common.BodyPrefix(decodedBody, 128)), zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.Int32("model_id", modelID), zap.String("model", model))
		return
	}
	zap.L().Info("streaming usage processed", zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.Int32("model_id", modelID), zap.String("model", model))
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
	if common.DetectPrompts(r.Context(), modelName, common.TextDirectionInput, parsedReq.Prompts, p.textPolicy) {
		zap.L().Warn("openai prompt rejected", zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.String("model", modelName))
		responseBody := []byte("prompt rejected\n")
		http.Error(w, "prompt rejected", http.StatusForbidden)
		go common.ProcessIntegralLogAsync(p.integralLogs, r, common.IntegralLogInfo{
			Provider:            common.ProviderOpenAI,
			APIFormat:           1,
			KeyID:               keyID,
			ChannelID:           channelID,
			Model:               modelName,
			Outcome:             common.IntegralOutcomeRejected,
			RejectionStage:      common.RejectionStageInput,
			RejectionReason:     common.RejectionReasonSensitive,
			ResponseStatus:      http.StatusForbidden,
			ResponseContentType: "text/plain; charset=utf-8",
			DecodeOK:            true,
		}, responseBody)
		return
	}
	modelID, err := common.ResolveModelFromStore(r.Context(), p.modelStore, channelID, modelName)
	if err != nil {
		if err == common.ErrModelDisabled {
			zap.L().Warn("openai model disabled", zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.String("model", modelName))
			http.Error(w, "model disabled", http.StatusForbidden)
			return
		}
		if err == common.ErrModelUnsupported {
			zap.L().Warn("openai model unsupported", zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.String("model", modelName))
			http.Error(w, "unsupported model", http.StatusBadRequest)
			return
		}
		zap.L().Error("resolve model failed", zap.Error(err), zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.String("model", modelName))
		http.Error(w, "model lookup failed", http.StatusInternalServerError)
		return
	}
	zap.L().Info("openai model resolved", zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.String("model", modelName), zap.Int32("model_id", modelID))

	r = common.SetContextInt32(r, common.ContextKeyID, keyID)
	r = common.SetContextInt32(r, common.ContextKeyChannelID, channelID)
	r = common.SetContextString(r, common.ContextKeyModel, modelName)
	r = common.SetContextInt32(r, common.ContextKeyModelID, modelID)
	p.coreProxy.ServeHTTP(w, r, coreproxy.Upstream{BaseURL: baseURL, APIKey: providerKey})
}
