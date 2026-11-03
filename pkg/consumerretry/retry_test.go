package consumerretry

import (
	"testing"
	"time"
)

func TestBackoffIncreasesAndCaps(t *testing.T) {
	wants := []time.Duration{500 * time.Millisecond, time.Second, 2 * time.Second, 4 * time.Second, 5 * time.Second, 5 * time.Second}
	for index, want := range wants {
		if got := Backoff(index + 1); got != want {
			t.Fatalf("Backoff(%d) = %v, want %v", index+1, got, want)
		}
	}
}

func TestAttemptConfiguration(t *testing.T) {
	if MaxAttempts != 6 || AttemptTimeout != 10*time.Second {
		t.Fatalf("MaxAttempts=%d AttemptTimeout=%v", MaxAttempts, AttemptTimeout)
	}
}
