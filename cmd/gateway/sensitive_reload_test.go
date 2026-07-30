package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/HyperToken-dev/fabric/internal/service"
)

func TestSensitiveLoaderReadsLatestRuntimeDictionaryFiles(t *testing.T) {
	baseDir := t.TempDir()
	store := service.NewSensitiveRuntimeStore(baseDir)
	if err := store.Ensure(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := store.SetEnabled(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateDictionary(context.Background(), "测试词库", []string{"gpt-5.5"}, true, []string{"old-word"}); err != nil {
		t.Fatal(err)
	}
	source := gatewaySensitiveSource{runtimeStore: store}

	loaded, err := source.State(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.Enabled || loaded.DictionaryCount != 1 {
		t.Fatalf("loaded = %#v", loaded)
	}
	if !loaded.Detector.Detect("gpt-5.5", "old-word").Rejected() {
		t.Fatal("initial dictionary content was not loaded")
	}

	state, err := store.ReadState(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	wordsPath := filepath.Join(baseDir, "dictionaries", state.Dictionaries[0].KeywordFile)
	if err := os.WriteFile(wordsPath, []byte("new-word\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	loaded, err = source.State(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Detector.Detect("gpt-5.5", "old-word").Rejected() {
		t.Fatal("old dictionary content still rejected after reload")
	}
	if !loaded.Detector.Detect("gpt-5.5", "new-word").Rejected() {
		t.Fatal("new dictionary content was not loaded")
	}
}

func TestSensitiveLoaderAppliesRuntimeDisable(t *testing.T) {
	store := service.NewSensitiveRuntimeStore(t.TempDir())
	if err := store.Ensure(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := store.SetEnabled(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateDictionary(context.Background(), "blocked", nil, true, []string{"blocked"}); err != nil {
		t.Fatal(err)
	}
	source := gatewaySensitiveSource{runtimeStore: store}

	loaded, err := source.State(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.Enabled {
		t.Fatal("initial load was disabled")
	}

	if err := store.SetEnabled(context.Background(), false); err != nil {
		t.Fatal(err)
	}
	loaded, err = source.State(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Enabled || loaded.Detector != nil || loaded.DictionaryCount != 0 {
		t.Fatalf("disabled load = %#v", loaded)
	}
}

func TestSensitiveLoaderAutoCreatesDisabledRuntimeStore(t *testing.T) {
	baseDir := t.TempDir()
	store := service.NewSensitiveRuntimeStore(baseDir)
	source := gatewaySensitiveSource{runtimeStore: store}

	loaded, err := source.State(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Enabled || loaded.Detector != nil || loaded.DictionaryCount != 0 {
		t.Fatalf("fresh runtime load = %#v", loaded)
	}
	if _, err := os.Stat(filepath.Join(baseDir, "state.json")); err != nil {
		t.Fatalf("state.json was not created: %v", err)
	}
}
