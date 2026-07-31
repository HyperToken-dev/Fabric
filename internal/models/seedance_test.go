package models

import "testing"

func TestLookupSeedanceModel(t *testing.T) {
	model, ok := Lookup(APIFormatSeedance, "  doubao-seedance-2-0-260128  ")
	if !ok {
		t.Fatal("expected doubao-seedance-2-0-260128 to be available")
	}
	if model.Name != "doubao-seedance-2-0-260128" || model.Type != ModelTypeVideo {
		t.Fatalf("model = %+v", model)
	}
}

func TestListSeedanceModelsSorted(t *testing.T) {
	models := List(APIFormatSeedance)
	if len(models) == 0 {
		t.Fatal("expected Seedance catalog models")
	}
	for i := 1; i < len(models); i++ {
		if models[i-1].Name > models[i].Name {
			t.Fatalf("models not sorted at %d: %q > %q", i, models[i-1].Name, models[i].Name)
		}
	}
}

func TestSeedanceAPIFormatIsRestricted(t *testing.T) {
	if !IsRestrictedAPIFormat(APIFormatSeedance) {
		t.Fatal("expected Seedance API format to be restricted")
	}
}
