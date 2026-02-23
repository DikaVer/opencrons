package daemon

import (
	"testing"
	"time"
)

func TestRetryDelay(t *testing.T) {
	const base = 30 * time.Second
	const max = 5 * time.Minute

	tests := []struct {
		name       string
		backoff    string
		retryIndex int
		want       time.Duration
	}{
		// Exponential (default — "" and "exponential" are equivalent)
		{"exp index 0", "", 0, base},           // 30s
		{"exp index 1", "", 1, 60 * time.Second},  // 60s
		{"exp index 2", "", 2, 120 * time.Second}, // 120s
		{"exp index 3", "", 3, 240 * time.Second}, // 240s
		{"exp index 4", "", 4, max},            // 480s capped to 5m
		{"exp index 5 (capped)", "", 5, max},   // already above cap
		{"exp index 9 (capped)", "", 9, max},   // well above cap

		// Linear
		{"linear index 0", "linear", 0, base},             // 30s
		{"linear index 1", "linear", 1, 60 * time.Second},  // 60s
		{"linear index 2", "linear", 2, 90 * time.Second},  // 90s
		{"linear index 9", "linear", 9, 300 * time.Second}, // 300s
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := retryDelay(tt.backoff, tt.retryIndex)
			if got != tt.want {
				t.Errorf("retryDelay(%q, %d) = %v, want %v", tt.backoff, tt.retryIndex, got, tt.want)
			}
		})
	}
}
