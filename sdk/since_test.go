package sdk

import (
	"testing"
	"time"
)

func TestParseSinceDuration(t *testing.T) {
	before := time.Now()
	got, err := ParseSince("5m")
	if err != nil {
		t.Fatalf("ParseSince(5m): %v", err)
	}
	// Should be roughly 5 minutes ago.
	diff := before.Sub(got)
	if diff < 4*time.Minute || diff > 6*time.Minute {
		t.Errorf("ParseSince(5m) = %v ago, want ~5m", diff)
	}
}

func TestParseSinceRFC3339(t *testing.T) {
	got, err := ParseSince("2026-01-15T10:00:00Z")
	if err != nil {
		t.Fatalf("ParseSince(RFC3339): %v", err)
	}
	want := time.Date(2026, 1, 15, 10, 0, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Errorf("got %v, want %v", got, want)
	}
}

func TestParseSinceEmpty(t *testing.T) {
	got, err := ParseSince("")
	if err != nil {
		t.Fatalf("ParseSince(''): %v", err)
	}
	if !got.IsZero() {
		t.Errorf("got %v, want zero", got)
	}
}

func TestParseSinceInvalid(t *testing.T) {
	_, err := ParseSince("yesterday")
	if err == nil {
		t.Fatal("expected error for invalid input")
	}
}
