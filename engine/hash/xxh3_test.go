package hash

import (
	"testing"
)

// TestXXH3 checks known digests and that every output is a
// consistent, collision-free 16-character lowercase hex string.
func TestXXH3(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"hello", "hello", "9555e8555c62dcfd"},
		{"world", "world", "d6476c25083d69be"},
		{"empty", "", "2d06800538d394c2"},
		{"test", "test", "9ec9f7918d7dfc40"},
	}

	seen := make(map[string]string)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := XXH3(tt.input)
			if got != tt.want {
				t.Errorf("XXH3(%q) = %q, want %q", tt.input, got, tt.want)
			}
			if len(got) != 16 {
				t.Errorf("XXH3(%q) length = %d, want 16", tt.input, len(got))
			}
			for _, c := range got {
				if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
					t.Errorf("XXH3(%q) contains invalid hex char: %c in %q", tt.input, c, got)
				}
			}
			if again := XXH3(tt.input); again != got {
				t.Errorf("XXH3(%q) not consistent: %q != %q", tt.input, got, again)
			}
			if prev, ok := seen[got]; ok {
				t.Errorf("XXH3(%q) collision with XXH3(%q): %q", tt.input, prev, got)
			}
			seen[got] = tt.input
		})
	}
}
