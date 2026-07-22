package main

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAlibabaBailianProxyCreateTaskForwardsWithProviderAuthAndAsyncHeader(t *testing.T) {
	proxy, err := NewAlibabaBailianProxy(AlibabaBailianProxyOptions{ModelStore: fakeModelStore{model: &ModelInfo{ID: 42, Status: ModelStatusActive}}})
	if err != nil {
		t.Fatal(err)
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s, want POST", r.Method)
		}
		if r.URL.Path != bailianVideoSynthesisPath {
			t.Errorf("path = %s, want %s", r.URL.Path, bailianVideoSynthesisPath)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer provider-key" {
			t.Errorf("Authorization = %q, want provider key", got)
		}
		if got := r.Header.Get(bailianAsyncHeader); got != bailianAsyncHeaderValue {
			t.Errorf("%s = %q, want %q", bailianAsyncHeader, got, bailianAsyncHeaderValue)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(body), `"model":"wan2.7-t2v-2026-06-12"`) {
			t.Fatalf("forwarded body = %s", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output":{"task_id":"task-1","task_status":"PENDING"}}`))
	}))
	defer upstream.Close()

	req := httptest.NewRequest(http.MethodPost, bailianVideoSynthesisPath, strings.NewReader(`{"model":"wan2.7-t2v-2026-06-12","input":{"prompt":"hello"}}`))
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req, 10, 20, upstream.URL, "provider-key")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%q", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "task-1") {
		t.Fatalf("response body = %q, want upstream response", rec.Body.String())
	}
}

func TestAlibabaBailianProxyCreateTaskValidation(t *testing.T) {
	tests := []struct {
		name        string
		providerKey string
		body        string
		store       fakeModelStore
		wantStatus  int
	}{
		{name: "missing provider key", providerKey: " ", body: `{"model":"wan2.7-t2v-2026-06-12"}`, store: fakeModelStore{model: &ModelInfo{ID: 1, Status: ModelStatusActive}}, wantStatus: http.StatusBadGateway},
		{name: "invalid json", providerKey: "provider-key", body: `{`, store: fakeModelStore{model: &ModelInfo{ID: 1, Status: ModelStatusActive}}, wantStatus: http.StatusBadRequest},
		{name: "missing model", providerKey: "provider-key", body: `{"input":{"prompt":"hello"}}`, store: fakeModelStore{model: &ModelInfo{ID: 1, Status: ModelStatusActive}}, wantStatus: http.StatusBadRequest},
		{name: "unsupported model", providerKey: "provider-key", body: `{"model":"wan2.7-t2v-2026-06-12"}`, store: fakeModelStore{}, wantStatus: http.StatusBadRequest},
		{name: "banned model", providerKey: "provider-key", body: `{"model":"wan2.7-t2v-2026-06-12"}`, store: fakeModelStore{model: &ModelInfo{ID: 1, Status: ModelStatusBanned}}, wantStatus: http.StatusForbidden},
		{name: "model lookup error", providerKey: "provider-key", body: `{"model":"wan2.7-t2v-2026-06-12"}`, store: fakeModelStore{err: errors.New("lookup failed")}, wantStatus: http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proxy, err := NewAlibabaBailianProxy(AlibabaBailianProxyOptions{ModelStore: tt.store})
			if err != nil {
				t.Fatal(err)
			}
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Fatal("upstream should not be called")
			}))
			defer upstream.Close()

			req := httptest.NewRequest(http.MethodPost, bailianVideoSynthesisPath, strings.NewReader(tt.body))
			rec := httptest.NewRecorder()
			proxy.ServeHTTP(rec, req, 10, 20, upstream.URL, tt.providerKey)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body=%q", rec.Code, tt.wantStatus, rec.Body.String())
			}
		})
	}
}

func TestAlibabaBailianProxyFetchTaskForwardsWithoutModelValidation(t *testing.T) {
	proxy, err := NewAlibabaBailianProxy(AlibabaBailianProxyOptions{ModelStore: fakeModelStore{err: errors.New("should not resolve model")}})
	if err != nil {
		t.Fatal(err)
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("method = %s, want GET", r.Method)
		}
		if r.URL.Path != "/api/v1/tasks/task-1" {
			t.Errorf("path = %s, want task path", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer provider-key" {
			t.Errorf("Authorization = %q, want provider key", got)
		}
		if got := r.Header.Get(bailianAsyncHeader); got != "" {
			t.Errorf("%s = %q, want empty", bailianAsyncHeader, got)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"output":{"task_id":"task-1","task_status":"SUCCEEDED"},"usage":{"duration":10}}`))
	}))
	defer upstream.Close()

	req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks/task-1", nil)
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req, 10, 20, upstream.URL, "provider-key")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%q", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"usage"`) {
		t.Fatalf("response body = %q, want upstream usage payload passthrough", rec.Body.String())
	}
}

func TestAlibabaBailianProxyRejectsUnsupportedPath(t *testing.T) {
	proxy, err := NewAlibabaBailianProxy(AlibabaBailianProxyOptions{ModelStore: fakeModelStore{model: &ModelInfo{ID: 1, Status: ModelStatusActive}}})
	if err != nil {
		t.Fatal(err)
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("upstream should not be called")
	}))
	defer upstream.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/unsupported", strings.NewReader(`{"model":"wan2.7-t2v-2026-06-12"}`))
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req, 10, 20, upstream.URL, "provider-key")
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body=%q", rec.Code, http.StatusNotFound, rec.Body.String())
	}
}
