package alibaba

import "github.com/HyperToken-dev/fabric/core/proxy"

func New(opts proxy.Options) (*proxy.Proxy, error) {
	proxy := proxy.New(opts)
	return proxy, nil
}
