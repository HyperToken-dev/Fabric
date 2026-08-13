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
	Index *int            `json:"index"`
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
	Text              string
	Kind              StreamTextKind
	Lane              string
	LanePrefix        string
	ChoiceIndex       int
	ResponsesMetadata ResponsesStreamDeltaMetadata
}

type responsesStreamEvent struct {
	Type         string `json:"type"`
	Delta        string `json:"delta"`
	Text         string `json:"text"`
	ItemID       string `json:"item_id"`
	OutputIndex  int    `json:"output_index"`
	ContentIndex int    `json:"content_index"`
	Part         struct {
		Text    string          `json:"text"`
		Content json.RawMessage `json:"content"`
	} `json:"part"`
	Item struct {
		ID      string              `json:"id"`
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

func ExtractChatCompletionStreamText(event sse.Event) ([]StreamText, bool, error) {
	data := strings.TrimSpace(event.Data)
	if data == "" || data == "[DONE]" {
		return nil, false, nil
	}

	var chunk chatStreamChunk
	if err := json.Unmarshal([]byte(data), &chunk); err != nil {
		return nil, false, fmt.Errorf("parse openai chat stream event: %w", err)
	}
	texts := make([]StreamText, 0, len(chunk.Choices))
	for position, choice := range chunk.Choices {
		if len(choice.Delta.Content) == 0 || string(choice.Delta.Content) == "null" {
			continue
		}
		var content string
		if err := json.Unmarshal(choice.Delta.Content, &content); err != nil {
			return nil, false, fmt.Errorf("parse openai chat stream content: %w", err)
		}
		choiceIndex := position
		if choice.Index != nil {
			choiceIndex = *choice.Index
		}
		texts = append(texts, StreamText{Text: content, Kind: StreamTextDelta, Lane: fmt.Sprintf("chat:%d", choiceIndex), ChoiceIndex: choiceIndex})
	}
	if len(texts) == 0 {
		return nil, false, nil
	}
	return texts, true, nil
}

func RewriteChatCompletionStreamText(event sse.Event, texts []StreamText) ([]byte, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	textByChoice := make(map[int]string, len(texts))
	shouldEmit := false
	for _, text := range texts {
		textByChoice[text.ChoiceIndex] = text.Text
		if text.Text != "" {
			shouldEmit = true
		}
	}
	if !shouldEmit {
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
	for position, rawChoice := range choices {
		choice, ok := rawChoice.(map[string]any)
		if !ok {
			continue
		}
		index := position
		if value, ok := choice["index"]; ok {
			switch value := value.(type) {
			case float64:
				index = int(value)
			case int:
				index = value
			}
		}
		text, ok := textByChoice[index]
		if !ok {
			continue
		}
		delta, ok := choice["delta"].(map[string]any)
		if !ok {
			continue
		}
		delta["content"] = text
	}

	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	return sse.FormatData(event.Event, encoded), nil
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
		metadata := ResponsesStreamDeltaMetadata{ItemID: streamEvent.ItemID, OutputIndex: streamEvent.OutputIndex, ContentIndex: streamEvent.ContentIndex}
		return StreamText{Text: streamEvent.Delta, Kind: StreamTextDelta, Lane: fmt.Sprintf("responses:%s:%d:%d", metadata.ItemID, metadata.OutputIndex, metadata.ContentIndex), ResponsesMetadata: metadata}, true, nil
	case "response.output_text.done":
		metadata := ResponsesStreamDeltaMetadata{ItemID: streamEvent.ItemID, OutputIndex: streamEvent.OutputIndex, ContentIndex: streamEvent.ContentIndex}
		return StreamText{Text: streamEvent.Text, Kind: StreamTextSnapshot, Lane: fmt.Sprintf("responses:%s:%d:%d", metadata.ItemID, metadata.OutputIndex, metadata.ContentIndex), ResponsesMetadata: metadata}, true, nil
	case "response.content_part.done":
		metadata := ResponsesStreamDeltaMetadata{ItemID: streamEvent.ItemID, OutputIndex: streamEvent.OutputIndex, ContentIndex: streamEvent.ContentIndex}
		texts := []string{streamEvent.Part.Text}
		texts = append(texts, rawTexts(streamEvent.Part.Content)...)
		return StreamText{Text: strings.Join(nonEmptyStrings(texts), "\n"), Kind: StreamTextSnapshot, Lane: fmt.Sprintf("responses:%s:%d:%d", metadata.ItemID, metadata.OutputIndex, metadata.ContentIndex), ResponsesMetadata: metadata}, true, nil
	case "response.output_item.done":
		text := strings.Join(partTexts(streamEvent.Item.Content), "\n")
		itemID := streamEvent.ItemID
		if itemID == "" {
			itemID = streamEvent.Item.ID
		}
		return StreamText{Text: text, Kind: StreamTextSnapshot, LanePrefix: fmt.Sprintf("responses:%s:%d:", itemID, streamEvent.OutputIndex)}, true, nil
	case "response.completed":
		text := streamEvent.Response.OutputText
		if strings.TrimSpace(text) == "" {
			var texts []string
			for _, output := range streamEvent.Response.Output {
				texts = append(texts, partTexts(output.Content)...)
			}
			text = strings.Join(texts, "\n")
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

func partTexts(parts []openAIContentPart) []string {
	var texts []string
	for _, part := range parts {
		texts = append(texts, part.Text)
		texts = append(texts, rawTexts(part.Content)...)
	}
	return nonEmptyStrings(texts)
}

func rawTexts(raw json.RawMessage) []string {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}

	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return nonEmptyStrings([]string{text})
	}

	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err == nil {
		var texts []string
		for _, item := range items {
			texts = append(texts, rawTexts(item)...)
		}
		return texts
	}

	var part openAIContentPart
	if err := json.Unmarshal(raw, &part); err != nil {
		return nil
	}
	texts := []string{part.Text}
	texts = append(texts, rawTexts(part.Content)...)
	return nonEmptyStrings(texts)
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
