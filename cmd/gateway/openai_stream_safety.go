package main

import (
	"bytes"
	"fmt"
	"net/http"
	"strings"

	sensitiveopenai "github.com/HyperToken-dev/fabric/business/sensitive/openai"
	coreproxy "github.com/HyperToken-dev/fabric/core/proxy"
	"github.com/HyperToken-dev/fabric/protocol/sse"

	"go.uber.org/zap"
)

var openAIStreamSafetyTailRunes = 256

type openAIStreamSafetyProcessor struct {
	proxy      *OpenAIProxy
	resp       *http.Response
	req        *http.Request
	codec      openAIStreamSafetyCodec
	model      string
	keyID      int32
	channelID  int32
	modelID    int32
	parser     sse.Parser
	tail       string       // tail string of sliding window
	rawBody    bytes.Buffer // upstream SSE bytes received before refusal, used for fallback usage
	clientBody bytes.Buffer // log all data that sent to client.use to log instantly when stream refused
	rejected   bool         // when stream was refused change this to true
	logged     bool         // make sure just log once
}

type openAIStreamSafetyCodec interface {
	Extract(event sse.Event) (sensitiveopenai.StreamText, bool, error)
	RewriteDelta(event sse.Event, text string) ([]byte, error)
	NewDelta(text string) []byte
	IsDone(event sse.Event) bool
	FlushBefore(event sse.Event) bool
}

type chatCompletionStreamSafetyCodec struct{}

type responsesStreamSafetyCodec struct {
	metadata sensitiveopenai.ResponsesStreamDeltaMetadata
}

func (p *OpenAIProxy) openAIStreamTransform(resp *http.Response) (coreproxy.StreamProcessor, bool, error) {
	if resp == nil || resp.Request == nil {
		return nil, false, nil
	}
	// when error or redirect,no detect
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, false, nil
	}

	if !isOpenAIUsageStream(resp.Request, resp.Header.Get("Content-Type")) {
		return nil, false, nil
	}
	codec, ok := newOpenAIStreamSafetyCodec(resp.Request.URL.Path)
	if !ok {
		return nil, false, nil
	}

	return &openAIStreamSafetyProcessor{
		proxy:     p,
		resp:      resp,
		req:       resp.Request,
		codec:     codec,
		model:     getContextString(resp.Request, ctxModel),
		keyID:     getContextInt32(resp.Request, ctxKeyID),
		channelID: getContextInt32(resp.Request, ctxChannelID),
		modelID:   getContextInt32(resp.Request, ctxModelID),
	}, true, nil
}

func newOpenAIStreamSafetyCodec(path string) (openAIStreamSafetyCodec, bool) {
	switch {
	case strings.Contains(path, "/v1/chat/completions"):
		return chatCompletionStreamSafetyCodec{}, true
	case strings.Contains(path, "/v1/responses"):
		return &responsesStreamSafetyCodec{}, true
	default:
		return nil, false
	}
}

func (p *openAIStreamSafetyProcessor) Write(chunk []byte) (coreproxy.StreamResult, error) {
	// when stream was refused,just droping data and interrupt the data stream
	if p.rejected {
		return coreproxy.StreamResult{Stop: true}, nil
	}
	_, _ = p.rawBody.Write(chunk)

	// put the net fragmentation to state machine,extract logically meaningful event
	events, err := p.parser.Write(chunk)
	if err != nil {
		return p.reject(fmt.Errorf("parse openai stream: %w", err)), nil
	}
	return p.processEvents(events), nil
}

// process finally,get the last data
func (p *openAIStreamSafetyProcessor) Finish() (coreproxy.StreamResult, error) {
	if p.rejected {
		return coreproxy.StreamResult{}, nil
	}

	// force the parser to output the remaining unread content
	events, err := p.parser.Finish()
	if err != nil {
		return p.reject(fmt.Errorf("finish openai stream parser: %w", err)), nil
	}
	result := p.processEvents(events)
	if result.Stop {
		return result, nil
	}

	// no sensitive words were triggered throughout the process and the connection is about to close
	// the tail data must now be sent to the frontend
	if p.tail != "" {
		tailEvent := p.codec.NewDelta(p.tail)
		result.Data = append(result.Data, tailEvent...)
		_, _ = p.clientBody.Write(tailEvent)
		p.tail = ""
	}
	return result, nil
}

func (p *openAIStreamSafetyProcessor) Close() error {
	return nil
}

// processEvents iterates through all parsed event packets to process them centrally.
func (p *openAIStreamSafetyProcessor) processEvents(events []sse.Event) coreproxy.StreamResult {
	var out []byte
	for _, event := range events {
		data, stop := p.processEvent(event)
		if len(data) > 0 {
			out = append(out, data...)
		}
		// if any of these events triggers sensitive detector,stop immediately and return
		if stop {
			if len(out) > 0 {
				_, _ = p.clientBody.Write(out)
			}
			return coreproxy.StreamResult{Data: out, Stop: true}
		}
	}
	if len(out) > 0 {
		_, _ = p.clientBody.Write(out)
	}
	return coreproxy.StreamResult{Data: out}
}

// for just one complete event use sliding window to splice and detect
func (p *openAIStreamSafetyProcessor) processEvent(event sse.Event) ([]byte, bool) {
	// if encounter a stream complete sign
	if p.codec.IsDone(event) {
		var out []byte
		if p.tail != "" {
			out = append(out, p.codec.NewDelta(p.tail)...)
			p.tail = ""
		}
		// append [DONE] stream event
		out = append(out, event.Raw...)
		return out, false
	}
	streamText, ok, err := p.codec.Extract(event)
	if err != nil {
		result := p.reject(err)
		return result.Data, result.Stop
	}
	if !ok {
		// unrecognizable content,may be tool call,just allow
		if p.codec.FlushBefore(event) && p.tail != "" {
			out := p.codec.NewDelta(p.tail)
			p.tail = ""
			out = append(out, event.Raw...)
			return out, false
		}
		return event.Raw, false
	}
	if streamText.Kind == sensitiveopenai.StreamTextSnapshot {
		if p.detectRejected(streamText.Text) {
			rejectResult := p.reject(nil)
			return rejectResult.Data, rejectResult.Stop
		}
		var out []byte
		if p.tail != "" {
			out = append(out, p.codec.NewDelta(p.tail)...)
			p.tail = ""
		}
		out = append(out, event.Raw...)
		return out, false
	}

	// history tail + latest string = all of the text that need to detect now
	candidate := p.tail + streamText.Text
	if p.detectRejected(candidate) {
		rejectResult := p.reject(nil)
		return rejectResult.Data, rejectResult.Stop
	}

	// program run to this site means it is safe content
	// safeText: the content prepare sending to client
	// tail: remain in the sliding window and used by next round
	safeText, tail := splitSafeStreamText(candidate, openAIStreamSafetyTailRunes)
	p.tail = tail

	// encapsulate the safe text back into the SSE packet
	rewritten, err := p.codec.RewriteDelta(event, safeText)
	if err != nil {
		result := p.reject(err)
		return result.Data, result.Stop
	}
	return rewritten, false
}

func (p *openAIStreamSafetyProcessor) detectRejected(text string) bool {
	policy := p.proxy.textPolicy
	if policy == nil {
		policy = NoopTextPolicy{}
	}
	result := policy.Detect(p.req.Context(), p.model, text)
	if !result.Rejected() {
		return false
	}
	zap.L().Info("sensitive text rejected",
		zap.String("direction", string(TextDirectionOutput)),
		zap.String("model", p.model),
		zap.String("text", text),
		zap.Any("matches", result.Matches),
	)
	return true
}

func (chatCompletionStreamSafetyCodec) Extract(event sse.Event) (sensitiveopenai.StreamText, bool, error) {
	text, ok, err := sensitiveopenai.ExtractChatCompletionStreamText(event)
	if err != nil || !ok {
		return sensitiveopenai.StreamText{}, ok, err
	}
	return sensitiveopenai.StreamText{Text: text, Kind: sensitiveopenai.StreamTextDelta}, true, nil
}

func (chatCompletionStreamSafetyCodec) RewriteDelta(event sse.Event, text string) ([]byte, error) {
	return sensitiveopenai.RewriteChatCompletionStreamText(event, text)
}

func (chatCompletionStreamSafetyCodec) NewDelta(text string) []byte {
	return sensitiveopenai.NewChatCompletionStreamTextEvent(text)
}

func (chatCompletionStreamSafetyCodec) IsDone(event sse.Event) bool {
	return event.DataEquals("[DONE]")
}

func (chatCompletionStreamSafetyCodec) FlushBefore(event sse.Event) bool {
	return false
}

func (c *responsesStreamSafetyCodec) Extract(event sse.Event) (sensitiveopenai.StreamText, bool, error) {
	metadata, ok, err := sensitiveopenai.ExtractResponsesStreamDeltaMetadata(event)
	if err != nil {
		return sensitiveopenai.StreamText{}, false, err
	}
	if ok {
		c.metadata = metadata
	}
	return sensitiveopenai.ExtractResponsesStreamText(event)
}

func (c *responsesStreamSafetyCodec) RewriteDelta(event sse.Event, text string) ([]byte, error) {
	return sensitiveopenai.RewriteResponsesStreamDelta(event, text)
}

func (c *responsesStreamSafetyCodec) NewDelta(text string) []byte {
	return sensitiveopenai.NewResponsesStreamDeltaEvent(text, c.metadata)
}

func (c *responsesStreamSafetyCodec) IsDone(event sse.Event) bool {
	return event.DataEquals("[DONE]")
}

func (c *responsesStreamSafetyCodec) FlushBefore(event sse.Event) bool {
	return event.Event == "response.output_text.done" || event.Event == "response.content_part.done" || event.Event == "response.output_item.done" || event.Event == "response.completed"
}

// breaking operation,execute blocking logic and send audit log
func (p *openAIStreamSafetyProcessor) reject(err error) coreproxy.StreamResult {
	if err != nil {
		zap.L().Error("openai stream output safety rejected stream", zap.Error(err), zap.Int32("key_id", p.keyID), zap.Int32("channel_id", p.channelID), zap.Int32("model_id", p.modelID), zap.String("model", p.model))
	}
	p.rejected = true
	p.tail = ""
	rejection := openAIStreamRejectionSSE()
	if !p.logged {
		p.logged = true
		rawBody := append([]byte(nil), p.rawBody.Bytes()...)
		go p.proxy.processStreamingUsageAsync(p.req, rawBody, p.keyID, p.channelID, p.modelID, p.model)
		// concatenate the part already sent to the user (which may contain the first half of the sensitive word) with the final packet that triggered the error
		responseBody := append([]byte(nil), p.clientBody.Bytes()...)
		responseBody = append(responseBody, rejection...)
		go processIntegralLogAsync(p.proxy.integralLogs, p.req, integralLogInfo{
			Provider:            ProviderOpenAI,
			APIFormat:           1,
			KeyID:               p.keyID,
			ChannelID:           p.channelID,
			ModelID:             p.modelID,
			Model:               p.model,
			Outcome:             integralOutcomeRejected,
			RejectionStage:      rejectionStageOutput,
			RejectionReason:     rejectionReasonSensitive,
			ResponseStatus:      p.resp.StatusCode,
			ResponseContentType: p.resp.Header.Get("Content-Type"),
			DecodeOK:            true,
		}, responseBody)
	}
	// return the struct that contains error data,notice core layer to stop proxy
	return coreproxy.StreamResult{Data: rejection, Stop: true}
}

// split text after confirming text is safe
func splitSafeStreamText(text string, tailRunes int) (string, string) {
	if tailRunes <= 0 {
		return text, ""
	}
	runes := []rune(text)
	if len(runes) <= tailRunes {
		return "", text
	}
	split := len(runes) - tailRunes
	return string(runes[:split]), string(runes[split:])
}

// generate sse error event with openai protocol style
func openAIStreamRejectionSSE() []byte {
	return []byte("event: error\ndata: {\"error\":{\"message\":\"model output rejected, please change your prompt\",\"type\":\"policy_violation\",\"code\":\"sensitive_output\"}}\n\ndata: [DONE]\n\n")
}
