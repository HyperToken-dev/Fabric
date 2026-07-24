package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/viper"
)

func TestSensitiveLoaderReadsLatestConfigAndDictionaryFiles(t *testing.T) {
	runPath := t.TempDir()
	configPath := filepath.Join(runPath, "config.yaml")
	dictDir := filepath.Join(runPath, "configs", "stwd")
	if err := os.MkdirAll(dictDir, 0o700); err != nil {
		t.Fatal(err)
	}
	writeTestDictionary(t, filepath.Join(dictDir, "blocked.txt"), "old-word\n")
	writeSensitiveReloadConfig(t, configPath, true, "blocked")

	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.SetConfigFile(configPath)
	source := gatewaySensitiveSource{workDir: filepath.Join(runPath, "bin"), runPath: runPath}

	loaded, err := source.State(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.Enabled || loaded.DictionaryCount != 1 {
		t.Fatalf("loaded = %#v", loaded)
	}
	if !loaded.Detector.Detect("gpt-5.5", "old-word").Rejected() {
		t.Fatal("initial dictionary content was not loaded")
	}

	writeTestDictionary(t, filepath.Join(dictDir, "blocked.txt"), "new-word\n")
	loaded, err = source.State(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Detector.Detect("gpt-5.5", "old-word").Rejected() {
		t.Fatal("old dictionary content still rejected after reload")
	}
	if !loaded.Detector.Detect("gpt-5.5", "new-word").Rejected() {
		t.Fatal("new dictionary content was not loaded")
	}
}

func TestSensitiveLoaderAppliesRuntimeDisable(t *testing.T) {
	runPath := t.TempDir()
	configPath := filepath.Join(runPath, "config.yaml")
	dictDir := filepath.Join(runPath, "configs", "stwd")
	if err := os.MkdirAll(dictDir, 0o700); err != nil {
		t.Fatal(err)
	}
	writeTestDictionary(t, filepath.Join(dictDir, "blocked.txt"), "blocked\n")
	writeSensitiveReloadConfig(t, configPath, true, "blocked")

	viper.Reset()
	t.Cleanup(viper.Reset)
	viper.SetConfigFile(configPath)
	source := gatewaySensitiveSource{workDir: filepath.Join(runPath, "bin"), runPath: runPath}

	loaded, err := source.State(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !loaded.Enabled {
		t.Fatal("initial load was disabled")
	}

	writeSensitiveReloadConfig(t, configPath, false, "blocked")
	loaded, err = source.State(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Enabled || loaded.Detector != nil || loaded.DictionaryCount != 0 {
		t.Fatalf("disabled load = %#v", loaded)
	}
}

func writeSensitiveReloadConfig(t *testing.T, path string, enabled bool, keywordFile string) {
	t.Helper()
	content := fmt.Sprintf(`proxyAddr: 3002
adminAddr: 9090
timeZone: UTC
sensitiveWordDetect: %t
sensitiveWordDictionaries:
  - name: test-rule
    effectModels: [gpt-5.5]
    keywordFileList: [%s]
`, enabled, keywordFile)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
