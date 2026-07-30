package service

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestSensitiveRuntimeStoreDeleteDictionaryKeepsStateWhenWordFileRemoveFails(t *testing.T) {
	ctx := context.Background()
	store := NewSensitiveRuntimeStore(t.TempDir())
	created, err := store.CreateDictionary(ctx, "测试词库", nil, true, []string{"blocked"})
	if err != nil {
		t.Fatal(err)
	}

	state, err := store.ReadState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	meta, _, ok := findSensitiveDictionary(state.Dictionaries, created.Name)
	if !ok {
		t.Fatal("created dictionary was not recorded in state")
	}
	wordsPath := store.wordsPath(meta.KeywordFile)
	if err := os.Remove(wordsPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(wordsPath, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wordsPath, "child"), []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := store.DeleteDictionary(ctx, created.Name); err == nil {
		t.Fatal("DeleteDictionary() error = nil")
	}
	state, err = store.ReadState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, ok := findSensitiveDictionary(state.Dictionaries, created.Name); !ok {
		t.Fatal("dictionary was removed from state after backing file deletion failed")
	}
}

func TestSensitiveRuntimeStoreDeleteDictionaryRemovesStateAndWordFile(t *testing.T) {
	ctx := context.Background()
	store := NewSensitiveRuntimeStore(t.TempDir())
	created, err := store.CreateDictionary(ctx, "测试词库", nil, true, []string{"blocked"})
	if err != nil {
		t.Fatal(err)
	}
	state, err := store.ReadState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	meta, _, ok := findSensitiveDictionary(state.Dictionaries, created.Name)
	if !ok {
		t.Fatal("created dictionary was not recorded in state")
	}
	wordsPath := store.wordsPath(meta.KeywordFile)

	if err := store.DeleteDictionary(ctx, created.Name); err != nil {
		t.Fatal(err)
	}
	state, err = store.ReadState(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, ok := findSensitiveDictionary(state.Dictionaries, created.Name); ok {
		t.Fatal("dictionary still exists in state")
	}
	if _, err := os.Stat(wordsPath); !os.IsNotExist(err) {
		t.Fatalf("word file stat error = %v, want not exist", err)
	}
}
