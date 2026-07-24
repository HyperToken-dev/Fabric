package sensitive

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/fsnotify/fsnotify"
)

type WatchOptions struct {
	Paths    []string
	Debounce time.Duration
	Reload   func(context.Context) error
}

func Watch(ctx context.Context, opts WatchOptions) error {
	if len(opts.Paths) == 0 {
		return errors.New("watch sensitive reload paths: paths are required")
	}
	if opts.Reload == nil {
		return errors.New("watch sensitive reload paths: reload callback is required")
	}
	debounce := opts.Debounce
	if debounce <= 0 {
		debounce = 500 * time.Millisecond
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("create sensitive reload watcher: %w", err)
	}
	defer watcher.Close()

	watched := false
	for _, path := range opts.Paths {
		if path == "" {
			continue
		}
		if _, err := os.Stat(path); err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return fmt.Errorf("stat sensitive reload path %q: %w", path, err)
		}
		if err := watcher.Add(path); err != nil {
			return fmt.Errorf("watch sensitive reload path %q: %w", path, err)
		}
		watched = true
	}
	if !watched {
		return errors.New("watch sensitive reload paths: no existing paths to watch")
	}

	var timer *time.Timer
	var timerC <-chan time.Time // avoid when begining timer is nil,time.C in select will panic
	stopTimer := func() {
		if timer == nil {
			return
		}
		if !timer.Stop() { // timer has already expired, compatible for go 1.23-
			select {
			case <-timer.C:
			default:
			}
		}
		timer = nil
		timerC = nil
	}
	schedule := func() {
		stopTimer()
		timer = time.NewTimer(debounce)
		timerC = timer.C
	}
	defer stopTimer()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case err := <-watcher.Errors:
			if err != nil {
				return fmt.Errorf("watch sensitive reload path: %w", err)
			}
		case event := <-watcher.Events:
			if event.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Rename|fsnotify.Remove|fsnotify.Chmod) != 0 {
				schedule()
			}
		case <-timerC:
			timer = nil
			timerC = nil
			if err := opts.Reload(ctx); err != nil {
				return err
			}
		}
	}
}
