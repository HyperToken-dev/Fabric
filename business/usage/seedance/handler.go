package seedance

import (
	"encoding/json"

	"github.com/HyperToken-dev/fabric/business/usage"
)

type taskUsageResponse struct {
	Usage struct {
		CompletionTokens int64 `json:"completion_tokens"`
	} `json:"usage"`
}

func ExtractTaskUsage(body []byte) (*usage.Usage, error) {
	var resp taskUsageResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}
	if resp.Usage.CompletionTokens <= 0 {
		return nil, nil
	}
	return &usage.Usage{CompletionTokens: resp.Usage.CompletionTokens}, nil
}
