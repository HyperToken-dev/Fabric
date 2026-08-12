package models

import "testing"

func TestExtrotecCatalogIncludesRequestedModels(t *testing.T) {
	want := map[string]int32{
		"MiniMax-H3":               ModelTypeVideo,
		"Z-imageturbo-t2i":         ModelTypeImage,
		"fluxklein-image-to-image": ModelTypeImage,
	}

	listed := List(APIFormatExtrotec)
	if len(listed) != len(want) {
		t.Fatalf("len(List(APIFormatExtrotec)) = %d, want %d", len(listed), len(want))
	}
	for _, model := range listed {
		wantType, ok := want[model.Name]
		if !ok {
			t.Fatalf("unexpected extrotec catalog model %q", model.Name)
		}
		if model.Type != wantType {
			t.Fatalf("List(APIFormatExtrotec) model %q Type = %d, want %d", model.Name, model.Type, wantType)
		}
	}

	for name, wantType := range want {
		model, ok := Lookup(APIFormatExtrotec, name)
		if !ok {
			t.Fatalf("Lookup(APIFormatExtrotec, %q) ok = false", name)
		}
		if model.Name != name || model.Type != wantType {
			t.Fatalf("Lookup(APIFormatExtrotec, %q) = %+v, want type %d", name, model, wantType)
		}
	}
}

func TestExtrotecCatalogIsRestrictedAPIFormat(t *testing.T) {
	if !IsRestrictedAPIFormat(APIFormatExtrotec) {
		t.Fatal("IsRestrictedAPIFormat(APIFormatExtrotec) = false, want true")
	}
}
