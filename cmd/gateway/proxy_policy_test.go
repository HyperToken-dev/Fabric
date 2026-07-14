package main

import (
	"context"
	"testing"

	"fabric/business/sensitive"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

type detectorPolicy struct {
	detector *sensitive.Detector
}

func (p detectorPolicy) Detect(ctx context.Context, model, text string) sensitive.Result {
	return p.detector.Detect(model, text)
}

func TestDetectPromptsUsesModelAndLogsCompleteText(t *testing.T) {
	detector, err := sensitive.NewDetector(sensitive.Dictionary{
		Name:         "scoped",
		Words:        []string{"blocked"},
		EffectModels: []string{"gpt-5.5"},
	})
	if err != nil {
		t.Fatal(err)
	}
	core, logs := observer.New(zap.InfoLevel)
	previous := zap.L()
	zap.ReplaceGlobals(zap.New(core))
	t.Cleanup(func() { zap.ReplaceGlobals(previous) })

	text := "complete prompt containing blocked content"
	if detectPrompts(context.Background(), "other-model", TextDirectionInput, []string{text}, detectorPolicy{detector}) {
		t.Fatal("prompt rejected for another model")
	}
	if !detectPrompts(context.Background(), "gpt-5.5", TextDirectionInput, []string{text}, detectorPolicy{detector}) {
		t.Fatal("prompt was not rejected for exact model")
	}

	entries := logs.FilterMessage("sensitive text rejected").All()
	if len(entries) != 1 {
		t.Fatalf("log entries = %d, want 1", len(entries))
	}
	fields := entries[0].ContextMap()
	if fields["text"] != text || fields["model"] != "gpt-5.5" || fields["direction"] != "input" {
		t.Fatalf("log context = %#v", fields)
	}
	if fields["matches"] == nil {
		t.Fatalf("matches missing from log context: %#v", fields)
	}
}
