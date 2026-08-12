package extrotec

import (
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
)

func TestExtractPromptRequestExtractsPromptFields(t *testing.T) {
	tests := []struct {
		name string
		body string
		want []string
	}{
		{name: "prompt", body: `{"model":"MiniMax-H3","prompt":"hello"}`, want: []string{"hello"}},
		{name: "forward prompt", body: `{"model":"MiniMax-H3","forward_prompt":"forward"}`, want: []string{"forward"}},
		{name: "negative prompt", body: `{"model":"MiniMax-H3","negative_prompt":"negative"}`, want: []string{"negative"}},
		{name: "all prompt fields", body: `{"model":"MiniMax-H3","prompt":"hello","forward_prompt":"forward","negative_prompt":"negative"}`, want: []string{"hello", "forward", "negative"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/v1/video/generations", strings.NewReader(tt.body))
			got, err := ExtractPromptRequest(req)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got.Prompts, tt.want) {
				t.Fatalf("Prompts = %#v, want %#v", got.Prompts, tt.want)
			}
		})
	}
}

func TestParsePromptRequestExtractsPromptFields(t *testing.T) {
	body := []byte(`{"model":"MiniMax-H3","prompt":"hello","forward_prompt":"forward","negative_prompt":"negative"}`)
	got, err := ParsePromptRequest(body)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"hello", "forward", "negative"}
	if !reflect.DeepEqual(got.Prompts, want) {
		t.Fatalf("Prompts = %#v, want %#v", got.Prompts, want)
	}
}

func TestExtractPromptRequestIgnoresEmptyPromptFields(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{name: "missing", body: `{"model":"MiniMax-H3"}`},
		{name: "null", body: `{"model":"MiniMax-H3","prompt":null,"forward_prompt":null,"negative_prompt":null}`},
		{name: "empty", body: `{"model":"MiniMax-H3","prompt":"","forward_prompt":"","negative_prompt":""}`},
		{name: "blank", body: `{"model":"MiniMax-H3","prompt":"  ","forward_prompt":"\t","negative_prompt":"\n"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/v1/video/generations", strings.NewReader(tt.body))
			got, err := ExtractPromptRequest(req)
			if err != nil {
				t.Fatal(err)
			}
			if len(got.Prompts) != 0 {
				t.Fatalf("Prompts = %#v, want empty", got.Prompts)
			}
		})
	}
}

func TestExtractPromptRequestRestoresBody(t *testing.T) {
	body := `{"model":"MiniMax-H3","prompt":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/video/generations", strings.NewReader(body))
	if _, err := ExtractPromptRequest(req); err != nil {
		t.Fatal(err)
	}

	restored, err := io.ReadAll(req.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(restored) != body {
		t.Fatalf("restored body = %q, want %q", restored, body)
	}
}

func TestExtractPromptRequestRejectsInvalidJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/video/generations", strings.NewReader(`{`))
	if _, err := ExtractPromptRequest(req); err == nil {
		t.Fatal("expected invalid JSON error")
	}
}
