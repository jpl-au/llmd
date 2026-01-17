package glob

import "testing"

func TestMatch(t *testing.T) {
	tests := []struct {
		pattern string
		path    string
		want    bool
	}{
		// Basic patterns
		{"*", "document", true},
		{"doc*", "document", true},
		{"*.md", "readme.md", true},
		{"test", "test", true},
		{"test", "other", false},

		// Single directory patterns
		{"notes/*", "notes/todo", true},
		{"notes/*", "notes/ideas", true},
		{"notes/*", "other/todo", false},

		// Double star - prefix only
		{"docs/**", "docs/api", true},
		{"docs/**", "docs/api/v1", true},
		{"docs/**", "other/api", false},

		// Double star - suffix only (the reported bug)
		{"**/document*", "test/document1", true},
		{"**/document*", "a/b/document2", true},
		{"**/document*", "document3", true},
		{"**/test*", "foo/test-file", true},
		{"**/test*", "foo/bar/testing", true},
		{"**/readme", "docs/readme", true},
		{"**/readme", "readme", true},
		{"**/readme", "docs/other", false},

		// Double star - both prefix and suffix
		{"docs/**/api*", "docs/v1/api-ref", true},
		{"docs/**/api*", "docs/api-main", true},
		{"docs/**/api*", "other/api-main", false},

		// Question mark
		{"doc?", "docs", true},
		{"doc?", "doc1", true},
		{"doc?", "document", false},

		// .md suffix handling
		{"notes/**", "notes/todo.md", true},
		{"test.md", "test", true},
	}

	for _, tc := range tests {
		t.Run(tc.pattern+"_"+tc.path, func(t *testing.T) {
			got, err := Match(tc.pattern, tc.path)
			if err != nil {
				t.Fatalf("Match(%q, %q) unexpected error: %v", tc.pattern, tc.path, err)
			}
			if got != tc.want {
				t.Errorf("Match(%q, %q) = %v, want %v", tc.pattern, tc.path, got, tc.want)
			}
		})
	}
}

func TestMatch_InvalidPattern(t *testing.T) {
	_, err := Match("[a-", "test")
	if err == nil {
		t.Error("Match with invalid pattern should return error")
	}
}
