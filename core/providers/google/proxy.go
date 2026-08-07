package google

import (
	"net/http"
	"net/http/httputil"
	"strings"

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
	pr.Out.URL.Path = normalizeOutboundPath(pr.In.URL.Path, pr.Out.URL.Path)
	pr.Out.URL.RawPath = ""
	return nil
}

func defaultAuthInjector(req *http.Request, upstream coreproxy.Upstream) {
	req.Header.Set("x-goog-api-key", upstream.APIKey)
	req.Header.Del("Authorization")
}

func normalizeOutboundPath(inputPath string, outputPath string) string {
	if inputPath == "" || inputPath == "/" || outputPath == "" || outputPath == "/" {
		return outputPath
	}
	if !strings.HasSuffix(outputPath, inputPath) {
		return outputPath
	}
	duplicatedPrefix := strings.TrimSuffix(outputPath, inputPath)
	if duplicatedPrefix == "" || duplicatedPrefix == "/" {
		return outputPath
	}
	if inputPath == duplicatedPrefix || strings.HasPrefix(inputPath, duplicatedPrefix+"/") {
		return inputPath
	}
	return outputPath
}
