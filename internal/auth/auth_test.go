package auth

import (
	"net/http/httptest"
	"testing"
)

func TestHashKey(t *testing.T) {
	got := HashKey("hy_test_key")
	const want = "53f00fdbf6681b4212275cab486fabac73f58315400abb6d0e9df054661c78ed"
	if got != want {
		t.Fatalf("HashKey() = %q, want %q", got, want)
	}
	if len(got) != 64 {
		t.Fatalf("HashKey() length = %d, want 64", len(got))
	}
	if got == HashKey("other") {
		t.Fatal("HashKey() returned same hash for different keys")
	}
}

func TestExtractKeyFromRequest(t *testing.T) {
	tests := []struct {
		name   string
		header string
		want   string
		wantErr bool
	}{
		{name: "bearer token", header: "Bearer hy_secret", want: "hy_secret"},
		{name: "token with padding", header: "Bearer  hy_secret ", want: "hy_secret"},
		{name: "empty bearer token", header: "Bearer ", wantErr: true},
		{name: "whitespace bearer token", header: "Bearer   ", wantErr: true},
		{name: "missing authorization", header: "", wantErr: true},
		{name: "wrong scheme", header: "Basic hy_secret", wantErr: true},
		{name: "lowercase scheme", header: "bearer hy_secret", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}

			got, err := ExtractKeyFromRequest(req)
			if tt.wantErr {
				if err == nil {
					t.Fatal("ExtractKeyFromRequest() error = nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("ExtractKeyFromRequest() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("ExtractKeyFromRequest() = %q, want %q", got, tt.want)
			}
		})
	}
}
