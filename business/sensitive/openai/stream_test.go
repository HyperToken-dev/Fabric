package openai

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/HyperToken-dev/fabric/protocol/sse"
)

func TestExtractChatCompletionStreamText(t *testing.T) {
	event := sse.Event{Data: `{"choices":[{"delta":{"content":"hello"}}]}`}
	text, ok, err := ExtractChatCompletionStreamText(event)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || text != "hello" {
		t.Fatalf("text = %q, ok = %v; want hello true", text, ok)
	}

	noText := sse.Event{Data: `{"choices":[{"delta":{"role":"assistant"}}]}`}
	text, ok, err = ExtractChatCompletionStreamText(noText)
	if err != nil {
		t.Fatal(err)
	}
	if ok || text != "" {
		t.Fatalf("text = %q, ok = %v; want empty false", text, ok)
	}
}

func TestRewriteChatCompletionStreamText(t *testing.T) {
	event := sse.Event{Data: `{"id":"chatcmpl","choices":[{"index":0,"delta":{"content":"unsafe"}}]}`}
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
	rewritten, err := RewriteChatCompletionStreamText(sse.Event{Data: `{"choices":[{"delta":{"content":"held"}}]}`}, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(rewritten) != 0 {
		t.Fatalf("rewritten = %q, want empty", rewritten)
	}
}

func TestExtractChatCompletionStreamTextInvalidJSON(t *testing.T) {
	_, _, err := ExtractChatCompletionStreamText(sse.Event{Data: `{`})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestExtractResponsesStreamTextDelta(t *testing.T) {
	event := sse.Event{Event: "response.output_text.delta", Data: `{"type":"response.output_text.delta","delta":"Hi"}`}
	text, ok, err := ExtractResponsesStreamText(event)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || text.Kind != StreamTextDelta || text.Text != "Hi" {
		t.Fatalf("text = %+v, ok = %v; want delta Hi", text, ok)
	}
}

func TestExtractResponsesStreamTextSnapshots(t *testing.T) {
	tests := []struct {
		name  string
		event sse.Event
	}{
		{name: "output text done", event: sse.Event{Event: "response.output_text.done", Data: `{"type":"response.output_text.done","text":"Hi there"}`}},
		{name: "content part done", event: sse.Event{Event: "response.content_part.done", Data: `{"type":"response.content_part.done","part":{"type":"output_text","text":"Hi there"}}`}},
		{name: "output item done", event: sse.Event{Event: "response.output_item.done", Data: `{"type":"response.output_item.done","item":{"content":[{"type":"output_text","text":"Hi there"}]}}`}},
		{name: "completed", event: sse.Event{Event: "response.completed", Data: `{"type":"response.completed","response":{"output_text":"Hi there"}}`}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			text, ok, err := ExtractResponsesStreamText(tt.event)
			if err != nil {
				t.Fatal(err)
			}
			if !ok || text.Kind != StreamTextSnapshot || text.Text != "Hi there" {
				t.Fatalf("text = %+v, ok = %v; want snapshot Hi there", text, ok)
			}
		})
	}
}

func TestRewriteResponsesStreamDelta(t *testing.T) {
	event := sse.Event{Event: "response.output_text.delta", Data: `{"type":"response.output_text.delta","item_id":"msg_1","output_index":0,"content_index":0,"delta":"unsafe"}`}
	rewritten, err := RewriteResponsesStreamDelta(event, "safe")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(rewritten), "event: response.output_text.delta\n") {
		t.Fatalf("rewritten event = %q", rewritten)
	}
	data := strings.TrimSpace(strings.TrimPrefix(strings.SplitN(string(rewritten), "data: ", 2)[1], "data: "))
	var payload struct {
		Delta string `json:"delta"`
	}
	if err := json.Unmarshal([]byte(data), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Delta != "safe" {
		t.Fatalf("delta = %q, want safe", payload.Delta)
	}
}

func TestNewResponsesStreamDeltaEventUsesMetadata(t *testing.T) {
	metadata := ResponsesStreamDeltaMetadata{ItemID: "msg_1", OutputIndex: 2, ContentIndex: 3}
	event := NewResponsesStreamDeltaEvent("tail", metadata)
	if !strings.Contains(string(event), "event: response.output_text.delta") {
		t.Fatalf("event = %q", event)
	}
	if !strings.Contains(string(event), `"item_id":"msg_1"`) || !strings.Contains(string(event), `"output_index":2`) || !strings.Contains(string(event), `"content_index":3`) {
		t.Fatalf("event = %q, want metadata", event)
	}
}
