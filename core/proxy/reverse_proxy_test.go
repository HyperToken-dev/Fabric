package proxy

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/andybalholm/brotli"
)

func TestProxyServeHTTPForwardsToUpstream(t *testing.T) {
	rewriteCalled := false
	modifyCalled := false
	var upstreamHost string

	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if r.URL.RawQuery != "foo=bar" {
			t.Errorf("query = %q", r.URL.RawQuery)
		}
		if got, want := r.Header.Get("Authorization"), "Bearer provider-key"; got != want {
			t.Errorf("Authorization = %q, want %q", got, want)
		}
		if r.Host != upstreamHost {
			t.Errorf("Host = %q, want %q", r.Host, upstreamHost)
		}
		if got := r.Header.Get("X-Rewrite"); got != "yes" {
			t.Errorf("X-Rewrite = %q, want yes", got)
		}
		w.Header().Set("X-Upstream", "ok")
		_, _ = w.Write([]byte("proxied"))
	}))
	defer upstreamServer.Close()
	upstreamURL, err := url.Parse(upstreamServer.URL)
	if err != nil {
		t.Fatal(err)
	}
	upstreamHost = upstreamURL.Host

	p := New(Options{
		Rewrite: func(pr *httputil.ProxyRequest) error {
			rewriteCalled = true
			pr.Out.Header.Set("X-Rewrite", "yes")
			return nil
		},
		ModifyResponse: func(resp *http.Response) error {
			modifyCalled = true
			resp.Header.Set("X-Modified", "yes")
			return nil
		},
	})

	req := httptest.NewRequest(http.MethodPost, "http://client.example/v1/chat/completions?foo=bar", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer client-key")
	rec := httptest.NewRecorder()

	p.ServeHTTP(rec, req, Upstream{BaseURL: upstreamServer.URL, APIKey: "provider-key"})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("X-Modified"); got != "yes" {
		t.Fatalf("X-Modified = %q, want yes", got)
	}
	body, err := io.ReadAll(rec.Result().Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "proxied" {
		t.Fatalf("body = %q, want proxied", body)
	}
	if !rewriteCalled {
		t.Fatal("rewrite callback was not called")
	}
	if !modifyCalled {
		t.Fatal("modify response callback was not called")
	}
}

func TestProxyServeHTTPRejectsInvalidBaseURL(t *testing.T) {
	p := New(Options{})

	tests := []string{"", "example.com", "http://", "http://example.com/base"}
	for _, baseURL := range tests {
		t.Run(baseURL, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
			p.ServeHTTP(rec, req, Upstream{BaseURL: baseURL, APIKey: "provider-key"})
			if rec.Code != http.StatusBadGateway {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadGateway)
			}
		})
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
			_, err := ParseBaseURL(tt.baseURL)
			if tt.wantErr && err == nil {
				t.Fatal("expected error")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestProxyServeHTTPReturnsBadGatewayForUpstreamError(t *testing.T) {
	p := New(Options{})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	p.ServeHTTP(rec, req, Upstream{BaseURL: "http://127.0.0.1:1", APIKey: "provider-key"})
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadGateway)
	}
}

func TestProxyServeHTTPReturnsBadGatewayForRewriteError(t *testing.T) {
	upstreamCalled := false
	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalled = true
		w.WriteHeader(http.StatusNoContent)
	}))
	defer upstreamServer.Close()

	rewriteErr := errors.New("custom rewrite error")
	p := New(Options{
		Rewrite: func(pr *httputil.ProxyRequest) error {
			return rewriteErr
		},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	p.ServeHTTP(rec, req, Upstream{BaseURL: upstreamServer.URL, APIKey: "provider-key"})
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadGateway)
	}
	if !strings.Contains(rec.Body.String(), rewriteErr.Error()) {
		t.Fatalf("body = %q, want rewrite error", rec.Body.String())
	}
	if upstreamCalled {
		t.Fatal("upstream was called after rewrite error")
	}
}

func TestProxyOnCompleteReceivesDecodedBodyAndPreservesRawPassthrough(t *testing.T) {
	var encoded bytes.Buffer
	brotliWriter := brotli.NewWriter(&encoded)
	if _, err := brotliWriter.Write([]byte(`{"ok":true}`)); err != nil {
		t.Fatal(err)
	}
	if err := brotliWriter.Close(); err != nil {
		t.Fatal(err)
	}

	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Encoding", "br")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(encoded.Bytes())
	}))
	defer upstreamServer.Close()

	completeCalled := false
	p := New(Options{
		OnComplete: func(resp *http.Response, decodedBody []byte) {
			completeCalled = true
			if resp.StatusCode != http.StatusOK {
				t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
			}
			if string(decodedBody) != `{"ok":true}` {
				t.Errorf("decoded body = %q", decodedBody)
			}
		},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	p.ServeHTTP(rec, req, Upstream{BaseURL: upstreamServer.URL, APIKey: "provider-key"})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Content-Encoding"); got != "br" {
		t.Fatalf("Content-Encoding = %q, want br", got)
	}
	if !bytes.Equal(rec.Body.Bytes(), encoded.Bytes()) {
		t.Fatalf("body was not preserved as raw encoded payload")
	}
	if !completeCalled {
		t.Fatal("onComplete was not called")
	}
}

func TestProxyOnCompleteCanMutateResponse(t *testing.T) {
	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"unsafe":true}`))
	}))
	defer upstreamServer.Close()

	p := New(Options{
		OnComplete: func(resp *http.Response, decodedBody []byte) {
			if string(decodedBody) != `{"unsafe":true}` {
				t.Errorf("decoded body = %q", decodedBody)
			}
			replacement := []byte(`{"error":"blocked"}`)
			resp.StatusCode = http.StatusUnprocessableEntity
			resp.Status = "422 Unprocessable Entity"
			resp.Header.Set("Content-Type", "application/json")
			resp.Header.Del("Content-Encoding")
			resp.Header.Set("Content-Length", "19")
			resp.Body = io.NopCloser(bytes.NewReader(replacement))
			resp.ContentLength = int64(len(replacement))
		},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	p.ServeHTTP(rec, req, Upstream{BaseURL: upstreamServer.URL, APIKey: "provider-key"})

	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, body = %q", rec.Code, rec.Body.String())
	}
	if rec.Body.String() != `{"error":"blocked"}` {
		t.Fatalf("body = %q", rec.Body.String())
	}
}

func TestProxyModifyResponseOverridesOnComplete(t *testing.T) {
	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("proxied"))
	}))
	defer upstreamServer.Close()

	onCompleteCalled := false
	modifyCalled := false
	p := New(Options{
		OnComplete: func(resp *http.Response, decodedBody []byte) {
			onCompleteCalled = true
		},
		ModifyResponse: func(resp *http.Response) error {
			modifyCalled = true
			return nil
		},
	})

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	p.ServeHTTP(rec, req, Upstream{BaseURL: upstreamServer.URL, APIKey: "provider-key"})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", rec.Code, rec.Body.String())
	}
	if !modifyCalled {
		t.Fatal("modify response callback was not called")
	}
	if onCompleteCalled {
		t.Fatal("onComplete should not be called when ModifyResponse is provided")
	}
}

func TestProxyStreamingOnCompleteDoesNotBlockFirstChunk(t *testing.T) {
	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("response writer is not a flusher")
		}
		_, _ = w.Write([]byte("data: first\n\n"))
		flusher.Flush()
		time.Sleep(50 * time.Millisecond)
		_, _ = w.Write([]byte("data: second\n\n"))
		flusher.Flush()
	}))
	defer upstreamServer.Close()

	completeCh := make(chan string, 1)
	p := New(Options{
		OnComplete: func(resp *http.Response, decodedBody []byte) {
			completeCh <- string(decodedBody)
		},
	})

	proxyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p.ServeHTTP(w, r, Upstream{BaseURL: upstreamServer.URL, APIKey: "provider-key"})
	}))
	defer proxyServer.Close()

	resp, err := http.Get(proxyServer.URL + "/stream")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Errorf("close response body: %v", err)
		}
	}()

	buf := make([]byte, len("data: first\n\n"))
	if _, err := io.ReadFull(resp.Body, buf); err != nil {
		t.Fatal(err)
	}
	if string(buf) != "data: first\n\n" {
		t.Fatalf("first chunk = %q", buf)
	}
	select {
	case got := <-completeCh:
		t.Fatalf("onComplete called before stream ended with %q", got)
	default:
	}

	remaining, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(remaining) != "data: second\n\n" {
		t.Fatalf("remaining = %q", remaining)
	}
	select {
	case got := <-completeCh:
		if got != "data: first\n\ndata: second\n\n" {
			t.Fatalf("decoded stream body = %q", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for onComplete")
	}
}

func TestProxyStreamingOnCompleteTriggeredByCloseBeforeEOF(t *testing.T) {
	clientClosed := make(chan struct{})
	upstreamServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		_, _ = w.Write([]byte("data: first\n\n"))
		flusher.Flush()
		<-clientClosed // Keep server open until client closes
	}))
	defer upstreamServer.Close()

	completeCh := make(chan string, 1)
	p := New(Options{
		OnComplete: func(resp *http.Response, decodedBody []byte) {
			completeCh <- string(decodedBody)
		},
	})

	proxyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p.ServeHTTP(w, r, Upstream{BaseURL: upstreamServer.URL, APIKey: "provider-key"})
	}))
	defer proxyServer.Close()

	resp, err := http.Get(proxyServer.URL + "/stream")
	if err != nil {
		t.Fatal(err)
	}

	buf := make([]byte, len("data: first\n\n"))
	if _, err := io.ReadFull(resp.Body, buf); err != nil {
		t.Fatal(err)
	}

	if err := resp.Body.Close(); err != nil {
		t.Fatal(err)
	}
	close(clientClosed)

	select {
	case got := <-completeCh:
		if got != "data: first\n\n" {
			t.Fatalf("decoded stream body = %q", got)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for onComplete")
	}
}
