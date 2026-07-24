package sensitive

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

func TestWatchDebouncesReloadEvents(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var reloads atomic.Int32
	done := make(chan error, 1)
	go func() {
		done <- Watch(ctx, WatchOptions{
			Paths:    []string{dir},
			Debounce: 25 * time.Millisecond,
			Reload: func(ctx context.Context) error {
				reloads.Add(1)
				cancel()
				return nil
			},
		})
	}()

	time.Sleep(50 * time.Millisecond)

	path := filepath.Join(dir, "dictionary.txt")
	for i := 0; i < 3; i++ {
		if err := os.WriteFile(path, []byte("blocked\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	select {
	case err := <-done:
		if err != nil && !errors.Is(err, context.Canceled) {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Watch() did not reload after file event")
	}
	if reloads.Load() != 1 {
		t.Fatalf("reloads = %d, want 1", reloads.Load())
	}
}

func TestWatchSkipsMissingPathsWhenAnotherPathExists(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := Watch(ctx, WatchOptions{
		Paths: []string{filepath.Join(dir, "missing"), dir},
		Reload: func(ctx context.Context) error {
			return nil
		},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("Watch() error = %v, want context.Canceled", err)
	}
}
