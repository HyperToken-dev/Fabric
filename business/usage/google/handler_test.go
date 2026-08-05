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

func TestExtractInteractionStreamingParsesCompletedEventUsage(t *testing.T) {
	body := []byte(`event: interaction.delta
data: {"event_type":"interaction.delta"}

event: interaction.completed
data:{
    "interaction": {
        "id": "v1_Chd0cGh5YXVTX0hkZUIycm9QM2NqdG9BcxIXdHBoeWF1U19IZGVCMnJvUDNjanRvQXM",
        "status": "completed",
        "usage": {
            "total_tokens": 64,
            "total_input_tokens": 22,
            "input_tokens_by_modality": [
                {
                    "modality": "text",
                    "tokens": 22
                }
            ],
            "total_cached_tokens": 0,
            "total_output_tokens": 8,
            "total_tool_use_tokens": 0,
            "total_thought_tokens": 34
        },
        "created": "2026-08-05T01:58:29Z",
        "updated": "2026-08-05T01:58:29Z",
        "service_tier": "standard",
        "object": "interaction",
        "model": "gemini-3-flash-preview"
    },
    "event_type": "interaction.completed"
}

data: [DONE]

`)

	parsed, err := ExtractInteractionStreaming(body)
	if err != nil {
		t.Fatalf("ExtractInteractionStreaming() error = %v", err)
	}
	if parsed.PromptTokens != 22 {
		t.Fatalf("PromptTokens = %d, want 22", parsed.PromptTokens)
	}
	if parsed.CompletionTokens != 8 {
		t.Fatalf("CompletionTokens = %d, want 8", parsed.CompletionTokens)
	}
	if parsed.ThoughtTokens != 34 {
		t.Fatalf("ThoughtTokens = %d, want 34", parsed.ThoughtTokens)
	}
	if parsed.TotalTokens != 64 {
		t.Fatalf("TotalTokens = %d, want 64", parsed.TotalTokens)
	}
	if len(parsed.InputTokensByModality) != 1 {
		t.Fatalf("len(InputTokensByModality) = %d, want 1", len(parsed.InputTokensByModality))
	}
	if parsed.InputTokensByModality[0].Modality != "text" || parsed.InputTokensByModality[0].Tokens != 22 {
		t.Fatalf("InputTokensByModality[0] = %+v", parsed.InputTokensByModality[0])
	}
}

func TestExtractInteractionMissingUsage(t *testing.T) {
	if _, err := ExtractInteraction([]byte(`{"object":"interaction"}`)); err == nil {
		t.Fatal("ExtractInteraction() error = nil, want error")
	}
}

func TestExtractInteractionStreamingMissingUsage(t *testing.T) {
	body := []byte("event: interaction.completed\ndata: {\"interaction\":{},\"event_type\":\"interaction.completed\"}\n\ndata: [DONE]\n\n")
	if _, err := ExtractInteractionStreaming(body); err == nil {
		t.Fatal("ExtractInteractionStreaming() error = nil, want error")
	}
}

func TestExtractInteractionStreamingIgnoresDone(t *testing.T) {
	if _, err := ExtractInteractionStreaming([]byte("data: [DONE]\n\n")); err == nil {
		t.Fatal("ExtractInteractionStreaming() error = nil, want error")
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

func TestExtractInteractionStreamingRejectsNegativeTokenValue(t *testing.T) {
	body := []byte("event: interaction.completed\ndata: {\"interaction\":{\"usage\":{\"total_input_tokens\":-1}},\"event_type\":\"interaction.completed\"}\n\n")
	if _, err := ExtractInteractionStreaming(body); err == nil {
		t.Fatal("ExtractInteractionStreaming() error = nil, want error")
	}
}
