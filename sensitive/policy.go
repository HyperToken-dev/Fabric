package sensitive

import "context"

type TextPolicy struct {
	detector *Detector
}

func NewTextPolicy(detector *Detector) TextPolicy {
	return TextPolicy{detector: detector}
}

func (p TextPolicy) Rejects(ctx context.Context, text string) bool {
	return p.detector.Detect(text)
}
