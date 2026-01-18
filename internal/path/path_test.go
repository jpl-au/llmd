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
