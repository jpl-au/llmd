package hash

import (
	"testing"
)

func TestBlake2b(t *testing.T) {
	// Test that hash produces consistent 32-char hex output
	h := Blake2b("hello")
	if len(h) != 32 {
		t.Errorf("Blake2b() length = %d, want 32", len(h))
	}

	// Test consistency
	h2 := Blake2b("hello")
	if h != h2 {
		t.Errorf("Blake2b() not consistent: %q != %q", h, h2)
	}

	// Test different input produces different hash
	h3 := Blake2b("world")
	if h == h3 {
		t.Errorf("Blake2b() collision: %q == %q", h, h3)
	}
}

func TestBlake2bEmpty(t *testing.T) {
	h := Blake2b("")
	if len(h) != 32 {
		t.Errorf("Blake2b(empty) length = %d, want 32", len(h))
	}

	// Empty input should still produce valid hex
	for _, c := range h {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("Blake2b(empty) contains invalid hex char: %c", c)
		}
	}
}

func TestBlake2bHexFormat(t *testing.T) {
	h := Blake2b("test")
	for _, c := range h {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("Blake2b() contains invalid hex char: %c in %q", c, h)
		}
	}
}
