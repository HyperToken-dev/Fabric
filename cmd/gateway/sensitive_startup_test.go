package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/HyperToken-dev/fabric/business/sensitive"
)

func TestLoadSensitiveDetectorLoadsOnlyConfiguredFiles(t *testing.T) {
	dir := t.TempDir()
	writeTestDictionary(t, filepath.Join(dir, "configured-a.txt"), "configured-a-word\n")
	writeTestDictionary(t, filepath.Join(dir, "configured-b.txt"), "configured-b-word\n")
	writeTestDictionary(t, filepath.Join(dir, "ignored.txt"), "ignored-word\n")

	detector, err := sensitive.LoadDetectorFromFiles(dir, []sensitive.DictionaryFileConfig{{
		Name:            "configured rule",
		EffectModels:    []string{"gpt-5.5"},
		KeywordFileList: []string{"configured-a", "configured-b"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !detector.Detect("gpt-5.5", "configured-a-word").Rejected() {
		t.Fatal("first configured dictionary did not apply")
	}
	if !detector.Detect("gpt-5.5", "configured-b-word").Rejected() {
		t.Fatal("second configured dictionary did not apply")
	}
	if detector.Detect("other-model", "configured-a-word").Rejected() {
		t.Fatal("configured dictionary applied to another model")
	}
	if detector.Detect("gpt-5.5", "ignored-word").Rejected() {
		t.Fatal("undeclared dictionary was loaded")
	}
}

func TestLoadSensitiveDetectorFailsForMissingConfiguredFile(t *testing.T) {
	_, err := sensitive.LoadDetectorFromFiles(t.TempDir(), []sensitive.DictionaryFileConfig{{
		Name:            "missing rule",
		KeywordFileList: []string{"missing"},
	}})
	if err == nil {
		t.Fatal("LoadDetectorFromFiles() error = nil")
	}
}

func TestLoadSensitiveDetectorValidatesConfig(t *testing.T) {
	dir := t.TempDir()
	writeTestDictionary(t, filepath.Join(dir, "valid.txt"), "word\n")

	tests := []struct {
		name    string
		config  sensitive.DictionaryFileConfig
		wantErr bool
	}{
		{name: "missing rule name", config: sensitive.DictionaryFileConfig{KeywordFileList: []string{"valid"}}, wantErr: true},
		{name: "blank rule name", config: sensitive.DictionaryFileConfig{Name: " ", KeywordFileList: []string{"valid"}}, wantErr: true},
		{name: "missing keyword files", config: sensitive.DictionaryFileConfig{Name: "rule"}, wantErr: true},
		{name: "path separator", config: sensitive.DictionaryFileConfig{Name: "rule", KeywordFileList: []string{"../valid"}}, wantErr: true},
		{name: "dot", config: sensitive.DictionaryFileConfig{Name: "rule", KeywordFileList: []string{"."}}, wantErr: true},
		{name: "dot dot", config: sensitive.DictionaryFileConfig{Name: "rule", KeywordFileList: []string{".."}}, wantErr: true},
		{name: "valid", config: sensitive.DictionaryFileConfig{Name: "rule", KeywordFileList: []string{"valid"}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := sensitive.LoadDetectorFromFiles(dir, []sensitive.DictionaryFileConfig{tt.config})
			if tt.wantErr && err == nil {
				t.Fatal("LoadDetectorFromFiles() error = nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("LoadDetectorFromFiles() error = %v", err)
			}
		})
	}
}

func writeTestDictionary(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
