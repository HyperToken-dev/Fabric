package sensitive

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadDetectorFromFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "blocked.txt"), []byte("blocked\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	detector, err := LoadDetectorFromFiles(dir, []DictionaryFileConfig{{
		Name:            "rule",
		EffectModels:    []string{"gpt-5.5"},
		KeywordFileList: []string{"blocked"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	if !detector.Detect("gpt-5.5", "blocked").Rejected() {
		t.Fatal("configured file was not loaded")
	}
	if detector.Detect("other", "blocked").Rejected() {
		t.Fatal("model scope was not applied")
	}
}

func TestLoadDetectorFromFilesValidatesFileNames(t *testing.T) {
	_, err := LoadDetectorFromFiles(t.TempDir(), []DictionaryFileConfig{{
		Name:            "rule",
		KeywordFileList: []string{"../blocked"},
	}})
	if err == nil {
		t.Fatal("LoadDetectorFromFiles() path traversal error = nil")
	}
}
