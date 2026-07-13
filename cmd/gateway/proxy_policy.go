package main

import (
	"context"
	"strings"
)

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

func detectPrompts(ctx context.Context, prompts []string, policy TextPolicy) bool {
	if policy == nil {
		policy = NoopTextPolicy{}
	}
	for _, prompt := range prompts {
		if strings.TrimSpace(prompt) == "" {
			continue
		}
		if policy.Rejects(ctx, prompt) {
			return true
		}
	}
	return false
}
