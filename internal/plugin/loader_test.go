package plugin

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStripPackageDecl(t *testing.T) {
	src := `package sample

import "fmt"

func Hello() { fmt.Println("hello") }
`
	got := stripPackageDecl(src)
	if strings.Contains(got, "package sample") {
		t.Fatalf("package decl not stripped:\n%s", got)
	}
	if !strings.Contains(got, `import "fmt"`) {
		t.Fatal("import was stripped")
	}
	if !strings.Contains(got, "func Hello()") {
		t.Fatal("function was stripped")
	}
}

func TestPkgName(t *testing.T) {
	tests := []struct {
		src  string
		want string
	}{
		{"package foo\n", "foo"},
		{"  package bar\n", "bar"},
		{"// comment\npackage baz\n", "baz"},
		{"", ""},
	}
	for _, tt := range tests {
		got := pkgName(tt.src)
		if got != tt.want {
			t.Errorf("pkgName(%q) = %q, want %q", tt.src, got, tt.want)
		}
	}
}

func TestReadSourceNoGoFiles(t *testing.T) {
	dir := t.TempDir()
	_, err := readSource(dir)
	if !errors.Is(err, ErrNoGoFiles) {
		t.Fatalf("got %v, want ErrNoGoFiles", err)
	}
}

func TestLoadNotPlugin(t *testing.T) {
	dir := t.TempDir()
	// New() returns int — calling Name() on it will fail
	if err := os.WriteFile(filepath.Join(dir, "bad.go"), []byte(`package bad
func New() int { return 42 }
`), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := load(dir)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
}

func TestLoadSamplePlugin(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("..", "..", "plugins", "sample", "sample.go"))
	if err != nil {
		t.Skipf("sample plugin not found: %v", err)
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "sample.go"), src, 0644); err != nil {
		t.Fatal(err)
	}

	p, err := load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	if p.Name() != "sample" {
		t.Fatalf("Name() = %q, want %q", p.Name(), "sample")
	}

	cmds := p.Commands()
	if len(cmds) != 3 {
		t.Fatalf("Commands() returned %d commands, want 3", len(cmds))
	}

	names := map[string]bool{}
	for _, c := range cmds {
		names[c.Name] = true
	}
	for _, want := range []string{"stat", "recent", "wc"} {
		if !names[want] {
			t.Errorf("missing command %q", want)
		}
	}
}

func TestLoadContinuesOnError(t *testing.T) {
	root := t.TempDir()

	// Broken plugin — New() returns wrong type
	broken := filepath.Join(root, "broken")
	if err := os.Mkdir(broken, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(broken, "broken.go"), []byte(`package broken
func New() int { return 42 }
`), 0644); err != nil {
		t.Fatal(err)
	}

	// Working plugin
	good := filepath.Join(root, "good")
	if err := os.Mkdir(good, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(good, "good.go"), []byte(`package good

import "github.com/jpl-au/llmd/sdk"

type Good struct{}

func New() *Good { return &Good{} }
func (g *Good) Name() string { return "good" }
func (g *Good) Commands() []sdk.Command { return nil }
func (g *Good) Exec(ctx sdk.Context, cmd string, args []string) (sdk.Response, error) { return nil, nil }
`), 0644); err != nil {
		t.Fatal(err)
	}

	seen := map[string]bool{}
	dirs := scan(root, seen)
	if len(dirs) != 2 {
		t.Fatalf("scan found %d dirs, want 2", len(dirs))
	}

	// Load should succeed for the good plugin despite the broken one
	var loaded int
	for _, dir := range dirs {
		p, err := load(dir)
		if err != nil {
			continue
		}
		if p.Name() == "good" {
			loaded++
		}
	}
	if loaded != 1 {
		t.Fatalf("loaded %d good plugins, want 1", loaded)
	}
}
