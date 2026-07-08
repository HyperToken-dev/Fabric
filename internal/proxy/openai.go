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
	"sync"
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

// SSE reader.processing SSE stream
type openAIResponsesUsageTrackingReader struct {
	reader          io.ReadCloser
	parser          openAIResponsesSSEUsageParser
	compressedBody  bytes.Buffer
	contentEncoding string
	keyID           int32
	channelID       int32
	modelID         int32
	model           string
	queries         *repository.Queries
	once            sync.Once
}

// SSE usage unmarshal struct
type openAIResponsesUsage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

// SSE event of OpenAI
type openAIResponsesStreamEvent struct {
	Type     string                `json:"type"`
	Usage    *openAIResponsesUsage `json:"usage"`
	Response struct {
		Usage *openAIResponsesUsage `json:"usage"`
	} `json:"response"`
}

type openAIResponsesSSEUsageParser struct {
	lineBuf bytes.Buffer //save a line that not completely read yet
	event   string       //save the event name of the SSE event now
	data    bytes.Buffer //the json data of the SSE event
	usage   *usageLog    //finally usage of this SSE stream
}

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

// Get the model name of req
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

	// process SSE
	if resp.StatusCode >= http.StatusOK && resp.StatusCode < http.StatusMultipleChoices && isOpenAIResponsesStream(resp.Request, resp.Header.Get("Content-Type")) {
		resp.Body = newOpenAIResponsesUsageTrackingReader(resp.Body, resp.Header.Get("Content-Encoding"), keyID, channelID, modelID, model, p.queries)
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

	// async process can improve efficiency,make sure client get realtime response
	go p.processUsageAsync(rawBody, resp.Header.Get("Content-Encoding"), resp.Header.Get("Content-Type"), keyID, channelID, modelID, model)

	return nil
}

// determine whether the data is SSE stram
func isOpenAIResponsesStream(req *http.Request, contentType string) bool {
	return strings.Contains(req.URL.Path, "/v1/responses") && strings.Contains(strings.ToLower(contentType), "text/event-stream")
}

// get a new reader in order to process SSE stream
// httputil.ReverseProxy will automatically call the Read function of this struct when copy upstream response body to client
func newOpenAIResponsesUsageTrackingReader(r io.ReadCloser, contentEncoding string, keyID, channelID, modelID int32, model string, queries *repository.Queries) *openAIResponsesUsageTrackingReader {
	return &openAIResponsesUsageTrackingReader{
		reader:          r,
		contentEncoding: strings.ToLower(strings.TrimSpace(contentEncoding)),
		keyID:           keyID,
		channelID:       channelID,
		modelID:         modelID,
		model:           model,
		queries:         queries,
	}
}

func (r *openAIResponsesUsageTrackingReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if n > 0 { //when read something, write it to parser
		switch r.contentEncoding {
		case "", "identity":
			if parseErr := r.parser.Write(p[:n]); parseErr != nil {
				//even if error occurred just log,make sure client get the p data
				zap.S().Errorf("Error catched: parse responses stream usage error: %v", parseErr)
			}
		case "gzip":
			if _, writeErr := r.compressedBody.Write(p[:n]); writeErr != nil {
				zap.S().Errorf("Error catched: buffer gzip responses stream usage data error: %v", writeErr)
			}
		}
	}
	// when SSE stream ends,io.EOF will be readed out.
	if err == io.EOF {
		// sync.Once make sure always call processUsage once
		// processUsage will call Finish Function to resolve the SSE stream dont return "\n\n" characters finally
		r.once.Do(r.processUsage)
	}
	return n, err
}

// close will be called by httputil.ReverseProxy
func (r *openAIResponsesUsageTrackingReader) Close() error {
	return r.reader.Close()
}

// when SSE ends this function will be called
func (r *openAIResponsesUsageTrackingReader) processUsage() {
	if r.contentEncoding == "gzip" {
		compressedBody := append([]byte(nil), r.compressedBody.Bytes()...)
		go r.processGzipUsage(compressedBody)
		return
	}
	if r.contentEncoding != "" && r.contentEncoding != "identity" {
		zap.S().Warnf("unsupported responses stream content encoding: %s", r.contentEncoding)
		return
	}

	// call Finish function when SSE ends
	if err := r.parser.Finish(); err != nil {
		zap.S().Errorf("Error catched: finish responses stream usage parser error: %v", err)
	}
	// process usage data and log it into database
	usage := r.parser.Usage()
	r.insertParsedUsage(usage)
}

// gzip SSE process
func (r *openAIResponsesUsageTrackingReader) processGzipUsage(compressedBody []byte) {
	reader, err := gzip.NewReader(bytes.NewReader(compressedBody))
	if err != nil {
		zap.S().Errorf("Error catched: create gzip responses stream usage reader error: %v", err)
		return
	}

	var parser openAIResponsesSSEUsageParser
	buf := make([]byte, 32*1024)
	for {
		n, readErr := reader.Read(buf)
		if n > 0 {
			if parseErr := parser.Write(buf[:n]); parseErr != nil {
				zap.S().Errorf("Error catched: parse gzip responses stream usage error: %v", parseErr)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			zap.S().Errorf("Error catched: read gzip responses stream usage data error: %v", readErr)
			break
		}
	}
	closeErr := reader.Close()
	if closeErr != nil {
		zap.S().Errorf("Error catched: close gzip responses stream usage reader error: %v", closeErr)
	}
	if err := parser.Finish(); err != nil {
		zap.S().Errorf("Error catched: finish gzip responses stream usage parser error: %v", err)
	}
	r.insertParsedUsage(parser.Usage())
}

func (r *openAIResponsesUsageTrackingReader) insertParsedUsage(usage *usageLog) {
	if usage == nil {
		zap.S().Warnf("responses stream usage missing: key_id=%d, channel_id=%d, model_id=%d", r.keyID, r.channelID, r.modelID)
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if r.modelID == 0 {
		// fallback create model when model does not exists
		row, err := r.queries.UpsertModel(ctx, repository.UpsertModelParams{
			ChannelID: r.channelID,
			ModelName: r.model,
			Status:    modelStatusPending,
			ModelType: modelTypeText,
		})
		if err != nil {
			zap.S().Errorf("Error catched: upsert responses stream fallback model error: %v", err)
			return
		}
		r.modelID = row.ID
	}
	if err := insertUsageLog(ctx, r.queries, r.keyID, r.channelID, r.modelID, usage); err != nil {
		zap.S().Errorf("Error catched: insert responses stream usage log error: %v", err)
	}
}

// The most important function of the SSE parser
// When a line has not yet ended --> read it into LineBuffer
// when data line has ended --> reset LineBuffer and call consumeLine function
func (p *openAIResponsesSSEUsageParser) Write(chunk []byte) error {
	// a SSE event may contains multiple errors across several lines.
	// this function can't return all of the error
	/*
		The parser merely tracks usage statistics on the side;
		the failure to parse a specific intermediate event should not compromise
		the opportunity to parse subsequent events.
	*/
	var firstErr error // just return the first error in one line

	for len(chunk) > 0 {
		// the line has not ended
		idx := bytes.IndexByte(chunk, '\n')
		if idx < 0 {
			_, err := p.lineBuf.Write(chunk)
			return err
		}

		// the line has ended
		if _, err := p.lineBuf.Write(chunk[:idx]); err != nil && firstErr == nil {
			firstErr = err
		}
		line := p.lineBuf.String()
		p.lineBuf.Reset()
		if err := p.consumeLine(strings.TrimSuffix(line, "\r")); err != nil && firstErr == nil {
			firstErr = err
		}
		chunk = chunk[idx+1:]
	}
	return firstErr
}

// when SSE ends call this function(called by processUsage)
// this function that resolve the problem that SSE not ended with "\n\n" or a empty line
func (p *openAIResponsesSSEUsageParser) Finish() error {
	var firstErr error
	if p.lineBuf.Len() > 0 {
		line := p.lineBuf.String()
		p.lineBuf.Reset()
		if err := p.consumeLine(strings.TrimSuffix(line, "\r")); err != nil {
			firstErr = err
		}
	}
	if p.data.Len() > 0 || p.event != "" {
		// data exists then call consumeEvent
		if err := p.consumeEvent(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

// return usage data of the SSE
func (p *openAIResponsesSSEUsageParser) Usage() *usageLog {
	return p.usage
}

// when line ends this function will be called
func (p *openAIResponsesSSEUsageParser) consumeLine(line string) error {
	// when read a empty line means that this SSE event has ended
	if line == "" {
		return p.consumeEvent()
	}

	// a line begin with ':' which is just a comment line
	// a simple example:
	// : this is a comment
	if strings.HasPrefix(line, ":") {
		return nil
	}

	field, value, found := strings.Cut(line, ":")
	if !found {
		field = line
		value = ""
	} else if strings.HasPrefix(value, " ") {
		value = strings.TrimPrefix(value, " ")
	}

	switch field {
	case "event":
		p.event = value
	case "data":
		// judge whether the p.data is exists
		if p.data.Len() > 0 {
			// json format when a line ends there will be a '\n'
			if err := p.data.WriteByte('\n'); err != nil {
				return err
			}
		}
		// then write the new data line to p.data
		_, err := p.data.WriteString(value)
		return err
	}
	return nil
}

// when SSE ends this function will be called
func (p *openAIResponsesSSEUsageParser) consumeEvent() error {
	data := strings.TrimSpace(p.data.String())
	event := p.event
	p.event = ""
	p.data.Reset()

	if data == "" || data == "[DONE]" {
		return nil
	}

	var streamEvent openAIResponsesStreamEvent
	if err := json.Unmarshal([]byte(data), &streamEvent); err != nil {
		return err
	}
	if event != "response.completed" && streamEvent.Type != "response.completed" {
		return nil
	}

	usage := streamEvent.Response.Usage
	if usage == nil {
		// fallback when response field does not exists
		usage = streamEvent.Usage
	}
	if usage == nil {
		return nil
	}

	// log it into SSE's usage logging field
	p.usage = &usageLog{
		PromptTokens:   usage.InputTokens,
		CompleteTokens: usage.OutputTokens,
	}
	return nil
}

// async usage processing
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
