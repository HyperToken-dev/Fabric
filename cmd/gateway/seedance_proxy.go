package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	coreseedance "github.com/HyperToken-dev/fabric/core/providers/seedance"
	coreproxy "github.com/HyperToken-dev/fabric/core/proxy"
	"github.com/HyperToken-dev/fabric/internal/models"
	"go.uber.org/zap"
)

const (
	seedanceTasksPath = "/api/v3/contents/generations/tasks"
)

type SeedanceProxy struct {
	coreProxy    *coreproxy.Proxy
	modelStore   ModelStore
	integralLogs IntegralLogHandler
}

type SeedanceProxyOptions struct {
	ModelStore         ModelStore
	IntegralLogHandler IntegralLogHandler
}

type seedanceVideoTaskRequest struct {
	Model string `json:"model"`
}

func NewSeedanceProxy(opts SeedanceProxyOptions) (*SeedanceProxy, error) {
	if opts.ModelStore == nil {
		opts.ModelStore = NoopModelStore{}
	}
	if opts.IntegralLogHandler == nil {
		opts.IntegralLogHandler = NoopIntegralLogHandler{}
	}
	p := &SeedanceProxy{modelStore: opts.ModelStore, integralLogs: opts.IntegralLogHandler}
	p.coreProxy = coreseedance.New(coreproxy.Options{OnComplete: p.onComplete})
	return p, nil
}

func (p *SeedanceProxy) onComplete(resp *http.Response, decodedBody []byte) {
	keyID := getContextInt32(resp.Request, ctxKeyID)
	channelID := getContextInt32(resp.Request, ctxChannelID)
	model := getContextString(resp.Request, ctxModel)
	modelID := getContextInt32(resp.Request, ctxModelID)
	info := integralLogInfo{
		Provider:                ProviderSeedance,
		APIFormat:               models.APIFormatSeedance,
		KeyID:                   keyID,
		ChannelID:               channelID,
		ModelID:                 modelID,
		Model:                   model,
		Outcome:                 responseOutcome(resp.StatusCode),
		ResponseStatus:          resp.StatusCode,
		ResponseContentType:     resp.Header.Get("Content-Type"),
		ResponseContentEncoding: resp.Header.Get("Content-Encoding"),
		DecodeOK:                decodedBody != nil,
	}
	go processIntegralLogAsync(p.integralLogs, resp.Request, info, decodedBody)
}

func (p *SeedanceProxy) ServeHTTP(w http.ResponseWriter, r *http.Request, keyID int32, channelID int32, baseURL string, providerKey string) {
	if strings.TrimSpace(providerKey) == "" {
		zap.L().Error("missing provider key", zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.String("provider", string(ProviderSeedance)), zap.String("method", r.Method), zap.String("path", r.URL.Path))
		http.Error(w, "missing provider key", http.StatusBadGateway)
		return
	}

	if isSeedanceCreateTaskRequest(r) {
		modelID, modelName, err := p.prepareCreateTaskRequest(r, channelID)
		if err != nil {
			p.writeModelError(w, r, keyID, channelID, err)
			return
		}
		zap.L().Info("seedance video task request received", zap.String("method", r.Method), zap.String("path", r.URL.Path), zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.String("model", modelName))
		r = setContextString(r, ctxModel, modelName)
		r = setContextInt32(r, ctxModelID, modelID)
	} else if !isSeedanceTaskManagementRequest(r) {
		zap.L().Warn("unsupported seedance proxy request", zap.String("method", r.Method), zap.String("path", r.URL.Path), zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID))
		http.Error(w, "unsupported seedance path", http.StatusNotFound)
		return
	}

	r = setContextInt32(r, ctxKeyID, keyID)
	r = setContextInt32(r, ctxChannelID, channelID)
	p.coreProxy.ServeHTTP(w, r, coreproxy.Upstream{BaseURL: baseURL, APIKey: providerKey})
}

func (p *SeedanceProxy) prepareCreateTaskRequest(r *http.Request, channelID int32) (int32, string, error) {
	if r.Body == nil {
		return 0, "", errMissingModel
	}
	body, err := io.ReadAll(r.Body)
	closeErr := r.Body.Close()
	if err != nil {
		if closeErr != nil {
			return 0, "", errors.Join(err, closeErr)
		}
		return 0, "", err
	}
	restoreRequestBody(r, body)
	if closeErr != nil {
		return 0, "", closeErr
	}

	var payload seedanceVideoTaskRequest
	if err := json.Unmarshal(body, &payload); err != nil {
		return 0, "", errInvalidRequestBody
	}
	modelName := strings.TrimSpace(payload.Model)
	if modelName == "" {
		return 0, "", errMissingModel
	}
	modelID, err := resolveModelFromStore(r.Context(), p.modelStore, channelID, modelName)
	if err != nil {
		return 0, "", err
	}
	return modelID, modelName, nil
}

func (p *SeedanceProxy) writeModelError(w http.ResponseWriter, r *http.Request, keyID int32, channelID int32, err error) {
	switch {
	case errors.Is(err, errInvalidRequestBody):
		zap.L().Error("parse seedance request failed", zap.Error(err), zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.String("method", r.Method), zap.String("path", r.URL.Path))
		http.Error(w, "invalid request body", http.StatusBadRequest)
	case errors.Is(err, errMissingModel):
		zap.L().Warn("missing seedance model", zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.String("method", r.Method), zap.String("path", r.URL.Path))
		http.Error(w, "missing model", http.StatusBadRequest)
	case errors.Is(err, errModelDisabled):
		zap.L().Warn("seedance model disabled", zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.String("method", r.Method), zap.String("path", r.URL.Path))
		http.Error(w, "model disabled", http.StatusForbidden)
	case errors.Is(err, errModelUnsupported):
		zap.L().Warn("seedance model unsupported", zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.String("method", r.Method), zap.String("path", r.URL.Path))
		http.Error(w, "unsupported model", http.StatusBadRequest)
	default:
		zap.L().Error("resolve seedance model failed", zap.Error(err), zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.String("method", r.Method), zap.String("path", r.URL.Path))
		http.Error(w, "model lookup failed", http.StatusInternalServerError)
	}
}

func isSeedanceCreateTaskRequest(r *http.Request) bool {
	return r.Method == http.MethodPost && r.URL.Path == seedanceTasksPath
}

func isSeedanceTaskManagementRequest(r *http.Request) bool {
	if r.URL.Path == seedanceTasksPath {
		return r.Method == http.MethodGet
	}
	if !strings.HasPrefix(r.URL.Path, seedanceTasksPath+"/") || strings.TrimPrefix(r.URL.Path, seedanceTasksPath+"/") == "" {
		return false
	}
	return r.Method == http.MethodGet || r.Method == http.MethodDelete
}
