package models

import "testing"

func TestGoogleCatalogIncludesRequestedTextModels(t *testing.T) {
	want := []string{
		"gemini-3.6-flash",
		"gemini-3.6-flash-latest",
		"gemini-3.5-flash",
		"gemini-3.5-flash-lite",
		"gemini-3.1-pro-preview",
		"gemini-3.1-pro-preview-customtools",
		"gemini-3.1-flash-lite",
		"gemini-3-flash-preview",
		"gemini-2.5-pro",
		"gemini-2.5-pro-latest",
		"gemini-2.5-flash",
		"gemini-2.5-flash-latest",
		"gemini-2.5-flash-lite",
		"gemini-1.5-pro",
		"gemini-1.5-pro-001",
		"gemini-1.5-pro-002",
		"gemini-1.5-flash",
		"gemini-1.5-flash-001",
		"gemini-1.5-flash-002",
		"gemini-1.5-flash-8b",
		"gemma-3-1b",
		"gemma-3-4b",
		"gemma-3-12b",
		"gemma-3-27b",
		"gemma-2-2b",
		"gemma-2-9b",
		"gemma-2-27b",
		"codegemma-2b",
		"codegemma-7b",
		"recurrentgemma-2b",
		"text-bison-001",
		"text-bison-002",
		"chat-bison-001",
		"chat-bison-002",
	}

	listed := List(APIFormatGoogle)
	if len(listed) != len(want) {
		t.Fatalf("len(List(APIFormatGoogle)) = %d, want %d", len(listed), len(want))
	}
	for _, name := range want {
		model, ok := Lookup(APIFormatGoogle, name)
		if !ok {
			t.Fatalf("Lookup(APIFormatGoogle, %q) ok = false", name)
		}
		if model.Name != name {
			t.Fatalf("Lookup(APIFormatGoogle, %q).Name = %q", name, model.Name)
		}
		if model.Type != ModelTypeText {
			t.Fatalf("Lookup(APIFormatGoogle, %q).Type = %d, want %d", name, model.Type, ModelTypeText)
		}
	}
}

func TestGoogleCatalogIsRestrictedAPIFormat(t *testing.T) {
	if !IsRestrictedAPIFormat(APIFormatGoogle) {
		t.Fatal("IsRestrictedAPIFormat(APIFormatGoogle) = false, want true")
	}
}
