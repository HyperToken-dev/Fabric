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

func TestSeedanceProxyCreateTaskForwardsWithProviderAuth(t *testing.T) {
	integralLogs := &recordingIntegralLogHandler{ch: make(chan recordedIntegralLog, 1)}
	proxy, err := NewSeedanceProxy(SeedanceProxyOptions{ModelStore: fakeModelStore{model: &ModelInfo{ID: 42, Status: ModelStatusActive}}, IntegralLogHandler: integralLogs})
	if err != nil {
		t.Fatal(err)
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != seedanceTasksPath {
			t.Errorf("path = %s, want %s", r.URL.Path, seedanceTasksPath)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer provider-key" {
			t.Errorf("Authorization = %q, want provider key", got)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(body), `"model":"doubao-seedance-2-0-260128"`) {
			t.Fatalf("forwarded body = %s", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"task-1","status":"queued"}`))
	}))
	defer upstream.Close()

	req := httptest.NewRequest(http.MethodPost, seedanceTasksPath, strings.NewReader(`{"model":"doubao-seedance-2-0-260128","content":[{"type":"text","text":"hello"}]}`))
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req, 10, 20, upstream.URL, "provider-key")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%q", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "task-1") {
		t.Fatalf("response body = %q, want upstream response", rec.Body.String())
	}
	got := receiveIntegralLog(t, integralLogs.ch)
	if got.keyID != 10 {
		t.Fatalf("keyID = %d, want 10", got.keyID)
	}
	if got.response != `{"id":"task-1","status":"queued"}` {
		t.Fatalf("response = %q", got.response)
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
	if loggedContext.Provider != "seedance" || loggedContext.APIFormat != 3 || loggedContext.Outcome != "ok" || loggedContext.Model != "doubao-seedance-2-0-260128" || loggedContext.ModelID != 42 || loggedContext.ChannelID != 20 {
		t.Fatalf("context = %+v", loggedContext)
	}
}

func TestSeedanceProxyCreateTaskValidation(t *testing.T) {
	tests := []struct {
		name        string
		providerKey string
		body        string
		store       fakeModelStore
		wantStatus  int
	}{
		{name: "missing provider key", providerKey: " ", body: `{"model":"doubao-seedance-2-0-260128"}`, store: fakeModelStore{model: &ModelInfo{ID: 1, Status: ModelStatusActive}}, wantStatus: http.StatusBadGateway},
		{name: "invalid json", providerKey: "provider-key", body: `{`, store: fakeModelStore{model: &ModelInfo{ID: 1, Status: ModelStatusActive}}, wantStatus: http.StatusBadRequest},
		{name: "missing model", providerKey: "provider-key", body: `{"content":[{"type":"text","text":"hello"}]}`, store: fakeModelStore{model: &ModelInfo{ID: 1, Status: ModelStatusActive}}, wantStatus: http.StatusBadRequest},
		{name: "unsupported model", providerKey: "provider-key", body: `{"model":"doubao-seedance-2-0-260128"}`, store: fakeModelStore{}, wantStatus: http.StatusBadRequest},
		{name: "banned model", providerKey: "provider-key", body: `{"model":"doubao-seedance-2-0-260128"}`, store: fakeModelStore{model: &ModelInfo{ID: 1, Status: ModelStatusBanned}}, wantStatus: http.StatusForbidden},
		{name: "model lookup error", providerKey: "provider-key", body: `{"model":"doubao-seedance-2-0-260128"}`, store: fakeModelStore{err: errors.New("lookup failed")}, wantStatus: http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proxy, err := NewSeedanceProxy(SeedanceProxyOptions{ModelStore: tt.store})
			if err != nil {
				t.Fatal(err)
			}
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Fatal("upstream should not be called")
			}))
			defer upstream.Close()

			req := httptest.NewRequest(http.MethodPost, seedanceTasksPath, strings.NewReader(tt.body))
			rec := httptest.NewRecorder()
			proxy.ServeHTTP(rec, req, 10, 20, upstream.URL, tt.providerKey)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body=%q", rec.Code, tt.wantStatus, rec.Body.String())
			}
		})
	}
}

func TestSeedanceProxyTaskManagementForwardsWithoutModelValidation(t *testing.T) {
	integralLogs := &recordingIntegralLogHandler{ch: make(chan recordedIntegralLog, 1)}
	proxy, err := NewSeedanceProxy(SeedanceProxyOptions{ModelStore: fakeModelStore{err: errors.New("should not resolve model")}, IntegralLogHandler: integralLogs})
	if err != nil {
		t.Fatal(err)
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != seedanceTasksPath+"/task-1" {
			t.Errorf("path = %s, want task path", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer provider-key" {
			t.Errorf("Authorization = %q, want provider key", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"task-1","status":"succeeded"}`))
	}))
	defer upstream.Close()

	req := httptest.NewRequest(http.MethodGet, seedanceTasksPath+"/task-1", nil)
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req, 10, 20, upstream.URL, "provider-key")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%q", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "succeeded") {
		t.Fatalf("response body = %q, want upstream payload passthrough", rec.Body.String())
	}
	got := receiveIntegralLog(t, integralLogs.ch)
	if got.response != `{"id":"task-1","status":"succeeded"}` {
		t.Fatalf("response = %q", got.response)
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
	if loggedContext.Provider != "seedance" || loggedContext.APIFormat != 3 || loggedContext.Outcome != "ok" || loggedContext.Model != "" || loggedContext.ModelID != 0 || loggedContext.ChannelID != 20 {
		t.Fatalf("context = %+v", loggedContext)
	}
}

func TestSeedanceProxyRejectsUnsupportedPath(t *testing.T) {
	integralLogs := &recordingIntegralLogHandler{ch: make(chan recordedIntegralLog, 1)}
	proxy, err := NewSeedanceProxy(SeedanceProxyOptions{ModelStore: fakeModelStore{model: &ModelInfo{ID: 1, Status: ModelStatusActive}}, IntegralLogHandler: integralLogs})
	if err != nil {
		t.Fatal(err)
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("upstream should not be called")
	}))
	defer upstream.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v3/unsupported", strings.NewReader(`{"model":"doubao-seedance-2-0-260128"}`))
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req, 10, 20, upstream.URL, "provider-key")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body=%q", rec.Code, http.StatusNotFound, rec.Body.String())
	}
	select {
	case got := <-integralLogs.ch:
		t.Fatalf("unexpected integral log for unsupported path: %+v", got)
	default:
	}
}

func TestSeedanceProxyCreateTaskTracksProviderTask(t *testing.T) {
	tasks := &recordingProviderTaskStore{createCh: make(chan ProviderTaskInfo, 1)}
	proxy, err := NewSeedanceProxy(SeedanceProxyOptions{
		ModelStore:         fakeModelStore{model: &ModelInfo{ID: 42, Status: ModelStatusActive}},
		ProviderTaskStore:  tasks,
		IntegralLogHandler: &recordingIntegralLogHandler{ch: make(chan recordedIntegralLog, 1)},
	})
	if err != nil {
		t.Fatal(err)
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"task-1","model":"doubao-seedance-2-0-260128","status":"queued"}`))
	}))
	defer upstream.Close()

	reqBody := `{"model":"doubao-seedance-2-0-260128","content":[{"type":"text","text":"hello"}]}`
	req := httptest.NewRequest(http.MethodPost, seedanceTasksPath, strings.NewReader(reqBody))
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req, 10, 20, upstream.URL, "provider-key")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%q", rec.Code, rec.Body.String())
	}

	created := receiveProviderTask(t, tasks.createCh)
	if created.Provider != ProviderSeedance || created.KeyID != 10 || created.ChannelID != 20 || created.ModelID != 42 || created.ProviderTaskID != "task-1" || created.Status != ProviderTaskStatusPending {
		t.Fatalf("created task = %+v", created)
	}
	if string(created.Request) != reqBody {
		t.Fatalf("request = %s, want %s", created.Request, reqBody)
	}
	if !strings.Contains(string(created.Response), `"task-1"`) {
		t.Fatalf("response = %s", created.Response)
	}
}

func TestSeedanceProxyQueryCompletionRecordsCompletionTokens(t *testing.T) {
	tasks := &recordingProviderTaskStore{completeCh: make(chan ProviderTaskCompletion, 1)}
	proxy, err := NewSeedanceProxy(SeedanceProxyOptions{ProviderTaskStore: tasks, IntegralLogHandler: &recordingIntegralLogHandler{ch: make(chan recordedIntegralLog, 1)}})
	if err != nil {
		t.Fatal(err)
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"task-1","model":"doubao-seedance-2-0-260128","status":"succeeded","usage":{"completion_tokens":108900,"total_tokens":108900}}`))
	}))
	defer upstream.Close()

	req := httptest.NewRequest(http.MethodGet, seedanceTasksPath+"/task-1", nil)
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req, 10, 20, upstream.URL, "provider-key")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%q", rec.Code, rec.Body.String())
	}

	completed := receiveProviderTaskCompletion(t, tasks.completeCh)
	if completed.Provider != ProviderSeedance || completed.ProviderTaskID != "task-1" || completed.Status != ProviderTaskStatusSuccess || completed.CompletionTokens != 108900 {
		t.Fatalf("completed task = %+v", completed)
	}
}

func TestSeedanceProxyTotalTokensAloneDoesNotRecordUsage(t *testing.T) {
	tasks := &recordingProviderTaskStore{completeCh: make(chan ProviderTaskCompletion, 1)}
	proxy, err := NewSeedanceProxy(SeedanceProxyOptions{ProviderTaskStore: tasks, IntegralLogHandler: &recordingIntegralLogHandler{ch: make(chan recordedIntegralLog, 1)}})
	if err != nil {
		t.Fatal(err)
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"task-1","model":"doubao-seedance-2-0-260128","status":"succeeded","usage":{"total_tokens":108900}}`))
	}))
	defer upstream.Close()

	req := httptest.NewRequest(http.MethodGet, seedanceTasksPath+"/task-1", nil)
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req, 10, 20, upstream.URL, "provider-key")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%q", rec.Code, rec.Body.String())
	}

	completed := receiveProviderTaskCompletion(t, tasks.completeCh)
	if completed.CompletionTokens != 0 {
		t.Fatalf("completion tokens = %d, want 0", completed.CompletionTokens)
	}
}

func TestSeedanceProxySensitivePromptRejected(t *testing.T) {
	integralLogs := &recordingIntegralLogHandler{ch: make(chan recordedIntegralLog, 1)}
	proxy, err := NewSeedanceProxy(SeedanceProxyOptions{
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

	req := httptest.NewRequest(http.MethodPost, seedanceTasksPath, strings.NewReader(`{"model":"doubao-seedance-2-0-260128","content":[{"type":"image_url","image_url":{"url":"https://example.test/blocked.png"}},{"type":"text","text":"blocked"}]}`))
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req, 10, 20, upstream.URL, "provider-key")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d; body=%q", rec.Code, http.StatusForbidden, rec.Body.String())
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
	if loggedContext.Provider != "seedance" || loggedContext.Outcome != "rejected" || loggedContext.RejectionStage != "input" || loggedContext.RejectionReason != "sensitive" || loggedContext.Model != "doubao-seedance-2-0-260128" || loggedContext.ModelID != 42 {
		t.Fatalf("context = %+v", loggedContext)
	}
}

type recordingProviderTaskStore struct {
	createCh   chan ProviderTaskInfo
	completeCh chan ProviderTaskCompletion
}

func (s *recordingProviderTaskStore) CreateProviderTask(ctx context.Context, task ProviderTaskInfo) error {
	if s.createCh != nil {
		s.createCh <- task
	}
	return nil
}

func (s *recordingProviderTaskStore) CompleteProviderTask(ctx context.Context, completion ProviderTaskCompletion) (bool, error) {
	if s.completeCh != nil {
		s.completeCh <- completion
	}
	return completion.CompletionTokens > 0, nil
}

func receiveProviderTask(t *testing.T, ch <-chan ProviderTaskInfo) ProviderTaskInfo {
	t.Helper()
	select {
	case got := <-ch:
		return got
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for provider task")
		return ProviderTaskInfo{}
	}
}

func receiveProviderTaskCompletion(t *testing.T, ch <-chan ProviderTaskCompletion) ProviderTaskCompletion {
	t.Helper()
	select {
	case got := <-ch:
		return got
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for provider task completion")
		return ProviderTaskCompletion{}
	}
}
