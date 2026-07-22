package models

import "testing"

func TestLookupOpenAIModel(t *testing.T) {
	model, ok := Lookup(APIFormatOpenAI, "  gpt-4o  ")
	if !ok {
		t.Fatal("expected gpt-4o to be available")
	}
	if model.Name != "gpt-4o" || model.Type != ModelTypeText {
		t.Fatalf("model = %+v", model)
	}
}

func TestLookupRejectsUnknownModel(t *testing.T) {
	if _, ok := Lookup(APIFormatOpenAI, "unknown-model"); ok {
		t.Fatal("expected unknown OpenAI model to be rejected")
	}
}

func TestListOpenAIModelsSorted(t *testing.T) {
	models := List(APIFormatOpenAI)
	if len(models) == 0 {
		t.Fatal("expected OpenAI catalog models")
	}
	for i := 1; i < len(models); i++ {
		if models[i-1].Name > models[i].Name {
			t.Fatalf("models not sorted at %d: %q > %q", i, models[i-1].Name, models[i].Name)
		}
	}
}
