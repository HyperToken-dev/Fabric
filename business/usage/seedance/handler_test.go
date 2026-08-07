package seedance

import "testing"

func TestExtractTaskUsage(t *testing.T) {
	tests := []struct {
		name           string
		body           []byte
		wantUsage      bool
		wantPrompt     int64
		wantCompletion int64
		wantErr        bool
	}{
		{
			name:           "completion tokens present",
			body:           []byte(`{"usage":{"completion_tokens":123}}`),
			wantUsage:      true,
			wantPrompt:     0,
			wantCompletion: 123,
		},
		{
			name: "ignores total tokens",
			body: []byte(`{"usage":{"total_tokens":123}}`),
		},
		{
			name: "missing usage",
			body: []byte(`{"id":"task-1","status":"success"}`),
		},
		{
			name: "zero completion tokens",
			body: []byte(`{"usage":{"completion_tokens":0}}`),
		},
		{
			name: "negative completion tokens",
			body: []byte(`{"usage":{"completion_tokens":-1}}`),
		},
		{
			name:    "invalid json",
			body:    []byte(`{"usage":`),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractTaskUsage(tt.body)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("ExtractTaskUsage() error = %v", err)
			}
			if !tt.wantUsage {
				if got != nil {
					t.Fatalf("ExtractTaskUsage() = %#v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatal("ExtractTaskUsage() = nil, want usage")
			}
			if got.PromptTokens != tt.wantPrompt || got.CompletionTokens != tt.wantCompletion {
				t.Fatalf("ExtractTaskUsage() = %#v, want prompt=%d completion=%d", got, tt.wantPrompt, tt.wantCompletion)
			}
		})
	}
}
