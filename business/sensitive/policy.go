package sensitive

import "context"

type TextPolicy struct {
	detector *Detector
}

func NewTextPolicy(detector *Detector) TextPolicy {
	return TextPolicy{detector: detector}
}

func (p TextPolicy) Detect(ctx context.Context, model, text string) Result {
	return p.detector.Detect(model, text)
}
