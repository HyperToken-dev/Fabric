package openai

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type PromptRequest struct {
	Model   string
	Stream  bool
	Prompts []string
}

type openAIMessage struct {
	Role    string          `json:"role"`
	Content json.RawMessage `json:"content"`
}

type openAIContentPart struct {
	Type    string          `json:"type"`
	Text    string          `json:"text"`
	Content json.RawMessage `json:"content"`
}

func ExtractPromptRequest(req *http.Request) (*PromptRequest, error) {
	body, err := readAndRestoreRequestBody(req)
	if err != nil {
		return nil, err
	}

	switch {
	case strings.Contains(req.URL.Path, "/v1/chat/completions"):
		return parseOpenAIChatPromptRequest(body)
	case strings.Contains(req.URL.Path, "/v1/responses"):
		return parseOpenAIResponsesPromptRequest(body)
	default:
		return parseOpenAIGenericPromptRequest(body)
	}
}

func readAndRestoreRequestBody(req *http.Request) ([]byte, error) {
	if req.Body == nil {
		return nil, errors.New("missing request body")
	}

	body, err := io.ReadAll(req.Body)
	closeErr := req.Body.Close()
	if err != nil {
		if closeErr != nil {
			return nil, fmt.Errorf("read request body: %w; close request body: %v", err, closeErr)
		}
		return nil, err
	}
	if closeErr != nil {
		return nil, closeErr
	}

	req.Body = io.NopCloser(bytes.NewReader(body))
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(body)), nil
	}
	return body, nil
}

func parseOpenAIChatPromptRequest(body []byte) (*PromptRequest, error) {
	var req struct {
		Model    string          `json:"model"`
		Stream   bool            `json:"stream"`
		Messages []openAIMessage `json:"messages"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, err
	}

	prompts := make([]string, 0, len(req.Messages))
	for _, message := range req.Messages {
		prompts = appendPromptTextsFromRawContent(prompts, message.Content)
	}

	return &PromptRequest{Model: req.Model, Stream: req.Stream, Prompts: prompts}, nil
}

func parseOpenAIResponsesPromptRequest(body []byte) (*PromptRequest, error) {
	var req struct {
		Model        string          `json:"model"`
		Input        json.RawMessage `json:"input"`
		Instructions string          `json:"instructions"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, err
	}

	var prompts []string
	if strings.TrimSpace(req.Instructions) != "" {
		prompts = append(prompts, req.Instructions)
	}
	prompts = appendPromptTextsFromRawContent(prompts, req.Input)

	return &PromptRequest{Model: req.Model, Prompts: prompts}, nil
}

func parseOpenAIGenericPromptRequest(body []byte) (*PromptRequest, error) {
	var req struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, err
	}
	return &PromptRequest{Model: req.Model}, nil
}

func appendPromptTextsFromRawContent(prompts []string, raw json.RawMessage) []string {
	if len(raw) == 0 || string(raw) == "null" {
		return prompts
	}

	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		if strings.TrimSpace(text) != "" {
			prompts = append(prompts, text)
		}
		return prompts
	}

	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err == nil {
		for _, item := range items {
			prompts = appendPromptTextsFromRawObject(prompts, item)
		}
		return prompts
	}

	return appendPromptTextsFromRawObject(prompts, raw)
}

func appendPromptTextsFromRawObject(prompts []string, raw json.RawMessage) []string {
	var item struct {
		Content json.RawMessage `json:"content"`
		Text    string          `json:"text"`
	}
	if err := json.Unmarshal(raw, &item); err != nil {
		return prompts
	}
	if strings.TrimSpace(item.Text) != "" {
		prompts = append(prompts, item.Text)
	}
	if len(item.Content) > 0 {
		prompts = appendPromptTextsFromRawContent(prompts, item.Content)
	}
	return prompts
}

func ExtractOutputTexts(req *http.Request, body []byte) ([]string, error) {
	switch {
	case strings.Contains(req.URL.Path, "/v1/chat/completions"):
		return extractOpenAIChatOutputTexts(body)
	case strings.Contains(req.URL.Path, "/v1/responses"):
		return extractOpenAIResponsesOutputTexts(body)
	default:
		return nil, nil
	}
}

func extractOpenAIChatOutputTexts(body []byte) ([]string, error) {
	var resp struct {
		Choices []struct {
			Message openAIMessage `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	texts := make([]string, 0, len(resp.Choices))
	for _, choice := range resp.Choices {
		texts = appendPromptTextsFromRawContent(texts, choice.Message.Content)
	}
	return texts, nil
}

func extractOpenAIResponsesOutputTexts(body []byte) ([]string, error) {
	var resp struct {
		OutputText string `json:"output_text"`
		Output     []struct {
			Content []openAIContentPart `json:"content"`
		} `json:"output"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, err
	}

	var texts []string
	if strings.TrimSpace(resp.OutputText) != "" {
		texts = append(texts, resp.OutputText)
	}
	for _, output := range resp.Output {
		for _, part := range output.Content {
			if strings.TrimSpace(part.Text) != "" {
				texts = append(texts, part.Text)
			}
		}
	}
	return texts, nil
}
