package sensitive

import (
	"context"
	"errors"
	"sync"
	"testing"
)

func TestReloadablePolicyReloadSwitchesDetector(t *testing.T) {
	initialDetector := mustTestDetector(t, "initial", "old")
	policy := NewReloadablePolicy(Snapshot{Enabled: true, Detector: initialDetector, DictionaryCount: 1})

	if !policy.Detect(context.Background(), "model", "old word").Rejected() {
		t.Fatal("initial detector did not reject old word")
	}

	newDetector := mustTestDetector(t, "new", "new")
	snapshot, err := policy.Reload(context.Background(), func(ctx context.Context) (SourceState, error) {
		return SourceState{Enabled: true, Detector: newDetector, DictionaryCount: 1}, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Version != 1 || !snapshot.Enabled || snapshot.DictionaryCount != 1 || snapshot.LoadedAt.IsZero() {
		t.Fatalf("snapshot = %#v", snapshot)
	}
	if policy.Detect(context.Background(), "model", "old word").Rejected() {
		t.Fatal("old detector still rejected after reload")
	}
	if !policy.Detect(context.Background(), "model", "new word").Rejected() {
		t.Fatal("new detector did not reject after reload")
	}
}

func TestReloadablePolicyReloadFailureKeepsPreviousSnapshot(t *testing.T) {
	detector := mustTestDetector(t, "initial", "blocked")
	policy := NewReloadablePolicy(Snapshot{Enabled: true, Detector: detector, DictionaryCount: 1})

	wantErr := errors.New("load failed")
	snapshot, err := policy.Reload(context.Background(), func(ctx context.Context) (SourceState, error) {
		return SourceState{}, wantErr
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Reload() error = %v, want %v", err, wantErr)
	}
	if snapshot.Version != 0 || !snapshot.Enabled {
		t.Fatalf("snapshot changed after failed reload: %#v", snapshot)
	}
	if !policy.Detect(context.Background(), "model", "blocked").Rejected() {
		t.Fatal("previous detector was not retained after failed reload")
	}
}

func TestReloadablePolicyEnableDisableTransitions(t *testing.T) {
	policy := NewReloadablePolicy(Snapshot{})
	if policy.Detect(context.Background(), "model", "blocked").Rejected() {
		t.Fatal("disabled policy rejected text")
	}

	detector := mustTestDetector(t, "enabled", "blocked")
	if _, err := policy.Reload(context.Background(), func(ctx context.Context) (SourceState, error) {
		return SourceState{Enabled: true, Detector: detector, DictionaryCount: 1}, nil
	}); err != nil {
		t.Fatal(err)
	}
	if !policy.Detect(context.Background(), "model", "blocked").Rejected() {
		t.Fatal("enabled policy did not reject text")
	}

	if _, err := policy.Reload(context.Background(), func(ctx context.Context) (SourceState, error) {
		return SourceState{Enabled: false}, nil
	}); err != nil {
		t.Fatal(err)
	}
	if policy.Detect(context.Background(), "model", "blocked").Rejected() {
		t.Fatal("disabled policy rejected text after transition")
	}
}

func TestReloadablePolicyDetectConcurrentWithReload(t *testing.T) {
	policy := NewReloadablePolicy(Snapshot{Enabled: true, Detector: mustTestDetector(t, "initial", "blocked"), DictionaryCount: 1})

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				_ = policy.Detect(context.Background(), "model", "blocked changed")
			}
		}()
	}

	for i := 0; i < 50; i++ {
		word := "blocked"
		if i%2 == 1 {
			word = "changed"
		}
		if _, err := policy.Reload(context.Background(), func(ctx context.Context) (SourceState, error) {
			return SourceState{Enabled: true, Detector: mustTestDetector(t, "reload", word), DictionaryCount: 1}, nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	wg.Wait()
}

func mustTestDetector(t *testing.T, name, word string) *Detector {
	t.Helper()
	detector, err := NewDetector(Dictionary{Name: name, Words: []string{word}})
	if err != nil {
		t.Fatal(err)
	}
	return detector
}
