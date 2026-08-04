package google

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/HyperToken-dev/fabric/business/usage"
)

var errMissingGoogleUsage = errors.New("missing google usage")

type interactionResponse struct {
	Usage *interactionUsage `json:"usage"`
}

type interactionUsage struct {
	InputTokensByModality []modalityTokenUsage `json:"input_tokens_by_modality"`
	TotalCachedTokens     int64                `json:"total_cached_tokens"`
	TotalInputTokens      int64                `json:"total_input_tokens"`
	TotalOutputTokens     int64                `json:"total_output_tokens"`
	TotalThoughtTokens    int64                `json:"total_thought_tokens"`
	TotalTokens           int64                `json:"total_tokens"`
	TotalToolUseTokens    int64                `json:"total_tool_use_tokens"`
}

type modalityTokenUsage struct {
	Modality string `json:"modality"`
	Tokens   int64  `json:"tokens"`
}

func ExtractInteraction(rawBody []byte) (*usage.Usage, error) {
	var resp interactionResponse
	if err := json.Unmarshal(rawBody, &resp); err != nil {
		return nil, err
	}
	if resp.Usage == nil {
		return nil, errMissingGoogleUsage
	}
	return googleUsageToUsage(resp.Usage)
}

func googleUsageToUsage(parsed *interactionUsage) (*usage.Usage, error) {
	if err := validateNonNegative("total_cached_tokens", parsed.TotalCachedTokens); err != nil {
		return nil, err
	}
	if err := validateNonNegative("total_input_tokens", parsed.TotalInputTokens); err != nil {
		return nil, err
	}
	if err := validateNonNegative("total_output_tokens", parsed.TotalOutputTokens); err != nil {
		return nil, err
	}
	if err := validateNonNegative("total_thought_tokens", parsed.TotalThoughtTokens); err != nil {
		return nil, err
	}
	if err := validateNonNegative("total_tokens", parsed.TotalTokens); err != nil {
		return nil, err
	}
	if err := validateNonNegative("total_tool_use_tokens", parsed.TotalToolUseTokens); err != nil {
		return nil, err
	}

	modalityTokens := make([]usage.ModalityTokenUsage, 0, len(parsed.InputTokensByModality))
	for index, modality := range parsed.InputTokensByModality {
		if err := validateNonNegative(fmt.Sprintf("input_tokens_by_modality[%d].tokens", index), modality.Tokens); err != nil {
			return nil, err
		}
		modalityTokens = append(modalityTokens, usage.ModalityTokenUsage{
			Modality: modality.Modality,
			Tokens:   modality.Tokens,
		})
	}

	return &usage.Usage{
		PromptTokens:          parsed.TotalInputTokens,
		CompletionTokens:      parsed.TotalOutputTokens,
		CachedTokens:          parsed.TotalCachedTokens,
		ThoughtTokens:         parsed.TotalThoughtTokens,
		ToolUseTokens:         parsed.TotalToolUseTokens,
		TotalTokens:           parsed.TotalTokens,
		InputTokensByModality: modalityTokens,
	}, nil
}

func validateNonNegative(field string, value int64) error {
	if value < 0 {
		return fmt.Errorf("negative google usage token count %s: %d", field, value)
	}
	return nil
}
