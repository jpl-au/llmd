package plugin

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jpl-au/llmd/sdk"
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

// stubDocs is a minimal DocumentStore that returns canned data.
// It proves Yaegi can resolve and call methods on sdk.Documents.
type stubDocs struct {
	docs map[string][]byte
}

func (s *stubDocs) Read(path string, _ int) ([]byte, error) {
	if c, ok := s.docs[path]; ok {
		return c, nil
	}
	return nil, fmt.Errorf("not found: %s", path)
}
func (s *stubDocs) Write(path string, content []byte, _, _ string) error {
	s.docs[path] = content
	return nil
}
func (s *stubDocs) Exists(path string) (bool, error) {
	_, ok := s.docs[path]
	return ok, nil
}
func (s *stubDocs) List(_ string, _ sdk.ListOpts) ([]sdk.Doc, error) {
	var out []sdk.Doc
	for p := range s.docs {
		out = append(out, sdk.Doc{Path: p, Version: 1, CreatedAt: 1700000000000})
	}
	return out, nil
}
func (s *stubDocs) History(path string, _ int) ([]sdk.Version, error) {
	if _, ok := s.docs[path]; !ok {
		return nil, fmt.Errorf("not found: %s", path)
	}
	return []sdk.Version{{Number: 1, Author: "stub", CreatedAt: 1700000000000}}, nil
}
func (s *stubDocs) Delete(string, string) error                        { return nil }
func (s *stubDocs) Restore(string, string) error                       { return nil }
func (s *stubDocs) Move(string, string, string) error                  { return nil }
func (s *stubDocs) Edit(string, string, string, string, string) error  { return nil }
func (s *stubDocs) Glob(string) ([]string, error)                      { return nil, nil }
func (s *stubDocs) Grep(string, sdk.GrepOpts) ([]sdk.GrepHit, error)   { return nil, nil }
func (s *stubDocs) Diff(string, string, int) (string, int, int, error) { return "", 0, 0, nil }
func (s *stubDocs) Revert(string, int, string, string) error           { return nil }
func (s *stubDocs) Vacuum() (sdk.VacuumResult, error)                  { return sdk.VacuumResult{}, nil }
func (s *stubDocs) Import(string, sdk.ImportOpts) (*sdk.ImportResult, error) {
	return &sdk.ImportResult{}, nil
}
func (s *stubDocs) Export(string, string, sdk.ExportOpts) (*sdk.ExportResult, error) {
	return &sdk.ExportResult{}, nil
}

// stubTasks is a minimal TaskStore for testing Yaegi access.
type stubTasks struct {
	tasks []*sdk.Task
}

func (s *stubTasks) Add(title string, _ []byte, opts sdk.TaskAddOpts) (*sdk.Task, error) {
	t := &sdk.Task{Key: fmt.Sprintf("t%d", len(s.tasks)+1), Title: title, Status: "backlog", Author: opts.Author}
	s.tasks = append(s.tasks, t)
	return t, nil
}
func (s *stubTasks) Read(key string) (*sdk.Task, error) {
	for _, t := range s.tasks {
		if t.Key == key {
			return t, nil
		}
	}
	return nil, fmt.Errorf("not found: %s", key)
}
func (s *stubTasks) List(_ sdk.TaskListOpts) ([]*sdk.Task, error) { return s.tasks, nil }
func (s *stubTasks) Move(string, string, string) error            { return nil }
func (s *stubTasks) Set(string, string, sdk.TaskSetOpts) error    { return nil }
func (s *stubTasks) Delete(string, string) (*sdk.Task, error)     { return nil, nil }
func (s *stubTasks) Restore(string, string) (*sdk.Task, error)    { return nil, nil }
func (s *stubTasks) Columns() ([]string, error)                   { return nil, nil }
func (s *stubTasks) AddColumn(string, string, string) error       { return nil }
func (s *stubTasks) RemoveColumn(string, string) error            { return nil }
func (s *stubTasks) MoveColumn(string, string, string) error      { return nil }
func (s *stubTasks) Log(string, int) ([]sdk.TaskEvent, error)     { return nil, nil }

// stubTags is a minimal TagStore for testing Yaegi access.
type stubTags struct {
	tags map[string][]string // path → tag names
}

func (s *stubTags) Add(path, name, _ string) error {
	s.tags[path] = append(s.tags[path], name)
	return nil
}
func (s *stubTags) Remove(string, string, string) error { return nil }
func (s *stubTags) List(path string) ([]sdk.Tag, error) {
	var out []sdk.Tag
	for _, n := range s.tags[path] {
		out = append(out, sdk.Tag{Name: n})
	}
	return out, nil
}
func (s *stubTags) All() ([]sdk.TagInfo, error)   { return nil, nil }
func (s *stubTags) Find(string) ([]string, error) { return nil, nil }

// stubLinks is a minimal LinkStore for testing Yaegi access.
type stubLinks struct {
	links []sdk.Link
}

func (s *stubLinks) Add(from, to, label, _ string) error {
	s.links = append(s.links, sdk.Link{From: from, To: to, Label: label})
	return nil
}
func (s *stubLinks) Remove(string, string, string) error { return nil }
func (s *stubLinks) List(path, dir string) ([]sdk.Link, error) {
	var out []sdk.Link
	for _, l := range s.links {
		if dir == "out" && l.From == path {
			out = append(out, l)
		}
	}
	return out, nil
}

// wireStubs sets the SDK globals to stub implementations and restores
// them when the test finishes.
func wireStubs(t *testing.T) (*stubDocs, *stubTasks, *stubTags, *stubLinks) {
	t.Helper()
	oldDocs, oldTasks, oldTags, oldLinks := sdk.Documents, sdk.Tasks, sdk.Tags, sdk.Links

	d := &stubDocs{docs: map[string][]byte{}}
	ta := &stubTasks{}
	tg := &stubTags{tags: map[string][]string{}}
	l := &stubLinks{}

	sdk.Documents = d
	sdk.Tasks = ta
	sdk.Tags = tg
	sdk.Links = l

	t.Cleanup(func() {
		sdk.Documents = oldDocs
		sdk.Tasks = oldTasks
		sdk.Tags = oldTags
		sdk.Links = oldLinks
	})

	return d, ta, tg, l
}

// TestSamplePluginExec loads the sample plugin through Yaegi and runs its
// commands against stub implementations. This verifies that the Yaegi
// symbol table correctly exposes sdk.Documents so that interpreted plugin
// code can resolve and call methods on the domain globals.
func TestSamplePluginExec(t *testing.T) {
	d, _, _, _ := wireStubs(t)
	d.docs["notes/hello"] = []byte("hello world\nline two\n")

	src, err := os.ReadFile(filepath.Join("..", "..", "plugins", "sample", "sample.go"))
	if err != nil {
		t.Skipf("sample plugin not found: %v", err)
	}
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "sample.go"), src, 0644)

	p, err := load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	ctx := sdk.Context{Author: "alice"}

	// Test "stat" — calls sdk.Documents.Exists and sdk.Documents.History
	resp, err := p.Exec(ctx, "stat", []string{"notes/hello"})
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if resp == nil {
		t.Fatal("stat returned nil response")
	}
	result, ok := resp.(sdk.Result)
	if !ok {
		t.Fatalf("stat response type = %T, want sdk.Result", resp)
	}
	if !strings.Contains(result.Text, "exists: true") {
		t.Errorf("stat text = %q, want contains 'exists: true'", result.Text)
	}

	// Test "wc" — calls sdk.Documents.Read
	resp, err = p.Exec(ctx, "wc", []string{"notes/hello"})
	if err != nil {
		t.Fatalf("wc: %v", err)
	}
	result, ok = resp.(sdk.Result)
	if !ok {
		t.Fatalf("wc response type = %T, want sdk.Result", resp)
	}
	if !strings.Contains(result.Text, "2 lines") {
		t.Errorf("wc text = %q, want contains '2 lines'", result.Text)
	}

	// Test "recent" — calls sdk.Documents.List
	resp, err = p.Exec(ctx, "recent", nil)
	if err != nil {
		t.Fatalf("recent: %v", err)
	}
	result, ok = resp.(sdk.Result)
	if !ok {
		t.Fatalf("recent response type = %T, want sdk.Result", resp)
	}
	if !strings.Contains(result.Text, "notes/hello") {
		t.Errorf("recent text = %q, want contains 'notes/hello'", result.Text)
	}

	// Test "stat" on missing document
	resp, err = p.Exec(ctx, "stat", []string{"nonexistent"})
	if err != nil {
		t.Fatalf("stat missing: %v", err)
	}
	result, ok = resp.(sdk.Result)
	if !ok {
		t.Fatalf("stat missing response type = %T, want sdk.Result", resp)
	}
	if !strings.Contains(result.Text, "not found") {
		t.Errorf("stat missing text = %q, want contains 'not found'", result.Text)
	}
}

// TestYaegiTasksAccess loads a minimal plugin that calls sdk.Tasks and
// verifies the domain global is accessible from interpreted code.
func TestYaegiTasksAccess(t *testing.T) {
	_, ta, _, _ := wireStubs(t)

	pluginSrc := `package taskplug

import "github.com/jpl-au/llmd/sdk"

type P struct{}

func New() *P { return &P{} }
func (p *P) Name() string { return "taskplug" }
func (p *P) Commands() []sdk.Command {
	return []sdk.Command{{Name: "addtask", Desc: "add a task"}}
}
func (p *P) Exec(ctx sdk.Context, cmd string, args []string) (sdk.Response, error) {
	task, err := sdk.Tasks.Add("from plugin", nil, sdk.TaskAddOpts{Author: ctx.Author})
	if err != nil {
		return nil, err
	}
	return sdk.Result{Text: task.Key, Data: task}, nil
}
`

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "taskplug.go"), []byte(pluginSrc), 0644)

	p, err := load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	resp, err := p.Exec(sdk.Context{Author: "alice"}, "addtask", nil)
	if err != nil {
		t.Fatalf("addtask: %v", err)
	}
	result, ok := resp.(sdk.Result)
	if !ok {
		t.Fatalf("response type = %T, want sdk.Result", resp)
	}
	if result.Text == "" {
		t.Error("task key is empty")
	}

	// Verify the task was stored
	if len(ta.tasks) != 1 {
		t.Fatalf("got %d tasks, want 1", len(ta.tasks))
	}
	if ta.tasks[0].Title != "from plugin" {
		t.Errorf("title = %q, want %q", ta.tasks[0].Title, "from plugin")
	}
}

// TestYaegiTagsLinksAccess loads a plugin that calls sdk.Tags and sdk.Links
// to verify all four domain globals work from interpreted code.
func TestYaegiTagsLinksAccess(t *testing.T) {
	d, _, tg, l := wireStubs(t)
	d.docs["a"] = []byte("x")
	d.docs["b"] = []byte("y")

	pluginSrc := `package tlplug

import "github.com/jpl-au/llmd/sdk"

type P struct{}

func New() *P { return &P{} }
func (p *P) Name() string { return "tlplug" }
func (p *P) Commands() []sdk.Command {
	return []sdk.Command{{Name: "wire", Desc: "tag and link"}}
}
func (p *P) Exec(ctx sdk.Context, cmd string, args []string) (sdk.Response, error) {
	if err := sdk.Tags.Add("a", "tested", ctx.Author); err != nil {
		return nil, err
	}
	if err := sdk.Links.Add("a", "b", "related", ctx.Author); err != nil {
		return nil, err
	}
	tags, err := sdk.Tags.List("a")
	if err != nil {
		return nil, err
	}
	links, err := sdk.Links.List("a", "out")
	if err != nil {
		return nil, err
	}
	return sdk.Result{
		Text: "ok",
		Data: map[string]any{"tags": len(tags), "links": len(links)},
	}, nil
}
`

	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "tlplug.go"), []byte(pluginSrc), 0644)

	p, err := load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	resp, err := p.Exec(sdk.Context{Author: "alice"}, "wire", nil)
	if err != nil {
		t.Fatalf("wire: %v", err)
	}
	result, ok := resp.(sdk.Result)
	if !ok {
		t.Fatalf("response type = %T, want sdk.Result", resp)
	}
	if result.Text != "ok" {
		t.Errorf("text = %q, want %q", result.Text, "ok")
	}

	// Verify from outside the plugin
	if len(tg.tags["a"]) != 1 || tg.tags["a"][0] != "tested" {
		t.Errorf("tags = %v, want [tested]", tg.tags["a"])
	}
	if len(l.links) != 1 || l.links[0].To != "b" {
		t.Errorf("links = %v, want [a->b]", l.links)
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
