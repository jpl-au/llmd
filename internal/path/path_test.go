package path

import "testing"

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

		// Leading/trailing slashes
		{"/docs/readme", "docs/readme", false},
		{"docs/readme/", "docs/readme", false},
		{"/docs/readme.md/", "docs/readme", false},

		// Traversal paths that resolve cleanly (not rejected)
		{"docs/../secret", "secret", false},

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
