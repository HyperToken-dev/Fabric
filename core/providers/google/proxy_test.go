package google

import "testing"

func TestNormalizeOutboundPath(t *testing.T) {
	tests := []struct {
		name       string
		inputPath  string
		outputPath string
		want       string
	}{
		{name: "no base prefix", inputPath: "/v1beta/interactions", outputPath: "/v1beta/interactions", want: "/v1beta/interactions"},
		{name: "base prefix request without prefix", inputPath: "/interactions", outputPath: "/v1beta/interactions", want: "/v1beta/interactions"},
		{name: "base prefix request with same prefix", inputPath: "/v1beta/interactions", outputPath: "/v1beta/v1beta/interactions", want: "/v1beta/interactions"},
		{name: "multi segment duplicate prefix", inputPath: "/google/v1beta/interactions", outputPath: "/google/v1beta/google/v1beta/interactions", want: "/google/v1beta/interactions"},
		{name: "similar prefix is not duplicate", inputPath: "/v1beta/interactions", outputPath: "/v1/v1beta/interactions", want: "/v1/v1beta/interactions"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizeOutboundPath(tt.inputPath, tt.outputPath); got != tt.want {
				t.Fatalf("normalizeOutboundPath(%q, %q) = %q, want %q", tt.inputPath, tt.outputPath, got, tt.want)
			}
		})
	}
}
