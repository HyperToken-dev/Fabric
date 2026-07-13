package openai

import (
	"io"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestParseOpenAIChatPromptRequest(t *testing.T) {
	body := `{
		"model":"gpt-4.1",
		"messages":[
			{"role":"system","content":"system prompt"},
			{"role":"assistant","content":"assistant history"},
			{"role":"user","content":[{"type":"text","text":"hello"},{"type":"image_url","image_url":{"url":"https://example.com/a.png"}}]}
		]
	}`
	req := httptest.NewRequest("POST", "/v1/chat/completions", strings.NewReader(body))

	parsed, err := ExtractPromptRequest(req)
	if err != nil {
		t.Fatalf("parseOpenAIPromptRequest returned error: %v", err)
	}
	if parsed.Model != "gpt-4.1" {
		t.Fatalf("model = %q, want %q", parsed.Model, "gpt-4.1")
	}
	wantPrompts := []string{"system prompt", "assistant history", "hello"}
	if !reflect.DeepEqual(parsed.Prompts, wantPrompts) {
		t.Fatalf("prompts = %#v, want %#v", parsed.Prompts, wantPrompts)
	}

	restored, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatalf("read restored body: %v", err)
	}
	if string(restored) != body {
		t.Fatalf("restored body = %q, want %q", string(restored), body)
	}
}

func TestParseOpenAIResponsesPromptRequest(t *testing.T) {
	body := `{
		"model":"gpt-4.1-mini",
		"instructions":"follow the policy",
		"input":[
			{"role":"user","content":[{"type":"input_text","text":"first input"}]},
			{"role":"assistant","content":"second input"},
			{"type":"input_text","text":"direct input text"}
		]
	}`
	req := httptest.NewRequest("POST", "/v1/responses", strings.NewReader(body))

	parsed, err := ExtractPromptRequest(req)
	if err != nil {
		t.Fatalf("parseOpenAIPromptRequest returned error: %v", err)
	}
	if parsed.Model != "gpt-4.1-mini" {
		t.Fatalf("model = %q, want %q", parsed.Model, "gpt-4.1-mini")
	}
	wantPrompts := []string{"follow the policy", "first input", "second input", "direct input text"}
	if !reflect.DeepEqual(parsed.Prompts, wantPrompts) {
		t.Fatalf("prompts = %#v, want %#v", parsed.Prompts, wantPrompts)
	}
}

func TestParseOpenAIResponsesPromptRequestWithStringInput(t *testing.T) {
	body := `{"model":"gpt-4.1-mini","input":"plain input"}`
	req := httptest.NewRequest("POST", "/v1/responses", strings.NewReader(body))

	parsed, err := ExtractPromptRequest(req)
	if err != nil {
		t.Fatalf("parseOpenAIPromptRequest returned error: %v", err)
	}
	wantPrompts := []string{"plain input"}
	if !reflect.DeepEqual(parsed.Prompts, wantPrompts) {
		t.Fatalf("prompts = %#v, want %#v", parsed.Prompts, wantPrompts)
	}
}

func TestParseOpenAIPromptRequestInvalidJSON(t *testing.T) {
	req := httptest.NewRequest("POST", "/v1/responses", strings.NewReader(`{`))

	_, err := ExtractPromptRequest(req)
	if err == nil {
		t.Fatal("parseOpenAIPromptRequest returned nil error, want error")
	}
}

func TestExtractOpenAIChatOutputTexts(t *testing.T) {
	body := []byte(`{
		"choices":[
			{"message":{"role":"assistant","content":"model answer"}},
			{"message":{"role":"assistant","content":[{"type":"text","text":"second answer"}]}}
		]
	}`)
	req := httptest.NewRequest("POST", "/v1/chat/completions", nil)

	texts, err := ExtractOutputTexts(req, body)
	if err != nil {
		t.Fatalf("extractOpenAIOutputTexts returned error: %v", err)
	}
	wantTexts := []string{"model answer", "second answer"}
	if !reflect.DeepEqual(texts, wantTexts) {
		t.Fatalf("texts = %#v, want %#v", texts, wantTexts)
	}
}

func TestExtractOpenAIResponsesOutputTexts(t *testing.T) {
	body := []byte(`{
		"output_text":"summary answer",
		"output":[
			{"content":[{"type":"output_text","text":"first answer"},{"type":"refusal","text":"refusal text"}]}
		]
	}`)
	req := httptest.NewRequest("POST", "/v1/responses", nil)

	texts, err := ExtractOutputTexts(req, body)
	if err != nil {
		t.Fatalf("extractOpenAIOutputTexts returned error: %v", err)
	}
	wantTexts := []string{"summary answer", "first answer", "refusal text"}
	if !reflect.DeepEqual(texts, wantTexts) {
		t.Fatalf("texts = %#v, want %#v", texts, wantTexts)
	}
}

func TestExtractOpenAIOutputTextsInvalidJSON(t *testing.T) {
	req := httptest.NewRequest("POST", "/v1/chat/completions", nil)

	_, err := ExtractOutputTexts(req, []byte(`{`))
	if err == nil {
		t.Fatal("extractOpenAIOutputTexts returned nil error, want error")
	}
}
