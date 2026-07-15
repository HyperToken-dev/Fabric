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

type RewriteFunc func(pr *httputil.ProxyRequest)

type ModifyResponseFunc func(resp *http.Response) error

type Proxy struct {
	proxy *httputil.ReverseProxy
}

type Options struct {
	Rewrite        RewriteFunc
	ModifyResponse ModifyResponseFunc
}

func New(opts Options) (*Proxy, error) {
	rewrite := opts.Rewrite
	return &Proxy{proxy: &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			upstream, ok := pr.In.Context().Value(ctxUpstream).(coreproxy.Upstream)
			if !ok {
				return
			}
			target, ok := pr.In.Context().Value(ctxUpstreamTarget).(*url.URL)
			if !ok {
				return
			}

			pr.SetURL(target)
			pr.Out.Host = target.Host
			pr.Out.Header.Set("Authorization", "Bearer "+upstream.APIKey)

			if rewrite != nil {
				rewrite(pr)
			}
		},
		ModifyResponse: opts.ModifyResponse,
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			http.Error(w, err.Error(), http.StatusBadGateway)
		},
	}}, nil
}

const ctxUpstreamTarget contextKey = "upstream_target"

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request, upstream coreproxy.Upstream) {
	target, err := ParseBaseURL(upstream.BaseURL)
	if err != nil {
		http.Error(w, "invalid upstream base url: "+err.Error(), http.StatusBadGateway)
		return
	}

	ctx := context.WithValue(r.Context(), ctxUpstream, upstream)
	ctx = context.WithValue(ctx, ctxUpstreamTarget, target)
	p.proxy.ServeHTTP(w, r.WithContext(ctx))
}

func ParseBaseURL(baseURL string) (*url.URL, error) {
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
