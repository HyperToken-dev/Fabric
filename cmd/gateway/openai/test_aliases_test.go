package openai

import (
	"context"

	"github.com/HyperToken-dev/fabric/business/sensitive"
	"github.com/HyperToken-dev/fabric/cmd/gateway/common"
)

type ModelInfo = common.ModelInfo
type UsageContext = common.UsageContext
type TextPolicy = common.TextPolicy

const (
	ModelStatusActive  = common.ModelStatusActive
	ModelStatusBanned  = common.ModelStatusBanned
	TextDirectionInput = common.TextDirectionInput
)

type detectorPolicy struct {
	detector *sensitive.Detector
}

func (p detectorPolicy) Detect(ctx context.Context, model, text string) sensitive.Result {
	return p.detector.Detect(model, text)
}
