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
	"fmt"
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
	case sdk.Markdown:
		return v.Text
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

	// Move to code.
	dispatch(t, "task", []string{"move", key, "in-progress"}, "alice", nil)

	// List code shows the task.
	r = dispatch(t, "task", []string{"list", "in-progress"}, "alice", nil)
	out = text(r)
	if !strings.Contains(out, "Build feature") {
		t.Errorf("task list code missing task: %s", out)
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

// TestGrepModes exercises grep through every mode the CLI exposes.
//
// llmd is an AI-first tool: agents are the primary user of grep, and
// the default behaviour must return bounded chunks of content (not
// whole documents) so an agent searching a long spec gets back just
// the relevant section without blowing its context window. These
// tests pin that contract.
func TestGrepModes(t *testing.T) {
	testHost(t)

	// A doc with several distinct sections so we can verify that the
	// default mode returns only the matching section, not the whole
	// document.
	dispatch(t, "write", []string{"api/spec"}, "alice", []byte(`# API Specification

## Overview

This document describes the v2 API surface.

## Authentication

We use OAuth2 with PKCE. Tokens expire after one hour.

## Errors

All endpoints return RFC 7807 problem+json.
`))

	// A short single-section doc to confirm grep still finds it.
	dispatch(t, "write", []string{"notes/auth"}, "alice", []byte(`# Auth notes

OAuth2 is the way.
`))

	t.Run("default mode returns sections", func(t *testing.T) {
		r := dispatch(t, "grep", []string{"OAuth2"}, "", nil)

		// Default grep must return sdk.Markdown so the host renders
		// it for human terminals and emits raw markdown for agents.
		md, ok := r.(sdk.Markdown)
		if !ok {
			t.Fatalf("default grep returned %T, want sdk.Markdown", r)
		}

		// The text output must contain the matching sections from
		// both documents but NOT the unrelated "Errors" section
		// from api/spec - that's the whole point of section-bounded
		// output.
		if !strings.Contains(md.Text, "## Authentication") {
			t.Errorf("missing matching section: %s", md.Text)
		}
		if !strings.Contains(md.Text, "OAuth2 with PKCE") {
			t.Errorf("missing section content: %s", md.Text)
		}
		if strings.Contains(md.Text, "RFC 7807") {
			t.Errorf("section bounding broken - leaked unrelated section: %s", md.Text)
		}
		if strings.Contains(md.Text, "## Overview") {
			t.Errorf("section bounding broken - leaked unrelated section: %s", md.Text)
		}

		// The structured Data field must always carry typed
		// GrepHit values for agents reading via --json or MCP.
		hits, ok := md.Data.([]sdk.GrepHit)
		if !ok {
			t.Fatalf("Data field = %T, want []sdk.GrepHit", md.Data)
		}
		if len(hits) == 0 {
			t.Fatal("Data has no hits")
		}
		// Section heading must be populated in section mode so the
		// agent knows which heading the match came from.
		foundAuth := false
		for _, h := range hits {
			if h.Section == "Authentication" {
				foundAuth = true
				break
			}
		}
		if !foundAuth {
			t.Errorf("no hit had Section=Authentication; got %+v", hits)
		}
	})

	t.Run("text format is path-header then content", func(t *testing.T) {
		r := dispatch(t, "grep", []string{"OAuth2"}, "", nil)
		md := r.(sdk.Markdown)

		// Each hit must lead with "path:" on its own line, then
		// the content underneath. This is the AI-first contract:
		// path is visually distinct from match content so neither
		// the agent nor the human has to guess where one ends and
		// the other begins.
		lines := strings.Split(md.Text, "\n")
		if len(lines) < 2 {
			t.Fatalf("output too short: %q", md.Text)
		}
		if !strings.HasSuffix(lines[0], ":") {
			t.Errorf("first line should be \"path:\", got %q", lines[0])
		}
	})

	t.Run("explicit --sections matches default", func(t *testing.T) {
		def := dispatch(t, "grep", []string{"OAuth2"}, "", nil)
		exp := dispatch(t, "grep", []string{"--sections", "OAuth2"}, "", nil)
		if def.(sdk.Markdown).Text != exp.(sdk.Markdown).Text {
			t.Errorf("--sections must equal the default mode")
		}
	})

	t.Run("--lines returns line snippets", func(t *testing.T) {
		r := dispatch(t, "grep", []string{"--lines", "OAuth2"}, "", nil)
		md, ok := r.(sdk.Markdown)
		if !ok {
			t.Fatalf("--lines returned %T, want sdk.Markdown", r)
		}
		// In lines mode each Match has Line set to the matching
		// line number, not 0 like sections.
		hits := md.Data.([]sdk.GrepHit)
		if len(hits) == 0 {
			t.Fatal("no hits")
		}
		// Lines mode should return individual lines, so the section
		// heading "## Authentication" must NOT be part of the
		// matched text (it's a different line from "OAuth2 with...")
		for _, h := range hits {
			if strings.Contains(h.Text, "RFC 7807") {
				t.Errorf("lines mode leaked unrelated content: %q", h.Text)
			}
		}
	})

	t.Run("--lines -n prefixes line numbers", func(t *testing.T) {
		r := dispatch(t, "grep", []string{"--lines", "-n", "OAuth2"}, "", nil)
		md := r.(sdk.Markdown)
		// -n with --lines should produce "N: text" lines under the
		// "path:" header. Verify by checking that at least one
		// content line starts with a digit followed by ":".
		out := md.Text
		hasNumberedLine := false
		for _, line := range strings.Split(out, "\n") {
			if len(line) >= 3 && line[0] >= '0' && line[0] <= '9' {
				if idx := strings.Index(line, ":"); idx > 0 && idx < 5 {
					hasNumberedLine = true
					break
				}
			}
		}
		if !hasNumberedLine {
			t.Errorf("expected numbered line in output: %q", out)
		}
	})

	t.Run("--full returns whole documents", func(t *testing.T) {
		r := dispatch(t, "grep", []string{"--full", "OAuth2"}, "", nil)
		md, ok := r.(sdk.Markdown)
		if !ok {
			t.Fatalf("--full returned %T, want sdk.Markdown", r)
		}
		// Full mode must include the unrelated sections from api/spec
		// because it returns the whole document, unlike --sections.
		if !strings.Contains(md.Text, "RFC 7807") {
			t.Errorf("--full should include unrelated sections; got %q", md.Text)
		}
		if !strings.Contains(md.Text, "Overview") {
			t.Errorf("--full should include unrelated sections; got %q", md.Text)
		}
	})

	t.Run("-l returns paths only as Result", func(t *testing.T) {
		r := dispatch(t, "grep", []string{"-l", "OAuth2"}, "", nil)
		// -l is a machine-friendly mode (xargs target), it must
		// stay as plain Result so the host never glamour-renders it.
		res, ok := r.(sdk.Result)
		if !ok {
			t.Fatalf("-l returned %T, want sdk.Result", r)
		}
		paths := strings.Split(res.Text, "\n")
		seen := make(map[string]bool)
		for _, p := range paths {
			seen[p] = true
		}
		if !seen["api/spec"] || !seen["notes/auth"] {
			t.Errorf("-l missing expected paths: %v", seen)
		}
		// No content should leak into -l output.
		if strings.Contains(res.Text, "OAuth2") {
			t.Errorf("-l leaked content: %s", res.Text)
		}
	})

	t.Run("-c returns counts as Result", func(t *testing.T) {
		r := dispatch(t, "grep", []string{"-c", "OAuth2"}, "", nil)
		res, ok := r.(sdk.Result)
		if !ok {
			t.Fatalf("-c returned %T, want sdk.Result", r)
		}
		// Each line should be "path:N".
		for _, line := range strings.Split(res.Text, "\n") {
			if !strings.Contains(line, ":") {
				t.Errorf("-c line missing colon: %q", line)
			}
		}
	})

	t.Run("no matches returns empty Markdown with empty Data", func(t *testing.T) {
		r := dispatch(t, "grep", []string{"definitelynotthere"}, "", nil)
		md, ok := r.(sdk.Markdown)
		if !ok {
			t.Fatalf("no-match returned %T, want sdk.Markdown", r)
		}
		if md.Text != "" {
			t.Errorf("no-match should have empty Text, got %q", md.Text)
		}
		hits := md.Data.([]sdk.GrepHit)
		if len(hits) != 0 {
			t.Errorf("no-match Data should be empty slice, got %d hits", len(hits))
		}
	})

	t.Run("path prefix filter narrows results", func(t *testing.T) {
		r := dispatch(t, "grep", []string{"OAuth2", "api/"}, "", nil)
		md := r.(sdk.Markdown)
		hits := md.Data.([]sdk.GrepHit)
		for _, h := range hits {
			if !strings.HasPrefix(h.Path, "api/") {
				t.Errorf("path filter leaked %s", h.Path)
			}
		}
		if len(hits) == 0 {
			t.Error("path filter dropped all results")
		}
	})
}

// TestCatSlicing exercises cat's --offset and --limit flags, the
// read half of the AI-first grep+read workflow. Agents must be able
// to fetch a bounded slice of a long document without pulling the
// whole file into their context window.
func TestCatSlicing(t *testing.T) {
	testHost(t)

	// A 20-line document so we can exercise offset/limit without
	// edge-case interactions.
	var lines []string
	for i := 1; i <= 20; i++ {
		lines = append(lines, fmt.Sprintf("line %d", i))
	}
	dispatch(t, "write", []string{"long/doc"}, "alice", []byte(strings.Join(lines, "\n")))

	t.Run("no flags returns whole document", func(t *testing.T) {
		r := dispatch(t, "cat", []string{"long/doc"}, "", nil)
		out := text(r)
		if !strings.Contains(out, "line 1") || !strings.Contains(out, "line 20") {
			t.Errorf("whole doc missing boundary lines: %q", out)
		}
	})

	t.Run("--limit caps from the top", func(t *testing.T) {
		r := dispatch(t, "cat", []string{"--limit", "5", "long/doc"}, "", nil)
		out := text(r)
		if !strings.Contains(out, "line 1") {
			t.Errorf("missing first line: %q", out)
		}
		if !strings.Contains(out, "line 5") {
			t.Errorf("missing fifth line: %q", out)
		}
		if strings.Contains(out, "line 6") {
			t.Errorf("limit leaked beyond 5 lines: %q", out)
		}
	})

	t.Run("--offset skips leading lines", func(t *testing.T) {
		r := dispatch(t, "cat", []string{"--offset", "10", "long/doc"}, "", nil)
		out := text(r)
		if strings.Contains(out, "line 9") {
			t.Errorf("offset leaked line 9: %q", out)
		}
		if !strings.Contains(out, "line 10") {
			t.Errorf("missing offset start line: %q", out)
		}
		if !strings.Contains(out, "line 20") {
			t.Errorf("missing last line: %q", out)
		}
	})

	t.Run("--offset with --limit returns a window", func(t *testing.T) {
		r := dispatch(t, "cat", []string{"--offset", "8", "--limit", "3", "long/doc"}, "", nil)
		out := text(r)
		for _, want := range []string{"line 8", "line 9", "line 10"} {
			if !strings.Contains(out, want) {
				t.Errorf("missing %q in window: %q", want, out)
			}
		}
		for _, leak := range []string{"line 7", "line 11"} {
			if strings.Contains(out, leak) {
				t.Errorf("window leaked %q: %q", leak, out)
			}
		}
	})

	t.Run("offset past end returns empty", func(t *testing.T) {
		r := dispatch(t, "cat", []string{"--offset", "9999", "long/doc"}, "", nil)
		// Empty content sliced through markdown still returns the
		// type with empty Text.
		md, ok := r.(sdk.Markdown)
		if !ok {
			t.Fatalf("expected sdk.Markdown, got %T", r)
		}
		if md.Text != "" {
			t.Errorf("expected empty slice, got %q", md.Text)
		}
	})

	t.Run("-n numbers match source line numbers when sliced", func(t *testing.T) {
		r := dispatch(t, "cat", []string{"--offset", "10", "--limit", "3", "-n", "long/doc"}, "", nil)
		out := text(r)
		// Line numbers should be 10, 11, 12 - matching the source,
		// not 1, 2, 3 as they would if numbering restarted.
		if !strings.Contains(out, "10  line 10") {
			t.Errorf("expected stable line numbers: %q", out)
		}
		if strings.Contains(out, " 1  line 10") {
			t.Errorf("line numbers restarted inside slice: %q", out)
		}
	})
}

// TestHistoryDefaults verifies the AI-first default limit on history.
// A heavily edited document would otherwise dump hundreds of rows into
// an agent's context; the default of 10 keeps it bounded, and --all
// is available for the rare case someone actually wants every version.
func TestHistoryDefaults(t *testing.T) {
	testHost(t)

	// Produce 15 versions of the same document so we can see the
	// default limit kick in and --all override it.
	for i := 1; i <= 15; i++ {
		content := fmt.Sprintf("version %d", i)
		dispatch(t, "write", []string{"churn/doc"}, "alice", []byte(content))
	}

	t.Run("default caps at 10 versions", func(t *testing.T) {
		r := dispatch(t, "history", []string{"churn/doc"}, "", nil)
		// The Data field carries the raw []sdk.Version regardless
		// of whether --json was set, so the test reads it directly
		// instead of parsing the lipgloss table text form.
		res, ok := r.(sdk.Result)
		if !ok {
			t.Fatalf("history returned %T, want sdk.Result", r)
		}
		versions, ok := res.Data.([]sdk.Version)
		if !ok {
			t.Fatalf("Data field = %T, want []sdk.Version", res.Data)
		}
		if len(versions) != 10 {
			t.Errorf("default history returned %d versions, want 10", len(versions))
		}
	})

	t.Run("-n overrides the default", func(t *testing.T) {
		r := dispatch(t, "history", []string{"-n", "3", "churn/doc"}, "", nil)
		res := r.(sdk.Result)
		versions := res.Data.([]sdk.Version)
		if len(versions) != 3 {
			t.Errorf("-n 3 returned %d versions, want 3", len(versions))
		}
	})

	t.Run("--all returns every version", func(t *testing.T) {
		r := dispatch(t, "history", []string{"--all", "churn/doc"}, "", nil)
		res := r.(sdk.Result)
		versions := res.Data.([]sdk.Version)
		if len(versions) != 15 {
			t.Errorf("--all returned %d versions, want 15", len(versions))
		}
	})
}

// TestDiffTruncation exercises the default line cap on diff. A huge
// rewrite diff would otherwise dump thousands of lines into an
// agent's context. The default cap is 500 lines with a footer
// pointing at --all for the full diff.
func TestDiffTruncation(t *testing.T) {
	testHost(t)

	// Build a version-1 doc with 1000 lines, then replace every one
	// of them in version 2 so the diff is ~2000 lines (every line
	// removed and re-added).
	var v1, v2 []string
	for i := 1; i <= 1000; i++ {
		v1 = append(v1, fmt.Sprintf("old line %d", i))
		v2 = append(v2, fmt.Sprintf("new line %d", i))
	}
	dispatch(t, "write", []string{"big/doc"}, "alice", []byte(strings.Join(v1, "\n")))
	dispatch(t, "write", []string{"big/doc"}, "alice", []byte(strings.Join(v2, "\n")))

	t.Run("default caps at 500 lines with footer", func(t *testing.T) {
		r := dispatch(t, "diff", []string{"big/doc"}, "", nil)
		out := text(r)
		// Footer explicitly mentions truncation and --all.
		if !strings.Contains(out, "truncated") {
			t.Errorf("expected truncation footer, got %d bytes: %.200q", len(out), out)
		}
		if !strings.Contains(out, "--all") {
			t.Errorf("footer should mention --all: %.200q", out)
		}
		// Line count must not exceed cap + a few lines for the
		// footer.
		lineCount := strings.Count(out, "\n")
		if lineCount > 510 {
			t.Errorf("diff exceeded cap: got %d lines, want <= ~510", lineCount)
		}
	})

	t.Run("--all returns the full diff", func(t *testing.T) {
		r := dispatch(t, "diff", []string{"--all", "big/doc"}, "", nil)
		out := text(r)
		if strings.Contains(out, "truncated") {
			t.Errorf("--all should not truncate: %.200q", out)
		}
		lineCount := strings.Count(out, "\n")
		if lineCount < 1000 {
			t.Errorf("--all diff too short: got %d lines", lineCount)
		}
	})

	t.Run("small diff is unchanged", func(t *testing.T) {
		dispatch(t, "write", []string{"small/doc"}, "alice", []byte("v1"))
		dispatch(t, "write", []string{"small/doc"}, "alice", []byte("v2"))
		r := dispatch(t, "diff", []string{"small/doc"}, "", nil)
		out := text(r)
		if strings.Contains(out, "truncated") {
			t.Errorf("small diff should not be truncated: %s", out)
		}
	})
}

// TestQueueHistoryDefault verifies the default cap on queue history
// so an agent pulling recent activity off a busy queue doesn't drown
// in thousands of old messages.
func TestQueueHistoryDefault(t *testing.T) {
	testHost(t)

	// Send 30 messages as alice so she's the consumer in history.
	for i := 1; i <= 30; i++ {
		dispatch(t, "queue", []string{"send", fmt.Sprintf("msg %d", i)}, "alice", nil)
	}

	t.Run("defaults to last 20 messages", func(t *testing.T) {
		r := dispatch(t, "queue", []string{"history"}, "alice", nil)
		res := r.(sdk.Result)
		msgs := res.Data.([]sdk.Message)
		if len(msgs) != 20 {
			t.Errorf("default queue history returned %d messages, want 20", len(msgs))
		}
	})

	t.Run("-n overrides the default", func(t *testing.T) {
		r := dispatch(t, "queue", []string{"history", "-n", "5"}, "alice", nil)
		res := r.(sdk.Result)
		msgs := res.Data.([]sdk.Message)
		if len(msgs) != 5 {
			t.Errorf("-n 5 returned %d messages, want 5", len(msgs))
		}
	})

	t.Run("--all returns every message", func(t *testing.T) {
		r := dispatch(t, "queue", []string{"history", "--all"}, "alice", nil)
		res := r.(sdk.Result)
		msgs := res.Data.([]sdk.Message)
		if len(msgs) != 30 {
			t.Errorf("--all returned %d messages, want 30", len(msgs))
		}
	})
}

// TestAuditShowTruncation exercises the cap on audit show. Long
// audit threads are common on code review cycles and would otherwise
// dump every back-and-forth into an agent's context. Default caps
// at 10 (root + last 9) with --all for the full thread.
func TestAuditShowTruncation(t *testing.T) {
	testHost(t)

	// Seed a document and create an audit on it.
	dispatch(t, "write", []string{"reviewed/doc"}, "alice", []byte("original content"))
	r := dispatch(t, "audit", []string{"add", "reviewed/doc", "Please review this."}, "alice", nil)
	rootID := r.(sdk.Result).Data.(*sdk.Audit).ID

	// Add 15 replies so the thread has 16 messages total.
	for i := 1; i <= 15; i++ {
		dispatch(t, "audit",
			[]string{"reply", rootID, fmt.Sprintf("reply %d", i)},
			"alice", nil)
	}

	t.Run("default shows root plus last 9 with gap notice", func(t *testing.T) {
		r := dispatch(t, "audit", []string{"show", rootID}, "alice", nil)
		res := r.(sdk.Result)
		thread := res.Data.([]sdk.Audit)
		if len(thread) != 10 {
			t.Errorf("default audit show returned %d messages, want 10", len(thread))
		}

		out := res.Text
		if !strings.Contains(out, "hidden") {
			t.Errorf("expected gap notice about hidden messages: %s", out)
		}
		if !strings.Contains(out, "Please review this.") {
			t.Errorf("root message should still be visible: %s", out)
		}
		// The last reply must be present - this is the "current
		// state" view the user actually wants.
		if !strings.Contains(out, "reply 15") {
			t.Errorf("latest reply missing: %s", out)
		}
	})

	t.Run("--all shows the full thread", func(t *testing.T) {
		r := dispatch(t, "audit", []string{"show", "--all", rootID}, "alice", nil)
		res := r.(sdk.Result)
		thread := res.Data.([]sdk.Audit)
		if len(thread) != 16 {
			t.Errorf("--all returned %d messages, want 16", len(thread))
		}
		if strings.Contains(res.Text, "hidden") {
			t.Errorf("--all should not show gap notice: %s", res.Text)
		}
	})

	t.Run("short threads are unchanged", func(t *testing.T) {
		dispatch(t, "write", []string{"short/doc"}, "alice", []byte("short"))
		r := dispatch(t, "audit", []string{"add", "short/doc", "Quick check."}, "alice", nil)
		shortID := r.(sdk.Result).Data.(*sdk.Audit).ID
		dispatch(t, "audit", []string{"reply", shortID, "Looks fine."}, "alice", nil)

		r = dispatch(t, "audit", []string{"show", shortID}, "alice", nil)
		res := r.(sdk.Result)
		if strings.Contains(res.Text, "hidden") {
			t.Errorf("short thread should not show gap notice: %s", res.Text)
		}
	})
}

// TestListCapping verifies the default 500-item cap on ls, find,
// and glob. A large store should not dump the full catalogue into an
// agent's context just because someone asked "what's in here?".
func TestListCapping(t *testing.T) {
	testHost(t)

	// Seed 600 documents so the default cap visibly kicks in.
	for i := 1; i <= 600; i++ {
		dispatch(t, "write",
			[]string{fmt.Sprintf("bulk/doc-%04d", i)},
			"alice", []byte("x"))
	}

	t.Run("ls caps at default 500", func(t *testing.T) {
		r := dispatch(t, "ls", []string{"bulk/"}, "", nil)
		out := text(r)
		count := strings.Count(out, "\n") + 1
		if count > 500 {
			t.Errorf("ls returned %d rows, want <= 500", count)
		}
		if count < 500 {
			t.Errorf("ls returned %d rows, want 500", count)
		}
	})

	t.Run("ls --all returns every document", func(t *testing.T) {
		r := dispatch(t, "ls", []string{"--all", "bulk/"}, "", nil)
		out := text(r)
		count := strings.Count(out, "\n") + 1
		if count != 600 {
			t.Errorf("ls --all returned %d rows, want 600", count)
		}
	})

	t.Run("ls --limit overrides the default", func(t *testing.T) {
		r := dispatch(t, "ls", []string{"--limit", "25", "bulk/"}, "", nil)
		out := text(r)
		count := strings.Count(out, "\n") + 1
		if count != 25 {
			t.Errorf("ls --limit 25 returned %d rows, want 25", count)
		}
	})

	t.Run("glob caps at default 500", func(t *testing.T) {
		r := dispatch(t, "glob", []string{"bulk/*"}, "", nil)
		res := r.(sdk.Result)
		paths := res.Data.([]string)
		if len(paths) != 500 {
			t.Errorf("glob returned %d paths, want 500", len(paths))
		}
	})

	t.Run("glob --all returns every match", func(t *testing.T) {
		r := dispatch(t, "glob", []string{"--all", "bulk/*"}, "", nil)
		res := r.(sdk.Result)
		paths := res.Data.([]string)
		if len(paths) != 600 {
			t.Errorf("glob --all returned %d paths, want 600", len(paths))
		}
	})
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
