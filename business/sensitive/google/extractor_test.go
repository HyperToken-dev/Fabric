package google

import (
	"io"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestExtractPromptRequestExtractsUserInputAndModelOutputText(t *testing.T) {
	req := httptest.NewRequest("POST", "/v1beta/interactions", strings.NewReader(`{
		"model":"gemini-3-flash-preview",
		"input":[
			{"type":"user_input","content":[{"type":"text","text":"Hello!"}]},
			{"type":"model_output","content":[{"type":"text","text":"Hi there!"}]},
			{"type":"user_input","content":[{"type":"text","text":"What is the capital of France?"}]}
		]
	}`))

	parsed, err := ExtractPromptRequest(req)
	if err != nil {
		t.Fatalf("ExtractPromptRequest() error = %v", err)
	}
	if parsed.Model != "gemini-3-flash-preview" {
		t.Fatalf("Model = %q, want gemini-3-flash-preview", parsed.Model)
	}
	want := []string{"Hello!", "Hi there!", "What is the capital of France?"}
	if len(parsed.Prompts) != len(want) {
		t.Fatalf("len(Prompts) = %d, want %d", len(parsed.Prompts), len(want))
	}
	for i := range want {
		if parsed.Prompts[i] != want[i] {
			t.Fatalf("Prompts[%d] = %q, want %q", i, parsed.Prompts[i], want[i])
		}
	}
}

func TestExtractPromptRequestExtractsSimpleStringInput(t *testing.T) {
	req := httptest.NewRequest("POST", "/v1beta/interactions", strings.NewReader(`{
		"model":"gemini-3-flash-preview",
		"input":"Hello, how are you?"
	}`))

	parsed, err := ExtractPromptRequest(req)
	if err != nil {
		t.Fatalf("ExtractPromptRequest() error = %v", err)
	}
	if parsed.Model != "gemini-3-flash-preview" {
		t.Fatalf("Model = %q, want gemini-3-flash-preview", parsed.Model)
	}
	if len(parsed.Prompts) != 1 || parsed.Prompts[0] != "Hello, how are you?" {
		t.Fatalf("Prompts = %#v, want simple input text", parsed.Prompts)
	}
}

func TestExtractPromptRequestIgnoresBlankSimpleStringInput(t *testing.T) {
	req := httptest.NewRequest("POST", "/v1beta/interactions", strings.NewReader(`{
		"model":"gemini-3-flash-preview",
		"input":" "
	}`))

	parsed, err := ExtractPromptRequest(req)
	if err != nil {
		t.Fatalf("ExtractPromptRequest() error = %v", err)
	}
	if len(parsed.Prompts) != 0 {
		t.Fatalf("Prompts = %#v, want empty", parsed.Prompts)
	}
}

func TestExtractPromptRequestIgnoresNonTextEmptyAndUnsupportedItems(t *testing.T) {
	req := httptest.NewRequest("POST", "/v1beta/interactions", strings.NewReader(`{
		"model":"gemini-3-flash-preview",
		"input":[
			{"type":"user_input","content":[{"type":"image","text":"ignored"},{"type":"text","text":" "},{"type":"text","text":"kept"}]},
			{"type":"system","content":[{"type":"text","text":"ignored system"}]}
		]
	}`))

	parsed, err := ExtractPromptRequest(req)
	if err != nil {
		t.Fatalf("ExtractPromptRequest() error = %v", err)
	}
	if len(parsed.Prompts) != 1 || parsed.Prompts[0] != "kept" {
		t.Fatalf("Prompts = %#v, want [kept]", parsed.Prompts)
	}
}

func TestExtractPromptRequestRestoresBody(t *testing.T) {
	body := `{"model":"gemini-3-flash-preview","input":[{"type":"user_input","content":[{"type":"text","text":"Hello!"}]}]}`
	req := httptest.NewRequest("POST", "/v1beta/interactions", strings.NewReader(body))

	if _, err := ExtractPromptRequest(req); err != nil {
		t.Fatalf("ExtractPromptRequest() error = %v", err)
	}
	restored, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(restored) != body {
		t.Fatalf("restored body = %q, want %q", string(restored), body)
	}
	if req.GetBody == nil {
		t.Fatal("GetBody is nil")
	}
	second, err := req.GetBody()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := io.ReadAll(second)
	closeErr := second.Close()
	if err != nil {
		t.Fatal(err)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	if string(raw) != body {
		t.Fatalf("GetBody body = %q, want %q", string(raw), body)
	}
}

func TestExtractPromptRequestMalformedJSON(t *testing.T) {
	req := httptest.NewRequest("POST", "/v1beta/interactions", strings.NewReader(`{"model":`))
	if _, err := ExtractPromptRequest(req); err == nil {
		t.Fatal("ExtractPromptRequest() error = nil, want error")
	}
}

func TestExtractPromptRequestRejectsUnsupportedInputShape(t *testing.T) {
	req := httptest.NewRequest("POST", "/v1beta/interactions", strings.NewReader(`{
		"model":"gemini-3-flash-preview",
		"input":{"text":"Hello"}
	}`))
	if _, err := ExtractPromptRequest(req); err == nil {
		t.Fatal("ExtractPromptRequest() error = nil, want error")
	}
}
