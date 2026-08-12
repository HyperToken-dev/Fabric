package openai

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestSSEParserEmitsEventsSplitAcrossChunks(t *testing.T) {
	var parser SSEParser
	events, err := parser.Write([]byte("event: message\ndata: {\"choices\""))
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 0 {
		t.Fatalf("events = %d, want 0", len(events))
	}
	events, err = parser.Write([]byte(":[{\"delta\":{\"content\":\"hi\"}}]}\n\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 1 {
		t.Fatalf("events = %d, want 1", len(events))
	}
	if events[0].Event != "message" {
		t.Fatalf("event = %q, want message", events[0].Event)
	}
	if !strings.Contains(events[0].Data, `"content":"hi"`) {
		t.Fatalf("data = %q", events[0].Data)
	}
}

func TestSSEParserHandlesCommentsAndDone(t *testing.T) {
	var parser SSEParser
	events, err := parser.Write([]byte(": keep-alive\n\ndata: [DONE]\n\n"))
	if err != nil {
		t.Fatal(err)
	}
	if len(events) != 2 {
		t.Fatalf("events = %d, want 2", len(events))
	}
	if _, ok, err := ExtractChatCompletionStreamText(events[0]); err != nil || ok {
		t.Fatalf("comment extraction ok = %v, err = %v; want no text", ok, err)
	}
	if !events[1].Done() {
		t.Fatal("second event should be DONE")
	}
}

func TestExtractChatCompletionStreamText(t *testing.T) {
	event := SSEEvent{Data: `{"choices":[{"delta":{"content":"hello"}}]}`}
	text, ok, err := ExtractChatCompletionStreamText(event)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || text != "hello" {
		t.Fatalf("text = %q, ok = %v; want hello true", text, ok)
	}

	noText := SSEEvent{Data: `{"choices":[{"delta":{"role":"assistant"}}]}`}
	text, ok, err = ExtractChatCompletionStreamText(noText)
	if err != nil {
		t.Fatal(err)
	}
	if ok || text != "" {
		t.Fatalf("text = %q, ok = %v; want empty false", text, ok)
	}
}

func TestRewriteChatCompletionStreamText(t *testing.T) {
	event := SSEEvent{Data: `{"id":"chatcmpl","choices":[{"index":0,"delta":{"content":"unsafe"}}]}`}
	rewritten, err := RewriteChatCompletionStreamText(event, "safe")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(rewritten), "data: ") {
		t.Fatalf("rewritten event = %q", rewritten)
	}
	data := strings.TrimSpace(strings.TrimPrefix(string(rewritten), "data: "))
	var payload struct {
		Choices []struct {
			Delta struct {
				Content string `json:"content"`
			} `json:"delta"`
		} `json:"choices"`
	}
	if err := json.Unmarshal([]byte(data), &payload); err != nil {
		t.Fatal(err)
	}
	if got := payload.Choices[0].Delta.Content; got != "safe" {
		t.Fatalf("content = %q, want safe", got)
	}
}

func TestRewriteChatCompletionStreamTextSkipsEmptySafeText(t *testing.T) {
	rewritten, err := RewriteChatCompletionStreamText(SSEEvent{Data: `{"choices":[{"delta":{"content":"held"}}]}`}, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(rewritten) != 0 {
		t.Fatalf("rewritten = %q, want empty", rewritten)
	}
}

func TestExtractChatCompletionStreamTextInvalidJSON(t *testing.T) {
	_, _, err := ExtractChatCompletionStreamText(SSEEvent{Data: `{`})
	if err == nil {
		t.Fatal("expected error")
	}
}
