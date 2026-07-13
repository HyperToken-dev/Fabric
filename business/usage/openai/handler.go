package openai

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync"

	"fabric/business/usage"

	"github.com/andybalholm/brotli"
	"go.uber.org/zap"
)

type UsageCallback func(*usage.Usage)

type openAIUsage struct {
	InputTokens      int `json:"input_tokens"`
	OutputTokens     int `json:"output_tokens"`
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

type openAIResponsesStreamEvent struct {
	Usage    *openAIUsage `json:"usage"`
	Response struct {
		Usage *openAIUsage `json:"usage"`
	} `json:"response"`
}

func ExtractNonStreaming(rawBody []byte, contentEncoding string) (*usage.Usage, error) {
	decodedBody, err := decodeResponseBody(rawBody, contentEncoding)
	if err != nil {
		return nil, fmt.Errorf("decode response body error: %w", err)
	}

	var resp struct {
		Usage openAIUsage `json:"usage"`
	}
	if err := json.Unmarshal(decodedBody, &resp); err != nil {
		return nil, err
	}
	return openAIUsageToUsage(&resp.Usage)
}

func NewTrackingReader(body io.ReadCloser, contentEncoding string, onUsage UsageCallback) io.ReadCloser {
	return &responsesUsageTrackingReader{
		reader:          body,
		contentEncoding: strings.ToLower(strings.TrimSpace(contentEncoding)),
		onUsage:         onUsage,
	}
}

func openAIUsageToUsage(openaiUsage *openAIUsage) (*usage.Usage, error) {
	if openaiUsage.InputTokens != 0 || openaiUsage.OutputTokens != 0 {
		return &usage.Usage{
			PromptTokens:     int64(openaiUsage.InputTokens),
			CompletionTokens: int64(openaiUsage.OutputTokens),
		}, nil
	}
	if openaiUsage.PromptTokens != 0 || openaiUsage.CompletionTokens != 0 {
		return &usage.Usage{
			PromptTokens:     int64(openaiUsage.PromptTokens),
			CompletionTokens: int64(openaiUsage.CompletionTokens),
		}, nil
	}
	return nil, errors.New("missing openai usage")
}

type responsesUsageTrackingReader struct {
	reader          io.ReadCloser
	parser          responsesSSEUsageParser
	compressedBody  bytes.Buffer
	contentEncoding string
	onUsage         UsageCallback
	once            sync.Once
}

func (r *responsesUsageTrackingReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if n > 0 {
		switch r.contentEncoding {
		case "", "identity":
			if parseErr := r.parser.Write(p[:n]); parseErr != nil {
				zap.S().Errorf("Error catched: parse responses stream usage error: %v", parseErr)
			}
		case "gzip":
			if _, writeErr := r.compressedBody.Write(p[:n]); writeErr != nil {
				zap.S().Errorf("Error catched: buffer gzip responses stream usage data error: %v", writeErr)
			}
		}
	}
	if err == io.EOF {
		r.once.Do(r.processUsage)
	}
	return n, err
}

func (r *responsesUsageTrackingReader) Close() error {
	return r.reader.Close()
}

func (r *responsesUsageTrackingReader) processUsage() {
	if r.contentEncoding != "" && r.contentEncoding != "identity" {
		compressedBody := append([]byte(nil), r.compressedBody.Bytes()...)
		go r.processEncodedUsage(compressedBody)
		return
	}

	if err := r.parser.Finish(); err != nil {
		zap.S().Errorf("Error catched: finish responses stream usage parser error: %v", err)
	}
	r.emitUsage(r.parser.Usage())
}

func (r *responsesUsageTrackingReader) processEncodedUsage(compressedBody []byte) {
	body, err := decodeResponseBody(compressedBody, r.contentEncoding)
	if err != nil {
		zap.S().Errorf("Error catched: decode responses stream usage body error: %v", err)
		return
	}

	var parser responsesSSEUsageParser
	if err := parser.Write(body); err != nil {
		zap.S().Errorf("Error catched: parse encoded responses stream usage error: %v", err)
	}
	if err := parser.Finish(); err != nil {
		zap.S().Errorf("Error catched: finish encoded responses stream usage parser error: %v", err)
	}
	r.emitUsage(parser.Usage())
}

func (r *responsesUsageTrackingReader) emitUsage(parsedUsage *usage.Usage) {
	if parsedUsage == nil {
		zap.S().Warn("responses stream usage missing")
		return
	}
	if r.onUsage != nil {
		r.onUsage(parsedUsage)
	}
}

type responsesSSEUsageParser struct {
	lineBuf bytes.Buffer
	event   string
	data    bytes.Buffer
	usage   *usage.Usage
}

func (p *responsesSSEUsageParser) Write(chunk []byte) error {
	var firstErr error
	for len(chunk) > 0 {
		idx := bytes.IndexByte(chunk, '\n')
		if idx < 0 {
			_, err := p.lineBuf.Write(chunk)
			return err
		}
		if _, err := p.lineBuf.Write(chunk[:idx]); err != nil && firstErr == nil {
			firstErr = err
		}
		line := p.lineBuf.String()
		p.lineBuf.Reset()
		if err := p.consumeLine(strings.TrimSuffix(line, "\r")); err != nil && firstErr == nil {
			firstErr = err
		}
		chunk = chunk[idx+1:]
	}
	return firstErr
}

func (p *responsesSSEUsageParser) Finish() error {
	var firstErr error
	if p.lineBuf.Len() > 0 {
		line := p.lineBuf.String()
		p.lineBuf.Reset()
		if err := p.consumeLine(strings.TrimSuffix(line, "\r")); err != nil {
			firstErr = err
		}
	}
	if p.data.Len() > 0 || p.event != "" {
		if err := p.consumeEvent(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (p *responsesSSEUsageParser) Usage() *usage.Usage {
	return p.usage
}

func (p *responsesSSEUsageParser) consumeLine(line string) error {
	if line == "" {
		return p.consumeEvent()
	}
	if strings.HasPrefix(line, ":") {
		return nil
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
		p.event = value
	case "data":
		if p.data.Len() > 0 {
			if err := p.data.WriteByte('\n'); err != nil {
				return err
			}
		}
		_, err := p.data.WriteString(value)
		return err
	}
	return nil
}

func (p *responsesSSEUsageParser) consumeEvent() error {
	data := strings.TrimSpace(p.data.String())
	p.event = ""
	p.data.Reset()

	if data == "" || data == "[DONE]" {
		return nil
	}

	var streamEvent openAIResponsesStreamEvent
	if err := json.Unmarshal([]byte(data), &streamEvent); err != nil {
		return err
	}
	openaiUsage := streamEvent.Response.Usage
	if openaiUsage == nil {
		openaiUsage = streamEvent.Usage
	}
	if openaiUsage == nil {
		return nil
	}

	parsedUsage, err := openAIUsageToUsage(openaiUsage)
	if err != nil {
		return err
	}
	p.usage = parsedUsage
	return nil
}

func decodeResponseBody(rawBody []byte, contentEncoding string) ([]byte, error) {
	encoding := strings.ToLower(strings.TrimSpace(contentEncoding))
	switch encoding {
	case "", "identity":
		return rawBody, nil
	case "gzip":
		reader, err := gzip.NewReader(bytes.NewReader(rawBody))
		if err != nil {
			return nil, err
		}
		decoded, readErr := io.ReadAll(reader)
		closeErr := reader.Close()
		if readErr != nil {
			return nil, readErr
		}
		if closeErr != nil {
			return nil, closeErr
		}
		return decoded, nil
	case "br":
		return io.ReadAll(brotli.NewReader(bytes.NewReader(rawBody)))
	default:
		return nil, fmt.Errorf("unsupported content encoding: %s", contentEncoding)
	}
}
