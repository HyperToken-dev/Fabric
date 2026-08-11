package extrotec

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
	Prompts []string
}

type generationRequest struct {
	Prompt         *string `json:"prompt"`
	ForwardPrompt  *string `json:"forward_prompt"`
	NegativePrompt *string `json:"negative_prompt"`
}

func ExtractPromptRequest(req *http.Request) (*PromptRequest, error) {
	body, err := readAndRestoreRequestBody(req)
	if err != nil {
		return nil, err
	}
	return parseGenerationPromptRequest(body)
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

func parseGenerationPromptRequest(body []byte) (*PromptRequest, error) {
	var req generationRequest
	if err := json.Unmarshal(body, &req); err != nil {
		return nil, err
	}

	fields := []*string{req.Prompt, req.ForwardPrompt, req.NegativePrompt}
	prompts := make([]string, 0, len(fields))
	for _, field := range fields {
		if field == nil || strings.TrimSpace(*field) == "" {
			continue
		}
		prompts = append(prompts, *field)
	}

	return &PromptRequest{Prompts: prompts}, nil
}
