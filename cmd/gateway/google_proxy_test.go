package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestGoogleProxyInteractionsForwardsWithProviderAuth(t *testing.T) {
	integralLogs := &recordingIntegralLogHandler{ch: make(chan recordedIntegralLog, 1)}
	usageHandler := &recordingGoogleUsageHandler{ch: make(chan googleUsageRecord, 1)}
	proxy, err := NewGoogleProxy(GoogleProxyOptions{
		ModelStore:         fakeModelStore{model: &ModelInfo{ID: 42, Status: ModelStatusActive}},
		UsageHandler:       usageHandler,
		IntegralLogHandler: integralLogs,
	})
	if err != nil {
		t.Fatal(err)
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != "/v1/interactions" {
			t.Errorf("path = %s, want /v1/interactions", r.URL.Path)
		}
		if got := r.Header.Get("x-goog-api-key"); got != "provider-key" {
			t.Errorf("x-goog-api-key = %q, want provider key", got)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(body), `"model":"gemini-3-flash-preview"`) {
			t.Fatalf("forwarded body = %s", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"object":"interaction","model":"gemini-3-flash-preview","usage":{"total_input_tokens":7,"total_output_tokens":20,"total_thought_tokens":22,"total_tokens":49,"total_tool_use_tokens":0,"total_cached_tokens":0,"input_tokens_by_modality":[{"modality":"text","tokens":7}]}}`))
	}))
	defer upstream.Close()

	req := httptest.NewRequest(http.MethodPost, "/interactions", strings.NewReader(`{"model":"gemini-3-flash-preview","input":[{"type":"user_input","content":[{"type":"text","text":"hello"}]}]}`))
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req, 10, 20, upstream.URL+"/v1", "provider-key")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%q", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"object":"interaction"`) {
		t.Fatalf("response body = %q, want upstream response", rec.Body.String())
	}

	usage := receiveGoogleUsageRecord(t, usageHandler.ch)
	if usage.info.KeyID != 10 || usage.info.ChannelID != 20 || usage.info.ModelID != 42 || usage.info.Model != "gemini-3-flash-preview" {
		t.Fatalf("usage context = %+v", usage.info)
	}
	if !strings.Contains(string(usage.rawBody), `"total_input_tokens":7`) {
		t.Fatalf("usage raw body = %s", usage.rawBody)
	}

	got := receiveIntegralLog(t, integralLogs.ch)
	if got.keyID != 10 {
		t.Fatalf("keyID = %d, want 10", got.keyID)
	}
	var loggedContext struct {
		Provider  string `json:"provider"`
		APIFormat int32  `json:"api_format"`
		Outcome   string `json:"outcome"`
		Model     string `json:"model"`
		ModelID   int32  `json:"model_id"`
		ChannelID int32  `json:"channel_id"`
	}
	if err := json.Unmarshal([]byte(got.context), &loggedContext); err != nil {
		t.Fatal(err)
	}
	if loggedContext.Provider != "google" || loggedContext.APIFormat != 4 || loggedContext.Outcome != "ok" || loggedContext.Model != "gemini-3-flash-preview" || loggedContext.ModelID != 42 || loggedContext.ChannelID != 20 {
		t.Fatalf("context = %+v", loggedContext)
	}
}

func TestGoogleProxyInteractionsProcessesStreamingUsage(t *testing.T) {
	integralLogs := &recordingIntegralLogHandler{ch: make(chan recordedIntegralLog, 1)}
	usageHandler := &recordingGoogleUsageHandler{streamingCh: make(chan googleUsageRecord, 1)}
	proxy, err := NewGoogleProxy(GoogleProxyOptions{
		ModelStore:         fakeModelStore{model: &ModelInfo{ID: 42, Status: ModelStatusActive}},
		UsageHandler:       usageHandler,
		IntegralLogHandler: integralLogs,
	})
	if err != nil {
		t.Fatal(err)
	}

	streamBody := "event: interaction.completed\n" +
		"data:{\"interaction\":{\"usage\":{\"total_tokens\":64,\"total_input_tokens\":22,\"total_output_tokens\":8,\"total_thought_tokens\":34,\"total_tool_use_tokens\":0,\"total_cached_tokens\":0,\"input_tokens_by_modality\":[{\"modality\":\"text\",\"tokens\":22}]}},\"event_type\":\"interaction.completed\"}\n\n" +
		"data: [DONE]\n\n"
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(streamBody))
	}))
	defer upstream.Close()

	req := httptest.NewRequest(http.MethodPost, "/interactions", strings.NewReader(`{"model":"gemini-3-flash-preview","input":[{"type":"user_input","content":[{"type":"text","text":"hello"}]}]}`))
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req, 10, 20, upstream.URL, "provider-key")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%q", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != streamBody {
		t.Fatalf("response body = %q, want stream body", rec.Body.String())
	}

	usage := receiveGoogleUsageRecord(t, usageHandler.streamingCh)
	if usage.info.KeyID != 10 || usage.info.ChannelID != 20 || usage.info.ModelID != 42 || usage.info.Model != "gemini-3-flash-preview" {
		t.Fatalf("usage context = %+v", usage.info)
	}
	if string(usage.rawBody) != streamBody {
		t.Fatalf("usage raw body = %q", usage.rawBody)
	}

	got := receiveIntegralLog(t, integralLogs.ch)
	if got.response != streamBody {
		t.Fatalf("integral response = %q, want stream body", got.response)
	}
}

func TestGoogleProxyInteractionsNormalizesUpstreamPath(t *testing.T) {
	tests := []struct {
		name        string
		basePrefix  string
		requestPath string
		wantPath    string
	}{
		{name: "base without prefix request with prefix", requestPath: "/v1beta/interactions", wantPath: "/v1beta/interactions"},
		{name: "base with prefix request without prefix", basePrefix: "/v1beta", requestPath: "/interactions", wantPath: "/v1beta/interactions"},
		{name: "base with prefix request with same prefix", basePrefix: "/v1beta", requestPath: "/v1beta/interactions", wantPath: "/v1beta/interactions"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proxy, err := NewGoogleProxy(GoogleProxyOptions{ModelStore: fakeModelStore{model: &ModelInfo{ID: 42, Status: ModelStatusActive}}})
			if err != nil {
				t.Fatal(err)
			}
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != tt.wantPath {
					t.Errorf("path = %s, want %s", r.URL.Path, tt.wantPath)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"object":"interaction","usage":{"total_input_tokens":1,"total_output_tokens":1}}`))
			}))
			defer upstream.Close()

			req := httptest.NewRequest(http.MethodPost, tt.requestPath, strings.NewReader(`{"model":"gemini-3-flash-preview","input":[{"type":"user_input","content":[{"type":"text","text":"hello"}]}]}`))
			rec := httptest.NewRecorder()
			proxy.ServeHTTP(rec, req, 10, 20, upstream.URL+tt.basePrefix, "provider-key")
			if rec.Code != http.StatusOK {
				t.Fatalf("status = %d, body=%q", rec.Code, rec.Body.String())
			}
		})
	}
}

func TestGoogleProxyInteractionsValidation(t *testing.T) {
	tests := []struct {
		name        string
		providerKey string
		method      string
		path        string
		body        string
		store       fakeModelStore
		wantStatus  int
	}{
		{name: "missing provider key", providerKey: " ", method: http.MethodPost, path: "/interactions", body: `{"model":"gemini-3-flash-preview"}`, store: fakeModelStore{model: &ModelInfo{ID: 1, Status: ModelStatusActive}}, wantStatus: http.StatusBadGateway},
		{name: "unsupported path", providerKey: "provider-key", method: http.MethodPost, path: "/models", body: `{"model":"gemini-3-flash-preview"}`, store: fakeModelStore{err: errors.New("should not resolve model")}, wantStatus: http.StatusNotFound},
		{name: "unsupported method", providerKey: "provider-key", method: http.MethodGet, path: "/interactions", body: `{"model":"gemini-3-flash-preview"}`, store: fakeModelStore{err: errors.New("should not resolve model")}, wantStatus: http.StatusNotFound},
		{name: "invalid json", providerKey: "provider-key", method: http.MethodPost, path: "/interactions", body: `{`, store: fakeModelStore{model: &ModelInfo{ID: 1, Status: ModelStatusActive}}, wantStatus: http.StatusBadRequest},
		{name: "missing model", providerKey: "provider-key", method: http.MethodPost, path: "/interactions", body: `{"input":"hello"}`, store: fakeModelStore{model: &ModelInfo{ID: 1, Status: ModelStatusActive}}, wantStatus: http.StatusBadRequest},
		{name: "unsupported model", providerKey: "provider-key", method: http.MethodPost, path: "/interactions", body: `{"model":"gemini-3-flash-preview"}`, store: fakeModelStore{}, wantStatus: http.StatusBadRequest},
		{name: "banned model", providerKey: "provider-key", method: http.MethodPost, path: "/interactions", body: `{"model":"gemini-3-flash-preview"}`, store: fakeModelStore{model: &ModelInfo{ID: 1, Status: ModelStatusBanned}}, wantStatus: http.StatusForbidden},
		{name: "model lookup error", providerKey: "provider-key", method: http.MethodPost, path: "/interactions", body: `{"model":"gemini-3-flash-preview"}`, store: fakeModelStore{err: errors.New("lookup failed")}, wantStatus: http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proxy, err := NewGoogleProxy(GoogleProxyOptions{ModelStore: tt.store, IntegralLogHandler: &recordingIntegralLogHandler{ch: make(chan recordedIntegralLog, 1)}})
			if err != nil {
				t.Fatal(err)
			}
			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			rec := httptest.NewRecorder()
			proxy.ServeHTTP(rec, req, 10, 20, "https://generativelanguage.googleapis.com", tt.providerKey)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d, body=%q", rec.Code, tt.wantStatus, rec.Body.String())
			}
		})
	}
}

func TestIsGoogleInteractionsRequest(t *testing.T) {
	tests := []struct {
		name    string
		method  string
		baseURL string
		path    string
		want    bool
	}{
		{name: "root base with versioned request path", method: http.MethodPost, baseURL: "https://example.com", path: "/v1beta/interactions", want: true},
		{name: "versioned base with short request path", method: http.MethodPost, baseURL: "https://example.com/v1beta", path: "/interactions", want: true},
		{name: "versioned base with versioned request path", method: http.MethodPost, baseURL: "https://example.com/v1beta", path: "/v1beta/interactions", want: true},
		{name: "v1 path rejected", method: http.MethodPost, baseURL: "https://example.com", path: "/v1/interactions", want: false},
		{name: "arbitrary suffix path rejected", method: http.MethodPost, baseURL: "https://example.com", path: "/foo/interactions", want: false},
		{name: "unsupported method rejected", method: http.MethodGet, baseURL: "https://example.com", path: "/v1beta/interactions", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, nil)
			if got := isGoogleInteractionsRequest(req, tt.baseURL); got != tt.want {
				t.Fatalf("isGoogleInteractionsRequest(%s, %s, %s) = %v, want %v", tt.method, tt.baseURL, tt.path, got, tt.want)
			}
		})
	}
}

func TestGoogleProxyInvalidRequestBodyRecordsIntegralLogAsInvalidRequest(t *testing.T) {
	integralLogs := &recordingIntegralLogHandler{ch: make(chan recordedIntegralLog, 1)}
	proxy, err := NewGoogleProxy(GoogleProxyOptions{ModelStore: fakeModelStore{model: &ModelInfo{ID: 42, Status: ModelStatusActive}}, IntegralLogHandler: integralLogs})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/interactions", strings.NewReader(`{`))
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req, 10, 20, "https://generativelanguage.googleapis.com", "provider-key")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%q", rec.Code, rec.Body.String())
	}

	got := receiveIntegralLog(t, integralLogs.ch)
	var loggedContext struct {
		Provider        string `json:"provider"`
		Outcome         string `json:"outcome"`
		RejectionStage  string `json:"rejection_stage"`
		RejectionReason string `json:"rejection_reason"`
		ResponseStatus  int    `json:"response_status"`
	}
	if err := json.Unmarshal([]byte(got.context), &loggedContext); err != nil {
		t.Fatal(err)
	}
	if loggedContext.Provider != "google" || loggedContext.Outcome != "rejected" || loggedContext.RejectionStage != "input" || loggedContext.RejectionReason != "invalid_request" || loggedContext.ResponseStatus != http.StatusBadRequest {
		t.Fatalf("context = %+v", loggedContext)
	}
}

func TestGoogleProxyModelRejectionRecordsIntegralLog(t *testing.T) {
	integralLogs := &recordingIntegralLogHandler{ch: make(chan recordedIntegralLog, 1)}
	proxy, err := NewGoogleProxy(GoogleProxyOptions{ModelStore: fakeModelStore{}, IntegralLogHandler: integralLogs})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/interactions", strings.NewReader(`{"model":"gemini-3-flash-preview"}`))
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req, 10, 20, "https://generativelanguage.googleapis.com", "provider-key")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, body=%q", rec.Code, rec.Body.String())
	}

	got := receiveIntegralLog(t, integralLogs.ch)
	var loggedContext struct {
		Provider        string `json:"provider"`
		Outcome         string `json:"outcome"`
		RejectionReason string `json:"rejection_reason"`
		Model           string `json:"model"`
	}
	if err := json.Unmarshal([]byte(got.context), &loggedContext); err != nil {
		t.Fatal(err)
	}
	if loggedContext.Provider != "google" || loggedContext.Outcome != "rejected" || loggedContext.RejectionReason != "model" || loggedContext.Model != "gemini-3-flash-preview" {
		t.Fatalf("context = %+v", loggedContext)
	}
}

func TestGoogleProxyModelLookupErrorRecordsIntegralLogAsError(t *testing.T) {
	integralLogs := &recordingIntegralLogHandler{ch: make(chan recordedIntegralLog, 1)}
	proxy, err := NewGoogleProxy(GoogleProxyOptions{ModelStore: fakeModelStore{err: errors.New("lookup failed")}, IntegralLogHandler: integralLogs})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/interactions", strings.NewReader(`{"model":"gemini-3-flash-preview"}`))
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req, 10, 20, "https://generativelanguage.googleapis.com", "provider-key")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, body=%q", rec.Code, rec.Body.String())
	}

	got := receiveIntegralLog(t, integralLogs.ch)
	var loggedContext struct {
		Provider        string `json:"provider"`
		Outcome         string `json:"outcome"`
		RejectionStage  string `json:"rejection_stage"`
		RejectionReason string `json:"rejection_reason"`
		ResponseStatus  int    `json:"response_status"`
	}
	if err := json.Unmarshal([]byte(got.context), &loggedContext); err != nil {
		t.Fatal(err)
	}
	if loggedContext.Provider != "google" || loggedContext.Outcome != "error" || loggedContext.ResponseStatus != http.StatusInternalServerError {
		t.Fatalf("context = %+v", loggedContext)
	}
	if loggedContext.RejectionStage != "" || loggedContext.RejectionReason != "" {
		t.Fatalf("rejection fields = %q/%q, want empty", loggedContext.RejectionStage, loggedContext.RejectionReason)
	}
}

func TestGoogleProxySensitiveInputRejected(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "user input text",
			body: `{"model":"gemini-3-flash-preview","input":[{"type":"user_input","content":[{"type":"text","text":"blocked"}]}]}`,
		},
		{
			name: "model output text",
			body: `{"model":"gemini-3-flash-preview","input":[{"type":"model_output","content":[{"type":"text","text":"blocked"}]}]}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			integralLogs := &recordingIntegralLogHandler{ch: make(chan recordedIntegralLog, 1)}
			proxy, err := NewGoogleProxy(GoogleProxyOptions{
				ModelStore:         fakeModelStore{model: &ModelInfo{ID: 42, Status: ModelStatusActive}},
				TextPolicy:         rejectingPolicy{},
				IntegralLogHandler: integralLogs,
			})
			if err != nil {
				t.Fatal(err)
			}

			upstreamReached := false
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				upstreamReached = true
				w.WriteHeader(http.StatusOK)
			}))
			defer upstream.Close()

			req := httptest.NewRequest(http.MethodPost, "/interactions", strings.NewReader(tt.body))
			rec := httptest.NewRecorder()
			proxy.ServeHTTP(rec, req, 10, 20, upstream.URL, "provider-key")
			if rec.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want %d, body=%q", rec.Code, http.StatusForbidden, rec.Body.String())
			}
			if upstreamReached {
				t.Fatal("upstream reached for rejected prompt")
			}

			got := receiveIntegralLog(t, integralLogs.ch)
			var loggedContext struct {
				Provider        string `json:"provider"`
				Outcome         string `json:"outcome"`
				RejectionStage  string `json:"rejection_stage"`
				RejectionReason string `json:"rejection_reason"`
				Model           string `json:"model"`
				ModelID         int32  `json:"model_id"`
			}
			if err := json.Unmarshal([]byte(got.context), &loggedContext); err != nil {
				t.Fatal(err)
			}
			if loggedContext.Provider != "google" || loggedContext.Outcome != "rejected" || loggedContext.RejectionStage != "input" || loggedContext.RejectionReason != "sensitive" || loggedContext.Model != "gemini-3-flash-preview" || loggedContext.ModelID != 42 {
				t.Fatalf("context = %+v", loggedContext)
			}
		})
	}
}

func TestGoogleEffectivePath(t *testing.T) {
	tests := []struct {
		baseURL     string
		requestPath string
		want        string
	}{
		{baseURL: "https://example.com", requestPath: "/v1beta/interactions", want: "/v1beta/interactions"},
		{baseURL: "https://example.com/v1beta", requestPath: "/interactions", want: "/v1beta/interactions"},
		{baseURL: "https://example.com/v1beta/", requestPath: "interactions", want: "/v1beta/interactions"},
	}
	for _, tt := range tests {
		if got := googleEffectivePath(tt.baseURL, tt.requestPath); got != tt.want {
			t.Fatalf("googleEffectivePath(%q, %q) = %q, want %q", tt.baseURL, tt.requestPath, got, tt.want)
		}
	}
}

type googleUsageRecord struct {
	rawBody []byte
	info    UsageContext
}

type recordingGoogleUsageHandler struct {
	ch          chan googleUsageRecord
	streamingCh chan googleUsageRecord
}

func (h *recordingGoogleUsageHandler) ProcessInteractionResponse(ctx context.Context, rawBody []byte, info UsageContext) error {
	if h.ch == nil {
		h.ch = make(chan googleUsageRecord, 1)
	}
	h.ch <- googleUsageRecord{rawBody: append([]byte(nil), rawBody...), info: info}
	return nil
}

func (h *recordingGoogleUsageHandler) ProcessInteractionStreamingResponse(ctx context.Context, decodedBody []byte, info UsageContext) error {
	if h.streamingCh == nil {
		h.streamingCh = make(chan googleUsageRecord, 1)
	}
	h.streamingCh <- googleUsageRecord{rawBody: append([]byte(nil), decodedBody...), info: info}
	return nil
}

func receiveGoogleUsageRecord(t *testing.T, ch <-chan googleUsageRecord) googleUsageRecord {
	t.Helper()
	select {
	case got := <-ch:
		return got
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for google usage")
		return googleUsageRecord{}
	}
}
