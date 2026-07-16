package main

import (
	"context"
	"strings"

	"github.com/HyperToken-dev/fabric/business/sensitive"

	"go.uber.org/zap"
)

type TextDirection string

const (
	TextDirectionInput  TextDirection = "input"
	TextDirectionOutput TextDirection = "output"
)

// allow or reject a request that send to upstream provider or vice versa
type TextPolicy interface {
	Detect(ctx context.Context, model, text string) sensitive.Result
}

// Default implements
type NoopTextPolicy struct{}

func (NoopTextPolicy) Detect(ctx context.Context, model, text string) sensitive.Result {
	return sensitive.Result{}
}

func detectPrompts(ctx context.Context, model string, direction TextDirection, prompts []string, policy TextPolicy) bool {
	if policy == nil {
		policy = NoopTextPolicy{}
	}
	for _, prompt := range prompts {
		if strings.TrimSpace(prompt) == "" {
			continue
		}
		result := policy.Detect(ctx, model, prompt)
		if result.Rejected() {
			zap.L().Info("sensitive text rejected",
				zap.String("direction", string(direction)),
				zap.String("model", model),
				zap.String("text", prompt),
				zap.Any("matches", result.Matches),
			)
			return true
		}
	}
	return false
}
