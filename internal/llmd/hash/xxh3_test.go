package hash

import (
	"testing"
)

func TestXXH3(t *testing.T) {
	// Test that hash produces consistent 32-char hex output
	h := XXH3("hello")
	if len(h) != 16 {
		t.Errorf("XXH3() length = %d, want 16", len(h))
	}

	// Test consistency
	h2 := XXH3("hello")
	if h != h2 {
		t.Errorf("XXH3() not consistent: %q != %q", h, h2)
	}

	// Test different input produces different hash
	h3 := XXH3("world")
	if h == h3 {
		t.Errorf("XXH3() collision: %q == %q", h, h3)
	}
}

func TestXXH3Empty(t *testing.T) {
	h := XXH3("")
	if len(h) != 16 {
		t.Errorf("XXH3(empty) length = %d, want 16", len(h))
	}

	// Empty input should still produce valid hex
	for _, c := range h {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("XXH3(empty) contains invalid hex char: %c", c)
		}
	}
}

func TestXXH3HexFormat(t *testing.T) {
	h := XXH3("test")
	for _, c := range h {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			t.Errorf("XXH3() contains invalid hex char: %c in %q", c, h)
		}
	}
}
