package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httputil"
	"strconv"
	"strings"

	corealibaba "github.com/HyperToken-dev/fabric/core/providers/Alibaba"
	coreproxy "github.com/HyperToken-dev/fabric/core/proxy"
	"go.uber.org/zap"
)

const (
	bailianVideoSynthesisPath = "/api/v1/services/aigc/video-generation/video-synthesis"
	bailianTasksPathPrefix    = "/api/v1/tasks/"
	bailianAsyncHeader        = "X-DashScope-Async"
	bailianAsyncHeaderValue   = "enable"
)

type AlibabaBailianProxy struct {
	coreProxy  *coreproxy.Proxy
	modelStore ModelStore
}

type AlibabaBailianProxyOptions struct {
	ModelStore ModelStore
}

type bailianVideoSynthesisRequest struct {
	Model string `json:"model"`
}

func NewAlibabaBailianProxy(opts AlibabaBailianProxyOptions) (*AlibabaBailianProxy, error) {
	if opts.ModelStore == nil {
		opts.ModelStore = NoopModelStore{}
	}
	p := &AlibabaBailianProxy{modelStore: opts.ModelStore}
	coreProxy, err := corealibaba.New(coreproxy.Options{Rewrite: p.rewrite})
	if err != nil {
		return nil, err
	}
	p.coreProxy = coreProxy
	return p, nil
}

func (p *AlibabaBailianProxy) ServeHTTP(w http.ResponseWriter, r *http.Request, keyID int32, channelID int32, baseURL string, providerKey string) {
	if strings.TrimSpace(providerKey) == "" {
		zap.L().Error("missing provider key", zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.String("provider", string(ProviderAlibaba)), zap.String("method", r.Method), zap.String("path", r.URL.Path))
		http.Error(w, "missing provider key", http.StatusBadGateway)
		return
	}

	if isBailianVideoSynthesisRequest(r) {
		modelName, err := p.prepareVideoSynthesisRequest(r, channelID)
		if err != nil {
			p.writeModelError(w, r, keyID, channelID, err)
			return
		}
		zap.L().Info("alibaba bailian video task request received", zap.String("method", r.Method), zap.String("path", r.URL.Path), zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.String("model", modelName))
	} else if !isBailianTaskFetchRequest(r) {
		zap.L().Warn("unsupported alibaba bailian proxy request", zap.String("method", r.Method), zap.String("path", r.URL.Path), zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID))
		http.Error(w, "unsupported alibaba bailian path", http.StatusNotFound)
		return
	}

	p.coreProxy.ServeHTTP(w, r, coreproxy.Upstream{BaseURL: baseURL, APIKey: providerKey})
}

func (p *AlibabaBailianProxy) prepareVideoSynthesisRequest(r *http.Request, channelID int32) (string, error) {
	if r.Body == nil {
		return "", errMissingModel
	}
	body, err := io.ReadAll(r.Body)
	closeErr := r.Body.Close()
	if err != nil {
		if closeErr != nil {
			return "", errors.Join(err, closeErr)
		}
		return "", err
	}
	restoreRequestBody(r, body)
	if closeErr != nil {
		return "", closeErr
	}

	var payload bailianVideoSynthesisRequest
	if err := json.Unmarshal(body, &payload); err != nil {
		return "", errInvalidRequestBody
	}
	modelName := strings.TrimSpace(payload.Model)
	if modelName == "" {
		return "", errMissingModel
	}
	if _, err := resolveModelFromStore(r.Context(), p.modelStore, channelID, modelName); err != nil {
		return "", err
	}
	return modelName, nil
}

func (p *AlibabaBailianProxy) rewrite(pr *httputil.ProxyRequest, upstream coreproxy.Upstream) error {
	if isBailianVideoSynthesisRequest(pr.Out) {
		pr.Out.Header.Set(bailianAsyncHeader, bailianAsyncHeaderValue)
	}
	return nil
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
