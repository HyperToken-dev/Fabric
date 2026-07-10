package sensitive

import "context"

type TextPolicy struct{}

func NewTextPolicy() TextPolicy {
	return TextPolicy{}
}

func (TextPolicy) Rejects(ctx context.Context, text string) bool {
	return DetectSensitiveWord(text)
}
