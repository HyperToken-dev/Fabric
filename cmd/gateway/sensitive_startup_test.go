package main

import (
	"os"
	"path/filepath"
	"testing"

	"fabric/internal/config"
)

func TestLoadSensitiveDetectorLoadsOnlyConfiguredFiles(t *testing.T) {
	dir := t.TempDir()
	writeTestDictionary(t, filepath.Join(dir, "configured-a.txt"), "configured-a-word\n")
	writeTestDictionary(t, filepath.Join(dir, "configured-b.txt"), "configured-b-word\n")
	writeTestDictionary(t, filepath.Join(dir, "ignored.txt"), "ignored-word\n")

	detector, err := loadSensitiveDetector(dir, []config.SensitiveDictionaryConfig{{
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
	_, err := loadSensitiveDetector(t.TempDir(), []config.SensitiveDictionaryConfig{{
		Name:            "missing rule",
		KeywordFileList: []string{"missing"},
	}})
	if err == nil {
		t.Fatal("loadSensitiveDetector() error = nil")
	}
}

func TestLoadSensitiveDetectorValidatesConfig(t *testing.T) {
	dir := t.TempDir()
	writeTestDictionary(t, filepath.Join(dir, "valid.txt"), "word\n")

	tests := []struct {
		name    string
		config  config.SensitiveDictionaryConfig
		wantErr bool
	}{
		{name: "missing rule name", config: config.SensitiveDictionaryConfig{KeywordFileList: []string{"valid"}}, wantErr: true},
		{name: "blank rule name", config: config.SensitiveDictionaryConfig{Name: " ", KeywordFileList: []string{"valid"}}, wantErr: true},
		{name: "missing keyword files", config: config.SensitiveDictionaryConfig{Name: "rule"}, wantErr: true},
		{name: "path separator", config: config.SensitiveDictionaryConfig{Name: "rule", KeywordFileList: []string{"../valid"}}, wantErr: true},
		{name: "dot", config: config.SensitiveDictionaryConfig{Name: "rule", KeywordFileList: []string{"."}}, wantErr: true},
		{name: "dot dot", config: config.SensitiveDictionaryConfig{Name: "rule", KeywordFileList: []string{".."}}, wantErr: true},
		{name: "valid", config: config.SensitiveDictionaryConfig{Name: "rule", KeywordFileList: []string{"valid"}}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := loadSensitiveDetector(dir, []config.SensitiveDictionaryConfig{tt.config})
			if tt.wantErr && err == nil {
				t.Fatal("loadSensitiveDetector() error = nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("loadSensitiveDetector() error = %v", err)
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
