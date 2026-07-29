package main

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/HyperToken-dev/fabric/business/sensitive"
	coreproxy "github.com/HyperToken-dev/fabric/core/proxy"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

type activeModelStore struct{}

func (activeModelStore) ResolveModel(ctx context.Context, channelID int32, modelName string) (*ModelInfo, error) {
	return &ModelInfo{ID: 1, Status: ModelStatusActive}, nil
}

func TestOpenAIProxyRequestDetectionUsesExactModel(t *testing.T) {
	policy := newSensitiveTestPolicy(t, []string{"gpt-5.5"})
	proxy, err := NewOpenAIProxy(OpenAIProxyOptions{ModelStore: activeModelStore{}, TextPolicy: policy})
	if err != nil {
		t.Fatal(err)
	}

	var upstreamCalls atomic.Int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		upstreamCalls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"safe"}}]}`))
	}))
	t.Cleanup(upstream.Close)

	rejectedReq := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-5.5","messages":[{"role":"user","content":"blocked prompt"}]}`))
	rejected := httptest.NewRecorder()
	proxy.ServeHTTP(rejected, rejectedReq, 1, 1, upstream.URL, "provider-key")
	if rejected.Code != http.StatusForbidden {
		t.Fatalf("exact model status = %d, want %d", rejected.Code, http.StatusForbidden)
	}
	if upstreamCalls.Load() != 0 {
		t.Fatalf("upstream calls after rejection = %d", upstreamCalls.Load())
	}

	allowedReq := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", strings.NewReader(`{"model":"gpt-5.5-mini","messages":[{"role":"user","content":"blocked prompt"}]}`))
	allowed := httptest.NewRecorder()
	proxy.ServeHTTP(allowed, allowedReq, 1, 1, upstream.URL, "provider-key")
	if allowed.Code != http.StatusOK {
		t.Fatalf("different model status = %d, want %d; body=%s", allowed.Code, http.StatusOK, allowed.Body.String())
	}
	if upstreamCalls.Load() != 1 {
		t.Fatalf("upstream calls after allowed request = %d, want 1", upstreamCalls.Load())
	}
}

func TestOpenAIProxyNonStreamingOutputUsesOriginalModel(t *testing.T) {
	policy := newSensitiveTestPolicy(t, []string{"gpt-5.5"})
	proxy, err := NewOpenAIProxy(OpenAIProxyOptions{ModelStore: activeModelStore{}, TextPolicy: policy})
	if err != nil {
		t.Fatal(err)
	}
	core, logs := observer.New(zap.InfoLevel)
	previous := zap.L()
	zap.ReplaceGlobals(zap.New(core))
	t.Cleanup(func() { zap.ReplaceGlobals(previous) })

	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", nil)
	req = setContextString(req, ctxModel, "gpt-5.5")
	req = setContextInt32(req, ctxModelID, 1)
	resp := &http.Response{
		StatusCode: http.StatusOK,
		Status:     "200 OK",
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(`{"choices":[{"message":{"content":"complete blocked output"}}]}`)),
		Request:    req,
	}
	resp.Header.Set("Content-Type", "application/json")

	if err := coreproxy.DefaultModifyResponse(proxy.onComplete)(resp); err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("status = %d, want %d", resp.StatusCode, http.StatusUnprocessableEntity)
	}
	entries := logs.FilterMessage("sensitive text rejected").All()
	if len(entries) != 1 {
		t.Fatalf("log entries = %d, want 1", len(entries))
	}
	fields := entries[0].ContextMap()
	if fields["direction"] != "output" || fields["model"] != "gpt-5.5" || fields["text"] != "complete blocked output" {
		t.Fatalf("log context = %#v", fields)
	}
}

func newSensitiveTestPolicy(t *testing.T, models []string) detectorPolicy {
	t.Helper()
	detector, err := sensitive.NewDetector(sensitive.Dictionary{
		Name:         "scoped",
		Words:        []string{"blocked"},
		EffectModels: models,
	})
	if err != nil {
		t.Fatal(err)
	}
	return detectorPolicy{detector: detector}
}
