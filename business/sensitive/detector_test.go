package sensitive

import (
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoadDictionary(t *testing.T) {
	path := filepath.Join(t.TempDir(), "custom.data")
	if err := os.WriteFile(path, []byte(" alpha \n\nbeta\nalpha\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	dict, err := LoadDictionary("common", path, []string{"gpt-5.5"})
	if err != nil {
		t.Fatalf("LoadDictionary() error = %v", err)
	}
	if dict.Name != "common" {
		t.Fatalf("Name = %q, want common", dict.Name)
	}
	if !reflect.DeepEqual(dict.Words, []string{"alpha", "beta"}) {
		t.Fatalf("Words = %#v", dict.Words)
	}
	if !reflect.DeepEqual(dict.EffectModels, []string{"gpt-5.5"}) {
		t.Fatalf("EffectiveModels = %#v", dict.EffectModels)
	}
}

func TestLoadDictionaryErrors(t *testing.T) {
	if _, err := LoadDictionary("missing", filepath.Join(t.TempDir(), "missing"), nil); err == nil {
		t.Fatal("LoadDictionary() missing file error = nil")
	}

	emptyPath := filepath.Join(t.TempDir(), "empty")
	if err := os.WriteFile(emptyPath, []byte(" \n\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadDictionary("empty", emptyPath, nil); err == nil {
		t.Fatal("LoadDictionary() empty file error = nil")
	}
}

func TestDetectorKeepsSameNameDictionariesIndependent(t *testing.T) {
	detector, err := NewDetector(
		Dictionary{Name: "common", Words: []string{"alpha", "beta"}, EffectModels: []string{"gpt-5.5"}},
		Dictionary{Name: "common", Words: []string{"beta", "gamma"}, EffectModels: []string{"gpt-5.5-mini"}},
		Dictionary{Name: "other", Words: []string{"alpha"}, EffectModels: []string{"gpt-5.5"}},
	)
	if err != nil {
		t.Fatal(err)
	}

	result := detector.Detect("gpt-5.5", "alpha beta gamma")
	want := Result{Matches: []Match{
		{Dictionary: "common", Words: []string{"alpha", "beta"}},
		{Dictionary: "other", Words: []string{"alpha"}},
	}}
	if !reflect.DeepEqual(result, want) {
		t.Fatalf("Detect() = %#v, want %#v", result, want)
	}
	if detector.Detect("gpt-5.5", "gamma").Rejected() {
		t.Fatal("same-name rule applied words from another model scope")
	}
	if !detector.Detect("gpt-5.5-mini", "gamma").Rejected() {
		t.Fatal("same-name rule did not apply to its own model scope")
	}
}

func TestDetectorEmptyModelScopeAppliesToAllModels(t *testing.T) {
	detector, err := NewDetector(
		Dictionary{Name: "common", Words: []string{"alpha"}, EffectModels: []string{"gpt-5.5"}},
		Dictionary{Name: "common", Words: []string{"beta"}},
	)
	if err != nil {
		t.Fatal(err)
	}
	if !detector.Detect("unlisted-model", "beta").Rejected() {
		t.Fatal("empty model scope did not apply to all models")
	}
	if detector.Detect("unlisted-model", "alpha").Rejected() {
		t.Fatal("scoped dictionary unexpectedly applied to unlisted model")
	}
}

func TestDetectorUsesExactModelNames(t *testing.T) {
	detector, err := NewDetector(Dictionary{
		Name:         "scoped",
		Words:        []string{"blocked"},
		EffectModels: []string{"gpt-5.5"},
	})
	if err != nil {
		t.Fatal(err)
	}

	if !detector.Detect("gpt-5.5", "blocked").Rejected() {
		t.Fatal("exact model did not match")
	}
	for _, model := range []string{"GPT-5.5", "gpt-5.5-mini"} {
		if detector.Detect(model, "blocked").Rejected() {
			t.Fatalf("model %q unexpectedly matched", model)
		}
	}
}

func TestNewDetectorValidatesDictionary(t *testing.T) {
	if _, err := NewDetector(Dictionary{Words: []string{"word"}}); err == nil {
		t.Fatal("NewDetector() empty name error = nil")
	}
	if _, err := NewDetector(Dictionary{Name: "empty", Words: []string{"", " "}}); err == nil {
		t.Fatal("NewDetector() empty words error = nil")
	}
}

func TestNilDetectorDoesNotReject(t *testing.T) {
	var detector *Detector
	result := detector.Detect("gpt-5.5", "blocked")
	if result.Rejected() {
		t.Fatalf("nil detector result = %#v, want no rejection", result)
	}
}

func TestDetectorDeduplicatesRepeatedMatches(t *testing.T) {
	detector, err := NewDetector(Dictionary{Name: "common", Words: []string{"alpha", " beta ", "alpha"}})
	if err != nil {
		t.Fatal(err)
	}

	result := detector.Detect("any", "alpha alpha beta")
	want := Result{Matches: []Match{{Dictionary: "common", Words: []string{"alpha", "beta"}}}}
	if !reflect.DeepEqual(result, want) {
		t.Fatalf("Detect() = %#v, want %#v", result, want)
	}
}

func TestDetectorEmptyTextDoesNotReject(t *testing.T) {
	detector, err := NewDetector(Dictionary{Name: "common", Words: []string{"alpha"}})
	if err != nil {
		t.Fatal(err)
	}
	if detector.Detect("any", "").Rejected() {
		t.Fatal("Detect() rejected empty text")
	}
}

func TestDetectorDeduplicatesEffectiveModels(t *testing.T) {
	detector, err := NewDetector(Dictionary{
		Name:         "scoped",
		Words:        []string{"blocked"},
		EffectModels: []string{"gpt-5.5", "gpt-5.5", "gpt-5.5-mini"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !detector.Detect("gpt-5.5-mini", "blocked").Rejected() {
		t.Fatal("deduplicated model scope did not include gpt-5.5-mini")
	}
	if detector.Detect("other", "blocked").Rejected() {
		t.Fatal("unexpected rejection for model outside deduplicated scope")
	}
}
