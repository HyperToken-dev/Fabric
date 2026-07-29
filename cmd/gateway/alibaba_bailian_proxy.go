package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	corealibaba "github.com/HyperToken-dev/fabric/core/providers/Alibaba"
	coreproxy "github.com/HyperToken-dev/fabric/core/proxy"
	"go.uber.org/zap"
)

const (
	bailianVideoSynthesisPath = "/api/v1/services/aigc/video-generation/video-synthesis"
	bailianTasksPathPrefix    = "/api/v1/tasks/"
)

type AlibabaBailianProxy struct {
	coreProxy    *coreproxy.Proxy
	modelStore   ModelStore
	integralLogs IntegralLogHandler
}

type AlibabaBailianProxyOptions struct {
	ModelStore         ModelStore
	IntegralLogHandler IntegralLogHandler
}

type bailianVideoSynthesisRequest struct {
	Model string `json:"model"`
}

func NewAlibabaBailianProxy(opts AlibabaBailianProxyOptions) (*AlibabaBailianProxy, error) {
	if opts.ModelStore == nil {
		opts.ModelStore = NoopModelStore{}
	}
	if opts.IntegralLogHandler == nil {
		opts.IntegralLogHandler = NoopIntegralLogHandler{}
	}
	p := &AlibabaBailianProxy{modelStore: opts.ModelStore, integralLogs: opts.IntegralLogHandler}
	coreProxy := corealibaba.New(coreproxy.Options{OnComplete: p.onComplete})
	p.coreProxy = coreProxy
	return p, nil
}

func (p *AlibabaBailianProxy) onComplete(resp *http.Response, decodedBody []byte) {
	keyID := getContextInt32(resp.Request, ctxKeyID)
	channelID := getContextInt32(resp.Request, ctxChannelID)
	model := getContextString(resp.Request, ctxModel)
	modelID := getContextInt32(resp.Request, ctxModelID)
	info := integralLogInfo{
		Provider:                ProviderAlibaba,
		APIFormat:               2,
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

func (p *AlibabaBailianProxy) ServeHTTP(w http.ResponseWriter, r *http.Request, keyID int32, channelID int32, baseURL string, providerKey string) {
	if strings.TrimSpace(providerKey) == "" {
		zap.L().Error("missing provider key", zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.String("provider", string(ProviderAlibaba)), zap.String("method", r.Method), zap.String("path", r.URL.Path))
		http.Error(w, "missing provider key", http.StatusBadGateway)
		return
	}

	if isBailianVideoSynthesisRequest(r) {
		modelID, modelName, err := p.prepareVideoSynthesisRequest(r, channelID)
		if err != nil {
			p.writeModelError(w, r, keyID, channelID, err)
			return
		}
		zap.L().Info("alibaba bailian video task request received", zap.String("method", r.Method), zap.String("path", r.URL.Path), zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.String("model", modelName))
		r = setContextString(r, ctxModel, modelName)
		r = setContextInt32(r, ctxModelID, modelID)
	} else if !isBailianTaskFetchRequest(r) {
		zap.L().Warn("unsupported alibaba bailian proxy request", zap.String("method", r.Method), zap.String("path", r.URL.Path), zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID))
		http.Error(w, "unsupported alibaba bailian path", http.StatusNotFound)
		return
	}

	r = setContextInt32(r, ctxKeyID, keyID)
	r = setContextInt32(r, ctxChannelID, channelID)
	p.coreProxy.ServeHTTP(w, r, coreproxy.Upstream{BaseURL: baseURL, APIKey: providerKey})
}

func (p *AlibabaBailianProxy) prepareVideoSynthesisRequest(r *http.Request, channelID int32) (int32, string, error) {
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

	var payload bailianVideoSynthesisRequest
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

func (p *AlibabaBailianProxy) writeModelError(w http.ResponseWriter, r *http.Request, keyID int32, channelID int32, err error) {
	switch {
	case errors.Is(err, errInvalidRequestBody):
		zap.L().Error("parse alibaba bailian request failed", zap.Error(err), zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.String("method", r.Method), zap.String("path", r.URL.Path))
		http.Error(w, "invalid request body", http.StatusBadRequest)
	case errors.Is(err, errMissingModel):
		zap.L().Warn("missing alibaba bailian model", zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.String("method", r.Method), zap.String("path", r.URL.Path))
		http.Error(w, "missing model", http.StatusBadRequest)
	case errors.Is(err, errModelDisabled):
		zap.L().Warn("alibaba bailian model disabled", zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.String("method", r.Method), zap.String("path", r.URL.Path))
		http.Error(w, "model disabled", http.StatusForbidden)
	case errors.Is(err, errModelUnsupported):
		zap.L().Warn("alibaba bailian model unsupported", zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.String("method", r.Method), zap.String("path", r.URL.Path))
		http.Error(w, "unsupported model", http.StatusBadRequest)
	default:
		zap.L().Error("resolve alibaba bailian model failed", zap.Error(err), zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.String("method", r.Method), zap.String("path", r.URL.Path))
		http.Error(w, "model lookup failed", http.StatusInternalServerError)
	}
}

func isBailianVideoSynthesisRequest(r *http.Request) bool {
	return r.Method == http.MethodPost && r.URL.Path == bailianVideoSynthesisPath
}

func isBailianTaskFetchRequest(r *http.Request) bool {
	return r.Method == http.MethodGet && strings.HasPrefix(r.URL.Path, bailianTasksPathPrefix) && strings.TrimPrefix(r.URL.Path, bailianTasksPathPrefix) != ""
}

func restoreRequestBody(req *http.Request, body []byte) {
	req.Body = io.NopCloser(bytes.NewReader(body))
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(body)), nil
	}
	req.ContentLength = int64(len(body))
	req.Header.Set("Content-Length", strconv.Itoa(len(body)))
}

var errInvalidRequestBody = errors.New("invalid request body")
var errMissingModel = errors.New("missing model")
