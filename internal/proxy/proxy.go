package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync"

	"hyper-token/internal/repository"
)

type Provider string

const (
	ProviderOpenAI Provider = "openai"
)

type contextKey string

const (
	ctxKeyID     contextKey = "key_id"
	ctxChannelID contextKey = "channel_id"
	ctxModel     contextKey = "model"
	ctxModelID   contextKey = "model_id"
	ctxProvider  contextKey = "provider"
)

type usageTrackingReader struct {
	reader    io.ReadCloser
	buf       bytes.Buffer
	keyID     int32
	channelID int32
	modelID   int32
	provider  Provider
	queries   *repository.Queries
	once      sync.Once
}

type usageLog struct {
	PromptTokens   int `json:"prompt_tokens"`
	CompleteTokens int `json:"complete_tokens"`
}

func newUsageTrackingReader(r io.ReadCloser, keyID, channelID, modelID int32, provider Provider, queries *repository.Queries) *usageTrackingReader {
	return &usageTrackingReader{
		reader:    r,
		keyID:     keyID,
		channelID: channelID,
		modelID:   modelID,
		provider:  provider,
		queries:   queries,
	}
}

func (r *usageTrackingReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if n > 0 {
		r.buf.Write(p[:n])
	}
	if err == io.EOF {
		r.once.Do(r.processUsage)
	}
	return n, err
}

func (r *usageTrackingReader) Close() error {
	return r.reader.Close()
}

func (r *usageTrackingReader) processUsage() {
	_, _ = r.queries.InsertUsageLog(context.Background(), repository.InsertUsageLogParams{
		KeyID:            r.keyID,
		ChannelID:        r.channelID,
		ModelID:          r.modelID,
		PromptTokens:     0,
		CompletionTokens: 0,
	})
}

func setContextInt32(r *http.Request, key contextKey, val int32) *http.Request {
	ctx := context.WithValue(r.Context(), key, val)
	return r.WithContext(ctx)
}

func setContextString(r *http.Request, key contextKey, val string) *http.Request {
	ctx := context.WithValue(r.Context(), key, val)
	return r.WithContext(ctx)
}

func getContextInt32(r *http.Request, key contextKey) int32 {
	v, _ := r.Context().Value(key).(int32)
	return v
}

func getContextString(r *http.Request, key contextKey) string {
	v, _ := r.Context().Value(key).(string)
	return v
}

func Route(r *http.Request) Provider {
	path := r.URL.Path

	if isOpenAIPath(path) {
		return ProviderOpenAI
	}
	return ""
}

func isOpenAIPath(path string) bool {
	return strings.HasPrefix(path, "/v1/")
}

func processNonStreaming(body []byte, keyID, channelID, modelID int32, provider Provider, queries *repository.Queries) error {
	usagelog, err := extractTokenUsage(body, provider)
	if err != nil {
		return err
	}
	_, _ = queries.InsertUsageLog(context.Background(), repository.InsertUsageLogParams{
		KeyID:            keyID,
		ChannelID:        channelID,
		ModelID:          modelID,
		PromptTokens:     int64(usagelog.PromptTokens),
		CompletionTokens: int64(usagelog.CompleteTokens),
	})
	return nil
}

func extractTokenUsage(body []byte, provider Provider) (*usageLog, error) {
	var usagelog *usageLog = new(usageLog)
	switch provider {
	case ProviderOpenAI:
		openaiUsage, err := extractOpenAITokenUsage(body)
		if err != nil {
			return nil, err
		}
		usagelog.PromptTokens = openaiUsage.Usage.InputTokens
		usagelog.CompleteTokens = openaiUsage.Usage.OutputTokens
		return usagelog, nil
	default:
		return nil, errors.New("Not supported provider.")
	}
}

func extractOpenAITokenUsage(body []byte) (*OpenAIUsage, error) {
	var openaiUsage *OpenAIUsage = new(OpenAIUsage)
	err := json.Unmarshal(body, &openaiUsage)
	if err != nil {
		return nil, err
	}
	return openaiUsage, nil
}
