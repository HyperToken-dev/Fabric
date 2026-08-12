package openai

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

type SSEEvent struct {
	Event string
	Data  string
	Raw   []byte
}

type SSEParser struct {
	line  bytes.Buffer
	event sseEventBuilder
}

type sseEventBuilder struct {
	event string
	data  bytes.Buffer
	raw   bytes.Buffer
}

func (p *SSEParser) Write(chunk []byte) ([]SSEEvent, error) {
	var events []SSEEvent
	for len(chunk) > 0 {
		idx := bytes.IndexByte(chunk, '\n')
		if idx < 0 {
			_, err := p.line.Write(chunk)
			return events, err
		}

		if _, err := p.line.Write(chunk[:idx+1]); err != nil {
			return events, err
		}
		lineBytes := append([]byte(nil), p.line.Bytes()...)
		p.line.Reset()

		event, ok, err := p.consumeLine(lineBytes)
		if err != nil {
			return events, err
		}
		if ok {
			events = append(events, event)
		}
		chunk = chunk[idx+1:]
	}
	return events, nil
}

func (p *SSEParser) Finish() ([]SSEEvent, error) {
	if p.line.Len() > 0 {
		lineBytes := append([]byte(nil), p.line.Bytes()...)
		p.line.Reset()
		if !bytes.HasSuffix(lineBytes, []byte("\n")) {
			lineBytes = append(lineBytes, '\n')
		}
		if _, _, err := p.consumeLine(lineBytes); err != nil {
			return nil, err
		}
	}
	if p.event.raw.Len() == 0 {
		return nil, nil
	}
	event := p.event.finish()
	p.event = sseEventBuilder{}
	return []SSEEvent{event}, nil
}

func (p *SSEParser) consumeLine(lineBytes []byte) (SSEEvent, bool, error) {
	line := strings.TrimSuffix(string(lineBytes), "\n")
	line = strings.TrimSuffix(line, "\r")
	if line == "" {
		if _, err := p.event.raw.Write(lineBytes); err != nil {
			return SSEEvent{}, false, err
		}
		event := p.event.finish()
		p.event = sseEventBuilder{}
		return event, true, nil
	}
	if _, err := p.event.raw.Write(lineBytes); err != nil {
		return SSEEvent{}, false, err
	}
	if strings.HasPrefix(line, ":") {
		return SSEEvent{}, false, nil
	}

	field, value, found := strings.Cut(line, ":")
	if !found {
		field = line
		value = ""
	} else if strings.HasPrefix(value, " ") {
		value = strings.TrimPrefix(value, " ")
	}

	switch field {
	case "event":
		p.event.event = value
	case "data":
		if p.event.data.Len() > 0 {
			if err := p.event.data.WriteByte('\n'); err != nil {
				return SSEEvent{}, false, err
			}
		}
		_, err := p.event.data.WriteString(value)
		return SSEEvent{}, false, err
	}
	return SSEEvent{}, false, nil
}

func (b *sseEventBuilder) finish() SSEEvent {
	return SSEEvent{
		Event: b.event,
		Data:  b.data.String(),
		Raw:   append([]byte(nil), b.raw.Bytes()...),
	}
}

func (e SSEEvent) Done() bool {
	return strings.TrimSpace(e.Data) == "[DONE]"
}

type chatStreamChunk struct {
	Choices []chatStreamChoice `json:"choices"`
}

type chatStreamChoice struct {
	Delta chatStreamDelta `json:"delta"`
}

type chatStreamDelta struct {
	Content json.RawMessage `json:"content,omitempty"`
}

func ExtractChatCompletionStreamText(event SSEEvent) (string, bool, error) {
	data := strings.TrimSpace(event.Data)
	if data == "" || data == "[DONE]" {
		return "", false, nil
	}

	var chunk chatStreamChunk
	if err := json.Unmarshal([]byte(data), &chunk); err != nil {
		return "", false, fmt.Errorf("parse openai chat stream event: %w", err)
	}
	var text strings.Builder
	for _, choice := range chunk.Choices {
		if len(choice.Delta.Content) == 0 || string(choice.Delta.Content) == "null" {
			continue
		}
		var content string
		if err := json.Unmarshal(choice.Delta.Content, &content); err != nil {
			return "", false, fmt.Errorf("parse openai chat stream content: %w", err)
		}
		_, _ = text.WriteString(content)
	}
	if text.Len() == 0 {
		return "", false, nil
	}
	return text.String(), true, nil
}

func RewriteChatCompletionStreamText(event SSEEvent, text string) ([]byte, error) {
	if text == "" {
		return nil, nil
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(event.Data), &payload); err != nil {
		return nil, fmt.Errorf("parse openai chat stream event for rewrite: %w", err)
	}
	choices, ok := payload["choices"].([]any)
	if !ok || len(choices) == 0 {
		return event.Raw, nil
	}
	choice, ok := choices[0].(map[string]any)
	if !ok {
		return event.Raw, nil
	}
	delta, ok := choice["delta"].(map[string]any)
	if !ok {
		return event.Raw, nil
	}
	delta["content"] = text

	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return formatSSEData(event.Event, encoded), nil
}

func NewChatCompletionStreamTextEvent(text string) []byte {
	if text == "" {
		return nil
	}
	payload := map[string]any{
		"choices": []any{
			map[string]any{
				"delta": map[string]any{"content": text},
				"index": 0,
			},
		},
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil
	}
	return formatSSEData("", encoded)
}

func formatSSEData(event string, data []byte) []byte {
	var out bytes.Buffer
	if event != "" {
		_, _ = out.WriteString("event: ")
		_, _ = out.WriteString(event)
		_, _ = out.WriteString("\n")
	}
	_, _ = out.WriteString("data: ")
	_, _ = out.Write(data)
	_, _ = out.WriteString("\n\n")
	return out.Bytes()
}
