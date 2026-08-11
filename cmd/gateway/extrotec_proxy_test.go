package main

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/HyperToken-dev/fabric/internal/models"
)

func TestExtrotecProxyGenerateForwardsWithProviderAuth(t *testing.T) {
	integralLogs := &recordingIntegralLogHandler{ch: make(chan recordedIntegralLog, 1)}
	proxy, err := NewExtrotecProxy(ExtrotecProxyOptions{ModelStore: fakeModelStore{model: &ModelInfo{ID: 42, Status: ModelStatusActive}}, IntegralLogHandler: integralLogs})
	if err != nil {
		t.Fatal(err)
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != extrotecImagePath {
			t.Errorf("path = %s, want %s", r.URL.Path, extrotecImagePath)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer provider-key" {
			t.Errorf("Authorization = %q, want provider key", got)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(body), `"model":"Z-imageturbo-t2i"`) {
			t.Fatalf("forwarded body = %s", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"job-1","status":"queued"}`))
	}))
	defer upstream.Close()

	req := httptest.NewRequest(http.MethodPost, extrotecImagePath, strings.NewReader(`{"model":"Z-imageturbo-t2i","prompt":"hello"}`))
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req, 10, 20, upstream.URL, "provider-key")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%q", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "job-1") {
		t.Fatalf("response body = %q, want upstream response", rec.Body.String())
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
	if loggedContext.Provider != "extrotec" || loggedContext.APIFormat != models.APIFormatExtrotec || loggedContext.Outcome != "ok" || loggedContext.Model != "Z-imageturbo-t2i" || loggedContext.ModelID != 42 || loggedContext.ChannelID != 20 {
		t.Fatalf("context = %+v", loggedContext)
	}
}

func TestExtrotecProxyVideoGenerateForwards(t *testing.T) {
	proxy, err := NewExtrotecProxy(ExtrotecProxyOptions{ModelStore: fakeModelStore{model: &ModelInfo{ID: 7, Status: ModelStatusActive}}})
	if err != nil {
		t.Fatal(err)
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != extrotecVideoPath {
			t.Errorf("path = %s, want %s", r.URL.Path, extrotecVideoPath)
		}
		_, _ = w.Write([]byte(`{"id":"video-job-1"}`))
	}))
	defer upstream.Close()

	req := httptest.NewRequest(http.MethodPost, extrotecVideoPath, strings.NewReader(`{"model":"MiniMax-H3","prompt":"hello"}`))
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req, 10, 20, upstream.URL, "provider-key")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%q", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "video-job-1") {
		t.Fatalf("response body = %q, want upstream response", rec.Body.String())
	}
}

func TestExtrotecProxyStatusCheckForwardsWithoutModelValidation(t *testing.T) {
	integralLogs := &recordingIntegralLogHandler{ch: make(chan recordedIntegralLog, 1)}
	proxy, err := NewExtrotecProxy(ExtrotecProxyOptions{ModelStore: fakeModelStore{err: errors.New("should not resolve model")}, IntegralLogHandler: integralLogs})
	if err != nil {
		t.Fatal(err)
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/v1/videos/task-1" {
			t.Errorf("path = %s, want task path", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer provider-key" {
			t.Errorf("Authorization = %q, want provider key", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"task-1","status":"succeeded"}`))
	}))
	defer upstream.Close()

	req := httptest.NewRequest(http.MethodGet, "/v1/videos/task-1", nil)
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req, 10, 20, upstream.URL, "provider-key")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%q", rec.Code, rec.Body.String())
	}

	got := receiveIntegralLog(t, integralLogs.ch)
	var loggedContext struct {
		Provider  string `json:"provider"`
		APIFormat int32  `json:"api_format"`
		Model     string `json:"model"`
		ModelID   int32  `json:"model_id"`
		ChannelID int32  `json:"channel_id"`
	}
	if err := json.Unmarshal([]byte(got.context), &loggedContext); err != nil {
		t.Fatal(err)
	}
	if loggedContext.Provider != "extrotec" || loggedContext.APIFormat != models.APIFormatExtrotec || loggedContext.Model != "" || loggedContext.ModelID != 0 || loggedContext.ChannelID != 20 {
		t.Fatalf("context = %+v", loggedContext)
	}
}

func TestExtrotecProxyGenerateValidation(t *testing.T) {
	tests := []struct {
		name        string
		providerKey string
		body        string
		store       fakeModelStore
		wantStatus  int
	}{
		{name: "missing provider key", providerKey: " ", body: `{"model":"MiniMax-H3"}`, store: fakeModelStore{model: &ModelInfo{ID: 1, Status: ModelStatusActive}}, wantStatus: http.StatusBadGateway},
		{name: "invalid json", providerKey: "provider-key", body: `{`, store: fakeModelStore{model: &ModelInfo{ID: 1, Status: ModelStatusActive}}, wantStatus: http.StatusBadRequest},
		{name: "missing model", providerKey: "provider-key", body: `{"prompt":"hello"}`, store: fakeModelStore{model: &ModelInfo{ID: 1, Status: ModelStatusActive}}, wantStatus: http.StatusBadRequest},
		{name: "unsupported model", providerKey: "provider-key", body: `{"model":"MiniMax-H3"}`, store: fakeModelStore{}, wantStatus: http.StatusBadRequest},
		{name: "banned model", providerKey: "provider-key", body: `{"model":"MiniMax-H3"}`, store: fakeModelStore{model: &ModelInfo{ID: 1, Status: ModelStatusBanned}}, wantStatus: http.StatusForbidden},
		{name: "model lookup error", providerKey: "provider-key", body: `{"model":"MiniMax-H3"}`, store: fakeModelStore{err: errors.New("lookup failed")}, wantStatus: http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proxy, err := NewExtrotecProxy(ExtrotecProxyOptions{ModelStore: tt.store})
			if err != nil {
				t.Fatal(err)
			}
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Fatal("upstream should not be called")
			}))
			defer upstream.Close()

			req := httptest.NewRequest(http.MethodPost, extrotecVideoPath, strings.NewReader(tt.body))
			rec := httptest.NewRecorder()
			proxy.ServeHTTP(rec, req, 10, 20, upstream.URL, tt.providerKey)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body=%q", rec.Code, tt.wantStatus, rec.Body.String())
			}
		})
	}
}

func TestExtrotecProxyRejectsSensitivePromptFields(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "prompt", body: `{"model":"MiniMax-H3","prompt":"blocked"}`},
		{name: "forward prompt", body: `{"model":"MiniMax-H3","forward_prompt":"blocked"}`},
		{name: "negative prompt", body: `{"model":"MiniMax-H3","negative_prompt":"blocked"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			integralLogs := &recordingIntegralLogHandler{ch: make(chan recordedIntegralLog, 1)}
			proxy, err := NewExtrotecProxy(ExtrotecProxyOptions{
				ModelStore:         fakeModelStore{model: &ModelInfo{ID: 42, Status: ModelStatusActive}},
				IntegralLogHandler: integralLogs,
				TextPolicy:         rejectingPolicy{},
			})
			if err != nil {
				t.Fatal(err)
			}
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Fatal("upstream should not be called")
			}))
			defer upstream.Close()

			req := httptest.NewRequest(http.MethodPost, extrotecVideoPath, strings.NewReader(tt.body))
			rec := httptest.NewRecorder()
			proxy.ServeHTTP(rec, req, 10, 20, upstream.URL, "provider-key")
			if rec.Code != http.StatusForbidden {
				t.Fatalf("status = %d, want %d; body=%q", rec.Code, http.StatusForbidden, rec.Body.String())
			}

			got := receiveIntegralLog(t, integralLogs.ch)
			var loggedContext struct {
				Provider        string `json:"provider"`
				APIFormat       int32  `json:"api_format"`
				Outcome         string `json:"outcome"`
				RejectionStage  string `json:"rejection_stage"`
				RejectionReason string `json:"rejection_reason"`
				Model           string `json:"model"`
				ModelID         int32  `json:"model_id"`
				ChannelID       int32  `json:"channel_id"`
			}
			if err := json.Unmarshal([]byte(got.context), &loggedContext); err != nil {
				t.Fatal(err)
			}
			if loggedContext.Provider != "extrotec" || loggedContext.APIFormat != models.APIFormatExtrotec || loggedContext.Outcome != integralOutcomeRejected || loggedContext.RejectionStage != rejectionStageInput || loggedContext.RejectionReason != rejectionReasonSensitive || loggedContext.Model != "MiniMax-H3" || loggedContext.ModelID != 42 || loggedContext.ChannelID != 20 {
				t.Fatalf("context = %+v", loggedContext)
			}
		})
	}
}

func TestExtrotecProxyAllowsEmptyPromptFields(t *testing.T) {
	proxy, err := NewExtrotecProxy(ExtrotecProxyOptions{
		ModelStore: fakeModelStore{model: &ModelInfo{ID: 7, Status: ModelStatusActive}},
		TextPolicy: rejectingPolicy{},
	})
	if err != nil {
		t.Fatal(err)
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(body), `"model":"MiniMax-H3"`) {
			t.Fatalf("forwarded body = %s", body)
		}
		_, _ = w.Write([]byte(`{"id":"video-job-1"}`))
	}))
	defer upstream.Close()

	body := `{"model":"MiniMax-H3","prompt":"","forward_prompt":"  ","negative_prompt":null}`
	req := httptest.NewRequest(http.MethodPost, extrotecVideoPath, strings.NewReader(body))
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req, 10, 20, upstream.URL, "provider-key")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%q", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "video-job-1") {
		t.Fatalf("response body = %q, want upstream response", rec.Body.String())
	}
}

func TestExtrotecProxyRejectsUnsupportedPath(t *testing.T) {
	proxy, err := NewExtrotecProxy(ExtrotecProxyOptions{ModelStore: fakeModelStore{model: &ModelInfo{ID: 1, Status: ModelStatusActive}}})
	if err != nil {
		t.Fatal(err)
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("upstream should not be called")
	}))
	defer upstream.Close()

	req := httptest.NewRequest(http.MethodPost, "/v1/unsupported", strings.NewReader(`{"model":"MiniMax-H3"}`))
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req, 10, 20, upstream.URL, "provider-key")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body=%q", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}
