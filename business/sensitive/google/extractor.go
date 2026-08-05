package google

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
	Prompts []string
}

type interactionRequest struct {
	Model string          `json:"model"`
	Input json.RawMessage `json:"input"`
}

type interactionItem struct {
	Type    string               `json:"type"`
	Content []interactionContent `json:"content"`
}

type interactionContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

func ExtractPromptRequest(req *http.Request) (*PromptRequest, error) {
	body, err := readAndRestoreRequestBody(req)
	if err != nil {
		return nil, err
	}
	return parseInteractionPromptRequest(body)
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

func parseInteractionPromptRequest(body []byte) (*PromptRequest, error) {
	var req interactionRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, err
	}
	prompts, err := interactionPromptTexts(req)
	if err != nil {
		return nil, err
	}
	return &PromptRequest{Model: req.Model, Prompts: prompts}, nil
}

func interactionPromptTexts(req interactionRequest) ([]string, error) {
	if len(req.Input) == 0 || string(req.Input) == "null" {
		return nil, nil
	}

	var text string
	if err := json.Unmarshal(req.Input, &text); err == nil {
		if strings.TrimSpace(text) == "" {
			return nil, nil
		}
		return []string{text}, nil
	}

	var items []interactionItem
	if err := json.Unmarshal(req.Input, &items); err != nil {
		return nil, fmt.Errorf("decode google interaction input: %w", err)
	}

	var prompts []string
	for _, item := range items {
		if item.Type != "user_input" && item.Type != "model_output" {
			continue
		}
		for _, content := range item.Content {
			if content.Type == "text" && strings.TrimSpace(content.Text) != "" {
				prompts = append(prompts, content.Text)
			}
		}
	}
	return prompts, nil
}
