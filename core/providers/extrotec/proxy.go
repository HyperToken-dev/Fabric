package extrotec

import (
	coreproxy "github.com/HyperToken-dev/fabric/core/proxy"
)

func New(opts coreproxy.Options) *coreproxy.Proxy {
	return coreproxy.New(opts)
}
