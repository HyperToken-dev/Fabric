package main

import (
	"bytes"
	"context"
	"encoding/json"
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
		ModifyResponse: p.modifyResponse,
	})
	p.coreProxy = coreProxy
	return p, nil
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
		contentEncoding := resp.Header.Get("Content-Encoding")
		resp.Body = p.usageHandler.WrapStreamingResponse(resp.Request, resp.Body, contentEncoding, UsageContext{
			KeyID:     keyID,
			ChannelID: channelID,
			ModelID:   modelID,
			Model:     model,
		}, func(responseBody []byte) {
			go p.processIntegralLogAsync(resp.Request, responseBody, keyID, channelID, modelID, model)
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
	integralResponseBody := decodedBody
	if err != nil {
		zap.L().Error("decode response body for output detection failed", zap.Error(err), zap.String("content_type", contentType), zap.String("content_encoding", contentEncoding), zap.String("raw_body_prefix", bodyPrefix(rawBody, 128)))
		integralResponseBody = rawBody
	} else {
		outputTexts, err := sensitiveopenai.ExtractOutputTexts(resp.Request, decodedBody)
		if err != nil {
			zap.L().Error("extract openai output texts failed", zap.Error(err), zap.String("content_type", contentType), zap.String("decoded_body_prefix", bodyPrefix(decodedBody, 128)))
		} else if detectPrompts(resp.Request.Context(), model, TextDirectionOutput, outputTexts, p.textPolicy) {
			integralResponseBody = rejectOpenAIOutputResponse(resp)
		}
	}

	// async process can improve efficiency,make sure client get realtime response
	go p.processUsageAsync(resp.Request, rawBody, contentEncoding, contentType, keyID, channelID, modelID, model)
	go p.processIntegralLogAsync(resp.Request, integralResponseBody, keyID, channelID, modelID, model)

	return nil
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

func bodyPrefix(body []byte, maxLen int) string {
	if len(body) > maxLen {
		body = body[:maxLen]
	}
	return string(body)
}

func (p *OpenAIProxy) processIntegralLogAsync(req *http.Request, responseBody []byte, keyID int32, channelID int32, modelID int32, model string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	contextJSON := integralLogContext(req, keyID, channelID, modelID, model)
	responseText := string(responseBody)

	if err := p.integralLogs.InsertIntegralLog(ctx, keyID, contextJSON, responseText); err != nil {
		zap.L().Error("insert integral log failed", zap.Error(err), zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.Int32("model_id", modelID), zap.String("model", model))
		return
	}
	zap.L().Info("integral log inserted", zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.Int32("model_id", modelID), zap.String("model", model), zap.Int("response_bytes", len(responseBody)))
}

func integralLogContext(req *http.Request, keyID int32, channelID int32, modelID int32, model string) string {
	entry := struct {
		Method    string          `json:"method"`
		Path      string          `json:"path"`
		Model     string          `json:"model"`
		ModelID   int32           `json:"model_id"`
		ChannelID int32           `json:"channel_id"`
		Request   json.RawMessage `json:"request"`
	}{
		Model:     model,
		ModelID:   modelID,
		ChannelID: channelID,
		Request:   json.RawMessage(`null`),
	}
	if req != nil {
		entry.Method = req.Method
		if req.URL != nil {
			entry.Path = req.URL.Path
		}
	}

	if req != nil && req.GetBody != nil {
		body, err := req.GetBody()
		if err != nil {
			zap.L().Error("get request body for integral log failed", zap.Error(err), zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.Int32("model_id", modelID), zap.String("model", model))
		} else {
			raw, readErr := io.ReadAll(body)
			closeErr := body.Close()
			if readErr != nil {
				zap.L().Error("read request body for integral log failed", zap.Error(readErr), zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.Int32("model_id", modelID), zap.String("model", model))
			} else if json.Valid(raw) {
				entry.Request = append(json.RawMessage(nil), raw...)
			} else {
				zap.L().Error("request body for integral log is not valid JSON", zap.String("raw_body_prefix", bodyPrefix(raw, 128)), zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.Int32("model_id", modelID), zap.String("model", model))
			}
			if closeErr != nil {
				zap.L().Error("close request body for integral log failed", zap.Error(closeErr), zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.Int32("model_id", modelID), zap.String("model", model))
			}
		}
	}

	encoded, err := json.Marshal(entry)
	if err != nil {
		zap.L().Error("marshal integral log context failed", zap.Error(err), zap.Int32("key_id", keyID), zap.Int32("channel_id", channelID), zap.Int32("model_id", modelID), zap.String("model", model))
		return `{"request":null}`
	}
	return string(encoded)
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
	p.coreProxy.ServeHTTP(w, r, coreproxy.Upstream{BaseURL: baseURL, APIKey: providerKey})
}
