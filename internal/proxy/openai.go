package proxy

import (
	"bytes"
	"compress/gzip"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"hyper-token/internal/repository"

	"github.com/andybalholm/brotli"
	"go.uber.org/zap"
)

type OpenAIProxy struct {
	proxy   *httputil.ReverseProxy
	queries *repository.Queries
}

type openaiChatRequest struct {
	Model  string `json:"model"`
	Stream bool   `json:"stream"`
}

type OpenAIUsage struct {
	Usage struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
}

const (
	modelStatusActive  int16 = 1
	modelStatusBanned  int16 = 2
	modelStatusPending int16 = 3
	modelTypeText      int32 = 1
)

func NewOpenAIProxy(queries *repository.Queries) *OpenAIProxy {
	p := &OpenAIProxy{
		queries: queries,
	}
	p.proxy = p.buildProxy()
	return p
}

func (p *OpenAIProxy) buildProxy() *httputil.ReverseProxy {
	director := func(req *http.Request) {
		model := getContextString(req, ctxModel)
		ctx := context.WithValue(req.Context(), ctxModel, model)
		ctx = context.WithValue(ctx, ctxProvider, string(ProviderOpenAI))
		*req = *req.WithContext(ctx)

		apiKey := getContextString(req, ctxAPIKey)
		req.Header.Set("Authorization", "Bearer "+apiKey)

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

func extractOpenAIModel(req *http.Request) string {
	if req.Body == nil {
		return "unknown"
	}

	body, err := io.ReadAll(req.Body)
	req.Body.Close()
	if err != nil {
		return "unknown"
	}

	req.Body = io.NopCloser(bytes.NewReader(body))
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(body)), nil
	}

	var chatReq openaiChatRequest
	if err := json.Unmarshal(body, &chatReq); err != nil {
		return "unknown"
	}

	if chatReq.Model == "" {
		return "unknown"
	}
	return chatReq.Model
}

func (p *OpenAIProxy) modifyResponse(resp *http.Response) error {
	keyID := getContextInt32(resp.Request, ctxKeyID)
	channelID := getContextInt32(resp.Request, ctxChannelID)
	model := getContextString(resp.Request, ctxModel)
	modelID := getContextInt32(resp.Request, ctxModelID)

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

	go p.processUsageAsync(rawBody, resp.Header.Get("Content-Encoding"), resp.Header.Get("Content-Type"), keyID, channelID, modelID, model)

	return nil
}

func (p *OpenAIProxy) processUsageAsync(rawBody []byte, contentEncoding string, contentType string, keyID int32, channelID int32, modelID int32, model string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	decodedBody, err := decodeResponseBody(rawBody, contentEncoding)
	if err != nil {
		zap.S().Errorf("Error catched: decode response body error: %v, content_type=%q, content_encoding=%q, raw_body_prefix=%q", err, contentType, contentEncoding, bodyPrefix(rawBody, 128))
		return
	}

	if modelID == 0 {
		row, err := p.queries.UpsertModel(ctx, repository.UpsertModelParams{
			ChannelID: channelID,
			ModelName: model,
			Status:    modelStatusPending,
			ModelType: modelTypeText,
		})
		if err != nil {
			zap.S().Errorf("Error catched: upsert fallback model error: %v", err)
			return
		}
		modelID = row.ID
	}

	err = processNonStreaming(ctx, decodedBody, keyID, channelID, modelID, ProviderOpenAI, p.queries)
	if err != nil {
		zap.S().Errorf("Error catched: process token usage error: %v, content_type=%q, content_encoding=%q, decoded_body_prefix=%q", err, contentType, contentEncoding, bodyPrefix(decodedBody, 128))
		return
	}
}

func decodeResponseBody(rawBody []byte, contentEncoding string) ([]byte, error) {
	encoding := strings.ToLower(strings.TrimSpace(contentEncoding))
	switch encoding {
	case "", "identity":
		return rawBody, nil
	case "gzip":
		reader, err := gzip.NewReader(bytes.NewReader(rawBody))
		if err != nil {
			return nil, err
		}
		decoded, readErr := io.ReadAll(reader)
		closeErr := reader.Close()
		if readErr != nil {
			return nil, readErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
		return decoded, nil
	case "br":
		return io.ReadAll(brotli.NewReader(bytes.NewReader(rawBody)))
	default:
		return nil, fmt.Errorf("unsupported content encoding: %s", contentEncoding)
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
	modelName := extractOpenAIModel(r)
	if modelName == "unknown" {
		http.Error(w, "missing model", http.StatusBadRequest)
		return
	}

	modelID, err := p.resolveModel(r.Context(), channelID, modelName)
	if err != nil {
		if err == errModelDisabled {
			http.Error(w, "model disabled", http.StatusForbidden)
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
	r = r.WithContext(context.WithValue(r.Context(), ctxUpstream, target))
	p.proxy.ServeHTTP(w, r)
}

var errModelDisabled = errors.New("model disabled")

func (p *OpenAIProxy) resolveModel(ctx context.Context, channelID int32, modelName string) (int32, error) {
	model, err := p.queries.GetModelByChannelAndName(ctx, repository.GetModelByChannelAndNameParams{
		ChannelID: channelID,
		ModelName: modelName,
	})
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	switch model.Status {
	case modelStatusActive:
		return model.ID, nil
	case modelStatusBanned:
		return 0, errModelDisabled
	default:
		return 0, nil
	}
}

func parseChannelBaseURL(baseURL string) (*url.URL, error) {
	target, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return nil, err
	}
	if target.Scheme == "" || target.Host == "" {
		return nil, url.InvalidHostError(baseURL)
	}
	if target.Path != "" && target.Path != "/" {
		return nil, url.InvalidHostError("base_url must not include path")
	}
	return target, nil
}
