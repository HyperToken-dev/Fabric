package openai

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/HyperToken-dev/fabric/protocol/sse"
)

type chatStreamChunk struct {
	Choices []chatStreamChoice `json:"choices"`
}

type chatStreamChoice struct {
	Delta chatStreamDelta `json:"delta"`
}

type chatStreamDelta struct {
	Content json.RawMessage `json:"content,omitempty"`
}

type StreamTextKind int

const (
	StreamTextDelta StreamTextKind = iota
	StreamTextSnapshot
)

type StreamText struct {
	Text string
	Kind StreamTextKind
}

type responsesStreamEvent struct {
	Type         string `json:"type"`
	Delta        string `json:"delta"`
	Text         string `json:"text"`
	ItemID       string `json:"item_id"`
	OutputIndex  int    `json:"output_index"`
	ContentIndex int    `json:"content_index"`
	Part         struct {
		Text string `json:"text"`
	} `json:"part"`
	Item struct {
		Content []openAIContentPart `json:"content"`
	} `json:"item"`
	Response struct {
		OutputText string `json:"output_text"`
		Output     []struct {
			Content []openAIContentPart `json:"content"`
		} `json:"output"`
	} `json:"response"`
}

type ResponsesStreamDeltaMetadata struct {
	ItemID       string
	OutputIndex  int
	ContentIndex int
}

func ExtractChatCompletionStreamText(event sse.Event) (string, bool, error) {
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

func RewriteChatCompletionStreamText(event sse.Event, text string) ([]byte, error) {
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
	return sse.FormatData(event.Event, encoded), nil
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
	return sse.FormatData("", encoded)
}

func ExtractResponsesStreamText(event sse.Event) (StreamText, bool, error) {
	data := strings.TrimSpace(event.Data)
	if data == "" || data == "[DONE]" {
		return StreamText{}, false, nil
	}

	var streamEvent responsesStreamEvent
	if err := json.Unmarshal([]byte(data), &streamEvent); err != nil {
		return StreamText{}, false, fmt.Errorf("parse openai responses stream event: %w", err)
	}
	eventType := streamEvent.Type
	if eventType == "" {
		eventType = event.Event
	}
	switch eventType {
	case "response.output_text.delta":
		if streamEvent.Delta == "" {
			return StreamText{}, false, nil
		}
		return StreamText{Text: streamEvent.Delta, Kind: StreamTextDelta}, true, nil
	case "response.output_text.done":
		if strings.TrimSpace(streamEvent.Text) == "" {
			return StreamText{}, false, nil
		}
		return StreamText{Text: streamEvent.Text, Kind: StreamTextSnapshot}, true, nil
	case "response.content_part.done":
		if strings.TrimSpace(streamEvent.Part.Text) == "" {
			return StreamText{}, false, nil
		}
		return StreamText{Text: streamEvent.Part.Text, Kind: StreamTextSnapshot}, true, nil
	case "response.output_item.done":
		text := joinContentPartTexts(streamEvent.Item.Content)
		if strings.TrimSpace(text) == "" {
			return StreamText{}, false, nil
		}
		return StreamText{Text: text, Kind: StreamTextSnapshot}, true, nil
	case "response.completed":
		texts := []string{streamEvent.Response.OutputText}
		for _, output := range streamEvent.Response.Output {
			texts = append(texts, joinContentPartTexts(output.Content))
		}
		text := strings.Join(nonEmptyStrings(texts), "\n")
		if strings.TrimSpace(text) == "" {
			return StreamText{}, false, nil
		}
		return StreamText{Text: text, Kind: StreamTextSnapshot}, true, nil
	default:
		return StreamText{}, false, nil
	}
}

func RewriteResponsesStreamDelta(event sse.Event, text string) ([]byte, error) {
	if text == "" {
		return nil, nil
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(event.Data), &payload); err != nil {
		return nil, fmt.Errorf("parse openai responses stream event for rewrite: %w", err)
	}
	payload["delta"] = text
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return sse.FormatData(event.Event, encoded), nil
}

func ExtractResponsesStreamDeltaMetadata(event sse.Event) (ResponsesStreamDeltaMetadata, bool, error) {
	data := strings.TrimSpace(event.Data)
	if data == "" || data == "[DONE]" {
		return ResponsesStreamDeltaMetadata{}, false, nil
	}
	var streamEvent responsesStreamEvent
	if err := json.Unmarshal([]byte(data), &streamEvent); err != nil {
		return ResponsesStreamDeltaMetadata{}, false, fmt.Errorf("parse openai responses stream metadata: %w", err)
	}
	eventType := streamEvent.Type
	if eventType == "" {
		eventType = event.Event
	}
	if eventType != "response.output_text.delta" {
		return ResponsesStreamDeltaMetadata{}, false, nil
	}
	return ResponsesStreamDeltaMetadata{
		ItemID:       streamEvent.ItemID,
		OutputIndex:  streamEvent.OutputIndex,
		ContentIndex: streamEvent.ContentIndex,
	}, true, nil
}

func NewResponsesStreamDeltaEvent(text string, metadata ResponsesStreamDeltaMetadata) []byte {
	if text == "" {
		return nil
	}
	payload := map[string]any{
		"type":          "response.output_text.delta",
		"item_id":       metadata.ItemID,
		"output_index":  metadata.OutputIndex,
		"content_index": metadata.ContentIndex,
		"delta":         text,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil
	}
	return sse.FormatData("response.output_text.delta", encoded)
}

func joinContentPartTexts(parts []openAIContentPart) string {
	texts := make([]string, 0, len(parts))
	for _, part := range parts {
		texts = append(texts, part.Text)
	}
	return strings.Join(nonEmptyStrings(texts), "\n")
}

func nonEmptyStrings(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			result = append(result, value)
		}
	}
	return result
}
