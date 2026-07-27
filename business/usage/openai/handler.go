package openai

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"

	"github.com/HyperToken-dev/fabric/business/usage"

	"github.com/andybalholm/brotli"
)

type UsageCallback func(*usage.Usage)
type StreamCompleteCallback func([]byte)

const (
	defaultOpenAIEncoding = "o200k_base"
	tokensPerMessage      = 3
	tokensPerName         = 1
	replyPrimingTokens    = 3
)

var errMissingOpenAIUsage = errors.New("missing openai usage")

type openAIUsage struct {
	InputTokens      int `json:"input_tokens"`
	OutputTokens     int `json:"output_tokens"`
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
}

type openAIResponsesStreamEvent struct {
	Usage    *openAIUsage `json:"usage"`
	Delta    string       `json:"delta"`
	Text     string       `json:"text"`
	Response struct {
		Usage      *openAIUsage   `json:"usage"`
		OutputText string         `json:"output_text"`
		Output     []openAIOutput `json:"output"`
	} `json:"response"`
	Choices []struct {
		Delta struct {
			Content json.RawMessage `json:"content"`
		} `json:"delta"`
	} `json:"choices"`
}

type openAIContentPart struct {
	Type    string          `json:"type"`
	Text    string          `json:"text"`
	Content json.RawMessage `json:"content"`
}

type openAIMessage struct {
	Role    string          `json:"role"`
	Name    string          `json:"name"`
	Content json.RawMessage `json:"content"`
}

type openAIOutput struct {
	Content []openAIContentPart `json:"content"`
}

// openai nonstream extract
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

func ExtractNonStreamingWithFallback(req *http.Request, rawBody []byte, contentEncoding string, model string) (*usage.Usage, error) {
	parsedUsage, err := ExtractNonStreaming(rawBody, contentEncoding)
	if err == nil {
		return parsedUsage, nil
	}
	if !errors.Is(err, errMissingOpenAIUsage) {
		return nil, err
	}

	decodedBody, decodeErr := decodeResponseBody(rawBody, contentEncoding)
	if decodeErr != nil {
		return nil, fmt.Errorf("decode response body for fallback: %w", decodeErr)
	}
	return fallbackUsage(req, decodedBody, nil, model)
}

// openai SSE stream extract
func NewTrackingReader(body io.ReadCloser, contentEncoding string, onUsage UsageCallback, onComplete StreamCompleteCallback) io.ReadCloser {
	return NewTrackingReaderWithFallback(nil, body, contentEncoding, "", onUsage, onComplete)
}

func NewTrackingReaderWithFallback(req *http.Request, body io.ReadCloser, contentEncoding string, model string, onUsage UsageCallback, onComplete StreamCompleteCallback) io.ReadCloser {
	return &responsesUsageTrackingReader{
		reader:          body,
		req:             req,
		model:           model,
		contentEncoding: strings.ToLower(strings.TrimSpace(contentEncoding)),
		onUsage:         onUsage,
		onComplete:      onComplete,
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
	return nil, errMissingOpenAIUsage
}

type responsesUsageTrackingReader struct {
	reader          io.ReadCloser
	req             *http.Request
	model           string
	parser          responsesSSEUsageParser
	body            bytes.Buffer
	contentEncoding string
	onUsage         UsageCallback
	onComplete      StreamCompleteCallback
	once            sync.Once
}

func (r *responsesUsageTrackingReader) Read(p []byte) (int, error) {
	n, err := r.reader.Read(p)
	if n > 0 {
		_, _ = r.body.Write(p[:n])
		switch r.contentEncoding {
		case "", "identity":
			_ = r.parser.Write(p[:n])
		}
	}
	if err == io.EOF {
		r.once.Do(r.processUsage)
	}
	return n, err
}

func (r *responsesUsageTrackingReader) Close() error {
	r.once.Do(r.processUsage)
	return r.reader.Close()
}

func (r *responsesUsageTrackingReader) processUsage() {
	if r.contentEncoding != "" && r.contentEncoding != "identity" {
		compressedBody := append([]byte(nil), r.body.Bytes()...)
		go r.processEncodedUsage(compressedBody)
		return
	}

	_ = r.parser.Finish()
	streamBody := append([]byte(nil), r.body.Bytes()...)
	r.emitUsage(r.usageOrFallback(&r.parser))
	r.emitComplete(streamBody)
}

func (r *responsesUsageTrackingReader) processEncodedUsage(compressedBody []byte) {
	body, err := decodeResponseBody(compressedBody, r.contentEncoding)
	if err != nil {
		return
	}

	var parser responsesSSEUsageParser
	_ = parser.Write(body)
	_ = parser.Finish()
	r.emitUsage(r.usageOrFallback(&parser))
	r.emitComplete(body)
}

func (r *responsesUsageTrackingReader) usageOrFallback(parser *responsesSSEUsageParser) *usage.Usage {
	parsedUsage := parser.Usage()
	if parsedUsage != nil {
		return parsedUsage
	}
	if r.req == nil {
		return nil
	}
	parsedUsage, err := fallbackUsage(r.req, nil, parser, r.model)
	if err != nil {
		return nil
	}
	return parsedUsage
}

func (r *responsesUsageTrackingReader) emitUsage(parsedUsage *usage.Usage) {
	if parsedUsage == nil {
		return
	}
	if r.onUsage != nil {
		r.onUsage(parsedUsage)
	}
}

func fallbackUsage(req *http.Request, decodedBody []byte, streamParser *responsesSSEUsageParser, model string) (*usage.Usage, error) {
	requestBody, err := readRequestBody(req)
	if err != nil {
		return nil, err
	}
	path := ""
	if req != nil && req.URL != nil {
		path = req.URL.Path
	}
	encoding := openAIEncoding(model)

	var promptTokens int
	switch {
	case strings.Contains(path, "/v1/chat/completions"):
		promptTokens, err = estimateChatPromptTokens(requestBody, encoding)
	case strings.Contains(path, "/v1/responses"):
		promptTokens, err = estimateResponsesPromptTokens(requestBody, encoding)
	default:
		return nil, fmt.Errorf("unsupported openai fallback path: %s", path)
	}
	if err != nil {
		return nil, err
	}

	var completionTexts []string
	if streamParser != nil {
		completionTexts, err = streamParser.CompletionTexts(path)
	} else {
		completionTexts, err = extractNonStreamingCompletionTexts(path, decodedBody)
	}
	if err != nil {
		return nil, err
	}
	completionTokens, err := countTextTokens(completionTexts, encoding)
	if err != nil {
		return nil, err
	}

	return &usage.Usage{PromptTokens: int64(promptTokens), CompletionTokens: int64(completionTokens)}, nil
}

func readRequestBody(req *http.Request) ([]byte, error) {
	if req == nil {
		return nil, errors.New("missing openai request for usage fallback")
	}
	if req.GetBody == nil {
		return nil, errors.New("missing openai request GetBody for usage fallback")
	}
	body, err := req.GetBody()
	if err != nil {
		return nil, fmt.Errorf("get openai request body for usage fallback: %w", err)
	}
	raw, readErr := io.ReadAll(body)
	closeErr := body.Close()
	if readErr != nil {
		if closeErr != nil {
			return nil, fmt.Errorf("read openai request body for usage fallback: %w; close request body: %v", readErr, closeErr)
		}
		return nil, fmt.Errorf("read openai request body for usage fallback: %w", readErr)
	}
	if closeErr != nil {
		return nil, fmt.Errorf("close openai request body for usage fallback: %w", closeErr)
	}
	return raw, nil
}

func estimateChatPromptTokens(requestBody []byte, encoding string) (int, error) {
	var req struct {
		Messages []openAIMessage `json:"messages"`
	}
	if err := json.Unmarshal(requestBody, &req); err != nil {
		return 0, fmt.Errorf("decode chat completions request for usage fallback: %w", err)
	}

	tokens := replyPrimingTokens
	for _, message := range req.Messages {
		tokens += tokensPerMessage
		roleTokens, err := usage.GetTextToken(message.Role, encoding)
		if err != nil {
			return 0, fmt.Errorf("count chat role tokens: %w", err)
		}
		tokens += roleTokens
		if strings.TrimSpace(message.Name) != "" {
			nameTokens, err := usage.GetTextToken(message.Name, encoding)
			if err != nil {
				return 0, fmt.Errorf("count chat name tokens: %w", err)
			}
			tokens += tokensPerName + nameTokens
		}
		contentTokens, err := countTextTokens(extractTextsFromRawContent(message.Content), encoding)
		if err != nil {
			return 0, fmt.Errorf("count chat content tokens: %w", err)
		}
		tokens += contentTokens
	}
	return tokens, nil
}

func estimateResponsesPromptTokens(requestBody []byte, encoding string) (int, error) {
	var req struct {
		Instructions string          `json:"instructions"`
		Input        json.RawMessage `json:"input"`
	}
	if err := json.Unmarshal(requestBody, &req); err != nil {
		return 0, fmt.Errorf("decode responses request for usage fallback: %w", err)
	}

	texts := make([]string, 0, 2)
	if strings.TrimSpace(req.Instructions) != "" {
		texts = append(texts, req.Instructions)
	}
	texts = append(texts, extractTextsFromRawContent(req.Input)...)
	return countTextTokens(texts, encoding)
}

func extractNonStreamingCompletionTexts(path string, decodedBody []byte) ([]string, error) {
	switch {
	case strings.Contains(path, "/v1/chat/completions"):
		return extractChatCompletionTexts(decodedBody)
	case strings.Contains(path, "/v1/responses"):
		return extractResponsesCompletionTexts(decodedBody)
	default:
		return nil, fmt.Errorf("unsupported openai fallback path: %s", path)
	}
}

func extractChatCompletionTexts(decodedBody []byte) ([]string, error) {
	var resp struct {
		Choices []struct {
			Message openAIMessage `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(decodedBody, &resp); err != nil {
		return nil, fmt.Errorf("decode chat completions response for usage fallback: %w", err)
	}
	texts := make([]string, 0, len(resp.Choices))
	for _, choice := range resp.Choices {
		texts = append(texts, extractTextsFromRawContent(choice.Message.Content)...)
	}
	return texts, nil
}

func extractResponsesCompletionTexts(decodedBody []byte) ([]string, error) {
	var resp struct {
		OutputText string         `json:"output_text"`
		Output     []openAIOutput `json:"output"`
	}
	if err := json.Unmarshal(decodedBody, &resp); err != nil {
		return nil, fmt.Errorf("decode responses response for usage fallback: %w", err)
	}
	return responsesOutputTexts(resp.OutputText, resp.Output), nil
}

func responsesOutputTexts(outputText string, output []openAIOutput) []string {
	var texts []string
	if strings.TrimSpace(outputText) != "" {
		texts = append(texts, outputText)
	}
	for _, item := range output {
		for _, part := range item.Content {
			if strings.TrimSpace(part.Text) != "" {
				texts = append(texts, part.Text)
			}
			texts = append(texts, extractTextsFromRawContent(part.Content)...)
		}
	}
	return texts
}

func extractTextsFromRawContent(raw json.RawMessage) []string {
	if len(raw) == 0 || string(raw) == "null" {
		return nil
	}

	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		if strings.TrimSpace(text) == "" {
			return nil
		}
		return []string{text}
	}

	var items []json.RawMessage
	if err := json.Unmarshal(raw, &items); err == nil {
		var texts []string
		for _, item := range items {
			texts = append(texts, extractTextsFromRawContent(item)...)
		}
		return texts
	}

	var item openAIContentPart
	if err := json.Unmarshal(raw, &item); err != nil {
		return nil
	}
	var texts []string
	if strings.TrimSpace(item.Text) != "" {
		texts = append(texts, item.Text)
	}
	texts = append(texts, extractTextsFromRawContent(item.Content)...)
	return texts
}

func countTextTokens(texts []string, encoding string) (int, error) {
	total := 0
	for _, text := range texts {
		if strings.TrimSpace(text) == "" {
			continue
		}
		count, err := usage.GetTextToken(text, encoding)
		if err != nil {
			return 0, err
		}
		total += count
	}
	return total, nil
}

func openAIEncoding(model string) string {
	return defaultOpenAIEncoding
}

func (r *responsesUsageTrackingReader) emitComplete(body []byte) {
	if r.onComplete != nil {
		r.onComplete(body)
	}
}

type responsesSSEUsageParser struct {
	lineBuf            bytes.Buffer
	event              string
	data               bytes.Buffer
	usage              *usage.Usage
	chatBuilder        strings.Builder
	responsesDeltas    strings.Builder
	responsesFinalText []string
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

func (p *responsesSSEUsageParser) CompletionTexts(path string) ([]string, error) {
	switch {
	case strings.Contains(path, "/v1/chat/completions"):
		text := p.chatBuilder.String()
		if strings.TrimSpace(text) == "" {
			return nil, nil
		}
		return []string{text}, nil
	case strings.Contains(path, "/v1/responses"):
		if len(p.responsesFinalText) > 0 {
			return p.responsesFinalText, nil
		}
		text := p.responsesDeltas.String()
		if strings.TrimSpace(text) == "" {
			return nil, nil
		}
		return []string{text}, nil
	default:
		return nil, fmt.Errorf("unsupported openai fallback path: %s", path)
	}
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
	if openaiUsage != nil {
		parsedUsage, err := openAIUsageToUsage(openaiUsage)
		if err != nil && !errors.Is(err, errMissingOpenAIUsage) {
			return err
		}
		if err == nil {
			p.usage = parsedUsage
		}
	}

	for _, choice := range streamEvent.Choices {
		texts := extractTextsFromRawContent(choice.Delta.Content)
		for _, text := range texts {
			_, _ = p.chatBuilder.WriteString(text)
		}
	}
	if streamEvent.Delta != "" {
		_, _ = p.responsesDeltas.WriteString(streamEvent.Delta)
	}
	if streamEvent.Text != "" {
		p.responsesFinalText = append(p.responsesFinalText, streamEvent.Text)
	}
	p.responsesFinalText = append(p.responsesFinalText, responsesOutputTexts(streamEvent.Response.OutputText, streamEvent.Response.Output)...)
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
