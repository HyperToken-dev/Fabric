package google

import (
	"net/http"
	"net/http/httputil"

	coreproxy "github.com/HyperToken-dev/fabric/core/proxy"
)

func New(opts coreproxy.Options) *coreproxy.Proxy {
	if opts.Rewrite == nil {
		opts.Rewrite = defaultRewrite
	}
	if opts.AuthInjector == nil {
		opts.AuthInjector = defaultAuthInjector
	}
	return coreproxy.New(opts)
}

func defaultRewrite(pr *httputil.ProxyRequest) error {
	return nil
}

func defaultAuthInjector(req *http.Request, upstream coreproxy.Upstream) {
	req.Header.Set("x-goog-api-key", upstream.APIKey)
}
