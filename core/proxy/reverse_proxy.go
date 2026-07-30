package proxy

import (
	"context"
	"errors"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
)

type contextKey string

const (
	ctxUpstream       contextKey = "upstream"
	ctxUpstreamTarget contextKey = "upstream_target"
)

var ErrRewriteFailed = errors.New("rewrite request failed")

type RewriteFunc func(pr *httputil.ProxyRequest) error

type ModifyResponseFunc func(resp *http.Response) error

type OnCompleteFunc func(resp *http.Response, decodedBody []byte)

type AuthInjector func(req *http.Request, upstream Upstream)

type ErrorHandler func(w http.ResponseWriter, r *http.Request, err error)

type Options struct {
	Rewrite        RewriteFunc
	ModifyResponse ModifyResponseFunc
	OnComplete     OnCompleteFunc
	AuthInjector   AuthInjector
	ErrorHandler   ErrorHandler
}

type Proxy struct {
	proxy *httputil.ReverseProxy
}

func New(opts Options) *Proxy {
	authInjector := opts.AuthInjector
	if authInjector == nil {
		authInjector = BearerAuth
	}
	errorHandler := opts.ErrorHandler
	if errorHandler == nil {
		errorHandler = defaultErrorHandler
	}

	modifyResponse := opts.ModifyResponse
	if modifyResponse == nil {
		modifyResponse = DefaultModifyResponse(opts.OnComplete)
	}

	return &Proxy{proxy: &httputil.ReverseProxy{
		Rewrite: func(pr *httputil.ProxyRequest) {
			upstream, ok := pr.In.Context().Value(ctxUpstream).(Upstream)
			if !ok {
				return
			}
			target, ok := pr.In.Context().Value(ctxUpstreamTarget).(*url.URL)
			if !ok {
				return
			}

			pr.SetURL(target)
			pr.Out.Host = target.Host
			authInjector(pr.Out, upstream)

			if opts.Rewrite != nil {
				if err := opts.Rewrite(pr); err != nil {
					pr.Out = pr.Out.WithContext(context.WithValue(pr.Out.Context(), ctxRewriteError, err))
				}
			}
		},
		ModifyResponse: modifyResponse,
		Transport:      rewriteErrorTransport{base: http.DefaultTransport},
		ErrorHandler:   errorHandler,
	}}
}

const ctxRewriteError contextKey = "rewrite_error"

func (p *Proxy) ServeHTTP(w http.ResponseWriter, r *http.Request, upstream Upstream) {
	target, err := ParseBaseURL(upstream.BaseURL)
	if err != nil {
		http.Error(w, "invalid upstream base url: "+err.Error(), http.StatusBadGateway)
		return
	}

	ctx := context.WithValue(r.Context(), ctxUpstream, upstream)
	ctx = context.WithValue(ctx, ctxUpstreamTarget, target)
	p.proxy.ServeHTTP(w, r.WithContext(ctx))
}

func BearerAuth(req *http.Request, upstream Upstream) {
	req.Header.Set("Authorization", "Bearer "+upstream.APIKey)
}

func NoopAuth(req *http.Request, upstream Upstream) {}

func ParseBaseURL(baseURL string) (*url.URL, error) {
	target, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return nil, err
	}
	if target.Scheme == "" || target.Host == "" {
		return nil, url.InvalidHostError(baseURL)
	}
	if strings.HasSuffix(target.Path, "/") {
		return nil, url.InvalidHostError("base_url must not end with slash")
	}
	return target, nil
}

func defaultErrorHandler(w http.ResponseWriter, r *http.Request, err error) {
	http.Error(w, err.Error(), http.StatusBadGateway)
}

type rewriteErrorTransport struct {
	base http.RoundTripper
}

func (t rewriteErrorTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if err, ok := req.Context().Value(ctxRewriteError).(error); ok && err != nil {
		return nil, errors.Join(ErrRewriteFailed, err)
	}
	base := t.base
	if base == nil {
		base = http.DefaultTransport
	}
	return base.RoundTrip(req)
}
