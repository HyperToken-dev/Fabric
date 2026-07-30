package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/HyperToken-dev/fabric/business/sensitive"
	proto "github.com/HyperToken-dev/fabric/gen"
)

func TestSensitiveWordServiceUpdateEnabledReportsReloadFailure(t *testing.T) {
	ctx := context.Background()
	store := NewSensitiveRuntimeStore(t.TempDir())
	initialSnapshot := sensitive.Snapshot{Enabled: false, Version: 7, LoadedAt: time.Unix(100, 0), DictionaryCount: 3}
	policy := sensitive.NewReloadablePolicy(initialSnapshot)
	svc := NewSensitiveWordService(store, policy, func(context.Context) (sensitive.SourceState, error) {
		return sensitive.SourceState{}, errors.New("reload failed")
	})

	if _, err := svc.UpdateSensitiveWordEnabled(ctx, &proto.UpdateSensitiveWordEnabledRequest{Enabled: true}); err == nil {
		t.Fatal("UpdateSensitiveWordEnabled() error = nil")
	}
	state, err := store.ReadState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !state.Enabled {
		t.Fatal("store enabled flag was not persisted")
	}
	if snapshot := policy.Snapshot(); snapshot.Version != initialSnapshot.Version {
		t.Fatalf("snapshot version = %d, want %d", snapshot.Version, initialSnapshot.Version)
	}
}

func TestSensitiveWordServiceCreateDictionaryReportsReloadFailure(t *testing.T) {
	ctx := context.Background()
	store := NewSensitiveRuntimeStore(t.TempDir())
	initialSnapshot := sensitive.Snapshot{Enabled: false, Version: 11, LoadedAt: time.Unix(200, 0), DictionaryCount: 4}
	policy := sensitive.NewReloadablePolicy(initialSnapshot)
	svc := NewSensitiveWordService(store, policy, func(context.Context) (sensitive.SourceState, error) {
		return sensitive.SourceState{}, errors.New("reload failed")
	})

	_, err := svc.CreateSensitiveDictionary(ctx, &proto.CreateSensitiveDictionaryRequest{
		Name:         "测试词库",
		EffectModels: []string{"gpt-4o"},
		Enabled:      true,
		Words:        []string{"blocked"},
	})
	if err == nil {
		t.Fatal("CreateSensitiveDictionary() error = nil")
	}
	stored, err := store.GetDictionary(ctx, "测试词库")
	if err != nil {
		t.Fatal(err)
	}
	if len(stored.Words) != 1 || stored.Words[0] != "blocked" {
		t.Fatalf("stored words = %v, want [blocked]", stored.Words)
	}
	if snapshot := policy.Snapshot(); snapshot.Version != initialSnapshot.Version {
		t.Fatalf("snapshot version = %d, want %d", snapshot.Version, initialSnapshot.Version)
	}
}
