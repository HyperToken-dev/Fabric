package models

import "testing"

func TestLookupIgnoresUnrestrictedAPIFormat(t *testing.T) {
	const unknownAPIFormat int32 = 999
	if _, ok := Lookup(unknownAPIFormat, "gpt-4o"); ok {
		t.Fatal("expected unrestricted API format not to use a restricted catalog")
	}
	if IsRestrictedAPIFormat(unknownAPIFormat) {
		t.Fatal("expected unknown API format not to be restricted")
	}
}

func TestListUnsupportedAPIFormatReturnsEmpty(t *testing.T) {
	if models := List(999); len(models) != 0 {
		t.Fatalf("expected no catalog models, got %d", len(models))
	}
}
