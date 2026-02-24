package key

import (
	"testing"
)

func TestGenerate(t *testing.T) {
	k := Generate()
	if len(k) != 9 {
		t.Errorf("Generate() = %q, want 9 characters", k)
	}
	if err := Validate(k); err != nil {
		t.Errorf("Generate() produced invalid key: %v", err)
	}
}

func TestGenerateAt(t *testing.T) {
	tests := []struct {
		name string
		ms   int64
		want string
	}{
		{"zero", 0, "000000000"},
		{"small", 1, "000000001"},
		{"36", 36, "000000010"},
		{"1000", 1000, "0000000rs"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GenerateAt(tt.ms)
			if got != tt.want {
				t.Errorf("GenerateAt(%d) = %q, want %q", tt.ms, got, tt.want)
			}
		})
	}
}

func TestGenerateAtLength(t *testing.T) {
	// All keys should be 9 characters regardless of input
	tests := []int64{0, 1, 100, 10000, 1000000000000, 2000000000000, 3000000000000}
	for _, ms := range tests {
		got := GenerateAt(ms)
		if len(got) != 9 {
			t.Errorf("GenerateAt(%d) = %q, length = %d, want 9", ms, got, len(got))
		}
		if err := Validate(got); err != nil {
			t.Errorf("GenerateAt(%d) = %q, validation failed: %v", ms, got, err)
		}
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name    string
		key     string
		wantErr error
	}{
		{"valid lowercase", "k7v3x8m2a", nil},
		{"valid digits", "123456789", nil},
		{"valid mixed", "abc123xyz", nil},
		{"valid zeros", "000000000", nil},

		{"too short", "k7v3x8m2", ErrInvalidLength},
		{"too long", "k7v3x8m2ab", ErrInvalidLength},
		{"empty", "", ErrInvalidLength},

		{"uppercase", "K7V3X8M2A", ErrInvalidChar},
		{"uppercase mixed", "k7v3x8m2A", ErrInvalidChar},
		{"special char", "k7v3x8m2!", ErrInvalidChar},
		{"space", "k7v3x8m2 ", ErrInvalidChar},
		{"underscore", "k7v3_8m2a", ErrInvalidChar},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Validate(tt.key)
			if err != tt.wantErr {
				t.Errorf("Validate(%q) = %v, want %v", tt.key, err, tt.wantErr)
			}
		})
	}
}

func TestGenerateAtUniqueness(t *testing.T) {
	// Different timestamps should produce different keys
	seen := make(map[string]bool)
	for ms := range int64(1000) {
		k := GenerateAt(ms)
		if seen[k] {
			t.Errorf("GenerateAt(%d) produced duplicate key: %s", ms, k)
		}
		seen[k] = true
	}
}

func TestGenerateAtMonotonic(t *testing.T) {
	// Keys should be lexicographically sorted when timestamps increase
	prev := GenerateAt(0)
	for ms := int64(1); ms < 100; ms++ {
		curr := GenerateAt(ms)
		if curr <= prev {
			t.Errorf("GenerateAt(%d) = %q <= GenerateAt(%d) = %q, want monotonically increasing",
				ms, curr, ms-1, prev)
		}
		prev = curr
	}
}

func TestGenerateUniqueness(t *testing.T) {
	// Generate() should produce unique keys even in tight loops
	seen := make(map[string]bool)
	for i := range 1000 {
		k := Generate()
		if seen[k] {
			t.Errorf("Generate() produced duplicate key on iteration %d: %s", i, k)
		}
		seen[k] = true
	}
}
