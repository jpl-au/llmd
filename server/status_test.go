package server

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
)

// TestStatusOfEveryKind checks that every error kind sdk declares has
// an HTTP status. task next with nothing ready, a failed edit and a
// missing git repository all once reached clients as 500s, because
// their kinds were never mapped.
func TestStatusOfEveryKind(t *testing.T) {
	mapped := map[string]bool{}
	for _, m := range statusOf {
		mapped[m.kind.Error()] = true
	}
	files, err := filepath.Glob(filepath.Join("..", "..", "sdk", "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	sentinel := regexp.MustCompile(`(Err\w+)\s*=\s*errors\.New\("([^"]*)"\)`)
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range sentinel.FindAllStringSubmatch(string(data), -1) {
			if !mapped[m[2]] {
				t.Errorf("sdk.%s has no HTTP status in statusOf", m[1])
			}
		}
	}
}
