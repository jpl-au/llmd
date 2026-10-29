package meta

import (
	"testing"
)

func TestCompute(t *testing.T) {
	tests := []struct {
		name      string
		content   string
		wantSize  int
		wantLines int
	}{
		{"empty", "", 0, 0},
		{"single char", "a", 1, 1},
		{"single line no newline", "hello", 5, 1},
		{"single line with newline", "hello\n", 6, 1},
		{"two lines", "hello\nworld", 11, 2},
		{"two lines trailing newline", "hello\nworld\n", 12, 2},
		{"three lines", "a\nb\nc", 5, 3},
		{"blank lines", "\n\n\n", 3, 3},
		{"unicode", "héllo wörld", 13, 1}, // é and ö are 2 bytes each
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := Compute(tt.content)
			if m.Size != tt.wantSize {
				t.Errorf("Compute(%q).Size = %d, want %d", tt.content, m.Size, tt.wantSize)
			}
			if m.Lines != tt.wantLines {
				t.Errorf("Compute(%q).Lines = %d, want %d", tt.content, m.Lines, tt.wantLines)
			}
		})
	}
}
