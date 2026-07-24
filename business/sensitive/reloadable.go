package sensitive

import (
	"context"
	"errors"
	"sync/atomic"
	"time"
)

type Snapshot struct {
	Enabled         bool
	Detector        *Detector
	Version         int64
	LoadedAt        time.Time
	DictionaryCount int
}

type SourceState struct {
	Enabled         bool
	Detector        *Detector
	DictionaryCount int
}

type SourceFunc func(ctx context.Context) (SourceState, error)

type ReloadablePolicy struct {
	snapshot atomic.Pointer[Snapshot]
	now      func() time.Time
}

func NewReloadablePolicy(initial Snapshot) *ReloadablePolicy {
	policy := &ReloadablePolicy{now: time.Now}
	if initial.LoadedAt.IsZero() {
		initial.LoadedAt = policy.now()
	}
	initialCopy := initial
	policy.snapshot.Store(&initialCopy)
	return policy
}

func (p *ReloadablePolicy) Detect(ctx context.Context, model, text string) Result {
	if p == nil {
		return Result{}
	}
	snapshot := p.snapshot.Load()
	if snapshot == nil || !snapshot.Enabled || snapshot.Detector == nil {
		return Result{}
	}
	return snapshot.Detector.Detect(model, text)
}

func (p *ReloadablePolicy) Snapshot() Snapshot {
	if p == nil {
		return Snapshot{}
	}
	snapshot := p.snapshot.Load()
	if snapshot == nil {
		return Snapshot{}
	}
	return *snapshot
}

func (p *ReloadablePolicy) Reload(ctx context.Context, source SourceFunc) (Snapshot, error) {
	if p == nil {
		return Snapshot{}, errors.New("reload sensitive policy: policy is nil")
	}
	if source == nil {
		return p.Snapshot(), errors.New("reload sensitive policy: source is nil")
	}

	result, err := source(ctx)
	if err != nil {
		return p.Snapshot(), err
	}
	if result.Enabled && result.Detector == nil {
		return p.Snapshot(), errors.New("reload sensitive policy: enabled result has nil detector")
	}

	previous := p.Snapshot()
	next := Snapshot{
		Enabled:         result.Enabled,
		Detector:        result.Detector,
		Version:         previous.Version + 1,
		LoadedAt:        p.now(),
		DictionaryCount: result.DictionaryCount,
	}
	p.snapshot.Store(&next)
	return next, nil
}
