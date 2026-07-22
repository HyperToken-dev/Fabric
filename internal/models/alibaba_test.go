package models

import "testing"

func TestLookupAlibabaBailianModel(t *testing.T) {
	model, ok := Lookup(APIFormatAlibabaBailian, "  wan2.7-t2v-2026-06-12  ")
	if !ok {
		t.Fatal("expected wan2.7-t2v-2026-06-12 to be available")
	}
	if model.Name != "wan2.7-t2v-2026-06-12" || model.Type != ModelTypeVideo {
		t.Fatalf("model = %+v", model)
	}
}

func TestListAlibabaBailianModelsSorted(t *testing.T) {
	models := List(APIFormatAlibabaBailian)
	if len(models) == 0 {
		t.Fatal("expected Alibaba Bailian catalog models")
	}
	for i := 1; i < len(models); i++ {
		if models[i-1].Name > models[i].Name {
			t.Fatalf("models not sorted at %d: %q > %q", i, models[i-1].Name, models[i].Name)
		}
	}
}
