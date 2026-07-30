package service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/HyperToken-dev/fabric/business/sensitive"
	proto "github.com/HyperToken-dev/fabric/gen"
)

func TestSensitiveWordServiceUpdateEnabledSucceedsWhenReloadFails(t *testing.T) {
	ctx := context.Background()
	store := NewSensitiveRuntimeStore(t.TempDir())
	initialSnapshot := sensitive.Snapshot{Enabled: false, Version: 7, LoadedAt: time.Unix(100, 0), DictionaryCount: 3}
	policy := sensitive.NewReloadablePolicy(initialSnapshot)
	svc := NewSensitiveWordService(store, policy, func(context.Context) (sensitive.SourceState, error) {
		return sensitive.SourceState{}, errors.New("reload failed")
	})

	resp, err := svc.UpdateSensitiveWordEnabled(ctx, &proto.UpdateSensitiveWordEnabledRequest{Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	state, err := store.ReadState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !state.Enabled {
		t.Fatal("store enabled flag was not persisted")
	}
	if resp.Snapshot == nil {
		t.Fatal("response snapshot is nil")
	}
	if resp.Snapshot.Version != initialSnapshot.Version {
		t.Fatalf("snapshot version = %d, want %d", resp.Snapshot.Version, initialSnapshot.Version)
	}
	if resp.Snapshot.Enabled != initialSnapshot.Enabled {
		t.Fatalf("snapshot enabled = %v, want %v", resp.Snapshot.Enabled, initialSnapshot.Enabled)
	}
}

func TestSensitiveWordServiceCreateDictionarySucceedsWhenReloadFails(t *testing.T) {
	ctx := context.Background()
	store := NewSensitiveRuntimeStore(t.TempDir())
	initialSnapshot := sensitive.Snapshot{Enabled: false, Version: 11, LoadedAt: time.Unix(200, 0), DictionaryCount: 4}
	policy := sensitive.NewReloadablePolicy(initialSnapshot)
	svc := NewSensitiveWordService(store, policy, func(context.Context) (sensitive.SourceState, error) {
		return sensitive.SourceState{}, errors.New("reload failed")
	})

	resp, err := svc.CreateSensitiveDictionary(ctx, &proto.CreateSensitiveDictionaryRequest{
		Name:         "测试词库",
		EffectModels: []string{"gpt-4o"},
		Enabled:      true,
		Words:        []string{"blocked"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if resp.Dictionary == nil {
		t.Fatal("response dictionary is nil")
	}
	if resp.Dictionary.Name != "测试词库" {
		t.Fatalf("dictionary name = %q, want %q", resp.Dictionary.Name, "测试词库")
	}
	if resp.Snapshot == nil {
		t.Fatal("response snapshot is nil")
	}
	if resp.Snapshot.Version != initialSnapshot.Version {
		t.Fatalf("snapshot version = %d, want %d", resp.Snapshot.Version, initialSnapshot.Version)
	}
	stored, err := store.GetDictionary(ctx, "测试词库")
	if err != nil {
		t.Fatal(err)
	}
	if len(stored.Words) != 1 || stored.Words[0] != "blocked" {
		t.Fatalf("stored words = %v, want [blocked]", stored.Words)
	}
}
