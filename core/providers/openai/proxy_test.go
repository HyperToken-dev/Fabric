package openai

import (
	"io"
	"net/http"
	"net/http/httputil"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	coreproxy "fabric/core/proxy"
)

func TestProxyServeHTTPForwardsToUpstream(t *testing.T) {
	rewriteCalled := false
	modifyCalled := false
	var upstreamHost string

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
	defer upstream.Close()
	upstreamURL, err := url.Parse(upstream.URL)
	if err != nil {
		t.Fatal(err)
	}
	upstreamHost = upstreamURL.Host

	p, err := New(Options{
		Rewrite: func(pr *httputil.ProxyRequest) {
			rewriteCalled = true
			pr.Out.Header.Set("X-Rewrite", "yes")
		},
		ModifyResponse: func(resp *http.Response) error {
			modifyCalled = true
			resp.Header.Set("X-Modified", "yes")
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodPost, "http://client.example/v1/chat/completions?foo=bar", strings.NewReader(`{}`))
	req.Header.Set("Authorization", "Bearer client-key")
	rec := httptest.NewRecorder()

	p.ServeHTTP(rec, req, coreproxy.Upstream{BaseURL: upstream.URL, APIKey: "provider-key"})

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
	p, err := New(Options{})
	if err != nil {
		t.Fatal(err)
	}

	tests := []string{"", "example.com", "http://", "http://example.com/base"}
	for _, baseURL := range tests {
		t.Run(baseURL, func(t *testing.T) {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
			p.ServeHTTP(rec, req, coreproxy.Upstream{BaseURL: baseURL, APIKey: "provider-key"})
			if rec.Code != http.StatusBadGateway {
				t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadGateway)
			}
		})
	}
}

func TestProxyServeHTTPReturnsBadGatewayForUpstreamError(t *testing.T) {
	p, err := New(Options{})
	if err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	p.ServeHTTP(rec, req, coreproxy.Upstream{BaseURL: "http://127.0.0.1:1", APIKey: "provider-key"})
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadGateway)
	}
}
