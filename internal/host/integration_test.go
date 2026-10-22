// integration_test.go exercises multi-step workflows through the host's
// Dispatch, the same entry point used by the CLI and MCP server. Each test chains
// several commands and verifies the result at each step.
//
// These complement the per-domain unit tests in api_test.go and
// api_tasks_test.go which call the store bridges directly. Integration tests
// catch issues in argument parsing, flag handling, response formatting,
// and cross-domain interactions.
package host

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	// Blank import registers the CLI extension so the host can route
	// commands. Same pattern as main.go.
	_ "github.com/jpl-au/llmd/cli"
	"github.com/jpl-au/llmd/sdk"
)

// dispatch runs a command through the host and fails the test on error.
func dispatch(t *testing.T, c sdk.Context, cmd string, args []string, author string, stdin []byte) sdk.Response {
	t.Helper()
	r, err := c.Runtime.Dispatch(t.Context(), cmd, args, author, stdin, "")
	if err != nil {
		t.Fatalf("dispatch %s %v: %v", cmd, args, err)
	}
	return r
}

// dispatchErr runs a command through the host and returns the error.
func dispatchErr(t *testing.T, c sdk.Context, cmd string, args []string, author string, stdin []byte) error {
	t.Helper()
	_, err := c.Runtime.Dispatch(t.Context(), cmd, args, author, stdin, "")
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
	c := testHost(t)

	// Write two documents.
	dispatch(t, c, "write", []string{"notes/hello"}, "alice", []byte("# Hello"))
	dispatch(t, c, "write", []string{"notes/world"}, "alice", []byte("# World"))

	// Cat reads content back.
	r := dispatch(t, c, "cat", []string{"notes/hello"}, "", nil)
	if got := text(r); got != "# Hello" {
		t.Errorf("cat notes/hello = %q, want %q", got, "# Hello")
	}

	// Ls lists both documents.
	r = dispatch(t, c, "ls", nil, "", nil)
	out := text(r)
	if !strings.Contains(out, "notes/hello") || !strings.Contains(out, "notes/world") {
		t.Errorf("ls missing documents: %s", out)
	}

	// Ls with prefix.
	r = dispatch(t, c, "ls", []string{"notes/"}, "", nil)
	out = text(r)
	if !strings.Contains(out, "notes/hello") || !strings.Contains(out, "notes/world") {
		t.Errorf("ls notes/ missing documents: %s", out)
	}

	// Rm soft-deletes.
	dispatch(t, c, "rm", []string{"notes/hello"}, "alice", nil)

	// Cat on deleted document fails.
	err := dispatchErr(t, c, "cat", []string{"notes/hello"}, "", nil)
	if err == nil {
		t.Error("cat deleted doc: expected error, got nil")
	}

	// Ls no longer shows deleted document.
	r = dispatch(t, c, "ls", nil, "", nil)
	out = text(r)
	if strings.Contains(out, "notes/hello") {
		t.Errorf("ls still shows deleted doc: %s", out)
	}

	// Restore brings it back.
	dispatch(t, c, "restore", []string{"notes/hello"}, "alice", nil)

	// Cat works again.
	r = dispatch(t, c, "cat", []string{"notes/hello"}, "", nil)
	if got := text(r); got != "# Hello" {
		t.Errorf("cat after restore = %q, want %q", got, "# Hello")
	}
}

func TestEditWorkflow(t *testing.T) {
	c := testHost(t)

	// Write initial content.
	dispatch(t, c, "write", []string{"doc"}, "alice", []byte("hello world"))

	// Sed substitution.
	dispatch(t, c, "sed", []string{"s/hello/goodbye/", "doc"}, "alice", nil)
	r := dispatch(t, c, "cat", []string{"doc"}, "", nil)
	if got := text(r); got != "goodbye world" {
		t.Errorf("after sed = %q, want %q", got, "goodbye world")
	}

	// Edit (search-and-replace).
	dispatch(t, c, "edit", []string{"doc", "goodbye", "farewell"}, "alice", nil)
	r = dispatch(t, c, "cat", []string{"doc"}, "", nil)
	if got := text(r); got != "farewell world" {
		t.Errorf("after edit = %q, want %q", got, "farewell world")
	}

	// History shows 3 versions.
	r = dispatch(t, c, "history", []string{"doc"}, "", nil)
	out := text(r)
	// History output is a table with version numbers - check for v1, v2, v3.
	if !strings.Contains(out, "1") || !strings.Contains(out, "3") {
		t.Errorf("history missing versions: %s", out)
	}

	// Diff shows change.
	r = dispatch(t, c, "diff", []string{"doc"}, "", nil)
	out = text(r)
	if out == "" {
		t.Error("diff returned empty output")
	}

	// Revert to version 1.
	dispatch(t, c, "revert", []string{"doc", "1"}, "alice", nil)
	r = dispatch(t, c, "cat", []string{"doc"}, "", nil)
	if got := text(r); got != "hello world" {
		t.Errorf("after revert = %q, want %q", got, "hello world")
	}
}

func TestSearchWorkflow(t *testing.T) {
	c := testHost(t)

	// Write several documents.
	dispatch(t, c, "write", []string{"docs/api"}, "alice", []byte("REST API reference"))
	dispatch(t, c, "write", []string{"docs/guide"}, "alice", []byte("Getting started guide"))
	dispatch(t, c, "write", []string{"notes/todo"}, "alice", []byte("Fix API bugs"))

	// Grep across all documents.
	r := dispatch(t, c, "grep", []string{"-l", "API"}, "", nil)
	out := text(r)
	if !strings.Contains(out, "docs/api") || !strings.Contains(out, "notes/todo") {
		t.Errorf("grep API missing matches: %s", out)
	}
	if strings.Contains(out, "docs/guide") {
		t.Errorf("grep API matched docs/guide unexpectedly: %s", out)
	}

	// Grep with prefix filter.
	r = dispatch(t, c, "grep", []string{"-l", "API", "docs/"}, "", nil)
	out = text(r)
	if !strings.Contains(out, "docs/api") {
		t.Errorf("grep API docs/ missing docs/api: %s", out)
	}
	if strings.Contains(out, "notes/todo") {
		t.Errorf("grep API docs/ matched notes/todo: %s", out)
	}

	// Find (FTS path search).
	r = dispatch(t, c, "find", []string{"guide"}, "", nil)
	out = text(r)
	if !strings.Contains(out, "docs/guide") {
		t.Errorf("find guide missing docs/guide: %s", out)
	}

	// Glob.
	r = dispatch(t, c, "glob", []string{"docs/*"}, "", nil)
	out = text(r)
	if !strings.Contains(out, "docs/api") || !strings.Contains(out, "docs/guide") {
		t.Errorf("glob docs/* missing results: %s", out)
	}
	if strings.Contains(out, "notes/todo") {
		t.Errorf("glob docs/* matched notes/todo: %s", out)
	}
}

func TestTaskWorkflow(t *testing.T) {
	c := testHost(t)

	// Add a task with a spec body.
	r := dispatch(t, c, "task", []string{"add", "Build feature"}, "alice",
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
	r = dispatch(t, c, "task", []string{"list", "backlog"}, "alice", nil)
	out = text(r)
	if !strings.Contains(out, "Build feature") {
		t.Errorf("task list backlog missing task: %s", out)
	}

	// Move to code.
	dispatch(t, c, "task", []string{"move", key, "in-progress"}, "alice", nil)

	// List code shows the task.
	r = dispatch(t, c, "task", []string{"list", "in-progress"}, "alice", nil)
	out = text(r)
	if !strings.Contains(out, "Build feature") {
		t.Errorf("task list code missing task: %s", out)
	}

	// Move to done.
	dispatch(t, c, "task", []string{"move", key, "done"}, "alice", nil)

	// List done shows the task.
	r = dispatch(t, c, "task", []string{"list", "done"}, "alice", nil)
	out = text(r)
	if !strings.Contains(out, "Build feature") {
		t.Errorf("task list done missing task: %s", out)
	}
}

func TestTagsAndLinks(t *testing.T) {
	c := testHost(t)

	// Write two documents.
	dispatch(t, c, "write", []string{"a"}, "alice", []byte("Doc A"))
	dispatch(t, c, "write", []string{"b"}, "alice", []byte("Doc B"))

	// Tag a document.
	dispatch(t, c, "tag", []string{"a", "important"}, "alice", nil)

	// List tags on document.
	r := dispatch(t, c, "tag", []string{"a"}, "alice", nil)
	out := text(r)
	if !strings.Contains(out, "important") {
		t.Errorf("tag a missing 'important': %s", out)
	}

	// Find by tag.
	r = dispatch(t, c, "tag", []string{"-f", "important"}, "alice", nil)
	out = text(r)
	if !strings.Contains(out, "a") {
		t.Errorf("tag -f important missing 'a': %s", out)
	}

	// Link two documents.
	dispatch(t, c, "link", []string{"a", "b"}, "alice", nil)

	// List outgoing links.
	r = dispatch(t, c, "link", []string{"a"}, "alice", nil)
	out = text(r)
	if !strings.Contains(out, "b") {
		t.Errorf("link a missing 'b': %s", out)
	}

	// Unlink.
	dispatch(t, c, "unlink", []string{"a", "b"}, "alice", nil)

	// Links now empty.
	r = dispatch(t, c, "link", []string{"a"}, "alice", nil)
	out = text(r)
	if strings.Contains(out, "b") {
		t.Errorf("link a still shows 'b' after unlink: %s", out)
	}
}

func TestBulkWorkflow(t *testing.T) {
	c := testHost(t)

	// Write documents.
	dispatch(t, c, "write", []string{"docs/one"}, "alice", []byte("One"))
	dispatch(t, c, "write", []string{"docs/two"}, "alice", []byte("Two"))

	// Export to filesystem.
	dir := t.TempDir()
	dispatch(t, c, "export", []string{"docs/", dir, "--overwrite"}, "", nil)

	// Verify files exist on disk. Export strips the prefix, so
	// docs/one → one.md relative to the destination directory.
	for _, name := range []string{"one.md", "two.md"} {
		p := filepath.Join(dir, name)
		if _, err := os.Stat(p); err != nil {
			t.Errorf("exported file %s not found: %v", p, err)
		}
	}

	// Delete from store.
	dispatch(t, c, "rm", []string{"docs/one"}, "alice", nil)
	dispatch(t, c, "rm", []string{"docs/two"}, "alice", nil)

	// Import from the exported directory with prefix to restore paths.
	dispatch(t, c, "import", []string{"--prefix", "docs/", dir}, "alice", nil)

	// Verify documents are back.
	r := dispatch(t, c, "cat", []string{"docs/one"}, "", nil)
	if got := text(r); got != "One" {
		t.Errorf("import docs/one = %q, want %q", got, "One")
	}
	r = dispatch(t, c, "cat", []string{"docs/two"}, "", nil)
	if got := text(r); got != "Two" {
		t.Errorf("import docs/two = %q, want %q", got, "Two")
	}
}

func TestFlagParsing(t *testing.T) {
	c := testHost(t)

	// Write documents for grep/cat tests.
	dispatch(t, c, "write", []string{"docs/api"}, "alice", []byte("REST API reference\nSecond line\nThird line"))
	dispatch(t, c, "write", []string{"docs/guide"}, "alice", []byte("Getting started guide"))
	dispatch(t, c, "write", []string{"notes/todo"}, "alice", []byte("Fix API bugs"))

	// Combined short bool flags: grep -nl
	r := dispatch(t, c, "grep", []string{"-nl", "API"}, "", nil)
	out := text(r)
	if !strings.Contains(out, "docs/api") {
		t.Errorf("grep -nl: missing docs/api: %s", out)
	}

	// Compact int with bool: grep -nC1
	r = dispatch(t, c, "grep", []string{"-nC1", "API"}, "", nil)
	out = text(r)
	if !strings.Contains(out, "docs/api") {
		t.Errorf("grep -nC1: missing docs/api: %s", out)
	}

	// Separate -C 1 form.
	r = dispatch(t, c, "grep", []string{"-n", "-C", "1", "API"}, "", nil)
	out = text(r)
	if !strings.Contains(out, "docs/api") {
		t.Errorf("grep -n -C 1: missing docs/api: %s", out)
	}

	// --key=value form for cat --version=1.
	dispatch(t, c, "write", []string{"docs/api"}, "alice", []byte("Updated API reference"))
	r = dispatch(t, c, "cat", []string{"--version=1", "docs/api"}, "", nil)
	if got := text(r); got != "REST API reference\nSecond line\nThird line" {
		t.Errorf("cat --version=1 = %q, want original content", got)
	}

	// Long flag with space: --version 1.
	r = dispatch(t, c, "cat", []string{"--version", "1", "docs/api"}, "", nil)
	if got := text(r); got != "REST API reference\nSecond line\nThird line" {
		t.Errorf("cat --version 1 = %q, want original content", got)
	}

	// -- terminator: everything after -- is positional.
	r = dispatch(t, c, "cat", []string{"--", "docs/api"}, "", nil)
	if got := text(r); got == "" {
		t.Error("cat -- docs/api returned empty")
	}
}

func TestMixedCommandAuthor(t *testing.T) {
	c := testHost(t)

	dispatch(t, c, "write", []string{"a"}, "alice", []byte("Doc A"))
	dispatch(t, c, "write", []string{"b"}, "alice", []byte("Doc B"))

	// Tag read operations work without author.
	dispatch(t, c, "tag", []string{"a", "important"}, "alice", nil)
	r := dispatch(t, c, "tag", []string{"a"}, "", nil)
	out := text(r)
	if !strings.Contains(out, "important") {
		t.Errorf("tag list (no author): missing 'important': %s", out)
	}

	// tag -f works without author.
	r = dispatch(t, c, "tag", []string{"-f", "important"}, "", nil)
	out = text(r)
	if !strings.Contains(out, "a") {
		t.Errorf("tag -f (no author): missing 'a': %s", out)
	}

	// Tag mutation without author fails.
	err := dispatchErr(t, c, "tag", []string{"a", "new-tag"}, "", nil)
	if err == nil {
		t.Error("tag mutation without author: expected error")
	}

	// Tag delete without author fails.
	err = dispatchErr(t, c, "tag", []string{"-d", "a", "important"}, "", nil)
	if err == nil {
		t.Error("tag -d without author: expected error")
	}

	// Link read works without author.
	dispatch(t, c, "link", []string{"a", "b"}, "alice", nil)
	r = dispatch(t, c, "link", []string{"a"}, "", nil)
	out = text(r)
	if !strings.Contains(out, "b") {
		t.Errorf("link list (no author): missing 'b': %s", out)
	}

	// Link creation without author fails.
	err = dispatchErr(t, c, "link", []string{"b", "a"}, "", nil)
	if err == nil {
		t.Error("link creation without author: expected error")
	}

	// Task read operations work without author.
	dispatch(t, c, "task", []string{"add", "Test task"}, "alice", []byte("# Spec\n\nDo it."))
	r = dispatch(t, c, "task", []string{"list"}, "", nil)
	out = text(r)
	if !strings.Contains(out, "Test task") {
		t.Errorf("task list (no author): missing task: %s", out)
	}

	// Task add without author fails.
	err = dispatchErr(t, c, "task", []string{"add", "No author"}, "", []byte("body"))
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
	c := testHost(t)

	// A doc with several distinct sections so we can verify that the
	// default mode returns only the matching section, not the whole
	// document.
	dispatch(t, c, "write", []string{"api/spec"}, "alice", []byte(`# API Specification

## Overview

This document describes the v2 API surface.

## Authentication

We use OAuth2 with PKCE. Tokens expire after one hour.

## Errors

All endpoints return RFC 7807 problem+json.
`))

	// A short single-section doc to confirm grep still finds it.
	dispatch(t, c, "write", []string{"notes/auth"}, "alice", []byte(`# Auth notes

OAuth2 is the way.
`))

	t.Run("default mode returns sections", func(t *testing.T) {
		r := dispatch(t, c, "grep", []string{"OAuth2"}, "", nil)

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
		r := dispatch(t, c, "grep", []string{"OAuth2"}, "", nil)
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
		def := dispatch(t, c, "grep", []string{"OAuth2"}, "", nil)
		exp := dispatch(t, c, "grep", []string{"--sections", "OAuth2"}, "", nil)
		if def.(sdk.Markdown).Text != exp.(sdk.Markdown).Text {
			t.Errorf("--sections must equal the default mode")
		}
	})

	t.Run("--lines returns line snippets", func(t *testing.T) {
		r := dispatch(t, c, "grep", []string{"--lines", "OAuth2"}, "", nil)
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
		r := dispatch(t, c, "grep", []string{"--lines", "-n", "OAuth2"}, "", nil)
		md := r.(sdk.Markdown)
		// -n with --lines should produce "N: text" lines under the
		// "path:" header. Verify by checking that at least one
		// content line starts with a digit followed by ":".
		out := md.Text
		hasNumberedLine := false
		for line := range strings.SplitSeq(out, "\n") {
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
		r := dispatch(t, c, "grep", []string{"--full", "OAuth2"}, "", nil)
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
		r := dispatch(t, c, "grep", []string{"-l", "OAuth2"}, "", nil)
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
		r := dispatch(t, c, "grep", []string{"-c", "OAuth2"}, "", nil)
		res, ok := r.(sdk.Result)
		if !ok {
			t.Fatalf("-c returned %T, want sdk.Result", r)
		}
		// Each line should be "path:N".
		for line := range strings.SplitSeq(res.Text, "\n") {
			if !strings.Contains(line, ":") {
				t.Errorf("-c line missing colon: %q", line)
			}
		}
	})

	t.Run("no matches returns empty Markdown with empty Data", func(t *testing.T) {
		r := dispatch(t, c, "grep", []string{"definitelynotthere"}, "", nil)
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
		r := dispatch(t, c, "grep", []string{"OAuth2", "api/"}, "", nil)
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
	c := testHost(t)

	// A 20-line document so we can exercise offset/limit without
	// edge-case interactions.
	var lines []string
	for i := 1; i <= 20; i++ {
		lines = append(lines, fmt.Sprintf("line %d", i))
	}
	dispatch(t, c, "write", []string{"long/doc"}, "alice", []byte(strings.Join(lines, "\n")))

	t.Run("no flags returns whole document", func(t *testing.T) {
		r := dispatch(t, c, "cat", []string{"long/doc"}, "", nil)
		out := text(r)
		if !strings.Contains(out, "line 1") || !strings.Contains(out, "line 20") {
			t.Errorf("whole doc missing boundary lines: %q", out)
		}
	})

	t.Run("--limit caps from the top", func(t *testing.T) {
		r := dispatch(t, c, "cat", []string{"--limit", "5", "long/doc"}, "", nil)
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
		r := dispatch(t, c, "cat", []string{"--offset", "10", "long/doc"}, "", nil)
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
		r := dispatch(t, c, "cat", []string{"--offset", "8", "--limit", "3", "long/doc"}, "", nil)
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
		r := dispatch(t, c, "cat", []string{"--offset", "9999", "long/doc"}, "", nil)
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
		r := dispatch(t, c, "cat", []string{"--offset", "10", "--limit", "3", "-n", "long/doc"}, "", nil)
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
//
// Each case produces 15 versions of the same document so the default
// limit kicks in, then reads the raw []sdk.Version from the Data
// field (carried regardless of --json) instead of parsing the
// lipgloss table text form.
func TestHistoryDefaults(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want int
	}{
		{"default caps at 10 versions", []string{"churn/doc"}, 10},
		{"-n overrides the default", []string{"-n", "3", "churn/doc"}, 3},
		{"--all returns every version", []string{"--all", "churn/doc"}, 15},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := testHost(t)
			for i := 1; i <= 15; i++ {
				dispatch(t, c, "write", []string{"churn/doc"}, "alice", []byte(fmt.Sprintf("version %d", i)))
			}
			r := dispatch(t, c, "history", tt.args, "", nil)
			res, ok := r.(sdk.Result)
			if !ok {
				t.Fatalf("history %v returned %T, want sdk.Result", tt.args, r)
			}
			versions, ok := res.Data.([]sdk.Version)
			if !ok {
				t.Fatalf("history %v Data = %T, want []sdk.Version", tt.args, res.Data)
			}
			if len(versions) != tt.want {
				t.Errorf("history %v returned %d versions, want %d", tt.args, len(versions), tt.want)
			}
		})
	}
}

// TestDiffTruncation exercises the default line cap on diff. A huge
// rewrite diff would otherwise dump thousands of lines into an
// agent's context. The default cap is 500 lines with a footer
// pointing at --all for the full diff.
func TestDiffTruncation(t *testing.T) {
	c := testHost(t)

	// Build a version-1 doc with 1000 lines, then replace every one
	// of them in version 2 so the diff is ~2000 lines (every line
	// removed and re-added).
	var v1, v2 []string
	for i := 1; i <= 1000; i++ {
		v1 = append(v1, fmt.Sprintf("old line %d", i))
		v2 = append(v2, fmt.Sprintf("new line %d", i))
	}
	dispatch(t, c, "write", []string{"big/doc"}, "alice", []byte(strings.Join(v1, "\n")))
	dispatch(t, c, "write", []string{"big/doc"}, "alice", []byte(strings.Join(v2, "\n")))

	t.Run("default caps at 500 lines with footer", func(t *testing.T) {
		r := dispatch(t, c, "diff", []string{"big/doc"}, "", nil)
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
		r := dispatch(t, c, "diff", []string{"--all", "big/doc"}, "", nil)
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
		dispatch(t, c, "write", []string{"small/doc"}, "alice", []byte("v1"))
		dispatch(t, c, "write", []string{"small/doc"}, "alice", []byte("v2"))
		r := dispatch(t, c, "diff", []string{"small/doc"}, "", nil)
		out := text(r)
		if strings.Contains(out, "truncated") {
			t.Errorf("small diff should not be truncated: %s", out)
		}
	})
}

// TestQueueHistoryDefault verifies the default cap on queue history
// so an agent pulling recent activity off a busy queue doesn't drown
// in thousands of old messages. Each case sends 30 messages as alice
// so she's the consumer in history.
func TestQueueHistoryDefault(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want int
	}{
		{"defaults to last 20 messages", []string{"history"}, 20},
		{"-n overrides the default", []string{"history", "-n", "5"}, 5},
		{"--all returns every message", []string{"history", "--all"}, 30},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := testHost(t)
			for i := 1; i <= 30; i++ {
				dispatch(t, c, "queue", []string{"send", fmt.Sprintf("msg %d", i)}, "alice", nil)
			}
			r := dispatch(t, c, "queue", tt.args, "alice", nil)
			res, ok := r.(sdk.Result)
			if !ok {
				t.Fatalf("queue %v returned %T, want sdk.Result", tt.args, r)
			}
			msgs, ok := res.Data.([]sdk.Message)
			if !ok {
				t.Fatalf("queue %v Data = %T, want []sdk.Message", tt.args, res.Data)
			}
			if len(msgs) != tt.want {
				t.Errorf("queue %v returned %d messages, want %d", tt.args, len(msgs), tt.want)
			}
		})
	}
}

// TestAuditShowTruncation exercises the cap on audit show. Long
// audit threads are common on code review cycles and would otherwise
// dump every back-and-forth into an agent's context. Default caps
// at 10 (root + last 9) with --all for the full thread.
func TestAuditShowTruncation(t *testing.T) {
	c := testHost(t)

	// Seed a document and create an audit on it.
	dispatch(t, c, "write", []string{"reviewed/doc"}, "alice", []byte("original content"))
	r := dispatch(t, c, "audit", []string{"add", "reviewed/doc", "Please review this."}, "alice", nil)
	rootID := r.(sdk.Result).Data.(*sdk.Audit).ID

	// Add 15 replies so the thread has 16 messages total.
	for i := 1; i <= 15; i++ {
		dispatch(t, c, "audit",
			[]string{"reply", rootID, fmt.Sprintf("reply %d", i)},
			"alice", nil)
	}

	t.Run("default shows root plus last 9 with gap notice", func(t *testing.T) {
		r := dispatch(t, c, "audit", []string{"show", rootID}, "alice", nil)
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
		r := dispatch(t, c, "audit", []string{"show", "--all", rootID}, "alice", nil)
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
		dispatch(t, c, "write", []string{"short/doc"}, "alice", []byte("short"))
		r := dispatch(t, c, "audit", []string{"add", "short/doc", "Quick check."}, "alice", nil)
		shortID := r.(sdk.Result).Data.(*sdk.Audit).ID
		dispatch(t, c, "audit", []string{"reply", shortID, "Looks fine."}, "alice", nil)

		r = dispatch(t, c, "audit", []string{"show", shortID}, "alice", nil)
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
	c := testHost(t)

	// Seed 600 documents so the default cap visibly kicks in.
	for i := 1; i <= 600; i++ {
		dispatch(t, c, "write",
			[]string{fmt.Sprintf("bulk/doc-%04d", i)},
			"alice", []byte("x"))
	}

	t.Run("ls caps at default 500", func(t *testing.T) {
		r := dispatch(t, c, "ls", []string{"bulk/"}, "", nil)
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
		r := dispatch(t, c, "ls", []string{"--all", "bulk/"}, "", nil)
		out := text(r)
		count := strings.Count(out, "\n") + 1
		if count != 600 {
			t.Errorf("ls --all returned %d rows, want 600", count)
		}
	})

	t.Run("ls --limit overrides the default", func(t *testing.T) {
		r := dispatch(t, c, "ls", []string{"--limit", "25", "bulk/"}, "", nil)
		out := text(r)
		count := strings.Count(out, "\n") + 1
		if count != 25 {
			t.Errorf("ls --limit 25 returned %d rows, want 25", count)
		}
	})

	t.Run("glob caps at default 500", func(t *testing.T) {
		r := dispatch(t, c, "glob", []string{"bulk/*"}, "", nil)
		res := r.(sdk.Result)
		paths := res.Data.([]string)
		if len(paths) != 500 {
			t.Errorf("glob returned %d paths, want 500", len(paths))
		}
	})

	t.Run("glob --all returns every match", func(t *testing.T) {
		r := dispatch(t, c, "glob", []string{"--all", "bulk/*"}, "", nil)
		res := r.(sdk.Result)
		paths := res.Data.([]string)
		if len(paths) != 600 {
			t.Errorf("glob --all returned %d paths, want 600", len(paths))
		}
	})
}

// TestErrorPaths checks that failures surface through Dispatch as
// the matching SDK sentinel error.
func TestErrorPaths(t *testing.T) {
	tests := []struct {
		name    string
		cmd     string
		args    []string
		author  string
		stdin   []byte
		wantErr error
	}{
		{"cat nonexistent", "cat", []string{"nonexistent"}, "", nil, sdk.ErrNotFound},
		{"write with no path", "write", nil, "alice", []byte("content"), sdk.ErrMissingArg},
		{"rm nonexistent", "rm", []string{"nonexistent"}, "alice", nil, sdk.ErrNotFound},
		{"task move bad key", "task", []string{"move", "bad-key", "done"}, "alice", nil, sdk.ErrNotFound},
		{"unknown command", "nonexistent-cmd", nil, "", nil, sdk.ErrUnknownCmd},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := testHost(t)
			err := dispatchErr(t, c, tt.cmd, tt.args, tt.author, tt.stdin)
			if !errors.Is(err, tt.wantErr) {
				t.Errorf("dispatch %s %v error = %v, want %v", tt.cmd, tt.args, err, tt.wantErr)
			}
		})
	}
}
