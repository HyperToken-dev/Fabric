package openai

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/HyperToken-dev/fabric/business/usage"

	"github.com/andybalholm/brotli"
)

func TestExtractNonStreaming(t *testing.T) {
	tests := []struct {
		name     string
		body     []byte
		encoding string
		want     *usage.Usage
		wantErr  bool
	}{
		{
			name: "chat usage identity",
			body: []byte(`{"usage":{"prompt_tokens":12,"completion_tokens":34}}`),
			want: &usage.Usage{PromptTokens: 12, CompletionTokens: 34},
		},
		{
			name:     "responses usage identity with whitespace encoding",
			body:     []byte(`{"usage":{"input_tokens":5,"output_tokens":8}}`),
			encoding: " identity ",
			want:     &usage.Usage{PromptTokens: 5, CompletionTokens: 8},
		},
		{
			name:     "gzip usage",
			body:     gzipBytes(t, []byte(`{"usage":{"prompt_tokens":7,"completion_tokens":9}}`)),
			encoding: "gzip",
			want:     &usage.Usage{PromptTokens: 7, CompletionTokens: 9},
		},
		{
			name:     "brotli usage",
			body:     brotliBytes(t, []byte(`{"usage":{"input_tokens":11,"output_tokens":13}}`)),
			encoding: "br",
			want:     &usage.Usage{PromptTokens: 11, CompletionTokens: 13},
		},
		{
			name:    "missing usage",
			body:    []byte(`{"id":"resp"}`),
			wantErr: true,
		},
		{
			name:     "unsupported encoding",
			body:     []byte(`{"usage":{"prompt_tokens":1,"completion_tokens":2}}`),
			encoding: "deflate",
			wantErr:  true,
		},
		{
			name:    "invalid json",
			body:    []byte(`{`),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ExtractNonStreaming(tt.body, tt.encoding)
			if tt.wantErr {
				if err == nil {
					t.Fatal("ExtractNonStreaming() error = nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("ExtractNonStreaming() error = %v", err)
			}
			assertUsage(t, got, tt.want)
		})
	}
}

func TestExtractNonStreamingWithFallbackChatCompletions(t *testing.T) {
	req := newOpenAITestRequest(t, "/v1/chat/completions", `{"model":"gpt-5.5","messages":[{"role":"user","content":"hello"}]}`)
	got, err := ExtractNonStreamingWithFallback(req, []byte(`{"choices":[{"message":{"content":"safe answer"}}]}`), "", "gpt-5.5")
	if err != nil {
		t.Fatalf("ExtractNonStreamingWithFallback() error = %v", err)
	}

	roleTokens := mustTokenCount(t, "user")
	promptTokens := int64(replyPrimingTokens + tokensPerMessage + roleTokens + mustTokenCount(t, "hello"))
	completionTokens := int64(mustTokenCount(t, "safe answer"))
	assertUsage(t, got, &usage.Usage{PromptTokens: promptTokens, CompletionTokens: completionTokens})
}

func TestExtractNonStreamingWithFallbackResponses(t *testing.T) {
	req := newOpenAITestRequest(t, "/v1/responses", `{"model":"gpt-5.5","instructions":"be brief","input":"hello"}`)
	got, err := ExtractNonStreamingWithFallback(req, []byte(`{"output_text":"summary answer"}`), "", "gpt-5.5")
	if err != nil {
		t.Fatalf("ExtractNonStreamingWithFallback() error = %v", err)
	}

	promptTokens := int64(mustTokenCount(t, "be brief") + mustTokenCount(t, "hello"))
	completionTokens := int64(mustTokenCount(t, "summary answer"))
	assertUsage(t, got, &usage.Usage{PromptTokens: promptTokens, CompletionTokens: completionTokens})
}

func TestExtractNonStreamingWithFallbackPrefersUpstreamUsage(t *testing.T) {
	got, err := ExtractNonStreamingWithFallback(nil, []byte(`{"usage":{"prompt_tokens":2,"completion_tokens":3},"choices":[{"message":{"content":"safe answer"}}]}`), "", "gpt-5.5")
	if err != nil {
		t.Fatalf("ExtractNonStreamingWithFallback() error = %v", err)
	}
	assertUsage(t, got, &usage.Usage{PromptTokens: 2, CompletionTokens: 3})
}

func TestExtractNonStreamingWithFallbackMissingGetBody(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-5.5","messages":[{"role":"user","content":"hello"}]}`))
	req.GetBody = nil
	_, err := ExtractNonStreamingWithFallback(req, []byte(`{"choices":[{"message":{"content":"safe answer"}}]}`), "", "gpt-5.5")
	if err == nil {
		t.Fatal("ExtractNonStreamingWithFallback() error = nil, want error")
	}
}

func TestNewTrackingReaderStreamsUsage(t *testing.T) {
	sse := strings.Join([]string{
		": comment",
		"event: response.completed",
		`data: {"response":{"usage":{"input_tokens":21,"output_tokens":34}}}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")
	usageCh := make(chan *usage.Usage, 1)

	reader := NewTrackingReader(io.NopCloser(strings.NewReader(sse)), "", func(u *usage.Usage) {
		usageCh <- u
	}, nil)
	if _, err := io.Copy(io.Discard, reader); err != nil {
		t.Fatalf("Copy() error = %v", err)
	}

	assertUsage(t, receiveUsage(t, usageCh), &usage.Usage{PromptTokens: 21, CompletionTokens: 34})
}

func TestNewTrackingReaderCallsCompleteWithStreamBody(t *testing.T) {
	sse := "data: {\"usage\":{\"prompt_tokens\":3,\"completion_tokens\":4}}\n\n"
	completeCh := make(chan []byte, 1)
	reader := NewTrackingReader(io.NopCloser(strings.NewReader(sse)), "", nil, func(body []byte) {
		completeCh <- body
	})
	if _, err := io.Copy(io.Discard, reader); err != nil {
		t.Fatalf("Copy() error = %v", err)
	}

	select {
	case got := <-completeCh:
		if string(got) != sse {
			t.Fatalf("complete body = %q, want %q", got, sse)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for complete body")
	}
}

func TestNewTrackingReaderCallsCompleteWithDecodedGzipBody(t *testing.T) {
	sse := []byte("data: {\"response\":{\"usage\":{\"input_tokens\":6,\"output_tokens\":7}}}\n\n")
	completeCh := make(chan []byte, 1)
	reader := NewTrackingReader(io.NopCloser(bytes.NewReader(gzipBytes(t, sse))), "gzip", nil, func(body []byte) {
		completeCh <- body
	})
	if _, err := io.Copy(io.Discard, reader); err != nil {
		t.Fatalf("Copy() error = %v", err)
	}

	select {
	case got := <-completeCh:
		if string(got) != string(sse) {
			t.Fatalf("complete body = %q, want %q", got, sse)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for complete body")
	}
}

func TestNewTrackingReaderStreamsTopLevelUsageAcrossChunks(t *testing.T) {
	usageCh := make(chan *usage.Usage, 1)
	reader := NewTrackingReader(io.NopCloser(strings.NewReader(`data: {"usage":{"prompt_tokens":3,"completion_tokens":4}}`)), "identity", func(u *usage.Usage) {
		usageCh <- u
	}, nil)
	buf := make([]byte, 5)
	for {
		_, err := reader.Read(buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Read() error = %v", err)
		}
	}

	assertUsage(t, receiveUsage(t, usageCh), &usage.Usage{PromptTokens: 3, CompletionTokens: 4})
}

func TestNewTrackingReaderGzipStreamUsage(t *testing.T) {
	usageCh := make(chan *usage.Usage, 1)
	body := gzipBytes(t, []byte("data: {\"response\":{\"usage\":{\"input_tokens\":6,\"output_tokens\":7}}}\n\n"))
	reader := NewTrackingReader(io.NopCloser(bytes.NewReader(body)), "gzip", func(u *usage.Usage) {
		usageCh <- u
	}, nil)
	if _, err := io.Copy(io.Discard, reader); err != nil {
		t.Fatalf("Copy() error = %v", err)
	}

	assertUsage(t, receiveUsage(t, usageCh), &usage.Usage{PromptTokens: 6, CompletionTokens: 7})
}

func TestNewTrackingReaderFallbackChatCompletions(t *testing.T) {
	req := newOpenAITestRequest(t, "/v1/chat/completions", `{"model":"gpt-5.5","stream":true,"messages":[{"role":"user","content":"hello"}]}`)
	sse := strings.Join([]string{
		`data: {"choices":[{"delta":{"content":"safe "}}]}`,
		"",
		`data: {"choices":[{"delta":{"content":"answer"}}]}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")
	usageCh := make(chan *usage.Usage, 1)

	reader := NewTrackingReaderWithFallback(req, io.NopCloser(strings.NewReader(sse)), "", "gpt-5.5", func(u *usage.Usage) {
		usageCh <- u
	}, nil)
	if _, err := io.Copy(io.Discard, reader); err != nil {
		t.Fatalf("Copy() error = %v", err)
	}

	roleTokens := mustTokenCount(t, "user")
	promptTokens := int64(replyPrimingTokens + tokensPerMessage + roleTokens + mustTokenCount(t, "hello"))
	completionTokens := int64(mustTokenCount(t, "safe answer"))
	assertUsage(t, receiveUsage(t, usageCh), &usage.Usage{PromptTokens: promptTokens, CompletionTokens: completionTokens})
}

func TestNewTrackingReaderFallbackResponses(t *testing.T) {
	req := newOpenAITestRequest(t, "/v1/responses", `{"model":"gpt-5.5","instructions":"be brief","input":"hello"}`)
	sse := strings.Join([]string{
		`event: response.output_text.delta`,
		`data: {"delta":"summary "}`,
		"",
		`event: response.output_text.delta`,
		`data: {"delta":"answer"}`,
		"",
		"data: [DONE]",
		"",
	}, "\n")
	usageCh := make(chan *usage.Usage, 1)

	reader := NewTrackingReaderWithFallback(req, io.NopCloser(strings.NewReader(sse)), "", "gpt-5.5", func(u *usage.Usage) {
		usageCh <- u
	}, nil)
	if _, err := io.Copy(io.Discard, reader); err != nil {
		t.Fatalf("Copy() error = %v", err)
	}

	promptTokens := int64(mustTokenCount(t, "be brief") + mustTokenCount(t, "hello"))
	completionTokens := int64(mustTokenCount(t, "summary answer"))
	assertUsage(t, receiveUsage(t, usageCh), &usage.Usage{PromptTokens: promptTokens, CompletionTokens: completionTokens})
}

func TestNewTrackingReaderNoUsageCallbackAndClose(t *testing.T) {
	inner := &trackingReadCloser{Reader: strings.NewReader("data: [DONE]\n\n")}
	reader := NewTrackingReader(inner, "", nil, nil)
	if _, err := io.Copy(io.Discard, reader); err != nil {
		t.Fatalf("Copy() error = %v", err)
	}
	if err := reader.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}
	if !inner.closed {
		t.Fatal("Close() did not close underlying reader")
	}
}

type trackingReadCloser struct {
	*strings.Reader
	closed bool
}

func (r *trackingReadCloser) Close() error {
	r.closed = true
	return nil
}

func gzipBytes(t *testing.T, body []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := gzip.NewWriter(&buf)
	if _, err := w.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func brotliBytes(t *testing.T, body []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	w := brotli.NewWriter(&buf)
	if _, err := w.Write(body); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func receiveUsage(t *testing.T, usageCh <-chan *usage.Usage) *usage.Usage {
	t.Helper()
	select {
	case got := <-usageCh:
		return got
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for usage")
		return nil
	}
}

func newOpenAITestRequest(t *testing.T, path string, body string) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(strings.NewReader(body)), nil
	}
	return req
}

func mustTokenCount(t *testing.T, text string) int {
	t.Helper()
	count, err := usage.GetTextToken(text, defaultOpenAIEncoding)
	if err != nil {
		t.Fatalf("GetTextToken(%q) error = %v", text, err)
	}
	return count
}

func assertUsage(t *testing.T, got, want *usage.Usage) {
	t.Helper()
	if got == nil {
		t.Fatal("usage = nil")
	}
	if got.PromptTokens != want.PromptTokens || got.CompletionTokens != want.CompletionTokens {
		t.Fatalf("usage = %+v, want %+v", got, want)
	}
}
