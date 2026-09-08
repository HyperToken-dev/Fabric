package google

import (
	"context"
	"errors"
	"github.com/HyperToken-dev/fabric/cmd/gateway/common"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	sensitivegoogle "github.com/HyperToken-dev/fabric/business/sensitive/google"
	coregoogle "github.com/HyperToken-dev/fabric/core/providers/google"
	coreproxy "github.com/HyperToken-dev/fabric/core/proxy"
	"github.com/HyperToken-dev/fabric/internal/models"

	"go.uber.org/zap"
)

const (
	googleInteractionsPath       = "/interactions"
	googleV1BetaInteractionsPath = "/v1beta/interactions"
)

// GoogleProxy validates, audits, and records usage for Google interaction requests.
type GoogleProxy struct {
	coreProxy    *coreproxy.Proxy
	modelStore   common.ModelStore
	usageHandler common.GoogleUsageHandler
	integralLogs common.IntegralLogHandler
	textPolicy   common.TextPolicy
}

// GoogleProxyOptions wires model, usage, audit, and text policy dependencies.
type GoogleProxyOptions struct {
	ModelStore         common.ModelStore
	UsageHandler       common.GoogleUsageHandler
	IntegralLogHandler common.IntegralLogHandler
	TextPolicy         common.TextPolicy
}

// NewGoogleProxy creates a Google interaction proxy with noop dependencies for nil options.
func NewGoogleProxy(opts GoogleProxyOptions) (*GoogleProxy, error) {
	if opts.ModelStore == nil {
		opts.ModelStore = common.NoopModelStore{}
	}
	if opts.UsageHandler == nil {
		opts.UsageHandler = common.NoopGoogleUsageHandler{}
	}
	if opts.IntegralLogHandler == nil {
		opts.IntegralLogHandler = common.NoopIntegralLogHandler{}
	}
	if opts.TextPolicy == nil {
		opts.TextPolicy = common.NoopTextPolicy{}
	}
	p := &GoogleProxy{modelStore: opts.ModelStore, usageHandler: opts.UsageHandler, integralLogs: opts.IntegralLogHandler, textPolicy: opts.TextPolicy}
	p.coreProxy = coregoogle.New(coreproxy.Options{OnComplete: p.onComplete})
	return p, nil
}

func (p *GoogleProxy) onComplete(resp *http.Response, decodedBody []byte) {
	keyID := common.GetContextInt32(resp.Request, common.ContextKeyID)
	channelID := common.GetContextInt32(resp.Request, common.ContextKeyChannelID)
	model := common.GetContextString(resp.Request, common.ContextKeyModel)
	modelID := common.GetContextInt32(resp.Request, common.ContextKeyModelID)
	contentType := resp.Header.Get("Content-Type")
	contentEncoding := resp.Header.Get("Content-Encoding")
	info := common.IntegralLogInfo{
		Provider:                common.ProviderGoogle,
		APIFormat:               models.APIFormatGoogle,
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
	if decodedBody != nil && resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
		if strings.Contains(strings.ToLower(contentType), "text/event-stream") {
			go p.processStreamingUsageAsync(decodedBody, keyID, channelID, modelID, model)
		} else {
			go p.processUsageAsync(decodedBody, keyID, channelID, modelID, model)
		}
	}
	go common.ProcessIntegralLogAsync(p.integralLogs, resp.Request, info, decodedBody)
}

func (p *GoogleProxy) ServeHTTP(w http.ResponseWriter, r *http.Request, keyID int32, channelID int32, baseURL string, providerKey string) {
	if strings.TrimSpace(providerKey) == "" {
		zap.L().Error("missing provider key", zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.String("provider", string(common.ProviderGoogle)), zap.String("method", r.Method), zap.String("path", r.URL.Path))
		http.Error(w, "missing provider key", http.StatusBadGateway)
		return
	}

	if !isGoogleInteractionsRequest(r, baseURL) {
		zap.L().Warn("unsupported google proxy request", zap.String("method", r.Method), zap.String("path", r.URL.Path), zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID))
		http.Error(w, "unsupported google path", http.StatusNotFound)
		return
	}

	modelID, modelName, err := p.prepareInteractionsRequest(r, channelID)
	if err != nil {
		if errors.Is(err, common.ErrPromptRejected) {
			p.writePromptRejected(w, r, keyID, channelID, modelID, modelName)
			return
		}
		p.writeModelError(w, r, keyID, channelID, modelID, modelName, err)
		return
	}
	zap.L().Info("google interactions request received", zap.String("method", r.Method), zap.String("path", r.URL.Path), zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.String("model", modelName))

	r = common.SetContextInt32(r, common.ContextKeyID, keyID)
	r = common.SetContextInt32(r, common.ContextKeyChannelID, channelID)
	r = common.SetContextString(r, common.ContextKeyModel, modelName)
	r = common.SetContextInt32(r, common.ContextKeyModelID, modelID)
	p.coreProxy.ServeHTTP(w, r, coreproxy.Upstream{BaseURL: baseURL, APIKey: providerKey})
}

func (p *GoogleProxy) prepareInteractionsRequest(r *http.Request, channelID int32) (int32, string, error) {
	parsedReq, err := sensitivegoogle.ExtractPromptRequest(r)
	if err != nil {
		return 0, "", common.ErrInvalidRequestBody
	}
	modelName := strings.TrimSpace(parsedReq.Model)
	if modelName == "" {
		return 0, "", common.ErrMissingModel
	}
	modelID, err := common.ResolveModelFromStore(r.Context(), p.modelStore, channelID, modelName)
	if err != nil {
		return 0, modelName, err
	}
	if common.DetectPrompts(r.Context(), modelName, common.TextDirectionInput, parsedReq.Prompts, p.textPolicy) {
		return modelID, modelName, common.ErrPromptRejected
	}
	return modelID, modelName, nil
}

func (p *GoogleProxy) writeModelError(w http.ResponseWriter, r *http.Request, keyID int32, channelID int32, modelID int32, modelName string, err error) {
	status := http.StatusInternalServerError
	message := "model lookup failed"
	outcome := common.IntegralOutcomeRejected
	rejectionStage := common.RejectionStageInput
	rejectionReason := common.RejectionReasonModel
	switch {
	case errors.Is(err, common.ErrInvalidRequestBody):
		status = http.StatusBadRequest
		message = "invalid request body"
		rejectionReason = common.RejectionReasonInvalidRequest
		zap.L().Error("parse google interactions request failed", zap.Error(err), zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.String("method", r.Method), zap.String("path", r.URL.Path))
	case errors.Is(err, common.ErrMissingModel):
		status = http.StatusBadRequest
		message = "missing model"
		zap.L().Warn("missing google model", zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.String("method", r.Method), zap.String("path", r.URL.Path))
	case errors.Is(err, common.ErrModelDisabled):
		status = http.StatusForbidden
		message = "model disabled"
		zap.L().Warn("google model disabled", zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.String("model", modelName), zap.String("method", r.Method), zap.String("path", r.URL.Path))
	case errors.Is(err, common.ErrModelUnsupported):
		status = http.StatusBadRequest
		message = "unsupported model"
		zap.L().Warn("google model unsupported", zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.String("model", modelName), zap.String("method", r.Method), zap.String("path", r.URL.Path))
	default:
		outcome = common.IntegralOutcomeError
		rejectionStage = ""
		rejectionReason = ""
		zap.L().Error("resolve google model failed", zap.Error(err), zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.String("model", modelName), zap.String("method", r.Method), zap.String("path", r.URL.Path))
	}

	responseBody := []byte(message + "\n")
	http.Error(w, message, status)
	go common.ProcessIntegralLogAsync(p.integralLogs, r, common.IntegralLogInfo{
		Provider:            common.ProviderGoogle,
		APIFormat:           models.APIFormatGoogle,
		KeyID:               keyID,
		ChannelID:           channelID,
		ModelID:             modelID,
		Model:               modelName,
		Outcome:             outcome,
		RejectionStage:      rejectionStage,
		RejectionReason:     rejectionReason,
		ResponseStatus:      status,
		ResponseContentType: "text/plain; charset=utf-8",
		DecodeOK:            true,
	}, responseBody)
}

func (p *GoogleProxy) writePromptRejected(w http.ResponseWriter, r *http.Request, keyID int32, channelID int32, modelID int32, modelName string) {
	zap.L().Warn("google prompt rejected", zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.Int32("model_id", modelID), zap.String("model", modelName), zap.String("method", r.Method), zap.String("path", r.URL.Path))
	responseBody := []byte("prompt rejected\n")
	http.Error(w, "prompt rejected", http.StatusForbidden)
	go common.ProcessIntegralLogAsync(p.integralLogs, r, common.IntegralLogInfo{
		Provider:            common.ProviderGoogle,
		APIFormat:           models.APIFormatGoogle,
		KeyID:               keyID,
		ChannelID:           channelID,
		ModelID:             modelID,
		Model:               modelName,
		Outcome:             common.IntegralOutcomeRejected,
		RejectionStage:      common.RejectionStageInput,
		RejectionReason:     common.RejectionReasonSensitive,
		ResponseStatus:      http.StatusForbidden,
		ResponseContentType: "text/plain; charset=utf-8",
		DecodeOK:            true,
	}, responseBody)
}

func (p *GoogleProxy) processUsageAsync(rawBody []byte, keyID int32, channelID int32, modelID int32, model string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if modelID == 0 {
		zap.L().Error("missing resolved model id for google usage", zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.String("model", model))
		return
	}
	if err := p.usageHandler.ProcessInteractionResponse(ctx, rawBody, common.UsageContext{KeyID: keyID, ChannelID: channelID, ModelID: modelID, Model: model}); err != nil {
		zap.L().Error("process google usage failed", zap.Error(err), zap.String("raw_body_prefix", common.BodyPrefix(rawBody, 128)), zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.Int32("model_id", modelID), zap.String("model", model))
		return
	}
	zap.L().Info("google usage processed", zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.Int32("model_id", modelID), zap.String("model", model))
}

func (p *GoogleProxy) processStreamingUsageAsync(decodedBody []byte, keyID int32, channelID int32, modelID int32, model string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if modelID == 0 {
		zap.L().Error("missing resolved model id for google streaming usage", zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.String("model", model))
		return
	}
	if err := p.usageHandler.ProcessInteractionStreamingResponse(ctx, decodedBody, common.UsageContext{KeyID: keyID, ChannelID: channelID, ModelID: modelID, Model: model}); err != nil {
		zap.L().Error("process google streaming usage failed", zap.Error(err), zap.String("raw_body_prefix", common.BodyPrefix(decodedBody, 128)), zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.Int32("model_id", modelID), zap.String("model", model))
		return
	}
	zap.L().Info("google streaming usage processed", zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.Int32("model_id", modelID), zap.String("model", model))
}

func isGoogleInteractionsRequest(r *http.Request, baseURL string) bool {
	if r.Method != http.MethodPost {
		return false
	}
	requestPath := path.Clean("/" + strings.TrimPrefix(r.URL.Path, "/"))
	effectivePath := googleEffectivePath(baseURL, r.URL.Path)
	return requestPath == googleInteractionsPath || requestPath == googleV1BetaInteractionsPath || effectivePath == googleV1BetaInteractionsPath
}

func googleEffectivePath(baseURL string, requestPath string) string {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || parsed.Path == "" || parsed.Path == "/" {
		return path.Clean("/" + strings.TrimPrefix(requestPath, "/"))
	}
	if requestPath == "" || requestPath == "/" {
		return path.Clean(parsed.Path)
	}
	return path.Clean(parsed.Path + "/" + strings.TrimPrefix(requestPath, "/"))
}
