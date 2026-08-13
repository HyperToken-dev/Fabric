package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/HyperToken-dev/fabric/business/sensitive"
	coreproxy "github.com/HyperToken-dev/fabric/core/proxy"
)

func TestOpenAIProxyServeHTTPValidationAndModelResolution(t *testing.T) {
	tests := []struct {
		name        string
		providerKey string
		body        string
		store       fakeModelStore
		policy      TextPolicy
		wantStatus  int
	}{
		{name: "missing provider key", providerKey: " ", body: `{"model":"gpt-5.5"}`, store: fakeModelStore{model: &ModelInfo{ID: 1, Status: ModelStatusActive}}, wantStatus: http.StatusBadGateway},
		{name: "invalid json", providerKey: "provider-key", body: `{`, store: fakeModelStore{model: &ModelInfo{ID: 1, Status: ModelStatusActive}}, wantStatus: http.StatusBadRequest},
		{name: "missing model", providerKey: "provider-key", body: `{"messages":[]}`, store: fakeModelStore{model: &ModelInfo{ID: 1, Status: ModelStatusActive}}, wantStatus: http.StatusBadRequest},
		{name: "prompt rejected", providerKey: "provider-key", body: `{"model":"gpt-5.5","messages":[{"role":"user","content":"blocked"}]}`, store: fakeModelStore{model: &ModelInfo{ID: 1, Status: ModelStatusActive}}, policy: rejectingPolicy{}, wantStatus: http.StatusForbidden},
		{name: "unsupported model", providerKey: "provider-key", body: `{"model":"gpt-5.5"}`, store: fakeModelStore{}, wantStatus: http.StatusBadRequest},
		{name: "banned model", providerKey: "provider-key", body: `{"model":"gpt-5.5"}`, store: fakeModelStore{model: &ModelInfo{ID: 1, Status: ModelStatusBanned}}, wantStatus: http.StatusForbidden},
		{name: "model lookup error", providerKey: "provider-key", body: `{"model":"gpt-5.5"}`, store: fakeModelStore{err: errors.New("lookup failed")}, wantStatus: http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			proxy, err := NewOpenAIProxy(OpenAIProxyOptions{ModelStore: tt.store, TextPolicy: tt.policy})
			if err != nil {
				t.Fatal(err)
			}
			upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				t.Fatal("upstream should not be called")
			}))
			defer upstream.Close()

			req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(tt.body))
			rec := httptest.NewRecorder()
			proxy.ServeHTTP(rec, req, 10, 20, upstream.URL, tt.providerKey)
			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d; body=%q", rec.Code, tt.wantStatus, rec.Body.String())
			}
		})
	}
}

func TestOpenAIProxyServeHTTPRecordsSensitiveInputRejectedIntegralLog(t *testing.T) {
	integralLogs := &recordingIntegralLogHandler{ch: make(chan recordedIntegralLog, 1)}
	proxy, err := NewOpenAIProxy(OpenAIProxyOptions{
		ModelStore:         fakeModelStore{model: &ModelInfo{ID: 1, Status: ModelStatusActive}},
		TextPolicy:         rejectingPolicy{},
		IntegralLogHandler: integralLogs,
	})
	if err != nil {
		t.Fatal(err)
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("upstream should not be called")
	}))
	defer upstream.Close()

	body := `{"model":"gpt-5.5","messages":[{"role":"user","content":"blocked"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req, 10, 20, upstream.URL, "provider-key")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}

	got := receiveIntegralLog(t, integralLogs.ch)
	if got.keyID != 10 {
		t.Fatalf("keyID = %d, want 10", got.keyID)
	}
	if got.response != "prompt rejected\n" {
		t.Fatalf("response = %q, want prompt rejected", got.response)
	}
	var loggedContext struct {
		Provider        string `json:"provider"`
		Outcome         string `json:"outcome"`
		RejectionStage  string `json:"rejection_stage"`
		RejectionReason string `json:"rejection_reason"`
		ResponseStatus  int    `json:"response_status"`
	}
	if err := json.Unmarshal([]byte(got.context), &loggedContext); err != nil {
		t.Fatal(err)
	}
	if loggedContext.Provider != "openai" || loggedContext.Outcome != "rejected" || loggedContext.RejectionStage != "input" || loggedContext.RejectionReason != "sensitive" || loggedContext.ResponseStatus != http.StatusForbidden {
		t.Fatalf("context = %+v", loggedContext)
	}
}

func TestOpenAIProxyServeHTTPForwardsActiveModelAndInjectsStreamOptions(t *testing.T) {
	usageHandler := &recordingUsageHandler{nonStreamingCh: make(chan UsageContext, 1)}
	proxy, err := NewOpenAIProxy(OpenAIProxyOptions{
		ModelStore:   fakeModelStore{model: &ModelInfo{ID: 42, Status: ModelStatusActive}},
		UsageHandler: usageHandler,
	})
	if err != nil {
		t.Fatal(err)
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer provider-key" {
			t.Errorf("Authorization = %q, want provider key", got)
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Contains(body, []byte(`"include_usage":true`)) {
			t.Fatalf("forwarded body = %s, want include_usage=true", body)
		}
		if !bytes.Contains(body, []byte(`"extra":"kept"`)) {
			t.Fatalf("forwarded body = %s, want existing stream option kept", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"usage":{"prompt_tokens":2,"completion_tokens":3},"choices":[{"message":{"content":"safe"}}]}`))
	}))
	defer upstream.Close()

	body := `{"model":"gpt-5.5","stream":true,"stream_options":{"extra":"kept"},"messages":[{"role":"user","content":"hello"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req, 10, 20, upstream.URL, "provider-key")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body=%q", rec.Code, rec.Body.String())
	}
	info := receiveUsageContext(t, usageHandler.nonStreamingCh)
	if info.KeyID != 10 || info.ChannelID != 20 || info.ModelID != 42 || info.Model != "gpt-5.5" {
		t.Fatalf("usage context = %+v", info)
	}
}

func TestInjectOpenAIChatStreamOptionsDoesNotModifyNonStreamingRequest(t *testing.T) {
	proxy, err := NewOpenAIProxy(OpenAIProxyOptions{ModelStore: fakeModelStore{model: &ModelInfo{ID: 1, Status: ModelStatusActive}}})
	if err != nil {
		t.Fatal(err)
	}

	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(body, []byte("stream_options")) {
			t.Fatalf("non-streaming body was modified: %s", body)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"usage":{"prompt_tokens":1,"completion_tokens":1}}`))
	}))
	defer upstream.Close()

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-5.5","messages":[]}`))
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req, 1, 1, upstream.URL, "provider-key")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
}

func TestOpenAIProxyOnCompleteProcessesStreamingUsage(t *testing.T) {
	usageHandler := &recordingUsageHandler{streamingCh: make(chan UsageContext, 1), streamingBodyCh: make(chan []byte, 1)}
	proxy, err := NewOpenAIProxy(OpenAIProxyOptions{UsageHandler: usageHandler})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", nil)
	req = setContextInt32(req, ctxKeyID, 1)
	req = setContextInt32(req, ctxChannelID, 2)
	req = setContextInt32(req, ctxModelID, 3)
	req = setContextString(req, ctxModel, "gpt-5.5")
	resp := &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("data: [DONE]\n\n")), Request: req}
	resp.Header.Set("Content-Type", "text/event-stream")
	resp.Header.Set("Content-Length", "99")

	if err := coreproxy.DefaultModifyResponse(proxy.onComplete)(resp); err != nil {
		t.Fatal(err)
	}
	if _, err := io.ReadAll(resp.Body); err != nil {
		t.Fatal(err)
	}
	info := receiveUsageContext(t, usageHandler.streamingCh)
	if info.KeyID != 1 || info.ChannelID != 2 || info.ModelID != 3 || info.Model != "gpt-5.5" {
		t.Fatalf("streaming context = %+v", info)
	}
	if resp.ContentLength != -1 || resp.Header.Get("Content-Length") != "" {
		t.Fatalf("streaming length not cleared: contentLength=%d header=%q", resp.ContentLength, resp.Header.Get("Content-Length"))
	}
}

func TestOpenAIProxyModifyResponseRecordsStreamingIntegralLog(t *testing.T) {
	usageHandler := &recordingUsageHandler{streamingCh: make(chan UsageContext, 1), streamingBodyCh: make(chan []byte, 1)}
	integralLogs := &recordingIntegralLogHandler{ch: make(chan recordedIntegralLog, 1)}
	proxy, err := NewOpenAIProxy(OpenAIProxyOptions{UsageHandler: usageHandler, IntegralLogHandler: integralLogs})
	if err != nil {
		t.Fatal(err)
	}
	body := []byte(`{"model":"gpt-5.5","input":"hello"}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(body)), nil
	}
	req = setContextInt32(req, ctxKeyID, 11)
	req = setContextInt32(req, ctxChannelID, 12)
	req = setContextInt32(req, ctxModelID, 13)
	req = setContextString(req, ctxModel, "gpt-5.5")
	streamBody := "event: response.completed\ndata: {\"response\":{\"usage\":{\"input_tokens\":2,\"output_tokens\":3}}}\n\ndata: [DONE]\n\n"
	resp := &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(streamBody)), Request: req}
	resp.Header.Set("Content-Type", "text/event-stream")

	if err := coreproxy.DefaultModifyResponse(proxy.onComplete)(resp); err != nil {
		t.Fatal(err)
	}
	gotBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotBody) != streamBody {
		t.Fatalf("stream body = %q, want %q", gotBody, streamBody)
	}
	got := receiveIntegralLog(t, integralLogs.ch)
	if got.keyID != 11 {
		t.Fatalf("keyID = %d, want 11", got.keyID)
	}
	if got.response != streamBody {
		t.Fatalf("response = %q, want %q", got.response, streamBody)
	}
	var loggedContext struct {
		Provider  string          `json:"provider"`
		Outcome   string          `json:"outcome"`
		Model     string          `json:"model"`
		ModelID   int32           `json:"model_id"`
		ChannelID int32           `json:"channel_id"`
		Request   json.RawMessage `json:"request"`
	}
	if err := json.Unmarshal([]byte(got.context), &loggedContext); err != nil {
		t.Fatal(err)
	}
	if loggedContext.Provider != "openai" || loggedContext.Outcome != "ok" || loggedContext.Model != "gpt-5.5" || loggedContext.ModelID != 13 || loggedContext.ChannelID != 12 {
		t.Fatalf("context = %+v", loggedContext)
	}
	if string(loggedContext.Request) != string(body) {
		t.Fatalf("request = %s, want %s", loggedContext.Request, body)
	}
}

func TestOpenAIProxyModifyResponseRecordsNonStreamingIntegralLog(t *testing.T) {
	usageHandler := &recordingUsageHandler{nonStreamingCh: make(chan UsageContext, 1)}
	integralLogs := &recordingIntegralLogHandler{ch: make(chan recordedIntegralLog, 1)}
	proxy, err := NewOpenAIProxy(OpenAIProxyOptions{UsageHandler: usageHandler, IntegralLogHandler: integralLogs})
	if err != nil {
		t.Fatal(err)
	}
	body := []byte(`{"model":"gpt-5.5","messages":[{"role":"user","content":"hello"}]}`)
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(body))
	req.GetBody = func() (io.ReadCloser, error) {
		return io.NopCloser(bytes.NewReader(body)), nil
	}
	req = setContextInt32(req, ctxKeyID, 21)
	req = setContextInt32(req, ctxChannelID, 22)
	req = setContextInt32(req, ctxModelID, 23)
	req = setContextString(req, ctxModel, "gpt-5.5")
	responseBody := `{"usage":{"prompt_tokens":4,"completion_tokens":5},"choices":[{"message":{"content":"safe"}}]}`
	resp := &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader(responseBody)), Request: req}
	resp.Header.Set("Content-Type", "application/json")

	if err := coreproxy.DefaultModifyResponse(proxy.onComplete)(resp); err != nil {
		t.Fatal(err)
	}
	gotResponse, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if string(gotResponse) != responseBody {
		t.Fatalf("response body = %q, want %q", gotResponse, responseBody)
	}
	info := receiveUsageContext(t, usageHandler.nonStreamingCh)
	if info.KeyID != 21 || info.ChannelID != 22 || info.ModelID != 23 || info.Model != "gpt-5.5" {
		t.Fatalf("usage context = %+v", info)
	}
	got := receiveIntegralLog(t, integralLogs.ch)
	if got.keyID != 21 {
		t.Fatalf("keyID = %d, want 21", got.keyID)
	}
	if got.response != responseBody {
		t.Fatalf("integral response = %q, want %q", got.response, responseBody)
	}
}

func TestOpenAIProxyModifyResponseNon2xxPassthrough(t *testing.T) {
	usageHandler := &recordingUsageHandler{nonStreamingCh: make(chan UsageContext, 1)}
	integralLogs := &recordingIntegralLogHandler{ch: make(chan recordedIntegralLog, 1)}
	proxy, err := NewOpenAIProxy(OpenAIProxyOptions{UsageHandler: usageHandler, TextPolicy: rejectingPolicy{}, IntegralLogHandler: integralLogs})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req = setContextInt32(req, ctxModelID, 1)
	resp := &http.Response{StatusCode: http.StatusBadRequest, Status: "400 Bad Request", Header: make(http.Header), Body: io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"blocked"}}]}`)), Request: req}
	resp.Header.Set("Content-Type", "application/json")

	if err := coreproxy.DefaultModifyResponse(proxy.onComplete)(resp); err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
	select {
	case got := <-usageHandler.nonStreamingCh:
		t.Fatalf("unexpected usage processing: %+v", got)
	default:
	}
	got := receiveIntegralLog(t, integralLogs.ch)
	var loggedContext struct {
		Outcome string `json:"outcome"`
	}
	if err := json.Unmarshal([]byte(got.context), &loggedContext); err != nil {
		t.Fatal(err)
	}
	if loggedContext.Outcome != "error" {
		t.Fatalf("outcome = %q, want error", loggedContext.Outcome)
	}
}

func TestOpenAIProxyModifyResponseRejectsOutputAndRestoresBody(t *testing.T) {
	usageHandler := &recordingUsageHandler{nonStreamingCh: make(chan UsageContext, 1)}
	integralLogs := &recordingIntegralLogHandler{ch: make(chan recordedIntegralLog, 1)}
	proxy, err := NewOpenAIProxy(OpenAIProxyOptions{UsageHandler: usageHandler, TextPolicy: rejectingPolicy{}, IntegralLogHandler: integralLogs})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req = setContextInt32(req, ctxKeyID, 7)
	req = setContextInt32(req, ctxChannelID, 8)
	req = setContextInt32(req, ctxModelID, 9)
	req = setContextString(req, ctxModel, "gpt-5.5")
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(`{"usage":{"prompt_tokens":4,"completion_tokens":5},"choices":[{"message":{"content":"blocked output"}}]}`)),
		Request:    req,
	}
	resp.Header.Set("Content-Type", "application/json")
	resp.Header.Set("Content-Encoding", "identity")

	if err := coreproxy.DefaultModifyResponse(proxy.onComplete)(resp); err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnprocessableEntity)
	}
	if resp.Header.Get("Content-Encoding") != "" {
		t.Fatalf("Content-Encoding = %q, want cleared", resp.Header.Get("Content-Encoding"))
	}
	rejectedBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(rejectedBody), "model output rejected") {
		t.Fatalf("body = %q, want output rejection", rejectedBody)
	}
	info := receiveUsageContext(t, usageHandler.nonStreamingCh)
	if info.KeyID != 7 || info.ChannelID != 8 || info.ModelID != 9 || info.Model != "gpt-5.5" {
		t.Fatalf("usage context = %+v", info)
	}
	got := receiveIntegralLog(t, integralLogs.ch)
	if !strings.Contains(got.response, "model output rejected") {
		t.Fatalf("integral response = %q, want output rejection", got.response)
	}
	var loggedContext struct {
		Outcome         string `json:"outcome"`
		RejectionStage  string `json:"rejection_stage"`
		RejectionReason string `json:"rejection_reason"`
		ResponseStatus  int    `json:"response_status"`
	}
	if err := json.Unmarshal([]byte(got.context), &loggedContext); err != nil {
		t.Fatal(err)
	}
	if loggedContext.Outcome != "rejected" || loggedContext.RejectionStage != "output" || loggedContext.RejectionReason != "sensitive" || loggedContext.ResponseStatus != http.StatusUnprocessableEntity {
		t.Fatalf("context = %+v", loggedContext)
	}
}

func TestOpenAIProxyStreamsSafeOutputWithRetainedTail(t *testing.T) {
	previousTail := openAIStreamSafetyTailRunes
	openAIStreamSafetyTailRunes = 1
	t.Cleanup(func() { openAIStreamSafetyTailRunes = previousTail })

	proxy, err := NewOpenAIProxy(OpenAIProxyOptions{
		ModelStore: fakeModelStore{model: &ModelInfo{ID: 1, Status: ModelStatusActive}},
		TextPolicy: newSensitiveTestPolicy(t, []string{"gpt-5.5"}),
	})
	if err != nil {
		t.Fatal(err)
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		_, _ = w.Write(openAIChatContentEvent("ab"))
		flusher.Flush()
		_, _ = w.Write(openAIChatContentEvent("cd"))
		flusher.Flush()
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
		flusher.Flush()
	}))
	defer upstream.Close()

	body := `{"model":"gpt-5.5","stream":true,"messages":[{"role":"user","content":"hello"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req, 1, 1, upstream.URL, "provider-key")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", rec.Code, rec.Body.String())
	}
	got := rec.Body.String()
	for _, want := range []string{`"content":"a"`, `"content":"bc"`, `"content":"d"`, "data: [DONE]"} {
		if !strings.Contains(got, want) {
			t.Fatalf("stream body = %q, want %s", got, want)
		}
	}
}

func TestOpenAIProxyRejectsSensitiveOutputSplitAcrossStreamChunks(t *testing.T) {
	previousTail := openAIStreamSafetyTailRunes
	openAIStreamSafetyTailRunes = 6
	t.Cleanup(func() { openAIStreamSafetyTailRunes = previousTail })

	usageHandler := &recordingUsageHandler{streamingCh: make(chan UsageContext, 1), streamingBodyCh: make(chan []byte, 1)}
	integralLogs := &recordingIntegralLogHandler{ch: make(chan recordedIntegralLog, 1)}
	proxy, err := NewOpenAIProxy(OpenAIProxyOptions{
		ModelStore:         fakeModelStore{model: &ModelInfo{ID: 33, Status: ModelStatusActive}},
		UsageHandler:       usageHandler,
		IntegralLogHandler: integralLogs,
		TextPolicy:         newSensitiveTestPolicy(t, []string{"gpt-5.5"}),
	})
	if err != nil {
		t.Fatal(err)
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		_, _ = w.Write(openAIChatContentEvent("hello blo"))
		flusher.Flush()
		_, _ = w.Write(openAIChatContentEvent("cked"))
		flusher.Flush()
		_, _ = w.Write(openAIChatContentEvent(" should not continue"))
		flusher.Flush()
	}))
	defer upstream.Close()

	body := `{"model":"gpt-5.5","stream":true,"messages":[{"role":"user","content":"hello"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req, 7, 8, upstream.URL, "provider-key")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", rec.Code, rec.Body.String())
	}
	gotBody := rec.Body.String()
	if !strings.Contains(gotBody, `"content":"hel"`) {
		t.Fatalf("body = %q, want safe prefix", gotBody)
	}
	if strings.Contains(gotBody, "blocked") || strings.Contains(gotBody, "should not continue") {
		t.Fatalf("body leaked unsafe upstream content: %q", gotBody)
	}
	if !strings.Contains(gotBody, `"code":"sensitive_output"`) || !strings.Contains(gotBody, "data: [DONE]") {
		t.Fatalf("body = %q, want rejection SSE", gotBody)
	}

	gotLog := receiveIntegralLog(t, integralLogs.ch)
	if gotLog.keyID != 7 {
		t.Fatalf("keyID = %d, want 7", gotLog.keyID)
	}
	if strings.Contains(gotLog.response, "blocked") {
		t.Fatalf("integral response leaked blocked text: %q", gotLog.response)
	}
	var loggedContext struct {
		Outcome         string `json:"outcome"`
		RejectionStage  string `json:"rejection_stage"`
		RejectionReason string `json:"rejection_reason"`
		ResponseStatus  int    `json:"response_status"`
		ModelID         int32  `json:"model_id"`
		ChannelID       int32  `json:"channel_id"`
	}
	if err := json.Unmarshal([]byte(gotLog.context), &loggedContext); err != nil {
		t.Fatal(err)
	}
	if loggedContext.Outcome != "rejected" || loggedContext.RejectionStage != "output" || loggedContext.RejectionReason != "sensitive" || loggedContext.ResponseStatus != http.StatusOK || loggedContext.ModelID != 33 || loggedContext.ChannelID != 8 {
		t.Fatalf("context = %+v", loggedContext)
	}
	usageInfo := receiveUsageContext(t, usageHandler.streamingCh)
	if usageInfo.KeyID != 7 || usageInfo.ChannelID != 8 || usageInfo.ModelID != 33 || usageInfo.Model != "gpt-5.5" {
		t.Fatalf("usage context = %+v", usageInfo)
	}
	select {
	case rawUsageBody := <-usageHandler.streamingBodyCh:
		if !strings.Contains(string(rawUsageBody), "cked") || strings.Contains(string(rawUsageBody), "sensitive_output") {
			t.Fatalf("usage body = %q, want upstream rejected stream fragment", rawUsageBody)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for streaming usage body")
	}
}

func TestStreamRejectLogIncludesSameChunkSafeOutput(t *testing.T) {
	previousTail := openAIStreamSafetyTailRunes
	openAIStreamSafetyTailRunes = 1
	t.Cleanup(func() { openAIStreamSafetyTailRunes = previousTail })

	integralLogs := &recordingIntegralLogHandler{ch: make(chan recordedIntegralLog, 1)}
	proxy, err := NewOpenAIProxy(OpenAIProxyOptions{
		ModelStore:         fakeModelStore{model: &ModelInfo{ID: 33, Status: ModelStatusActive}},
		IntegralLogHandler: integralLogs,
		TextPolicy:         newSensitiveTestPolicy(t, []string{"gpt-5.5"}),
	})
	if err != nil {
		t.Fatal(err)
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		chunk := append(openAIChatContentEvent("ab"), openAIChatContentEvent("blocked")...)
		_, _ = w.Write(chunk)
	}))
	defer upstream.Close()

	body := `{"model":"gpt-5.5","stream":true,"messages":[{"role":"user","content":"hello"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req, 7, 8, upstream.URL, "provider-key")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", rec.Code, rec.Body.String())
	}
	gotBody := rec.Body.String()
	if !strings.Contains(gotBody, `"content":"a"`) || !strings.Contains(gotBody, `"code":"sensitive_output"`) {
		t.Fatalf("body = %q, want safe prefix and rejection", gotBody)
	}
	gotLog := receiveIntegralLog(t, integralLogs.ch)
	if !strings.Contains(gotLog.response, `"content":"a"`) || !strings.Contains(gotLog.response, `"code":"sensitive_output"`) {
		t.Fatalf("integral response = %q, want same-chunk safe prefix and rejection", gotLog.response)
	}
	if strings.Contains(gotLog.response, "blocked") {
		t.Fatalf("integral response leaked rejected text: %q", gotLog.response)
	}
}

func TestChatStreamRejectsSecondChoice(t *testing.T) {
	previousTail := openAIStreamSafetyTailRunes
	openAIStreamSafetyTailRunes = 6
	t.Cleanup(func() { openAIStreamSafetyTailRunes = previousTail })

	proxy, err := NewOpenAIProxy(OpenAIProxyOptions{
		ModelStore: fakeModelStore{model: &ModelInfo{ID: 33, Status: ModelStatusActive}},
		TextPolicy: newSensitiveTestPolicy(t, []string{"gpt-5.5"}),
	})
	if err != nil {
		t.Fatal(err)
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		_, _ = w.Write(chatChoicesEvent(map[int]string{0: "safe one", 1: "hello blo"}))
		flusher.Flush()
		_, _ = w.Write(chatChoicesEvent(map[int]string{0: "still safe", 1: "cked"}))
		flusher.Flush()
	}))
	defer upstream.Close()

	body := `{"model":"gpt-5.5","stream":true,"messages":[{"role":"user","content":"hello"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(body))
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req, 7, 8, upstream.URL, "provider-key")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", rec.Code, rec.Body.String())
	}
	gotBody := rec.Body.String()
	if strings.Contains(gotBody, "blocked") || strings.Contains(gotBody, "hello blo") || strings.Contains(gotBody, "cked") {
		t.Fatalf("body leaked unsafe second choice: %q", gotBody)
	}
	if !strings.Contains(gotBody, `"code":"sensitive_output"`) {
		t.Fatalf("body = %q, want rejection SSE", gotBody)
	}
}

func TestOpenAIProxyStreamsSafeResponsesOutputWithRetainedTail(t *testing.T) {
	previousTail := openAIStreamSafetyTailRunes
	openAIStreamSafetyTailRunes = 1
	t.Cleanup(func() { openAIStreamSafetyTailRunes = previousTail })

	proxy, err := NewOpenAIProxy(OpenAIProxyOptions{
		ModelStore: fakeModelStore{model: &ModelInfo{ID: 1, Status: ModelStatusActive}},
		TextPolicy: newSensitiveTestPolicy(t, []string{"gpt-5.5"}),
	})
	if err != nil {
		t.Fatal(err)
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		_, _ = w.Write(openAIResponsesDeltaEvent("ab"))
		flusher.Flush()
		_, _ = w.Write(openAIResponsesDeltaEvent("cd"))
		flusher.Flush()
		_, _ = w.Write(openAIResponsesDoneEvent("abcd"))
		flusher.Flush()
	}))
	defer upstream.Close()

	body := `{"model":"gpt-5.5","input":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req, 1, 1, upstream.URL, "provider-key")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", rec.Code, rec.Body.String())
	}
	got := rec.Body.String()
	for _, want := range []string{`"delta":"a"`, `"delta":"bc"`, `"delta":"d"`, "event: response.output_text.done"} {
		if !strings.Contains(got, want) {
			t.Fatalf("stream body = %q, want %s", got, want)
		}
	}
}

func TestResponsesStreamKeepsLanes(t *testing.T) {
	previousTail := openAIStreamSafetyTailRunes
	openAIStreamSafetyTailRunes = 1
	t.Cleanup(func() { openAIStreamSafetyTailRunes = previousTail })

	proxy, err := NewOpenAIProxy(OpenAIProxyOptions{
		ModelStore: fakeModelStore{model: &ModelInfo{ID: 1, Status: ModelStatusActive}},
		TextPolicy: newSensitiveTestPolicy(t, []string{"gpt-5.5"}),
	})
	if err != nil {
		t.Fatal(err)
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		_, _ = w.Write(respDeltaEvent("msg_1", 0, 0, "ab"))
		flusher.Flush()
		_, _ = w.Write(respDeltaEvent("msg_2", 1, 0, "xy"))
		flusher.Flush()
		_, _ = w.Write(respDeltaEvent("msg_1", 0, 0, "cd"))
		flusher.Flush()
		_, _ = w.Write(respDeltaEvent("msg_2", 1, 0, "z"))
		flusher.Flush()
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
		flusher.Flush()
	}))
	defer upstream.Close()

	body := `{"model":"gpt-5.5","input":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req, 1, 1, upstream.URL, "provider-key")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", rec.Code, rec.Body.String())
	}
	got := rec.Body.String()
	for _, want := range []string{`"item_id":"msg_1"`, `"output_index":0`, `"delta":"a"`, `"delta":"bc"`, `"delta":"d"`, `"item_id":"msg_2"`, `"output_index":1`, `"delta":"x"`, `"delta":"y"`, `"delta":"z"`} {
		if !strings.Contains(got, want) {
			t.Fatalf("stream body = %q, want %s", got, want)
		}
	}
	if strings.Contains(got, `"delta":"bxy"`) || strings.Contains(got, `"delta":"ycd"`) {
		t.Fatalf("stream body mixed response lanes: %q", got)
	}
}

func TestResponsesStreamDoneFlushesLane(t *testing.T) {
	previousTail := openAIStreamSafetyTailRunes
	openAIStreamSafetyTailRunes = 1
	t.Cleanup(func() { openAIStreamSafetyTailRunes = previousTail })

	proxy, err := NewOpenAIProxy(OpenAIProxyOptions{
		ModelStore: fakeModelStore{model: &ModelInfo{ID: 1, Status: ModelStatusActive}},
		TextPolicy: newSensitiveTestPolicy(t, []string{"gpt-5.5"}),
	})
	if err != nil {
		t.Fatal(err)
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		_, _ = w.Write(respDeltaEvent("msg_1", 0, 0, "ab"))
		flusher.Flush()
		_, _ = w.Write(respDeltaEvent("msg_2", 1, 0, "xy"))
		flusher.Flush()
		_, _ = w.Write([]byte(`event: response.output_text.done
data: {"type":"response.output_text.done","item_id":"msg_1","output_index":0,"content_index":0}

`))
		flusher.Flush()
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
		flusher.Flush()
	}))
	defer upstream.Close()

	body := `{"model":"gpt-5.5","input":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req, 1, 1, upstream.URL, "provider-key")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", rec.Code, rec.Body.String())
	}
	got := rec.Body.String()
	msg1Done := strings.Index(got, "response.output_text.done")
	msg2Tail := strings.Index(got, `"delta":"y"`)
	if msg1Done < 0 || msg2Tail < 0 || msg2Tail < msg1Done {
		t.Fatalf("stream body = %q, want msg_2 tail after msg_1 done", got)
	}
	if !strings.Contains(got, `"item_id":"msg_1"`) || !strings.Contains(got, `"delta":"b"`) || !strings.Contains(got, `"delta":"y"`) {
		t.Fatalf("stream body = %q, want both lane tails", got)
	}
}

func TestResponsesItemDoneKeepsOtherItemTail(t *testing.T) {
	previousTail := openAIStreamSafetyTailRunes
	openAIStreamSafetyTailRunes = 1
	t.Cleanup(func() { openAIStreamSafetyTailRunes = previousTail })

	proxy, err := NewOpenAIProxy(OpenAIProxyOptions{
		ModelStore: fakeModelStore{model: &ModelInfo{ID: 1, Status: ModelStatusActive}},
		TextPolicy: newSensitiveTestPolicy(t, []string{"gpt-5.5"}),
	})
	if err != nil {
		t.Fatal(err)
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		_, _ = w.Write(respDeltaEvent("msg_1", 0, 0, "ab"))
		flusher.Flush()
		_, _ = w.Write(respDeltaEvent("msg_2", 1, 0, "xy"))
		flusher.Flush()
		_, _ = w.Write(itemDoneEvent("msg_1", 0, "done one"))
		flusher.Flush()
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
		flusher.Flush()
	}))
	defer upstream.Close()

	body := `{"model":"gpt-5.5","input":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req, 1, 1, upstream.URL, "provider-key")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", rec.Code, rec.Body.String())
	}
	got := rec.Body.String()
	itemDone := strings.Index(got, "response.output_item.done")
	msg2Tail := strings.Index(got, `"delta":"y"`)
	if itemDone < 0 || msg2Tail < 0 || msg2Tail < itemDone {
		t.Fatalf("stream body = %q, want msg_2 tail after msg_1 item done", got)
	}
	if !strings.Contains(got, `"delta":"b"`) {
		t.Fatalf("stream body = %q, want msg_1 tail flushed by item done", got)
	}
}

func TestResponsesItemDoneDoesNotRejectOtherItemTail(t *testing.T) {
	previousTail := openAIStreamSafetyTailRunes
	openAIStreamSafetyTailRunes = 3
	t.Cleanup(func() { openAIStreamSafetyTailRunes = previousTail })

	proxy, err := NewOpenAIProxy(OpenAIProxyOptions{
		ModelStore: fakeModelStore{model: &ModelInfo{ID: 1, Status: ModelStatusActive}},
		TextPolicy: newSensitiveTestPolicy(t, []string{"gpt-5.5"}),
	})
	if err != nil {
		t.Fatal(err)
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		_, _ = w.Write(respDeltaEvent("msg_2", 1, 0, "blo"))
		flusher.Flush()
		_, _ = w.Write(itemDoneEvent("msg_1", 0, "cked"))
		flusher.Flush()
		_, _ = w.Write([]byte("data: [DONE]\n\n"))
		flusher.Flush()
	}))
	defer upstream.Close()

	body := `{"model":"gpt-5.5","input":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req, 1, 1, upstream.URL, "provider-key")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", rec.Code, rec.Body.String())
	}
	got := rec.Body.String()
	if strings.Contains(got, `"code":"sensitive_output"`) {
		t.Fatalf("stream body = %q, want no rejection from unrelated item tail", got)
	}
	itemDone := strings.Index(got, "response.output_item.done")
	msg2Tail := strings.Index(got, `"delta":"blo"`)
	if itemDone < 0 || msg2Tail < 0 || msg2Tail < itemDone {
		t.Fatalf("stream body = %q, want msg_2 tail after msg_1 item done", got)
	}
}

func TestResponsesSnapshotRejectsBeforeTailFlush(t *testing.T) {
	previousTail := openAIStreamSafetyTailRunes
	openAIStreamSafetyTailRunes = 3
	t.Cleanup(func() { openAIStreamSafetyTailRunes = previousTail })

	proxy, err := NewOpenAIProxy(OpenAIProxyOptions{
		ModelStore: fakeModelStore{model: &ModelInfo{ID: 1, Status: ModelStatusActive}},
		TextPolicy: newSensitiveTestPolicy(t, []string{"gpt-5.5"}),
	})
	if err != nil {
		t.Fatal(err)
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		_, _ = w.Write(respDeltaEvent("msg_1", 0, 0, "blo"))
		flusher.Flush()
		_, _ = w.Write(openAIResponsesDoneEvent("blocked"))
		flusher.Flush()
	}))
	defer upstream.Close()

	body := `{"model":"gpt-5.5","input":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req, 1, 1, upstream.URL, "provider-key")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", rec.Code, rec.Body.String())
	}
	got := rec.Body.String()
	if strings.Contains(got, `"delta":"blo"`) || strings.Contains(got, "blocked") || !strings.Contains(got, `"code":"sensitive_output"`) {
		t.Fatalf("body = %q, want rejection without retained tail or snapshot text", got)
	}
}

func TestResponsesSnapshotChecksFullText(t *testing.T) {
	previousTail := openAIStreamSafetyTailRunes
	openAIStreamSafetyTailRunes = 3
	t.Cleanup(func() { openAIStreamSafetyTailRunes = previousTail })

	detector, err := sensitive.NewDetector(sensitive.Dictionary{
		Name:         "scoped",
		Words:        []string{"bloblocked"},
		EffectModels: []string{"gpt-5.5"},
	})
	if err != nil {
		t.Fatal(err)
	}
	proxy, err := NewOpenAIProxy(OpenAIProxyOptions{
		ModelStore: fakeModelStore{model: &ModelInfo{ID: 1, Status: ModelStatusActive}},
		TextPolicy: detectorPolicy{detector: detector},
	})
	if err != nil {
		t.Fatal(err)
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		_, _ = w.Write(respDeltaEvent("msg_1", 0, 0, "blo"))
		flusher.Flush()
		_, _ = w.Write(openAIResponsesDoneEvent("blocked"))
		flusher.Flush()
	}))
	defer upstream.Close()

	body := `{"model":"gpt-5.5","input":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req, 1, 1, upstream.URL, "provider-key")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", rec.Code, rec.Body.String())
	}
	got := rec.Body.String()
	if strings.Contains(got, `"code":"sensitive_output"`) {
		t.Fatalf("body = %q, want no rejection from retained tail plus full snapshot", got)
	}
	if !strings.Contains(got, `"delta":"blo"`) || !strings.Contains(got, "blocked") {
		t.Fatalf("body = %q, want retained tail and snapshot", got)
	}
}

func TestOpenAIProxyRejectsSensitiveResponsesOutputSplitAcrossDeltas(t *testing.T) {
	previousTail := openAIStreamSafetyTailRunes
	openAIStreamSafetyTailRunes = 6
	t.Cleanup(func() { openAIStreamSafetyTailRunes = previousTail })

	usageHandler := &recordingUsageHandler{streamingCh: make(chan UsageContext, 1), streamingBodyCh: make(chan []byte, 1)}
	integralLogs := &recordingIntegralLogHandler{ch: make(chan recordedIntegralLog, 1)}
	proxy, err := NewOpenAIProxy(OpenAIProxyOptions{
		ModelStore:         fakeModelStore{model: &ModelInfo{ID: 44, Status: ModelStatusActive}},
		UsageHandler:       usageHandler,
		IntegralLogHandler: integralLogs,
		TextPolicy:         newSensitiveTestPolicy(t, []string{"gpt-5.5"}),
	})
	if err != nil {
		t.Fatal(err)
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		_, _ = w.Write(openAIResponsesDeltaEvent("hello blo"))
		flusher.Flush()
		_, _ = w.Write(openAIResponsesDeltaEvent("cked"))
		flusher.Flush()
	}))
	defer upstream.Close()

	body := `{"model":"gpt-5.5","input":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req, 9, 10, upstream.URL, "provider-key")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", rec.Code, rec.Body.String())
	}
	gotBody := rec.Body.String()
	if !strings.Contains(gotBody, `"delta":"hel"`) {
		t.Fatalf("body = %q, want safe prefix", gotBody)
	}
	if strings.Contains(gotBody, "blocked") || !strings.Contains(gotBody, `"code":"sensitive_output"`) {
		t.Fatalf("body = %q, want rejection without sensitive text", gotBody)
	}

	gotLog := receiveIntegralLog(t, integralLogs.ch)
	var loggedContext struct {
		Outcome         string `json:"outcome"`
		RejectionStage  string `json:"rejection_stage"`
		RejectionReason string `json:"rejection_reason"`
		ModelID         int32  `json:"model_id"`
		ChannelID       int32  `json:"channel_id"`
	}
	if err := json.Unmarshal([]byte(gotLog.context), &loggedContext); err != nil {
		t.Fatal(err)
	}
	if loggedContext.Outcome != "rejected" || loggedContext.RejectionStage != "output" || loggedContext.RejectionReason != "sensitive" || loggedContext.ModelID != 44 || loggedContext.ChannelID != 10 {
		t.Fatalf("context = %+v", loggedContext)
	}
	usageInfo := receiveUsageContext(t, usageHandler.streamingCh)
	if usageInfo.KeyID != 9 || usageInfo.ChannelID != 10 || usageInfo.ModelID != 44 || usageInfo.Model != "gpt-5.5" {
		t.Fatalf("usage context = %+v", usageInfo)
	}
	select {
	case rawUsageBody := <-usageHandler.streamingBodyCh:
		if !strings.Contains(string(rawUsageBody), "cked") || strings.Contains(string(rawUsageBody), "sensitive_output") {
			t.Fatalf("usage body = %q, want upstream responses stream fragment", rawUsageBody)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for streaming usage body")
	}
}

func TestResponsesStreamRejectsLane(t *testing.T) {
	previousTail := openAIStreamSafetyTailRunes
	openAIStreamSafetyTailRunes = 6
	t.Cleanup(func() { openAIStreamSafetyTailRunes = previousTail })

	proxy, err := NewOpenAIProxy(OpenAIProxyOptions{
		ModelStore: fakeModelStore{model: &ModelInfo{ID: 44, Status: ModelStatusActive}},
		TextPolicy: newSensitiveTestPolicy(t, []string{"gpt-5.5"}),
	})
	if err != nil {
		t.Fatal(err)
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		_, _ = w.Write(respDeltaEvent("msg_1", 0, 0, "hello blo"))
		flusher.Flush()
		_, _ = w.Write(respDeltaEvent("msg_2", 1, 0, "safe text"))
		flusher.Flush()
		_, _ = w.Write(respDeltaEvent("msg_1", 0, 0, "cked"))
		flusher.Flush()
	}))
	defer upstream.Close()

	body := `{"model":"gpt-5.5","input":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req, 9, 10, upstream.URL, "provider-key")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", rec.Code, rec.Body.String())
	}
	gotBody := rec.Body.String()
	if strings.Contains(gotBody, "blocked") || strings.Contains(gotBody, "cked") || !strings.Contains(gotBody, `"code":"sensitive_output"`) {
		t.Fatalf("body = %q, want lane-specific rejection without sensitive text", gotBody)
	}
}

func TestOpenAIProxyRejectsSensitiveResponsesSnapshotOutput(t *testing.T) {
	usageHandler := &recordingUsageHandler{streamingCh: make(chan UsageContext, 1), streamingBodyCh: make(chan []byte, 1)}
	integralLogs := &recordingIntegralLogHandler{ch: make(chan recordedIntegralLog, 1)}
	proxy, err := NewOpenAIProxy(OpenAIProxyOptions{
		ModelStore:         fakeModelStore{model: &ModelInfo{ID: 55, Status: ModelStatusActive}},
		UsageHandler:       usageHandler,
		IntegralLogHandler: integralLogs,
		TextPolicy:         newSensitiveTestPolicy(t, []string{"gpt-5.5"}),
	})
	if err != nil {
		t.Fatal(err)
	}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write(openAIResponsesDoneEvent("blocked snapshot"))
	}))
	defer upstream.Close()

	body := `{"model":"gpt-5.5","input":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
	rec := httptest.NewRecorder()
	proxy.ServeHTTP(rec, req, 11, 12, upstream.URL, "provider-key")

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", rec.Code, rec.Body.String())
	}
	gotBody := rec.Body.String()
	if strings.Contains(gotBody, "blocked snapshot") || !strings.Contains(gotBody, `"code":"sensitive_output"`) {
		t.Fatalf("body = %q, want snapshot rejection without unsafe text", gotBody)
	}
	_ = receiveIntegralLog(t, integralLogs.ch)
	usageInfo := receiveUsageContext(t, usageHandler.streamingCh)
	if usageInfo.KeyID != 11 || usageInfo.ChannelID != 12 || usageInfo.ModelID != 55 || usageInfo.Model != "gpt-5.5" {
		t.Fatalf("usage context = %+v", usageInfo)
	}
}

func TestOpenAIProxyRejectsGzipSensitiveResponsesStream(t *testing.T) {
	usageHandler := &recordingUsageHandler{streamingCh: make(chan UsageContext, 1), streamingBodyCh: make(chan []byte, 1)}
	integralLogs := &recordingIntegralLogHandler{ch: make(chan recordedIntegralLog, 1)}
	proxy, err := NewOpenAIProxy(OpenAIProxyOptions{
		ModelStore:         fakeModelStore{model: &ModelInfo{ID: 66, Status: ModelStatusActive}},
		UsageHandler:       usageHandler,
		IntegralLogHandler: integralLogs,
		TextPolicy:         newSensitiveTestPolicy(t, []string{"gpt-5.5"}),
	})
	if err != nil {
		t.Fatal(err)
	}
	body := `{"model":"gpt-5.5","input":"hello"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/responses", strings.NewReader(body))
	req = setContextInt32(req, ctxKeyID, 13)
	req = setContextInt32(req, ctxChannelID, 14)
	req = setContextInt32(req, ctxModelID, 66)
	req = setContextString(req, ctxModel, "gpt-5.5")
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewReader(gzipBytes(t, openAIResponsesDeltaEvent("blocked")))),
		Request:    req,
	}
	resp.Header.Set("Content-Type", "text/event-stream")
	resp.Header.Set("Content-Encoding", "gzip")

	if err := coreproxy.DefaultModifyResponse(proxy.onComplete, proxy.openAIStreamTransform)(resp); err != nil {
		t.Fatal(err)
	}
	if got := resp.Header.Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("Content-Encoding = %q, want gzip", got)
	}
	compressedBody, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	decodedBody := string(gunzipBytes(t, compressedBody))
	if strings.Contains(decodedBody, "blocked") || !strings.Contains(decodedBody, `"code":"sensitive_output"`) {
		t.Fatalf("decoded body = %q, want gzip rejection without unsafe text", decodedBody)
	}
	_ = receiveIntegralLog(t, integralLogs.ch)
	usageInfo := receiveUsageContext(t, usageHandler.streamingCh)
	if usageInfo.KeyID != 13 || usageInfo.ChannelID != 14 || usageInfo.ModelID != 66 || usageInfo.Model != "gpt-5.5" {
		t.Fatalf("usage context = %+v", usageInfo)
	}
	select {
	case rawUsageBody := <-usageHandler.streamingBodyCh:
		if !strings.Contains(string(rawUsageBody), "blocked") || strings.Contains(string(rawUsageBody), "sensitive_output") {
			t.Fatalf("usage body = %q, want decoded upstream gzip stream fragment", rawUsageBody)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for streaming usage body")
	}
}

func openAIChatContentEvent(content string) []byte {
	return []byte(`data: {"choices":[{"index":0,"delta":{"content":` + strconv.Quote(content) + `}}]}` + "\n\n")
}

func chatChoicesEvent(choices map[int]string) []byte {
	parts := make([]string, 0, len(choices))
	indexes := make([]int, 0, len(choices))
	for index := range choices {
		indexes = append(indexes, index)
	}
	sort.Ints(indexes)
	for _, index := range indexes {
		parts = append(parts, `{"index":`+strconv.Itoa(index)+`,"delta":{"content":`+strconv.Quote(choices[index])+`}}`)
	}
	return []byte(`data: {"choices":[` + strings.Join(parts, ",") + `]}` + "\n\n")
}

func openAIResponsesDeltaEvent(delta string) []byte {
	return respDeltaEvent("msg_1", 0, 0, delta)
}

func respDeltaEvent(itemID string, outputIndex, contentIndex int, delta string) []byte {
	return []byte(`event: response.output_text.delta
data: {"type":"response.output_text.delta","item_id":` + strconv.Quote(itemID) + `,"output_index":` + strconv.Itoa(outputIndex) + `,"content_index":` + strconv.Itoa(contentIndex) + `,"delta":` + strconv.Quote(delta) + `}

`)
}

func itemDoneEvent(itemID string, outputIndex int, text string) []byte {
	return []byte(`event: response.output_item.done
data: {"type":"response.output_item.done","output_index":` + strconv.Itoa(outputIndex) + `,"item":{"id":` + strconv.Quote(itemID) + `,"content":[{"type":"output_text","text":` + strconv.Quote(text) + `}]}}

`)
}

func openAIResponsesDoneEvent(text string) []byte {
	return []byte(`event: response.output_text.done
data: {"type":"response.output_text.done","item_id":"msg_1","output_index":0,"content_index":0,"text":` + strconv.Quote(text) + `}

`)
}

func gzipBytes(t *testing.T, data []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	writer := gzip.NewWriter(&buf)
	if _, err := writer.Write(data); err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func gunzipBytes(t *testing.T, data []byte) []byte {
	t.Helper()
	reader, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	decoded, readErr := io.ReadAll(reader)
	closeErr := reader.Close()
	if readErr != nil {
		t.Fatal(readErr)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
	return decoded
}

type fakeModelStore struct {
	model *ModelInfo
	err   error
}

func (s fakeModelStore) ResolveModel(ctx context.Context, channelID int32, modelName string) (*ModelInfo, error) {
	return s.model, s.err
}

type rejectingPolicy struct{}

func (rejectingPolicy) Detect(ctx context.Context, model, text string) sensitive.Result {
	if strings.TrimSpace(text) == "" {
		return sensitive.Result{}
	}
	return sensitive.Result{Matches: []sensitive.Match{{Dictionary: "test", Words: []string{"blocked"}}}}
}

type recordingUsageHandler struct {
	streamingCh     chan UsageContext
	nonStreamingCh  chan UsageContext
	streamingBodyCh chan []byte
}

func (h *recordingUsageHandler) ProcessNonStreamingResponse(ctx context.Context, req *http.Request, rawBody []byte, contentEncoding string, contentType string, info UsageContext) error {
	if h.nonStreamingCh == nil {
		h.nonStreamingCh = make(chan UsageContext, 1)
	}
	h.nonStreamingCh <- info
	return nil
}

func (h *recordingUsageHandler) ProcessStreamingResponse(ctx context.Context, req *http.Request, decodedBody []byte, info UsageContext) error {
	if h.streamingCh == nil {
		h.streamingCh = make(chan UsageContext, 1)
	}
	h.streamingCh <- info
	if h.streamingBodyCh != nil {
		h.streamingBodyCh <- append([]byte(nil), decodedBody...)
	}
	return nil
}

func receiveUsageContext(t *testing.T, ch <-chan UsageContext) UsageContext {
	t.Helper()
	select {
	case got := <-ch:
		return got
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for usage context")
		return UsageContext{}
	}
}

type recordedIntegralLog struct {
	keyID    int32
	context  string
	response string
}

type recordingIntegralLogHandler struct {
	ch chan recordedIntegralLog
}

func (h *recordingIntegralLogHandler) InsertIntegralLog(ctx context.Context, keyID int32, context string, response string) error {
	h.ch <- recordedIntegralLog{keyID: keyID, context: context, response: response}
	return nil
}

func receiveIntegralLog(t *testing.T, ch <-chan recordedIntegralLog) recordedIntegralLog {
	t.Helper()
	select {
	case got := <-ch:
		return got
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for integral log")
		return recordedIntegralLog{}
	}
}
