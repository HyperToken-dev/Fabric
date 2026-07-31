package main

import "testing"

func TestNormalizeProviderTaskStatus(t *testing.T) {
	tests := []struct {
		status string
		want   ProviderTaskStatus
	}{
		{status: "queued", want: ProviderTaskStatusPending},
		{status: "running", want: ProviderTaskStatusPending},
		{status: "succeeded", want: ProviderTaskStatusSuccess},
		{status: "success", want: ProviderTaskStatusSuccess},
		{status: "failed", want: ProviderTaskStatusFail},
		{status: "cancelled", want: ProviderTaskStatusFail},
		{status: "unknown", want: ProviderTaskStatusPending},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			if got := normalizeProviderTaskStatus(tt.status); got != tt.want {
				t.Fatalf("status = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestProviderTaskTerminalStatus(t *testing.T) {
	if isTerminalProviderTaskStatus(ProviderTaskStatusPending) {
		t.Fatal("pending should not be terminal")
	}
	if !isTerminalProviderTaskStatus(ProviderTaskStatusSuccess) {
		t.Fatal("success should be terminal")
	}
	if !isTerminalProviderTaskStatus(ProviderTaskStatusFail) {
		t.Fatal("fail should be terminal")
	}
}
