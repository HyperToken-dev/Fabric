package alibaba

import (
	"net/http"
	"net/http/httputil"

	"github.com/HyperToken-dev/fabric/core/proxy"
)

const bailianVideoSynthesisPath = "/api/v1/services/aigc/video-generation/video-synthesis"

func New(opts proxy.Options) *proxy.Proxy {
	if opts.Rewrite == nil {
		opts.Rewrite = defaultRewrite
	}

	return proxy.New(opts)
}

func defaultRewrite(pr *httputil.ProxyRequest) error {
	if isBailianVideoSynthesisRequest(pr.Out) {
		pr.Out.Header.Set("X-DashScope-Async", "enable")
	}
	return nil
}

func isBailianVideoSynthesisRequest(r *http.Request) bool {
	return r.Method == http.MethodPost && r.URL.Path == bailianVideoSynthesisPath
}
