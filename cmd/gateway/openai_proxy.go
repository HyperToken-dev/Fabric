package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httputil"
	"strconv"
	"strings"
	"time"

	sensitiveopenai "fabric/business/sensitive/openai"
	coreopenai "fabric/core/providers/openai"
	coreproxy "fabric/core/proxy"

	"go.uber.org/zap"
)

type OpenAIProxy struct {
	coreProxy    *coreopenai.Proxy
	modelStore   ModelStore
	usageHandler UsageHandler
	textPolicy   TextPolicy
}

type OpenAIProxyOptions struct {
	ModelStore   ModelStore
	UsageHandler UsageHandler
	TextPolicy   TextPolicy
}

type openaiChatRequest struct {
	Model  string `json:"model"`
	Stream bool   `json:"stream"`
}

func NewOpenAIProxy(opts OpenAIProxyOptions) (*OpenAIProxy, error) {
	if opts.ModelStore == nil {
		opts.ModelStore = NoopModelStore{}
	}
	if opts.UsageHandler == nil {
		opts.UsageHandler = NoopUsageHandler{}
	}
	if opts.TextPolicy == nil {
		opts.TextPolicy = NoopTextPolicy{}
	}
	p := &OpenAIProxy{
		modelStore:   opts.ModelStore,
		usageHandler: opts.UsageHandler,
		textPolicy:   opts.TextPolicy,
	}
	coreProxy, err := coreopenai.New(coreopenai.Options{
		Rewrite:        p.rewrite,
		ModifyResponse: p.modifyResponse,
	})
	if err != nil {
		return nil, err
	}
	p.coreProxy = coreProxy
	return p, nil
}

func (p *OpenAIProxy) rewrite(pr *httputil.ProxyRequest) {
	if pr.Out.URL.Path == "/v1/chat/completions" && getContextBool(pr.In, ctxStreamKey) {
		if err := injectOpenAIChatStreamOptions(pr.Out); err != nil {
			zap.L().Error("inject openai chat stream options failed", zap.Error(err))
		}
	}
}

// if stream = true and /v1/chat/com we need insert a new field "include_usage"
func injectOpenAIChatStreamOptions(req *http.Request) error {
	if req.Body == nil {
		return nil
	}

	body, err := io.ReadAll(req.Body)
	closeErr := req.Body.Close()
	if err != nil {
		if closeErr != nil {
			return errors.Join(err, closeErr)
		}
		return err
	}
	if closeErr != nil {
		return closeErr
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		restoreOpenAIRequestBody(req, body)
		return err
	}

	streamOptions, ok := payload["stream_options"].(map[string]any)
	if !ok {
		streamOptions = make(map[string]any)
	}
	streamOptions["include_usage"] = true
	payload["stream_options"] = streamOptions

	newBody, err := json.Marshal(payload)
	if err != nil {
		restoreOpenAIRequestBody(req, body)
		return err
	}
	restoreOpenAIRequestBody(req, newBody)
	return nil
}

func restoreOpenAIRequestBody(req *http.Request, body []byte) {
	req.Body = io.NopCloser(bytes.NewReader(body))
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(body)), nil
	}
	req.ContentLength = int64(len(body))
	req.Header.Set("Content-Length", strconv.Itoa(len(body)))
}

func (p *OpenAIProxy) modifyResponse(resp *http.Response) error {
	keyID := getContextInt32(resp.Request, ctxKeyID)
	channelID := getContextInt32(resp.Request, ctxChannelID)
	model := getContextString(resp.Request, ctxModel)
	modelID := getContextInt32(resp.Request, ctxModelID)

	// process SSE
	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices && isOpenAIUsageStream(resp.Request, resp.Header.Get("Content-Type")) {
		zap.L().Info("openai upstream streaming response received",
			zap.Int("status_code", resp.StatusCode),
			zap.String("content_type", resp.Header.Get("Content-Type")),
			zap.String("content_encoding", resp.Header.Get("Content-Encoding")),
			zap.Int32("key_id", keyID),
			zap.Int32("channel_id", channelID),
			zap.Int32("model_id", modelID),
			zap.String("model", model),
		)
		resp.Body = p.usageHandler.WrapStreamingResponse(resp.Body, resp.Header.Get("Content-Encoding"), UsageContext{
			KeyID:     keyID,
			ChannelID: channelID,
			ModelID:   modelID,
			Model:     model,
		})
		resp.ContentLength = -1
		resp.Header.Del("Content-Length")
		return nil
	}

	// process none SSE
	rawBody, err := io.ReadAll(resp.Body)
	closeErr := resp.Body.Close()
	// all error in this function shouldn't return an error to client
	// just log
	if err != nil {
		zap.L().Error("read upstream response body failed", zap.Error(err))
		if closeErr != nil {
			zap.L().Error("close upstream response body failed", zap.Error(closeErr))
		}
		return nil
	}
	if closeErr != nil {
		zap.L().Error("close upstream response body failed", zap.Error(closeErr))
	}
	resp.Body = io.NopCloser(bytes.NewReader(rawBody))
	resp.ContentLength = int64(len(rawBody))
	zap.L().Info("openai upstream response received",
		zap.Int("status_code", resp.StatusCode),
		zap.String("content_type", resp.Header.Get("Content-Type")),
		zap.String("content_encoding", resp.Header.Get("Content-Encoding")),
		zap.Int32("key_id", keyID),
		zap.Int32("channel_id", channelID),
		zap.Int32("model_id", modelID),
		zap.String("model", model),
		zap.Int("body_bytes", len(rawBody)),
	)
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil
	}
	contentEncoding := resp.Header.Get("Content-Encoding")
	contentType := resp.Header.Get("Content-Type")
	decodedBody, err := decodeResponseBody(rawBody, contentEncoding)
	if err != nil {
		zap.L().Error("decode response body for output detection failed", zap.Error(err), zap.String("content_type", contentType), zap.String("content_encoding", contentEncoding), zap.String("raw_body_prefix", bodyPrefix(rawBody, 128)))
	} else {
		outputTexts, err := sensitiveopenai.ExtractOutputTexts(resp.Request, decodedBody)
		if err != nil {
			zap.L().Error("extract openai output texts failed", zap.Error(err), zap.String("content_type", contentType), zap.String("decoded_body_prefix", bodyPrefix(decodedBody, 128)))
		} else if detectPrompts(resp.Request.Context(), model, TextDirectionOutput, outputTexts, p.textPolicy) {
			rejectOpenAIOutputResponse(resp)
		}
	}

	// async process can improve efficiency,make sure client get realtime response
	go p.processUsageAsync(rawBody, contentEncoding, contentType, keyID, channelID, modelID, model)

	return nil
}

func rejectOpenAIOutputResponse(resp *http.Response) {
	errorBody := []byte(`{"error":"model output rejected, please change your prompt"}`)
	resp.StatusCode = http.StatusUnprocessableEntity
	resp.Status = strconv.Itoa(http.StatusUnprocessableEntity) + " " + http.StatusText(http.StatusUnprocessableEntity)
	resp.Header.Set("Content-Type", "application/json")
	resp.Header.Del("Content-Encoding")
	resp.Header.Set("Content-Length", strconv.Itoa(len(errorBody)))
	resp.Body = io.NopCloser(bytes.NewReader(errorBody))
	resp.ContentLength = int64(len(errorBody))
}

// determine whether the data is SSE stram
func isOpenAIUsageStream(req *http.Request, contentType string) bool {
	isOpenAIUsagePath := strings.Contains(req.URL.Path, "/v1/responses") || strings.Contains(req.URL.Path, "/v1/chat/completions")
	return isOpenAIUsagePath && strings.Contains(strings.ToLower(contentType), "text/event-stream")
}

// async usage processing
func (p *OpenAIProxy) processUsageAsync(rawBody []byte, contentEncoding string, contentType string, keyID int32, channelID int32, modelID int32, model string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if modelID == 0 {
		zap.L().Error("missing resolved model id for non-streaming usage", zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.String("model", model))
		return
	}

	if err := p.usageHandler.ProcessNonStreamingResponse(ctx, rawBody, contentEncoding, contentType, UsageContext{
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
		http.Error(w, "prompt rejected", http.StatusForbidden)
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
	r = setContextBool(r, ctxStreamKey, parsedReq.Stream)
	p.coreProxy.ServeHTTP(w, r, coreproxy.Upstream{BaseURL: baseURL, APIKey: providerKey})
}
