package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"

	sensitiveopenai "github.com/HyperToken-dev/fabric/business/sensitive/openai"
	coreproxy "github.com/HyperToken-dev/fabric/core/proxy"
	"github.com/HyperToken-dev/fabric/protocol/sse"

	"go.uber.org/zap"
)

// openAIStreamSafetyTailRunes is the maximum suffix kept private per stream lane.
//
// Holding a suffix lets the detector see sensitive words that may be split across
// adjacent SSE deltas before those bytes are released to the client.
var openAIStreamSafetyTailRunes = 256

// openAIStreamSafetyProcessor inspects OpenAI-compatible SSE output before it is
// forwarded to the client.
//
// Concurrency: core proxy code calls Write, Finish, and Close for one response
// stream. The processor owns mutable parser, buffer, and tail state and is not
// safe for concurrent use.
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
	tails      map[string]sensitiveopenai.StreamText // lane => unreleased suffix waiting for cross-chunk detection
	rawBody    bytes.Buffer                          // upstream SSE bytes received before refusal, used for fallback usage
	clientBody bytes.Buffer                          // downstream SSE bytes sent before refusal, used for rejection audit logs
	rejected   bool                                  // set after refusal so later upstream chunks are dropped
	logged     bool                                  // guards async usage and integral logging from duplicate execution
}

// openAIStreamSafetyCodec isolates endpoint-specific SSE payload handling.
//
// Implementations must preserve provider protocol shape while exposing only
// text-bearing events to the safety processor.
type openAIStreamSafetyCodec interface {
	// Extract returns stream text entries from a complete SSE event.
	//
	// The bool is false when the event is not a supported text event and should
	// pass through unchanged unless FlushBefore says pending tails must be emitted
	// first.
	Extract(event sse.Event) ([]sensitiveopenai.StreamText, bool, error)
	// RewriteDelta rebuilds an existing delta event with the already-approved text.
	RewriteDelta(event sse.Event, texts []sensitiveopenai.StreamText) ([]byte, error)
	// NewDelta creates a protocol-compatible event for a previously withheld tail.
	NewDelta(text sensitiveopenai.StreamText) []byte
	// IsDone reports whether the event terminates the stream.
	IsDone(event sse.Event) bool
	// FlushBefore reports whether pending tails must be emitted before this event.
	FlushBefore(event sse.Event) bool
}

type chatCompletionStreamSafetyCodec struct{}

type responsesStreamSafetyCodec struct{}

func (p *OpenAIProxy) openAIStreamTransform(resp *http.Response) (coreproxy.StreamProcessor, bool, error) {
	if resp == nil || resp.Request == nil {
		return nil, false, nil
	}
	// Error and redirect responses are passed through because they are not model output streams.
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
		tails:     make(map[string]sensitiveopenai.StreamText),
	}, true, nil
}

// newOpenAIStreamSafetyCodec selects the protocol adapter for supported OpenAI
// streaming endpoints.
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

// Write buffers upstream SSE bytes, parses complete events, and releases only
// text that has passed output safety detection.
func (p *openAIStreamSafetyProcessor) Write(chunk []byte) (coreproxy.StreamResult, error) {
	// After refusal the downstream stream is already closed by the core layer.
	if p.rejected {
		return coreproxy.StreamResult{Stop: true}, nil
	}
	_, _ = p.rawBody.Write(chunk)

	// Network fragments are parsed into complete SSE events before inspection.
	events, err := p.parser.Write(chunk)
	if err != nil {
		return p.reject(fmt.Errorf("parse openai stream: %w", err)), nil
	}
	return p.processEvents(events), nil
}

// Finish flushes parser and safety tail state when upstream closes normally.
func (p *openAIStreamSafetyProcessor) Finish() (coreproxy.StreamResult, error) {
	if p.rejected {
		return coreproxy.StreamResult{}, nil
	}

	// Force the parser to output any remaining complete event buffered from TCP fragments.
	events, err := p.parser.Finish()
	if err != nil {
		return p.reject(fmt.Errorf("finish openai stream parser: %w", err)), nil
	}
	result := p.processEvents(events)
	if result.Stop {
		return result, nil
	}

	// No sensitive output was found. The held suffixes are now safe to release
	// because no later delta can join with them.
	if len(p.tails) > 0 {
		tailEvents := p.flushAllTails()
		result.Data = append(result.Data, tailEvents...)
		_, _ = p.clientBody.Write(tailEvents)
	}
	return result, nil
}

// Close satisfies coreproxy.StreamProcessor. The processor does not own external
// resources; stream accounting is handled by Finish or reject.
func (p *openAIStreamSafetyProcessor) Close() error {
	return nil
}

// processEvents applies output safety processing to parsed SSE events in order.
func (p *openAIStreamSafetyProcessor) processEvents(events []sse.Event) coreproxy.StreamResult {
	var out []byte
	for _, event := range events {
		data, rejected, err := p.processEvent(event)
		if len(data) > 0 {
			out = append(out, data...)
		}
		// Once any event is rejected, stop forwarding upstream data and send the
		// protocol-level refusal event as the final client-visible payload.
		if rejected || err != nil {
			if len(out) > 0 {
				_, _ = p.clientBody.Write(out)
			}
			rejectResult := p.reject(err)
			out = append(out, rejectResult.Data...)
			return coreproxy.StreamResult{Data: out, Stop: true}
		}
	}
	if len(out) > 0 {
		_, _ = p.clientBody.Write(out)
	}
	return coreproxy.StreamResult{Data: out}
}

// processEvent checks one complete SSE event and rewrites text deltas using a
// per-lane sliding tail window.
func (p *openAIStreamSafetyProcessor) processEvent(event sse.Event) ([]byte, bool, error) {
	if p.codec.IsDone(event) {
		out := p.flushAllTails()
		// The protocol terminator must remain last after releasing withheld tails.
		out = append(out, event.Raw...)
		return out, false, nil
	}
	streamTexts, ok, err := p.codec.Extract(event)
	if err != nil {
		return nil, false, err
	}
	if ok && len(streamTexts) == 0 {
		return event.Raw, false, nil
	}
	if !ok {
		// Non-text events are allowed, but response lifecycle events may need to
		// follow text that is currently held in the safety tail buffer.
		if p.codec.FlushBefore(event) && len(p.tails) > 0 {
			out := p.flushAllTails()
			out = append(out, event.Raw...)
			return out, false, nil
		}
		return event.Raw, false, nil
	}
	snapshotEvent := true
	for _, streamText := range streamTexts {
		if streamText.Kind != sensitiveopenai.StreamTextSnapshot {
			snapshotEvent = false
			break
		}
	}
	if snapshotEvent {
		// Snapshot events contain full current text instead of append-only deltas, so
		// they can be checked directly and then used to release matching old tails.
		for _, streamText := range streamTexts {
			if strings.TrimSpace(streamText.Text) != "" && p.detectRejected(streamText.Text) {
				return nil, true, nil
			}
		}
		var out []byte
		flushAll := false
		for _, streamText := range streamTexts {
			if streamText.LanePrefix != "" {
				lanes := make([]string, 0)
				for lane := range p.tails {
					if strings.HasPrefix(lane, streamText.LanePrefix) {
						lanes = append(lanes, lane)
					}
				}
				sort.Strings(lanes)
				for _, lane := range lanes {
					out = append(out, p.flushTail(lane)...)
				}
				continue
			}
			if streamText.Lane == "" {
				flushAll = true
				continue
			}
			out = append(out, p.flushTail(streamText.Lane)...)
		}
		if flushAll {
			out = append(out, p.flushAllTails()...)
		}
		out = append(out, event.Raw...)
		return out, false, nil
	}
	for _, streamText := range streamTexts {
		if streamText.Kind != sensitiveopenai.StreamTextDelta {
			return nil, false, fmt.Errorf("mixed openai stream text kinds")
		}
	}

	rewrittenTexts := make([]sensitiveopenai.StreamText, 0, len(streamTexts))
	for _, streamText := range streamTexts {
		lane := streamText.Lane
		if lane == "" {
			lane = "default"
			streamText.Lane = lane
		}
		candidate := p.tails[lane].Text + streamText.Text
		if p.detectRejected(candidate) {
			return nil, true, nil
		}

		// safeText is sent now; tail stays private until the next chunk can be
		// reviewed with it for sensitive words split across deltas.
		safeText, tail := splitSafeStreamText(candidate, openAIStreamSafetyTailRunes)
		if tail == "" {
			delete(p.tails, lane)
		} else {
			streamText.Text = tail
			p.tails[lane] = streamText
		}
		streamText.Text = safeText
		rewrittenTexts = append(rewrittenTexts, streamText)
	}

	// Re-encapsulate the safe text back into the original SSE event shape.
	rewritten, err := p.codec.RewriteDelta(event, rewrittenTexts)
	if err != nil {
		return nil, false, err
	}
	return rewritten, false, nil
}

// detectRejected evaluates candidate output text against the configured text policy.
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

// Extract exposes chat completion delta text for shared stream safety processing.
func (chatCompletionStreamSafetyCodec) Extract(event sse.Event) ([]sensitiveopenai.StreamText, bool, error) {
	return sensitiveopenai.ExtractChatCompletionStreamText(event)
}

// RewriteDelta rebuilds a chat completion delta event after unsafe suffixes have
// been withheld.
func (chatCompletionStreamSafetyCodec) RewriteDelta(event sse.Event, texts []sensitiveopenai.StreamText) ([]byte, error) {
	return sensitiveopenai.RewriteChatCompletionStreamText(event, texts)
}

// NewDelta creates a chat completion delta event for a withheld tail that later
// became safe to release.
func (chatCompletionStreamSafetyCodec) NewDelta(text sensitiveopenai.StreamText) []byte {
	if text.Text == "" {
		return nil
	}
	payload := map[string]any{
		"choices": []any{
			map[string]any{
				"delta": map[string]any{"content": text.Text},
				"index": text.ChoiceIndex,
			},
		},
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil
	}
	return sse.FormatData("", encoded)
}

// IsDone reports whether a chat completion event is the OpenAI stream terminator.
func (chatCompletionStreamSafetyCodec) IsDone(event sse.Event) bool {
	return event.DataEquals("[DONE]")
}

// FlushBefore is false for chat completions because non-text events do not close
// nested response content scopes.
func (chatCompletionStreamSafetyCodec) FlushBefore(event sse.Event) bool {
	return false
}

// Extract exposes Responses API output text events for shared stream safety processing.
func (c *responsesStreamSafetyCodec) Extract(event sse.Event) ([]sensitiveopenai.StreamText, bool, error) {
	streamText, ok, err := sensitiveopenai.ExtractResponsesStreamText(event)
	if err != nil || !ok {
		return nil, ok, err
	}
	return []sensitiveopenai.StreamText{streamText}, true, nil
}

// RewriteDelta rebuilds a Responses API text delta event after unsafe suffixes
// have been withheld.
func (c *responsesStreamSafetyCodec) RewriteDelta(event sse.Event, texts []sensitiveopenai.StreamText) ([]byte, error) {
	if len(texts) == 0 {
		return nil, nil
	}
	return sensitiveopenai.RewriteResponsesStreamDelta(event, texts[0].Text)
}

// NewDelta creates a Responses API delta event for a withheld tail that later
// became safe to release.
func (c *responsesStreamSafetyCodec) NewDelta(text sensitiveopenai.StreamText) []byte {
	if text.Text == "" {
		return nil
	}
	payload := map[string]any{
		"type":          "response.output_text.delta",
		"item_id":       text.ResponsesMetadata.ItemID,
		"output_index":  text.ResponsesMetadata.OutputIndex,
		"content_index": text.ResponsesMetadata.ContentIndex,
		"delta":         text.Text,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return nil
	}
	return sse.FormatData("response.output_text.delta", encoded)
}

// IsDone reports whether a Responses API event is the OpenAI stream terminator.
func (c *responsesStreamSafetyCodec) IsDone(event sse.Event) bool {
	return event.DataEquals("[DONE]")
}

// FlushBefore reports lifecycle events that must not overtake withheld text
// deltas in the Responses API stream.
func (c *responsesStreamSafetyCodec) FlushBefore(event sse.Event) bool {
	return event.Event == "response.output_text.done" || event.Event == "response.content_part.done" || event.Event == "response.output_item.done" || event.Event == "response.completed"
}

// reject emits an OpenAI-compatible refusal event and schedules audit side effects.
//
// The method is idempotent for logging because parser errors and content
// rejection can both route through the same terminal path.
func (p *openAIStreamSafetyProcessor) reject(err error) coreproxy.StreamResult {
	if err != nil {
		zap.L().Error("openai stream output safety rejected stream", zap.Error(err), zap.Int32("key_id", p.keyID), zap.Int32("channel_id", p.channelID), zap.Int32("model_id", p.modelID), zap.String("model", p.model))
	}
	p.rejected = true
	p.tails = nil
	rejection := openAIStreamRejectionSSE()
	if !p.logged {
		p.logged = true
		rawBody := append([]byte(nil), p.rawBody.Bytes()...)
		go p.proxy.processStreamingUsageAsync(p.req, rawBody, p.keyID, p.channelID, p.modelID, p.model)
		// Audit the exact client-visible body, including the final refusal event.
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
	// Stop tells the core layer to close the downstream stream after this payload.
	return coreproxy.StreamResult{Data: rejection, Stop: true}
}

// flushTail releases and clears the withheld suffix for one logical stream lane.
func (p *openAIStreamSafetyProcessor) flushTail(lane string) []byte {
	tail, ok := p.tails[lane]
	if !ok || tail.Text == "" {
		return nil
	}
	delete(p.tails, lane)
	return p.codec.NewDelta(tail)
}

// flushAllTails releases withheld suffixes in deterministic lane order.
func (p *openAIStreamSafetyProcessor) flushAllTails() []byte {
	if len(p.tails) == 0 {
		return nil
	}
	lanes := make([]string, 0, len(p.tails))
	for lane := range p.tails {
		lanes = append(lanes, lane)
	}
	sort.Strings(lanes)
	var out []byte
	for _, lane := range lanes {
		out = append(out, p.flushTail(lane)...)
	}
	return out
}

// splitSafeStreamText returns the immediately releasable prefix and the suffix
// that must remain withheld for cross-delta detection.
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

// openAIStreamRejectionSSE returns the client-visible refusal sequence in an
// OpenAI-compatible SSE shape.
func openAIStreamRejectionSSE() []byte {
	return []byte("event: error\ndata: {\"error\":{\"message\":\"model output rejected, please change your prompt\",\"type\":\"policy_violation\",\"code\":\"sensitive_output\"}}\n\ndata: [DONE]\n\n")
}
