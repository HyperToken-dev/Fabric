package openai

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/HyperToken-dev/fabric/protocol/sse"
)

func TestExtractChatCompletionStreamText(t *testing.T) {
	event := sse.Event{Data: `{"choices":[{"index":0,"delta":{"content":"hello"}},{"index":1,"delta":{"content":"there"}}]}`}
	texts, ok, err := ExtractChatCompletionStreamText(event)
	if err != nil {
		t.Fatal(err)
	}
	if !ok || len(texts) != 2 {
		t.Fatalf("texts = %+v, ok = %v; want two texts", texts, ok)
	}
	if texts[0].Text != "hello" || texts[0].ChoiceIndex != 0 || texts[1].Text != "there" || texts[1].ChoiceIndex != 1 {
		t.Fatalf("texts = %+v, want separate choices", texts)
	}

	noText := sse.Event{Data: `{"choices":[{"delta":{"role":"assistant"}}]}`}
	texts, ok, err = ExtractChatCompletionStreamText(noText)
	if err != nil {
		t.Fatal(err)
	}
	if ok || len(texts) != 0 {
		t.Fatalf("texts = %+v, ok = %v; want empty false", texts, ok)
	}
}

func TestRewriteChatCompletionStreamText(t *testing.T) {
	event := sse.Event{Data: `{"id":"chatcmpl","choices":[{"index":0,"delta":{"content":"unsafe"}},{"index":1,"delta":{"content":"leaked"}}]}`}
	rewritten, err := RewriteChatCompletionStreamText(event, []StreamText{
		{Text: "safe", ChoiceIndex: 0},
		{Text: "clean", ChoiceIndex: 1},
	})
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
		t.Fatalf("choice 0 content = %q, want safe", got)
	}
	if got := payload.Choices[1].Delta.Content; got != "clean" {
		t.Fatalf("choice 1 content = %q, want clean", got)
	}
}

func TestRewriteChatCompletionStreamTextSkipsEmptySafeText(t *testing.T) {
	rewritten, err := RewriteChatCompletionStreamText(sse.Event{Data: `{"choices":[{"index":0,"delta":{"content":"held"}}]}`}, []StreamText{{Text: "", ChoiceIndex: 0}})
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
