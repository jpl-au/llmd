package errs

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestNoStackedErrors scans the module for the patterns that once
// stacked phrases onto error messages ("mv: already exists: destination
// already exists: p"). An error kind belongs in New, Mark or
// sdk.Errorf, never in the message text. cli's TestErrorStyle checks
// the messages commands actually produce; this keeps the source from
// growing new ones.
func TestNoStackedErrors(t *testing.T) {
	rules := []struct {
		re  *regexp.Regexp
		why string
	}{
		{regexp.MustCompile(`Errorf\("%w: `), `puts a sentinel's text in front of the message; use errs.New or sdk.Errorf`},
		{regexp.MustCompile(`Errorf\("[^"]*: %w: `), `puts a sentinel's text inside the message; use errs.New or sdk.Errorf`},
		{regexp.MustCompile(`Errorf\("[a-z ]+: %w", sdk\.Err`), `makes an SDK kind the whole message; write the sentence with sdk.Errorf`},
		{regexp.MustCompile(`(Errorf|errors\.New|errs\.New)\([^)]*"[^"]* - [^"]*"`), `uses " - " for an aside; write a second sentence`},
	}

	root := moduleRoot(t)
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if name := d.Name(); name == ".git" || name == "testdata" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		rel, _ := filepath.Rel(root, path)
		sc := bufio.NewScanner(f)
		for n := 1; sc.Scan(); n++ {
			line := sc.Text()
			if strings.HasPrefix(strings.TrimSpace(line), "//") {
				continue
			}
			for _, r := range rules {
				if r.re.MatchString(line) {
					t.Errorf("%s:%d %s:\n\t%s", rel, n, r.why, strings.TrimSpace(line))
				}
			}
		}
		return sc.Err()
	})
	if err != nil {
		t.Fatal(err)
	}
}

// moduleRoot finds the directory holding go.mod above the test.
func moduleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("go.mod not found above the test")
		}
		dir = parent
	}
}
