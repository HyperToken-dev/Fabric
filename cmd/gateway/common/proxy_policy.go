package common

import (
	"context"
	"strings"

	"github.com/HyperToken-dev/fabric/business/sensitive"

	"go.uber.org/zap"
)

// TextDirection labels whether sensitive-word detection is checking inbound or outbound text.
type TextDirection string

const (
	TextDirectionInput  TextDirection = "input"
	TextDirectionOutput TextDirection = "output"
)

// TextPolicy evaluates provider text against the reloadable sensitive-word policy.
type TextPolicy interface {
	Detect(ctx context.Context, model, text string) sensitive.Result
}

// NoopTextPolicy allows providers to run without sensitive-word enforcement.
type NoopTextPolicy struct{}

func (NoopTextPolicy) Detect(ctx context.Context, model, text string) sensitive.Result {
	return sensitive.Result{}
}

// DetectPrompts returns true when any non-empty prompt is rejected for the model.
func DetectPrompts(ctx context.Context, model string, direction TextDirection, prompts []string, policy TextPolicy) bool {
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
