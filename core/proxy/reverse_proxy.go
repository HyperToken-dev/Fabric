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
	// ctxUpstream stores provider credentials for the request currently entering
	// httputil.ReverseProxy.
	ctxUpstream contextKey = "upstream"
	// ctxUpstreamTarget stores the parsed upstream base URL selected by ServeHTTP.
	ctxUpstreamTarget contextKey = "upstream_target"
)

// ErrRewriteFailed marks failures returned by a caller-provided RewriteFunc.
//
// httputil.ProxyRequest.Rewrite cannot return errors directly, so this package
// stores rewrite failures on the outbound request context and surfaces them from
// the transport layer with this sentinel attached.
var ErrRewriteFailed = errors.New("rewrite request failed")

// RewriteFunc customizes the outbound provider request after the upstream URL
// and authentication headers have been applied.
type RewriteFunc func(pr *httputil.ProxyRequest) error

// ModifyResponseFunc can inspect or replace the upstream response before it is
// copied to the client.
type ModifyResponseFunc func(resp *http.Response) error

// OnCompleteFunc observes a response after its body has been captured and
// decoded when possible.
//
// decodedBody is nil when the response is intentionally not captured, cannot be
// decoded, or uses an unsupported encoding. Implementations must not retain or
// mutate resp.Body because the proxy still owns response delivery.
type OnCompleteFunc func(resp *http.Response, decodedBody []byte)

// StreamTransformFunc optionally creates a per-response processor for SSE
// streams.
//
// Returning ok=false leaves the stream unmodified. Returning ok=true with a nil
// processor is treated as no transform by callers and should be avoided.
type StreamTransformFunc func(resp *http.Response) (StreamProcessor, bool, error)

// StreamProcessor transforms decoded streaming bytes before they are sent to the
// client.
//
// Concurrency: a processor instance is owned by one response stream and is
// called serially by the proxy. Implementations do not need to be safe for
// concurrent use unless they share state outside the instance.
type StreamProcessor interface {
	// Write consumes one decoded upstream chunk and may return replacement bytes.
	Write(chunk []byte) (StreamResult, error)
	// Finish flushes any buffered processor state after upstream reaches EOF.
	Finish() (StreamResult, error)
	// Close releases processor-owned resources. It may be called after Stop or
	// client-side close and must tolerate repeated lifecycle paths.
	Close() error
}

// StreamResult is the processor output for one streaming step.
type StreamResult struct {
	// Data is already decoded data that the proxy will re-encode for the client.
	Data []byte
	// Stop asks the proxy to stop reading upstream after Data is sent.
	Stop bool
}

// AuthInjector applies provider authentication to the outbound request.
type AuthInjector func(req *http.Request, upstream Upstream)

// ErrorHandler writes the downstream error response for proxy-level failures.
type ErrorHandler func(w http.ResponseWriter, r *http.Request, err error)

// Options configures the provider-neutral reverse proxy.
//
// Nil hooks use safe defaults: bearer authentication, DefaultModifyResponse, and
// a generic 502 error handler. Callers normally provide OnComplete for usage and
// audit logging, and StreamTransform when provider output needs inline safety
// filtering.
type Options struct {
	Rewrite         RewriteFunc
	ModifyResponse  ModifyResponseFunc
	OnComplete      OnCompleteFunc
	StreamTransform StreamTransformFunc
	AuthInjector    AuthInjector
	ErrorHandler    ErrorHandler
}

// Proxy routes one incoming request to the provider upstream selected by ServeHTTP.
//
// Proxy is safe for concurrent use after construction, following the concurrency
// guarantees of httputil.ReverseProxy. Per-request state is stored on request
// contexts rather than on the Proxy value.
type Proxy struct {
	proxy *httputil.ReverseProxy
}

// New constructs a provider-neutral reverse proxy with default hooks filled in.
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
		modifyResponse = DefaultModifyResponse(opts.OnComplete, opts.StreamTransform)
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

// ServeHTTP proxies r to upstream after validating and parsing upstream.BaseURL.
//
// The upstream value is scoped to the request context so a single Proxy instance
// can safely serve different provider channels concurrently.
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

// BearerAuth injects the upstream API key as an Authorization bearer token.
func BearerAuth(req *http.Request, upstream Upstream) {
	req.Header.Set("Authorization", "Bearer "+upstream.APIKey)
}

// NoopAuth intentionally leaves provider authentication unchanged.
func NoopAuth(req *http.Request, upstream Upstream) {}

// ParseBaseURL validates an upstream base URL used by the reverse proxy.
//
// The URL must include both scheme and host. Paths are allowed and are handled
// by httputil.ProxyRequest.SetURL when forwarding the incoming request path.
func ParseBaseURL(baseURL string) (*url.URL, error) {
	target, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil {
		return nil, err
	}
	if target.Scheme == "" || target.Host == "" {
		return nil, url.InvalidHostError(baseURL)
	}
	return target, nil
}

// defaultErrorHandler maps proxy errors to a Bad Gateway response.
func defaultErrorHandler(w http.ResponseWriter, r *http.Request, err error) {
	http.Error(w, err.Error(), http.StatusBadGateway)
}

// rewriteErrorTransport converts deferred RewriteFunc failures into transport
// errors so httputil.ReverseProxy can route them through ErrorHandler.
type rewriteErrorTransport struct {
	base http.RoundTripper
}

// RoundTrip aborts outbound requests that carry a rewrite error in context.
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
