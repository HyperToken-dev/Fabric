package extrotec

import (
	"encoding/json"
	"errors"
	"github.com/HyperToken-dev/fabric/cmd/gateway/common"
	"io"
	"net/http"
	"strings"

	sensitiveextrotec "github.com/HyperToken-dev/fabric/business/sensitive/extrotec"
	coreextrotec "github.com/HyperToken-dev/fabric/core/providers/extrotec"
	coreproxy "github.com/HyperToken-dev/fabric/core/proxy"
	"github.com/HyperToken-dev/fabric/internal/models"
	"go.uber.org/zap"
)

const (
	extrotecTextToVideoPath       = "/v1/video/generations"
	extrotecImageToVideoPath      = "/v1/video/i2v"
	extrotecMultiImageToVideoPath = "/v1/video/ref2v"
	extrotecImagePath             = "/v1/images/generations"

	extrotecCheckOrGetPath = "/v1/videos"
)

// ExtrotecProxy validates and audits Extrotec image and video requests.
type ExtrotecProxy struct {
	coreProxy    *coreproxy.Proxy
	modelStore   common.ModelStore
	integralLogs common.IntegralLogHandler
	textPolicy   common.TextPolicy
}

// ExtrotecProxyOptions wires model, audit, and text policy dependencies.
type ExtrotecProxyOptions struct {
	ModelStore         common.ModelStore
	IntegralLogHandler common.IntegralLogHandler
	TextPolicy         common.TextPolicy
}

// ExtrotecVideoRequest contains the model selector shared by Extrotec generation endpoints.
type ExtrotecVideoRequest struct {
	Model string `json:"model"`
}

// NewExtrotecProxy creates an Extrotec proxy with noop dependencies for nil options.
func NewExtrotecProxy(opts ExtrotecProxyOptions) (*ExtrotecProxy, error) {
	if opts.ModelStore == nil {
		opts.ModelStore = common.NoopModelStore{}
	}
	if opts.IntegralLogHandler == nil {
		opts.IntegralLogHandler = common.NoopIntegralLogHandler{}
	}
	if opts.TextPolicy == nil {
		opts.TextPolicy = common.NoopTextPolicy{}
	}
	p := &ExtrotecProxy{modelStore: opts.ModelStore, integralLogs: opts.IntegralLogHandler, textPolicy: opts.TextPolicy}
	coreProxy := coreextrotec.New(coreproxy.Options{OnComplete: p.onComplete})
	p.coreProxy = coreProxy
	return p, nil
}

func (p *ExtrotecProxy) onComplete(resp *http.Response, decodedBody []byte) {
	keyID := common.GetContextInt32(resp.Request, common.ContextKeyID)
	channelID := common.GetContextInt32(resp.Request, common.ContextKeyChannelID)
	model := common.GetContextString(resp.Request, common.ContextKeyModel)
	modelID := common.GetContextInt32(resp.Request, common.ContextKeyModelID)
	info := common.IntegralLogInfo{
		Provider:                common.ProviderExtrotec,
		APIFormat:               models.APIFormatExtrotec,
		KeyID:                   keyID,
		ChannelID:               channelID,
		ModelID:                 modelID,
		Model:                   model,
		Outcome:                 common.ResponseOutcome(resp.StatusCode),
		ResponseStatus:          resp.StatusCode,
		ResponseContentType:     resp.Header.Get("Content-Type"),
		ResponseContentEncoding: resp.Header.Get("Content-Encoding"),
		DecodeOK:                decodedBody != nil,
	}
	go common.ProcessIntegralLogAsync(p.integralLogs, resp.Request, info, decodedBody)
}

func (p *ExtrotecProxy) ServeHTTP(w http.ResponseWriter, r *http.Request, keyID int32, channelID int32, baseURL string, providerKey string) {
	if strings.TrimSpace(providerKey) == "" {
		zap.L().Error("missing provider key", zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.String("provider", string(common.ProviderExtrotec)), zap.String("method", r.Method), zap.String("path", r.URL.Path))
		http.Error(w, "missing provider key", http.StatusBadGateway)
		return
	}

	if isExtrotecGenerateRequest(r) {
		modelID, modelName, err := p.prepareRequest(r, channelID)
		if err != nil {
			if errors.Is(err, common.ErrPromptRejected) {
				p.writePromptRejected(w, r, keyID, channelID, modelID, modelName)
				return
			}
			p.writeModelError(w, r, keyID, channelID, err)
			return
		}
		zap.L().Info("extrotec generation request received", zap.String("method", r.Method), zap.String("path", r.URL.Path), zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.String("model", modelName))
		r = common.SetContextString(r, common.ContextKeyModel, modelName)
		r = common.SetContextInt32(r, common.ContextKeyModelID, modelID)
	} else if isExtrotecCheckRequest(r) {
		paths := strings.Split(r.URL.Path, "/")
		zap.L().Info("extrotec status check path received", zap.String("task", paths[len(paths)-1]))
	} else {
		zap.L().Warn("unsupported extrotec proxy request", zap.String("method", r.Method), zap.String("path", r.URL.Path), zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID))
		http.Error(w, "unsupported extrotec path", http.StatusNotFound)
		return
	}

	r = common.SetContextInt32(r, common.ContextKeyID, keyID)
	r = common.SetContextInt32(r, common.ContextKeyChannelID, channelID)
	p.coreProxy.ServeHTTP(w, r, coreproxy.Upstream{BaseURL: baseURL, APIKey: providerKey})
}

func (p *ExtrotecProxy) prepareRequest(r *http.Request, channelID int32) (int32, string, error) {
	if r.Body == nil {
		return 0, "", common.ErrMissingModel
	}

	body, err := io.ReadAll(r.Body)
	closeErr := r.Body.Close()
	if err != nil {
		if closeErr != nil {
			return 0, "", errors.Join(err, closeErr)
		}
		return 0, "", err
	}
	common.RestoreRequestBody(r, body)
	if closeErr != nil {
		return 0, "", closeErr
	}

	var payload ExtrotecVideoRequest
	if err := json.Unmarshal(body, &payload); err != nil {
		return 0, "", common.ErrInvalidRequestBody
	}
	modelName := strings.TrimSpace(payload.Model)
	if modelName == "" {
		return 0, "", common.ErrMissingModel
	}
	modelID, err := common.ResolveModelFromStore(r.Context(), p.modelStore, channelID, modelName)
	if err != nil {
		return 0, "", err
	}
	promptReq, err := sensitiveextrotec.ParsePromptRequest(body)
	if err != nil {
		return 0, "", common.ErrInvalidRequestBody
	}
	if common.DetectPrompts(r.Context(), modelName, common.TextDirectionInput, promptReq.Prompts, p.textPolicy) {
		return modelID, modelName, common.ErrPromptRejected
	}
	return modelID, modelName, nil
}

func (p *ExtrotecProxy) writeModelError(w http.ResponseWriter, r *http.Request, keyID int32, channelID int32, err error) {
	switch {
	case errors.Is(err, common.ErrInvalidRequestBody):
		zap.L().Error("parse extrotec request failed", zap.Error(err), zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.String("method", r.Method), zap.String("path", r.URL.Path))
		http.Error(w, "invalid request body", http.StatusBadRequest)
	case errors.Is(err, common.ErrMissingModel):
		zap.L().Warn("missing extrotec model", zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.String("method", r.Method), zap.String("path", r.URL.Path))
		http.Error(w, "missing model", http.StatusBadRequest)
	case errors.Is(err, common.ErrModelDisabled):
		zap.L().Warn("extrotec model disabled", zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.String("method", r.Method), zap.String("path", r.URL.Path))
		http.Error(w, "model disabled", http.StatusForbidden)
	case errors.Is(err, common.ErrModelUnsupported):
		zap.L().Warn("extrotec model unsupported", zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.String("method", r.Method), zap.String("path", r.URL.Path))
		http.Error(w, "unsupported model", http.StatusBadRequest)
	default:
		zap.L().Error("resolve extrotec model failed", zap.Error(err), zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.String("method", r.Method), zap.String("path", r.URL.Path))
		http.Error(w, "model lookup failed", http.StatusInternalServerError)
	}
}

func (p *ExtrotecProxy) writePromptRejected(w http.ResponseWriter, r *http.Request, keyID int32, channelID int32, modelID int32, modelName string) {
	zap.L().Warn("extrotec prompt rejected", zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.Int32("model_id", modelID), zap.String("model", modelName), zap.String("method", r.Method), zap.String("path", r.URL.Path))
	responseBody := []byte("prompt rejected\n")
	http.Error(w, "prompt rejected", http.StatusForbidden)
	go common.ProcessIntegralLogAsync(p.integralLogs, r, common.IntegralLogInfo{
		Provider:            common.ProviderExtrotec,
		APIFormat:           models.APIFormatExtrotec,
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

func isExtrotecGenerateRequest(r *http.Request) bool {
	if r.Method != http.MethodPost {
		return false
	}
	return r.URL.Path == extrotecTextToVideoPath || r.URL.Path == extrotecImageToVideoPath || r.URL.Path == extrotecMultiImageToVideoPath || r.URL.Path == extrotecImagePath
}

func isExtrotecCheckRequest(r *http.Request) bool {
	if r.Method != http.MethodGet {
		return false
	}
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 4 || len(parts) > 5 || parts[1] != "v1" || parts[2] != "videos" || strings.TrimSpace(parts[3]) == "" || len(parts) == 5 && parts[4] != "content" {
		return false
	}
	return true
}
