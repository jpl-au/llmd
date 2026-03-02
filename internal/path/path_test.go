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

		// Absolute paths — rejected
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

func TestDirect(t *testing.T) {
	tests := []struct {
		path   string
		prefix string
		want   bool
	}{
		// Direct children of "docs"
		{"docs/readme", "docs", true},
		{"docs/api", "docs", true},
		{"docs/api/auth", "docs", false}, // nested

		// Exact match
		{"docs", "docs", true},

		// Top-level (empty prefix)
		{"readme", "", true},
		{"docs/readme", "", false}, // nested

		// Trailing slash in prefix
		{"docs/readme", "docs/", true},

		// Windows backslash in prefix (cross-platform)
		{"docs/readme", "docs\\", true},
		{"docs/api/auth", "docs\\api", true},

		// No match
		{"notes/meeting", "docs", false},
	}

	for _, tt := range tests {
		name := tt.path + "_" + tt.prefix
		t.Run(name, func(t *testing.T) {
			got := Direct(tt.path, tt.prefix)
			if got != tt.want {
				t.Errorf("Direct(%q, %q) = %v, want %v", tt.path, tt.prefix, got, tt.want)
			}
		})
	}
}

func TestResolveDB(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"", filepath.Join(".llmd", "llmd.db")},
		{"docs", filepath.Join(".llmd", "llmd-docs.db")},
		{"notes", filepath.Join(".llmd", "llmd-notes.db")},
		{filepath.Join(".llmd", "llmd.db"), filepath.Join(".llmd", "llmd.db")},
		{filepath.Join(".llmd", "custom.db"), filepath.Join(".llmd", "custom.db")},
		{"my-store.db", "my-store.db"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := ResolveDB(tt.input)
			if got != tt.want {
				t.Errorf("ResolveDB(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestToFS(t *testing.T) {
	tests := []struct {
		dir     string
		docPath string
		wantEnd string // just check the suffix since separator varies by OS
	}{
		{"/out", "docs/readme", "readme.md"},
		{"/out", "docs/readme.txt", "readme.txt"},
		{"/out", "api/v2/auth", "auth.md"},
	}

	for _, tt := range tests {
		t.Run(tt.docPath, func(t *testing.T) {
			got := ToFS(tt.dir, tt.docPath)
			if got == "" {
				t.Errorf("ToFS(%q, %q) returned empty", tt.dir, tt.docPath)
			}
			// Verify it ends with the expected filename
			if len(got) < len(tt.wantEnd) || got[len(got)-len(tt.wantEnd):] != tt.wantEnd {
				t.Errorf("ToFS(%q, %q) = %q, want suffix %q", tt.dir, tt.docPath, got, tt.wantEnd)
			}
		})
	}
}
