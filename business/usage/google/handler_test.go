package google

import "testing"

func TestExtractInteractionParsesAllUsageFields(t *testing.T) {
	body := []byte(`{
		"usage": {
			"input_tokens_by_modality": [
				{"modality": "text", "tokens": 7},
				{"modality": "image", "tokens": 3}
			],
			"total_cached_tokens": 1,
			"total_input_tokens": 10,
			"total_output_tokens": 20,
			"total_thought_tokens": 22,
			"total_tokens": 53,
			"total_tool_use_tokens": 4
		}
	}`)

	parsed, err := ExtractInteraction(body)
	if err != nil {
		t.Fatalf("ExtractInteraction() error = %v", err)
	}
	if parsed.PromptTokens != 10 {
		t.Fatalf("PromptTokens = %d, want 10", parsed.PromptTokens)
	}
	if parsed.CompletionTokens != 20 {
		t.Fatalf("CompletionTokens = %d, want 20", parsed.CompletionTokens)
	}
	if parsed.CachedTokens != 1 {
		t.Fatalf("CachedTokens = %d, want 1", parsed.CachedTokens)
	}
	if parsed.ThoughtTokens != 22 {
		t.Fatalf("ThoughtTokens = %d, want 22", parsed.ThoughtTokens)
	}
	if parsed.TotalTokens != 53 {
		t.Fatalf("TotalTokens = %d, want 53", parsed.TotalTokens)
	}
	if parsed.ToolUseTokens != 4 {
		t.Fatalf("ToolUseTokens = %d, want 4", parsed.ToolUseTokens)
	}
	if len(parsed.InputTokensByModality) != 2 {
		t.Fatalf("len(InputTokensByModality) = %d, want 2", len(parsed.InputTokensByModality))
	}
	if parsed.InputTokensByModality[0].Modality != "text" || parsed.InputTokensByModality[0].Tokens != 7 {
		t.Fatalf("InputTokensByModality[0] = %+v", parsed.InputTokensByModality[0])
	}
	if parsed.InputTokensByModality[1].Modality != "image" || parsed.InputTokensByModality[1].Tokens != 3 {
		t.Fatalf("InputTokensByModality[1] = %+v", parsed.InputTokensByModality[1])
	}
}

func TestExtractInteractionMissingUsage(t *testing.T) {
	if _, err := ExtractInteraction([]byte(`{"object":"interaction"}`)); err == nil {
		t.Fatal("ExtractInteraction() error = nil, want error")
	}
}

func TestExtractInteractionMalformedJSON(t *testing.T) {
	if _, err := ExtractInteraction([]byte(`{"usage":`)); err == nil {
		t.Fatal("ExtractInteraction() error = nil, want error")
	}
}

func TestExtractInteractionRejectsNegativeTokenValue(t *testing.T) {
	if _, err := ExtractInteraction([]byte(`{"usage":{"total_input_tokens":-1}}`)); err == nil {
		t.Fatal("ExtractInteraction() error = nil, want error")
	}
}

func TestExtractInteractionRejectsNegativeModalityTokenValue(t *testing.T) {
	body := []byte(`{"usage":{"input_tokens_by_modality":[{"modality":"text","tokens":-1}]}}`)
	if _, err := ExtractInteraction(body); err == nil {
		t.Fatal("ExtractInteraction() error = nil, want error")
	}
}
