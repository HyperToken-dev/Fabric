package openai

import (
	"context"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	coreproxy "fabric/core/proxy"
)

type contextKey string

const ctxUpstream contextKey = "upstream"

type ModifyResponseFunc func(resp *http.Response) error

type Proxy struct {
	proxy *httputil.ReverseProxy
}

type Options struct {
	ModifyResponse ModifyResponseFunc
}

func New(opts Options) *Proxy {
	director := func(req *http.Request) {
		upstream, ok := req.Context().Value(ctxUpstream).(coreproxy.Upstream)
		if !ok {
			return
		}

		target, ok := req.Context().Value(ctxUpstreamTarget).(*url.URL)
		if !ok {
			return
		}

		req.Header.Set("Authorization", "Bearer "+upstream.APIKey)
		req.URL.Scheme = target.Scheme
		req.URL.Host = target.Host
		req.Host = target.Host
	}

	return &Proxy{proxy: &httputil.ReverseProxy{
		Director:       director,
		ModifyResponse: opts.ModifyResponse,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			http.Error(w, "bad gateway", http.StatusBadGateway)
		},
	}}
}

const ctxUpstreamTarget contextKey = "upstream_target"

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request, upstream coreproxy.Upstream) {
	target, err := parseBaseURL(upstream.BaseURL)
	if err != nil {
		http.Error(w, "invalid upstream base url", http.StatusBadGateway)
		return
	}

	ctx := context.WithValue(r.Context(), ctxUpstream, upstream)
	ctx = context.WithValue(ctx, ctxUpstreamTarget, target)
	p.proxy.ServeHTTP(w, r.WithContext(ctx))
}

func parseBaseURL(baseURL string) (*url.URL, error) {
	target, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return nil, err
	}
	if target.Scheme == "" || target.Host == "" {
		return nil, url.InvalidHostError(baseURL)
	}
	if target.Path != "" && target.Path != "/" {
		return nil, url.InvalidHostError("base_url must not include path")
	}
	return target, nil
}
