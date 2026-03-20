// integration_test.go exercises multi-step workflows through sdk.Dispatch,
// the same entry point used by the CLI and MCP server. Each test chains
// several commands and verifies the result at each step.
//
// These complement the per-domain unit tests in api_test.go and
// api_tasks_test.go which call SDK globals directly. Integration tests
// catch issues in argument parsing, flag handling, response formatting,
// and cross-domain interactions.
package host

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	// Blank import registers the CLI extension so sdk.Dispatch can
	// route commands. Same pattern as main.go.
	_ "github.com/jpl-au/llmd/cli"
	"github.com/jpl-au/llmd/sdk"
)

// dispatch calls sdk.Dispatch and fails the test on error.
func dispatch(t *testing.T, cmd string, args []string, author string, stdin []byte) sdk.Response {
	t.Helper()
	r, err := sdk.Dispatch(context.Background(), cmd, args, author, stdin, "")
	if err != nil {
		t.Fatalf("dispatch %s %v: %v", cmd, args, err)
	}
	return r
}

// dispatchErr calls sdk.Dispatch and returns the error.
func dispatchErr(t *testing.T, cmd string, args []string, author string, stdin []byte) error {
	t.Helper()
	_, err := sdk.Dispatch(context.Background(), cmd, args, author, stdin, "")
	return err
}

// text extracts the string content from a response.
func text(r sdk.Response) string {
	switch v := r.(type) {
	case sdk.Text:
		return string(v)
	case sdk.Result:
		return v.Text
	default:
		return ""
	}
}

func TestCRUD(t *testing.T) {
	testHost(t)

	// Write two documents.
	dispatch(t, "write", []string{"notes/hello"}, "alice", []byte("# Hello"))
	dispatch(t, "write", []string{"notes/world"}, "alice", []byte("# World"))

	// Cat reads content back.
	r := dispatch(t, "cat", []string{"notes/hello"}, "", nil)
	if got := text(r); got != "# Hello" {
		t.Errorf("cat notes/hello = %q, want %q", got, "# Hello")
	}

	// Ls lists both documents.
	r = dispatch(t, "ls", nil, "", nil)
	out := text(r)
	if !strings.Contains(out, "notes/hello") || !strings.Contains(out, "notes/world") {
		t.Errorf("ls missing documents: %s", out)
	}

	// Ls with prefix.
	r = dispatch(t, "ls", []string{"notes/"}, "", nil)
	out = text(r)
	if !strings.Contains(out, "notes/hello") || !strings.Contains(out, "notes/world") {
		t.Errorf("ls notes/ missing documents: %s", out)
	}

	// Rm soft-deletes.
	dispatch(t, "rm", []string{"notes/hello"}, "alice", nil)

	// Cat on deleted document fails.
	err := dispatchErr(t, "cat", []string{"notes/hello"}, "", nil)
	if err == nil {
		t.Error("cat deleted doc: expected error, got nil")
	}

	// Ls no longer shows deleted document.
	r = dispatch(t, "ls", nil, "", nil)
	out = text(r)
	if strings.Contains(out, "notes/hello") {
		t.Errorf("ls still shows deleted doc: %s", out)
	}

	// Restore brings it back.
	dispatch(t, "restore", []string{"notes/hello"}, "alice", nil)

	// Cat works again.
	r = dispatch(t, "cat", []string{"notes/hello"}, "", nil)
	if got := text(r); got != "# Hello" {
		t.Errorf("cat after restore = %q, want %q", got, "# Hello")
	}
}

func TestEditWorkflow(t *testing.T) {
	testHost(t)

	// Write initial content.
	dispatch(t, "write", []string{"doc"}, "alice", []byte("hello world"))

	// Sed substitution.
	dispatch(t, "sed", []string{"s/hello/goodbye/", "doc"}, "alice", nil)
	r := dispatch(t, "cat", []string{"doc"}, "", nil)
	if got := text(r); got != "goodbye world" {
		t.Errorf("after sed = %q, want %q", got, "goodbye world")
	}

	// Edit (search-and-replace).
	dispatch(t, "edit", []string{"doc", "goodbye", "farewell"}, "alice", nil)
	r = dispatch(t, "cat", []string{"doc"}, "", nil)
	if got := text(r); got != "farewell world" {
		t.Errorf("after edit = %q, want %q", got, "farewell world")
	}

	// History shows 3 versions.
	r = dispatch(t, "history", []string{"doc"}, "", nil)
	out := text(r)
	// History output is a table with version numbers - check for v1, v2, v3.
	if !strings.Contains(out, "1") || !strings.Contains(out, "3") {
		t.Errorf("history missing versions: %s", out)
	}

	// Diff shows change.
	r = dispatch(t, "diff", []string{"doc"}, "", nil)
	out = text(r)
	if out == "" {
		t.Error("diff returned empty output")
	}

	// Revert to version 1.
	dispatch(t, "revert", []string{"doc", "1"}, "alice", nil)
	r = dispatch(t, "cat", []string{"doc"}, "", nil)
	if got := text(r); got != "hello world" {
		t.Errorf("after revert = %q, want %q", got, "hello world")
	}
}

func TestSearchWorkflow(t *testing.T) {
	testHost(t)

	// Write several documents.
	dispatch(t, "write", []string{"docs/api"}, "alice", []byte("REST API reference"))
	dispatch(t, "write", []string{"docs/guide"}, "alice", []byte("Getting started guide"))
	dispatch(t, "write", []string{"notes/todo"}, "alice", []byte("Fix API bugs"))

	// Grep across all documents.
	r := dispatch(t, "grep", []string{"-l", "API"}, "", nil)
	out := text(r)
	if !strings.Contains(out, "docs/api") || !strings.Contains(out, "notes/todo") {
		t.Errorf("grep API missing matches: %s", out)
	}
	if strings.Contains(out, "docs/guide") {
		t.Errorf("grep API matched docs/guide unexpectedly: %s", out)
	}

	// Grep with prefix filter.
	r = dispatch(t, "grep", []string{"-l", "API", "docs/"}, "", nil)
	out = text(r)
	if !strings.Contains(out, "docs/api") {
		t.Errorf("grep API docs/ missing docs/api: %s", out)
	}
	if strings.Contains(out, "notes/todo") {
		t.Errorf("grep API docs/ matched notes/todo: %s", out)
	}

	// Find (FTS path search).
	r = dispatch(t, "find", []string{"guide"}, "", nil)
	out = text(r)
	if !strings.Contains(out, "docs/guide") {
		t.Errorf("find guide missing docs/guide: %s", out)
	}

	// Glob.
	r = dispatch(t, "glob", []string{"docs/*"}, "", nil)
	out = text(r)
	if !strings.Contains(out, "docs/api") || !strings.Contains(out, "docs/guide") {
		t.Errorf("glob docs/* missing results: %s", out)
	}
	if strings.Contains(out, "notes/todo") {
		t.Errorf("glob docs/* matched notes/todo: %s", out)
	}
}

func TestTaskWorkflow(t *testing.T) {
	testHost(t)

	// Add a task with a spec body.
	r := dispatch(t, "task", []string{"add", "Build feature"}, "alice",
		[]byte("# Build feature\n\nImplement the new feature."))
	out := text(r)
	if !strings.Contains(out, "Build feature") {
		t.Fatalf("task add response missing title: %s", out)
	}

	// Extract task key from the Result data.
	result, ok := r.(sdk.Result)
	if !ok {
		t.Fatal("task add did not return sdk.Result")
	}
	task, ok := result.Data.(*sdk.Task)
	if !ok {
		t.Fatal("task add Data is not *sdk.Task")
	}
	key := task.Key

	// List shows the task in backlog.
	r = dispatch(t, "task", []string{"list", "backlog"}, "alice", nil)
	out = text(r)
	if !strings.Contains(out, "Build feature") {
		t.Errorf("task list backlog missing task: %s", out)
	}

	// Move to in-progress.
	dispatch(t, "task", []string{"move", key, "in-progress"}, "alice", nil)

	// List in-progress shows the task.
	r = dispatch(t, "task", []string{"list", "in-progress"}, "alice", nil)
	out = text(r)
	if !strings.Contains(out, "Build feature") {
		t.Errorf("task list in-progress missing task: %s", out)
	}

	// Move to done.
	dispatch(t, "task", []string{"move", key, "done"}, "alice", nil)

	// List done shows the task.
	r = dispatch(t, "task", []string{"list", "done"}, "alice", nil)
	out = text(r)
	if !strings.Contains(out, "Build feature") {
		t.Errorf("task list done missing task: %s", out)
	}
}

func TestTagsAndLinks(t *testing.T) {
	testHost(t)

	// Write two documents.
	dispatch(t, "write", []string{"a"}, "alice", []byte("Doc A"))
	dispatch(t, "write", []string{"b"}, "alice", []byte("Doc B"))

	// Tag a document.
	dispatch(t, "tag", []string{"a", "important"}, "alice", nil)

	// List tags on document.
	r := dispatch(t, "tag", []string{"a"}, "alice", nil)
	out := text(r)
	if !strings.Contains(out, "important") {
		t.Errorf("tag a missing 'important': %s", out)
	}

	// Find by tag.
	r = dispatch(t, "tag", []string{"-f", "important"}, "alice", nil)
	out = text(r)
	if !strings.Contains(out, "a") {
		t.Errorf("tag -f important missing 'a': %s", out)
	}

	// Link two documents.
	dispatch(t, "link", []string{"a", "b"}, "alice", nil)

	// List outgoing links.
	r = dispatch(t, "link", []string{"a"}, "alice", nil)
	out = text(r)
	if !strings.Contains(out, "b") {
		t.Errorf("link a missing 'b': %s", out)
	}

	// Unlink.
	dispatch(t, "unlink", []string{"a", "b"}, "alice", nil)

	// Links now empty.
	r = dispatch(t, "link", []string{"a"}, "alice", nil)
	out = text(r)
	if strings.Contains(out, "b") {
		t.Errorf("link a still shows 'b' after unlink: %s", out)
	}
}

func TestBulkWorkflow(t *testing.T) {
	testHost(t)

	// Write documents.
	dispatch(t, "write", []string{"docs/one"}, "alice", []byte("One"))
	dispatch(t, "write", []string{"docs/two"}, "alice", []byte("Two"))

	// Export to filesystem.
	dir := t.TempDir()
	dispatch(t, "export", []string{"docs/", dir, "--overwrite"}, "", nil)

	// Verify files exist on disk. Export strips the prefix, so
	// docs/one → one.md relative to the destination directory.
	for _, name := range []string{"one.md", "two.md"} {
		p := filepath.Join(dir, name)
		if _, err := os.Stat(p); err != nil {
			t.Errorf("exported file %s not found: %v", p, err)
		}
	}

	// Delete from store.
	dispatch(t, "rm", []string{"docs/one"}, "alice", nil)
	dispatch(t, "rm", []string{"docs/two"}, "alice", nil)

	// Import from the exported directory with prefix to restore paths.
	dispatch(t, "import", []string{"--prefix", "docs/", dir}, "alice", nil)

	// Verify documents are back.
	r := dispatch(t, "cat", []string{"docs/one"}, "", nil)
	if got := text(r); got != "One" {
		t.Errorf("import docs/one = %q, want %q", got, "One")
	}
	r = dispatch(t, "cat", []string{"docs/two"}, "", nil)
	if got := text(r); got != "Two" {
		t.Errorf("import docs/two = %q, want %q", got, "Two")
	}
}

func TestFlagParsing(t *testing.T) {
	testHost(t)

	// Write documents for grep/cat tests.
	dispatch(t, "write", []string{"docs/api"}, "alice", []byte("REST API reference\nSecond line\nThird line"))
	dispatch(t, "write", []string{"docs/guide"}, "alice", []byte("Getting started guide"))
	dispatch(t, "write", []string{"notes/todo"}, "alice", []byte("Fix API bugs"))

	// Combined short bool flags: grep -nl
	r := dispatch(t, "grep", []string{"-nl", "API"}, "", nil)
	out := text(r)
	if !strings.Contains(out, "docs/api") {
		t.Errorf("grep -nl: missing docs/api: %s", out)
	}

	// Compact int with bool: grep -nC1
	r = dispatch(t, "grep", []string{"-nC1", "API"}, "", nil)
	out = text(r)
	if !strings.Contains(out, "docs/api") {
		t.Errorf("grep -nC1: missing docs/api: %s", out)
	}

	// Separate -C 1 form.
	r = dispatch(t, "grep", []string{"-n", "-C", "1", "API"}, "", nil)
	out = text(r)
	if !strings.Contains(out, "docs/api") {
		t.Errorf("grep -n -C 1: missing docs/api: %s", out)
	}

	// --key=value form for cat --version=1.
	dispatch(t, "write", []string{"docs/api"}, "alice", []byte("Updated API reference"))
	r = dispatch(t, "cat", []string{"--version=1", "docs/api"}, "", nil)
	if got := text(r); got != "REST API reference\nSecond line\nThird line" {
		t.Errorf("cat --version=1 = %q, want original content", got)
	}

	// Long flag with space: --version 1.
	r = dispatch(t, "cat", []string{"--version", "1", "docs/api"}, "", nil)
	if got := text(r); got != "REST API reference\nSecond line\nThird line" {
		t.Errorf("cat --version 1 = %q, want original content", got)
	}

	// -- terminator: everything after -- is positional.
	r = dispatch(t, "cat", []string{"--", "docs/api"}, "", nil)
	if got := text(r); got == "" {
		t.Error("cat -- docs/api returned empty")
	}
}

func TestMixedCommandAuthor(t *testing.T) {
	testHost(t)

	dispatch(t, "write", []string{"a"}, "alice", []byte("Doc A"))
	dispatch(t, "write", []string{"b"}, "alice", []byte("Doc B"))

	// Tag read operations work without author.
	dispatch(t, "tag", []string{"a", "important"}, "alice", nil)
	r := dispatch(t, "tag", []string{"a"}, "", nil)
	out := text(r)
	if !strings.Contains(out, "important") {
		t.Errorf("tag list (no author): missing 'important': %s", out)
	}

	// tag -f works without author.
	r = dispatch(t, "tag", []string{"-f", "important"}, "", nil)
	out = text(r)
	if !strings.Contains(out, "a") {
		t.Errorf("tag -f (no author): missing 'a': %s", out)
	}

	// Tag mutation without author fails.
	err := dispatchErr(t, "tag", []string{"a", "new-tag"}, "", nil)
	if err == nil {
		t.Error("tag mutation without author: expected error")
	}

	// Tag delete without author fails.
	err = dispatchErr(t, "tag", []string{"-d", "a", "important"}, "", nil)
	if err == nil {
		t.Error("tag -d without author: expected error")
	}

	// Link read works without author.
	dispatch(t, "link", []string{"a", "b"}, "alice", nil)
	r = dispatch(t, "link", []string{"a"}, "", nil)
	out = text(r)
	if !strings.Contains(out, "b") {
		t.Errorf("link list (no author): missing 'b': %s", out)
	}

	// Link creation without author fails.
	err = dispatchErr(t, "link", []string{"b", "a"}, "", nil)
	if err == nil {
		t.Error("link creation without author: expected error")
	}

	// Task read operations work without author.
	dispatch(t, "task", []string{"add", "Test task"}, "alice", []byte("# Spec\n\nDo it."))
	r = dispatch(t, "task", []string{"list"}, "", nil)
	out = text(r)
	if !strings.Contains(out, "Test task") {
		t.Errorf("task list (no author): missing task: %s", out)
	}

	// Task add without author fails.
	err = dispatchErr(t, "task", []string{"add", "No author"}, "", []byte("body"))
	if err == nil {
		t.Error("task add without author: expected error")
	}
}

func TestErrorPaths(t *testing.T) {
	testHost(t)

	// Cat nonexistent document.
	err := dispatchErr(t, "cat", []string{"nonexistent"}, "", nil)
	if !errors.Is(err, sdk.ErrNotFound) {
		t.Errorf("cat nonexistent: got %v, want ErrNotFound", err)
	}

	// Write with no path.
	err = dispatchErr(t, "write", nil, "alice", []byte("content"))
	if !errors.Is(err, sdk.ErrMissingArg) {
		t.Errorf("write no path: got %v, want ErrMissingArg", err)
	}

	// Rm nonexistent.
	err = dispatchErr(t, "rm", []string{"nonexistent"}, "alice", nil)
	if !errors.Is(err, sdk.ErrNotFound) {
		t.Errorf("rm nonexistent: got %v, want ErrNotFound", err)
	}

	// Task move with bad key.
	err = dispatchErr(t, "task", []string{"move", "bad-key", "done"}, "alice", nil)
	if !errors.Is(err, sdk.ErrNotFound) {
		t.Errorf("task move bad-key: got %v, want ErrNotFound", err)
	}

	// Unknown command.
	err = dispatchErr(t, "nonexistent-cmd", nil, "", nil)
	if !errors.Is(err, sdk.ErrUnknownCmd) {
		t.Errorf("unknown cmd: got %v, want ErrUnknownCmd", err)
	}
}
