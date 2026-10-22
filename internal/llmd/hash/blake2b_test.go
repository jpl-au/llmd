package hash

import (
	"testing"
)

// TestBlake2b checks known digests and that every output is a
// consistent, collision-free 32-character lowercase hex string.
func TestBlake2b(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"hello", "hello", "46fb7408d4f285228f4af516ea25851b"},
		{"world", "world", "9ecd3b969b678b616aa9e7d6d14a4d49"},
		{"empty", "", "cae66941d9efbd404e4d88758ea67670"},
		{"test", "test", "44a8995dd50b6657a037a7839304535b"},
	}

	seen := make(map[string]string)
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Blake2b(tt.input)
			if got != tt.want {
				t.Errorf("Blake2b(%q) = %q, want %q", tt.input, got, tt.want)
			}
			if len(got) != 32 {
				t.Errorf("Blake2b(%q) length = %d, want 32", tt.input, len(got))
			}
			for _, c := range got {
				if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
					t.Errorf("Blake2b(%q) contains invalid hex char: %c in %q", tt.input, c, got)
				}
			}
			if again := Blake2b(tt.input); again != got {
				t.Errorf("Blake2b(%q) not consistent: %q != %q", tt.input, got, again)
			}
			if prev, ok := seen[got]; ok {
				t.Errorf("Blake2b(%q) collision with Blake2b(%q): %q", tt.input, prev, got)
			}
			seen[got] = tt.input
		})
	}
}
