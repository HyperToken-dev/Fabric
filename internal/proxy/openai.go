package proxy

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"

	"hyper-token/internal/repository"

	"go.uber.org/zap"
)

type OpenAIProxy struct {
	proxy       *httputil.ReverseProxy
	upstreamURL string
	apiKey      string
	queries     *repository.Queries
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

func NewOpenAIProxy(upstreamURL, apiKey string, queries *repository.Queries) *OpenAIProxy {
	p := &OpenAIProxy{
		upstreamURL: upstreamURL,
		apiKey:      apiKey,
		queries:     queries,
	}
	p.proxy = p.buildProxy()
	return p
}

func (p *OpenAIProxy) buildProxy() *httputil.ReverseProxy {
	target, _ := url.Parse(p.upstreamURL)

	director := func(req *http.Request) {
		model := extractOpenAIModel(req)
		req = setContextString(req, ctxModel, model)
		req = setContextString(req, ctxProvider, string(ProviderOpenAI))

		req.Header.Set("Authorization", "Bearer "+p.apiKey)

		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.Host = target.Host
	}

	return &httputil.ReverseProxy{
		Director:       director,
		ModifyResponse: p.modifyResponse,
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

	// Upsert model
	m, err := p.queries.UpsertModel(resp.Request.Context(), repository.UpsertModelParams{
		ChannelID: channelID,
		ModelName: model,
	})

	var modelID int32
	switch err {
	case nil:
		modelID = m.ID
	case sql.ErrNoRows:
		row, err2 := p.queries.GetModelByChannelAndName(resp.Request.Context(), repository.GetModelByChannelAndNameParams{
			ChannelID: channelID,
			ModelName: model,
		})
		if err2 != nil {
			return nil
		}
		modelID = row.ID
	default:
		zap.S().Errorf("Error catched: %v", err)
		return nil
	}

	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	// all error in this function shouldn't return an error to client
	// just log
	if err != nil {
		zap.S().Errorf("Error catched: upstream error: %v", err)
		return nil
	}

	err = processNonStreaming(body, keyID, channelID, modelID, ProviderOpenAI, p.queries)
	if err != nil {
		zap.S().Errorf("Error catched: process token usage error: %v", err)
		return nil
	}
	resp.Body = io.NopCloser(bytes.NewReader(body))
	resp.ContentLength = int64(len(body))

	return nil
}

func (p *OpenAIProxy) ServeHTTP(w http.ResponseWriter, r *http.Request, keyID int32, channelID int32) {
	r = setContextInt32(r, ctxKeyID, keyID)
	r = setContextInt32(r, ctxChannelID, channelID)
	p.proxy.ServeHTTP(w, r)
}
