package main

import (
	"context"
	"errors"
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

type GoogleProxy struct {
	coreProxy    *coreproxy.Proxy
	modelStore   ModelStore
	usageHandler GoogleUsageHandler
	integralLogs IntegralLogHandler
	textPolicy   TextPolicy
}

type GoogleProxyOptions struct {
	ModelStore         ModelStore
	UsageHandler       GoogleUsageHandler
	IntegralLogHandler IntegralLogHandler
	TextPolicy         TextPolicy
}

type GoogleUsageHandler interface {
	ProcessInteractionResponse(ctx context.Context, rawBody []byte, info UsageContext) error
}

func NewGoogleProxy(opts GoogleProxyOptions) (*GoogleProxy, error) {
	if opts.ModelStore == nil {
		opts.ModelStore = NoopModelStore{}
	}
	if opts.UsageHandler == nil {
		opts.UsageHandler = NoopGoogleUsageHandler{}
	}
	if opts.IntegralLogHandler == nil {
		opts.IntegralLogHandler = NoopIntegralLogHandler{}
	}
	if opts.TextPolicy == nil {
		opts.TextPolicy = NoopTextPolicy{}
	}
	p := &GoogleProxy{modelStore: opts.ModelStore, usageHandler: opts.UsageHandler, integralLogs: opts.IntegralLogHandler, textPolicy: opts.TextPolicy}
	p.coreProxy = coregoogle.New(coreproxy.Options{OnComplete: p.onComplete})
	return p, nil
}

func (p *GoogleProxy) onComplete(resp *http.Response, decodedBody []byte) {
	keyID := getContextInt32(resp.Request, ctxKeyID)
	channelID := getContextInt32(resp.Request, ctxChannelID)
	model := getContextString(resp.Request, ctxModel)
	modelID := getContextInt32(resp.Request, ctxModelID)
	contentType := resp.Header.Get("Content-Type")
	contentEncoding := resp.Header.Get("Content-Encoding")
	info := integralLogInfo{
		Provider:                ProviderGoogle,
		APIFormat:               models.APIFormatGoogle,
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
	if decodedBody != nil && resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices {
		go p.processUsageAsync(decodedBody, keyID, channelID, modelID, model)
	}
	go processIntegralLogAsync(p.integralLogs, resp.Request, info, decodedBody)
}

func (p *GoogleProxy) ServeHTTP(w http.ResponseWriter, r *http.Request, keyID int32, channelID int32, baseURL string, providerKey string) {
	if strings.TrimSpace(providerKey) == "" {
		zap.L().Error("missing provider key", zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.String("provider", string(ProviderGoogle)), zap.String("method", r.Method), zap.String("path", r.URL.Path))
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
		if errors.Is(err, errPromptRejected) {
			p.writePromptRejected(w, r, keyID, channelID, modelID, modelName)
			return
		}
		p.writeModelError(w, r, keyID, channelID, modelID, modelName, err)
		return
	}
	zap.L().Info("google interactions request received", zap.String("method", r.Method), zap.String("path", r.URL.Path), zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.String("model", modelName))

	r = setContextInt32(r, ctxKeyID, keyID)
	r = setContextInt32(r, ctxChannelID, channelID)
	r = setContextString(r, ctxModel, modelName)
	r = setContextInt32(r, ctxModelID, modelID)
	p.coreProxy.ServeHTTP(w, r, coreproxy.Upstream{BaseURL: baseURL, APIKey: providerKey})
}

func (p *GoogleProxy) prepareInteractionsRequest(r *http.Request, channelID int32) (int32, string, error) {
	parsedReq, err := sensitivegoogle.ExtractPromptRequest(r)
	if err != nil {
		return 0, "", errInvalidRequestBody
	}
	modelName := strings.TrimSpace(parsedReq.Model)
	if modelName == "" {
		return 0, "", errMissingModel
	}
	modelID, err := resolveModelFromStore(r.Context(), p.modelStore, channelID, modelName)
	if err != nil {
		return 0, modelName, err
	}
	if detectPrompts(r.Context(), modelName, TextDirectionInput, parsedReq.Prompts, p.textPolicy) {
		return modelID, modelName, errPromptRejected
	}
	return modelID, modelName, nil
}

func (p *GoogleProxy) writeModelError(w http.ResponseWriter, r *http.Request, keyID int32, channelID int32, modelID int32, modelName string, err error) {
	status := http.StatusInternalServerError
	message := "model lookup failed"
	outcome := integralOutcomeRejected
	rejectionStage := rejectionStageInput
	rejectionReason := rejectionReasonModel
	switch {
	case errors.Is(err, errInvalidRequestBody):
		status = http.StatusBadRequest
		message = "invalid request body"
		rejectionReason = rejectionReasonInvalidRequest
		zap.L().Error("parse google interactions request failed", zap.Error(err), zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.String("method", r.Method), zap.String("path", r.URL.Path))
	case errors.Is(err, errMissingModel):
		status = http.StatusBadRequest
		message = "missing model"
		zap.L().Warn("missing google model", zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.String("method", r.Method), zap.String("path", r.URL.Path))
	case errors.Is(err, errModelDisabled):
		status = http.StatusForbidden
		message = "model disabled"
		zap.L().Warn("google model disabled", zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.String("model", modelName), zap.String("method", r.Method), zap.String("path", r.URL.Path))
	case errors.Is(err, errModelUnsupported):
		status = http.StatusBadRequest
		message = "unsupported model"
		zap.L().Warn("google model unsupported", zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.String("model", modelName), zap.String("method", r.Method), zap.String("path", r.URL.Path))
	default:
		outcome = integralOutcomeError
		rejectionStage = ""
		rejectionReason = ""
		zap.L().Error("resolve google model failed", zap.Error(err), zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.String("model", modelName), zap.String("method", r.Method), zap.String("path", r.URL.Path))
	}

	responseBody := []byte(message + "\n")
	http.Error(w, message, status)
	go processIntegralLogAsync(p.integralLogs, r, integralLogInfo{
		Provider:            ProviderGoogle,
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
	go processIntegralLogAsync(p.integralLogs, r, integralLogInfo{
		Provider:            ProviderGoogle,
		APIFormat:           models.APIFormatGoogle,
		KeyID:               keyID,
		ChannelID:           channelID,
		ModelID:             modelID,
		Model:               modelName,
		Outcome:             integralOutcomeRejected,
		RejectionStage:      rejectionStageInput,
		RejectionReason:     rejectionReasonSensitive,
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
	if err := p.usageHandler.ProcessInteractionResponse(ctx, rawBody, UsageContext{KeyID: keyID, ChannelID: channelID, ModelID: modelID, Model: model}); err != nil {
		zap.L().Error("process google usage failed", zap.Error(err), zap.String("raw_body_prefix", bodyPrefix(rawBody, 128)), zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.Int32("model_id", modelID), zap.String("model", model))
		return
	}
	zap.L().Info("google usage processed", zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.Int32("model_id", modelID), zap.String("model", model))
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

type NoopGoogleUsageHandler struct{}

func (NoopGoogleUsageHandler) ProcessInteractionResponse(ctx context.Context, rawBody []byte, info UsageContext) error {
	return nil
}
