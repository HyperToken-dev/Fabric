package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
)

type OpenAIProxy struct {
	proxy        *httputil.ReverseProxy
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

func NewOpenAIProxy(opts OpenAIProxyOptions) *OpenAIProxy {
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
	p.proxy = p.buildProxy()
	return p
}

func (p *OpenAIProxy) buildProxy() *httputil.ReverseProxy {
	director := func(req *http.Request) {
		model := getContextString(req, ctxModel)
		stream := getContextBool(req, ctxStreamKey)
		ctx := context.WithValue(req.Context(), ctxModel, model)
		ctx = context.WithValue(ctx, ctxProvider, string(ProviderOpenAI))
		*req = *req.WithContext(ctx)

		apiKey := getContextString(req, ctxAPIKey)
		req.Header.Set("Authorization", "Bearer "+apiKey)
		if req.URL.Path == "/v1/chat/completions" && stream {
			if err := injectOpenAIChatStreamOptions(req); err != nil {
				zap.S().Errorf("Error catched: inject openai chat stream options error: %v", err)
			}
		}

		target, ok := req.Context().Value(ctxUpstream).(*url.URL)
		if !ok {
			zap.S().Error("Error catched: missing upstream target")
			return
		}
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.Host = target.Host
	}

	return &httputil.ReverseProxy{
		Director:       director,
		ModifyResponse: p.modifyResponse,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			zap.S().Errorf("Error catched: proxy error: %v", err)
			http.Error(w, "bad gateway", http.StatusBadGateway)
		},
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
		zap.S().Errorf("Error catched: upstream error: %v", err)
		if closeErr != nil {
			zap.S().Errorf("Error catched: upstream body close error: %v", closeErr)
		}
		return nil
	}
	if closeErr != nil {
		zap.S().Errorf("Error catched: upstream body close error: %v", closeErr)
	}
	resp.Body = io.NopCloser(bytes.NewReader(rawBody))
	resp.ContentLength = int64(len(rawBody))
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil
	}
	contentEncoding := resp.Header.Get("Content-Encoding")
	contentType := resp.Header.Get("Content-Type")
	decodedBody, err := decodeResponseBody(rawBody, contentEncoding)
	if err != nil {
		zap.S().Errorf("Error catched: decode response body for output detection error: %v, content_type=%q, content_encoding=%q, raw_body_prefix=%q", err, contentType, contentEncoding, bodyPrefix(rawBody, 128))
	} else {
		outputTexts, err := extractOpenAIOutputTexts(resp.Request, decodedBody)
		if err != nil {
			zap.S().Errorf("Error catched: extract openai output texts error: %v, content_type=%q, decoded_body_prefix=%q", err, contentType, bodyPrefix(decodedBody, 128))
		} else if detectPrompts(resp.Request.Context(), outputTexts, p.textPolicy) {
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
		zap.S().Errorf("Error catched: missing resolved model id for non-streaming usage: key_id=%d, channel_id=%d, model=%q", keyID, channelID, model)
		return
	}

	if err := p.usageHandler.ProcessNonStreamingResponse(ctx, rawBody, contentEncoding, contentType, UsageContext{
		KeyID:     keyID,
		ChannelID: channelID,
		ModelID:   modelID,
		Model:     model,
	}); err != nil {
		zap.S().Errorf("Error catched: process token usage error: %v, content_type=%q, content_encoding=%q, raw_body_prefix=%q", err, contentType, contentEncoding, bodyPrefix(rawBody, 128))
		return
	}
}

func bodyPrefix(body []byte, maxLen int) string {
	if len(body) > maxLen {
		body = body[:maxLen]
	}
	return string(body)
}

func (p *OpenAIProxy) ServeHTTP(w http.ResponseWriter, r *http.Request, keyID int32, channelID int32, baseURL string, providerKey string) {
	target, err := parseChannelBaseURL(baseURL)
	if err != nil {
		zap.S().Errorf("Error catched: invalid channel base url: %v", err)
		http.Error(w, "invalid channel base url", http.StatusBadGateway)
		return
	}
	if strings.TrimSpace(providerKey) == "" {
		http.Error(w, "missing provider key", http.StatusBadGateway)
		return
	}

	// parse request and apply text policy
	parsedReq, err := parseOpenAIPromptRequest(r)
	if err != nil {
		zap.S().Errorf("Error catched: parse openai prompt request error: %v", err)
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	modelName := parsedReq.Model
	if modelName == "" {
		http.Error(w, "missing model", http.StatusBadRequest)
		return
	}
	if detectPrompts(r.Context(), parsedReq.Prompts, p.textPolicy) {
		http.Error(w, "prompt rejected", http.StatusForbidden)
		return
	}

	modelID, err := p.resolveModel(r.Context(), channelID, modelName)
	if err != nil {
		if err == errModelDisabled {
			http.Error(w, "model disabled", http.StatusForbidden)
			return
		}
		if err == errModelUnsupported {
			http.Error(w, "unsupported model", http.StatusBadRequest)
			return
		}
		zap.S().Errorf("Error catched: resolve model error: %v", err)
		http.Error(w, "model lookup failed", http.StatusInternalServerError)
		return
	}

	r = setContextInt32(r, ctxKeyID, keyID)
	r = setContextInt32(r, ctxChannelID, channelID)
	r = setContextString(r, ctxModel, modelName)
	r = setContextInt32(r, ctxModelID, modelID)
	r = setContextString(r, ctxAPIKey, providerKey)
	r = setContextBool(r, ctxStreamKey, parsedReq.Stream)
	r = r.WithContext(context.WithValue(r.Context(), ctxUpstream, target))
	p.proxy.ServeHTTP(w, r)
}
