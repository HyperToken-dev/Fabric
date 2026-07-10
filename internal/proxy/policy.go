package proxy

import "context"

// allow or reject a request that send to upstream provider or vice versa
type TextPolicy interface {
	// when return true, reject the req
	Rejects(ctx context.Context, text string) bool
}

// Default implements
type NoopTextPolicy struct{}

func (NoopTextPolicy) Rejects(ctx context.Context, text string) bool {
	return false
}
