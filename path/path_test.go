package path

import (
	"path/filepath"
	"testing"
)

func TestNormalise(t *testing.T) {
	tests := []struct {
		input   string
		want    string
		wantErr bool
	}{
		// Basic paths
		{"docs/readme", "docs/readme", false},
		{"docs/readme.md", "docs/readme", false},
		{"docs/readme.MD", "docs/readme", false},
		{"docs/readme.Md", "docs/readme", false},
		{"docs/readme.mD", "docs/readme", false},

		// Nested paths
		{"docs/api/auth.md", "docs/api/auth", false},

		// Trailing slash stripped
		{"docs/readme/", "docs/readme", false},

		// Traversal paths that resolve cleanly (not rejected)
		{"docs/../secret", "secret", false},

		// Backslash paths (converted to forward slashes)
		{"docs\\readme", "docs/readme", false},
		{"docs\\api\\auth.md", "docs/api/auth", false},

		// Absolute paths - rejected
		{"/docs/readme", "", true},
		{"/docs/readme.md/", "", true},
		{"/etc/passwd", "", true},
		{"\\\\server\\share\\doc", "", true}, // UNC path

		// Invalid paths
		{"", "", true},
		{".", "", true},
		{"..", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := Normalise(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("Normalise(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Normalise(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestResolveDB(t *testing.T) {
	tests := []struct {
		input   string
		want    string
		wantErr bool
	}{
		// Default path.
		{"", filepath.Join(".llmd", "llmd.db"), false},

		// Shorthand names.
		{"docs", filepath.Join(".llmd", "llmd-docs.db"), false},
		{"notes", filepath.Join(".llmd", "llmd-notes.db"), false},

		// Sanitisation: spaces become dashes.
		{"my store", filepath.Join(".llmd", "llmd-my-store.db"), false},

		// Sanitisation: consecutive dashes collapse.
		{"my--store", filepath.Join(".llmd", "llmd-my-store.db"), false},

		// Sanitisation: leading/trailing dashes trimmed.
		{"-docs-", filepath.Join(".llmd", "llmd-docs.db"), false},

		// Sanitisation: spaces + dashes combined.
		{" my - store ", filepath.Join(".llmd", "llmd-my-store.db"), false},

		// Explicit paths returned unchanged.
		{filepath.Join(".llmd", "llmd.db"), filepath.Join(".llmd", "llmd.db"), false},
		{filepath.Join(".llmd", "custom.db"), filepath.Join(".llmd", "custom.db"), false},
		{"my-store.db", "my-store.db", false},

		// Rejection: control characters.
		{"docs\x00name", "", true},
		{"docs\x01name", "", true},
		{"docs\nname", "", true},

		// Rejection: Windows-illegal characters.
		{"my<store", "", true},
		{"my>store", "", true},
		{"my:store", "", true},
		{"my\"store", "", true},
		{"my|store", "", true},
		{"my?store", "", true},
		{"my*store", "", true},

		// Rejection: path traversal.
		{"..", "", true},
		{"docs..name", "", true},

		// Rejection: empty after sanitisation.
		{" ", "", true},
		{"---", "", true},
		{"- -", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ResolveDB(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ResolveDB(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ResolveDB(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
