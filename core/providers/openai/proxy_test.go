package openai

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"strings"
	"testing"

	coreproxy "github.com/HyperToken-dev/fabric/core/proxy"
)

func TestOpenAIProxyUsesBearerAuthAndRewriteHook(t *testing.T) {
	rewriteCalled := false

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.Header.Get("Authorization"), "Bearer provider-key"; got != want {
			t.Errorf("Authorization = %q, want %q", got, want)
		}
		if got := r.Header.Get("X-Rewrite"); got != "yes" {
			t.Errorf("X-Rewrite = %q, want yes", got)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()

	p := New(coreproxy.Options{
		Rewrite: func(pr *httputil.ProxyRequest, upstream coreproxy.Upstream) error {
			rewriteCalled = true
			pr.Out.Header.Set("X-Rewrite", "yes")
			return nil
		},
	})

	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	rec := httptest.NewRecorder()

	p.ServeHTTP(rec, req, coreproxy.Upstream{BaseURL: upstream.URL, APIKey: "provider-key"})

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %q", rec.Code, rec.Body.String())
	}
	if !rewriteCalled {
		t.Fatal("rewrite callback was not called")
	}
}

func TestOpenAIProxyDefaultRewriteInjectsChatStreamUsageOption(t *testing.T) {
	var forwarded map[string]any
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read upstream body: %v", err)
		}
		if err := r.Body.Close(); err != nil {
			t.Errorf("close upstream body: %v", err)
		}
		if err := json.Unmarshal(body, &forwarded); err != nil {
			t.Errorf("unmarshal upstream body: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()

	p := New(coreproxy.Options{})
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-5","stream":true}`))
	rec := httptest.NewRecorder()

	p.ServeHTTP(rec, req, coreproxy.Upstream{BaseURL: upstream.URL, APIKey: "provider-key"})

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %q", rec.Code, rec.Body.String())
	}
	streamOptions, ok := forwarded["stream_options"].(map[string]any)
	if !ok {
		t.Fatalf("stream_options = %#v, want object", forwarded["stream_options"])
	}
	if got, want := streamOptions["include_usage"], true; got != want {
		t.Fatalf("include_usage = %#v, want true", got)
	}
}

func TestOpenAIProxyDefaultRewriteDoesNotInjectNonStreamChatCompletion(t *testing.T) {
	var forwarded map[string]any
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read upstream body: %v", err)
		}
		if err := r.Body.Close(); err != nil {
			t.Errorf("close upstream body: %v", err)
		}
		if err := json.Unmarshal(body, &forwarded); err != nil {
			t.Errorf("unmarshal upstream body: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()

	p := New(coreproxy.Options{})
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-5","stream":false}`))
	rec := httptest.NewRecorder()

	p.ServeHTTP(rec, req, coreproxy.Upstream{BaseURL: upstream.URL, APIKey: "provider-key"})

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %q", rec.Code, rec.Body.String())
	}
	if _, ok := forwarded["stream_options"]; ok {
		t.Fatalf("stream_options = %#v, want absent", forwarded["stream_options"])
	}
}

func TestOpenAIProxyDefaultRewriteSkipsNonChatCompletion(t *testing.T) {
	var forwarded map[string]any
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read upstream body: %v", err)
		}
		if err := r.Body.Close(); err != nil {
			t.Errorf("close upstream body: %v", err)
		}
		if err := json.Unmarshal(body, &forwarded); err != nil {
			t.Errorf("unmarshal upstream body: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()

	p := New(coreproxy.Options{})
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(`{"model":"gpt-5","stream":true}`))
	rec := httptest.NewRecorder()

	p.ServeHTTP(rec, req, coreproxy.Upstream{BaseURL: upstream.URL, APIKey: "provider-key"})

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %q", rec.Code, rec.Body.String())
	}
	if _, ok := forwarded["stream_options"]; ok {
		t.Fatalf("stream_options = %#v, want absent", forwarded["stream_options"])
	}
}

func TestOpenAIProxyDefaultRewritePreservesExistingStreamOptions(t *testing.T) {
	var forwarded map[string]any
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read upstream body: %v", err)
		}
		if err := r.Body.Close(); err != nil {
			t.Errorf("close upstream body: %v", err)
		}
		if err := json.Unmarshal(body, &forwarded); err != nil {
			t.Errorf("unmarshal upstream body: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()

	p := New(coreproxy.Options{})
	body := `{"model":"gpt-5","stream":true,"stream_options":{"foo":"bar"}}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	rec := httptest.NewRecorder()

	p.ServeHTTP(rec, req, coreproxy.Upstream{BaseURL: upstream.URL, APIKey: "provider-key"})

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %q", rec.Code, rec.Body.String())
	}
	streamOptions, ok := forwarded["stream_options"].(map[string]any)
	if !ok {
		t.Fatalf("stream_options = %#v, want object", forwarded["stream_options"])
	}
	if got, want := streamOptions["foo"], "bar"; got != want {
		t.Fatalf("foo = %#v, want %q", got, want)
	}
	if got, want := streamOptions["include_usage"], true; got != want {
		t.Fatalf("include_usage = %#v, want true", got)
	}
}

func TestOpenAIProxyDefaultRewriteReturnsInvalidJSONError(t *testing.T) {
	called := false
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()

	p := New(coreproxy.Options{})
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{invalid`))
	rec := httptest.NewRecorder()

	p.ServeHTTP(rec, req, coreproxy.Upstream{BaseURL: upstream.URL, APIKey: "provider-key"})

	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, body = %q", rec.Code, rec.Body.String())
	}
	if called {
		t.Fatal("upstream was called after rewrite error")
	}
}

func TestOpenAIProxyCustomRewriteOverridesDefaultRewrite(t *testing.T) {
	var forwarded map[string]any
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read upstream body: %v", err)
		}
		if err := r.Body.Close(); err != nil {
			t.Errorf("close upstream body: %v", err)
		}
		if err := json.Unmarshal(body, &forwarded); err != nil {
			t.Errorf("unmarshal upstream body: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstream.Close()

	p := New(coreproxy.Options{
		Rewrite: func(pr *httputil.ProxyRequest, upstream coreproxy.Upstream) error {
			pr.Out.Header.Set("X-Custom-Rewrite", "yes")
			return nil
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-5","stream":true}`))
	rec := httptest.NewRecorder()

	p.ServeHTTP(rec, req, coreproxy.Upstream{BaseURL: upstream.URL, APIKey: "provider-key"})

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %q", rec.Code, rec.Body.String())
	}
	if _, ok := forwarded["stream_options"]; ok {
		t.Fatalf("stream_options = %#v, want absent", forwarded["stream_options"])
	}
}

func TestParseBaseURL(t *testing.T) {
	tests := []struct {
		name    string
		baseURL string
		wantErr bool
	}{
		{name: "root without slash", baseURL: "https://api.openai.com"},
		{name: "root with slash", baseURL: "https://api.openai.com/"},
		{name: "trim spaces", baseURL: " https://api.openai.com "},
		{name: "empty", baseURL: "", wantErr: true},
		{name: "missing scheme", baseURL: "api.openai.com", wantErr: true},
		{name: "missing host", baseURL: "https://", wantErr: true},
		{name: "path", baseURL: "https://api.openai.com/v1", wantErr: true},
		{name: "path with slash", baseURL: "https://api.openai.com/v1/", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := coreproxy.ParseBaseURL(tt.baseURL)
			if tt.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
