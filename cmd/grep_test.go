package cmd

import (
	"strings"
	"testing"
)

const apiDoc = `# API Reference

## Authentication

All API requests require authentication via JWT tokens.
Include the token in the Authorisation header.

## Error Handling

The API returns standard HTTP error codes:
- 400: Bad Request - invalid parameters
- 401: Unauthorised - missing or invalid token
- 403: Forbidden - insufficient permissions
- 404: Not Found - resource does not exist
- 500: Internal Server Error - server-side error

## Rate Limiting

Requests are limited to 100 per minute per API key.
`

const notesDoc = `# Meeting Notes - 2024-01-15

## Attendees
- Alice (Engineering)
- Bob (Product)
- Charlie (Design)

## Discussion

Discussed the authentication flow for the new API.
Bob raised concerns about error handling UX.
Charlie will design new error screens.

## Action Items
- TODO: Alice to implement JWT refresh
- TODO: Bob to write error message copy
- TODO: Charlie to create error mockups
`

func TestGrep_Recursive(t *testing.T) {
	t.Run("without -r only searches direct children", func(t *testing.T) {
		env := newTestEnv(t)
		// Create top-level doc and nested doc
		env.runStdin("top level TODO item", "write", "readme")
		env.runStdin(notesDoc, "write", "docs/meeting")

		// Without -r, should only find top-level doc
		out := env.run("grep", "TODO")
		env.contains(out, "readme")
		if strings.Contains(out, "docs/meeting") {
			t.Error("Grep without -r found nested doc, want only direct children")
		}
	})

	t.Run("with -r searches all nested documents", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("top level TODO item", "write", "readme")
		env.runStdin(notesDoc, "write", "docs/meeting")

		// With -r, should find both
		out := env.run("grep", "-r", "TODO")
		env.contains(out, "readme")
		env.contains(out, "docs/meeting")
	})

	t.Run("without -r with path prefix searches direct children of prefix", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin(apiDoc, "write", "docs/api")
		env.runStdin(notesDoc, "write", "docs/notes/meeting")

		// Without -r, docs/ should only find docs/api, not docs/notes/meeting
		out := env.run("grep", "authentication", "docs/")
		env.contains(out, "docs/api")
		if strings.Contains(out, "docs/notes/meeting") {
			t.Error("Grep docs/ without -r found deeply nested doc")
		}
	})

	t.Run("with -r and path prefix searches all under prefix", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin(apiDoc, "write", "docs/api")
		env.runStdin(notesDoc, "write", "docs/notes/meeting")

		// With -r, should find both under docs/
		out := env.run("grep", "-r", "authentication", "docs/")
		env.contains(out, "docs/api")
		env.contains(out, "docs/notes/meeting")
	})
}

func TestGrep(t *testing.T) {
	t.Run("basic match with recursive", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin(apiDoc, "write", "docs/api")

		out := env.run("grep", "-r", "authentication")
		env.contains(out, "docs/api")
		env.contains(out, "authentication")
	})

	t.Run("no match", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin(apiDoc, "write", "docs/api")

		out := env.run("grep", "-r", "NONEXISTENT_TERM_12345")
		if strings.Contains(out, "docs/api") {
			t.Error("Grep(nonexistent) matched, want no match")
		}
	})

	t.Run("case sensitive by default", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin(apiDoc, "write", "docs/api")

		// Regex is case-sensitive by default (like Unix grep)
		out := env.run("grep", "-r", "jwt")
		if strings.Contains(out, "docs/api") {
			t.Error("Grep(jwt) matched JWT, want case-sensitive (no match)")
		}

		// Exact case should match
		out = env.run("grep", "-r", "JWT")
		env.contains(out, "docs/api")

		// -i flag enables case-insensitive matching
		out = env.run("grep", "-r", "-i", "jwt")
		env.contains(out, "docs/api")
	})

	t.Run("JSON output", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin(apiDoc, "write", "docs/api")

		out := env.run("grep", "-r", "authentication", "-o", "json")
		env.contains(out, `"path"`)
		env.contains(out, "docs/api")
	})
}

func TestGrep_Scope(t *testing.T) {
	t.Run("path scope excludes other paths", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin(apiDoc, "write", "docs/api")
		env.runStdin(notesDoc, "write", "notes/meeting")

		// Scoped to docs/, should not find notes/
		out := env.run("grep", "-r", "authentication", "docs/")
		env.contains(out, "docs/api")
		if strings.Contains(out, "notes/meeting") {
			t.Error("Grep(docs/) contains notes/meeting, want excluded")
		}
	})

	t.Run("multiple matches across paths", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin(apiDoc, "write", "docs/api")
		env.runStdin(notesDoc, "write", "notes/meeting")

		out := env.run("grep", "-r", "-i", "error")
		env.contains(out, "docs/api")
		env.contains(out, "notes/meeting")
	})
}

func TestGrep_PathsOnly(t *testing.T) {
	env := newTestEnv(t)
	env.runStdin(apiDoc, "write", "docs/api")
	env.runStdin(notesDoc, "write", "notes/meeting")

	out := env.run("grep", "-r", "-l", "TODO")
	env.contains(out, "notes/meeting")
	if strings.Contains(out, "Alice to implement") {
		t.Error("Grep(-l) contains content, want paths only")
	}
}

func TestGrep_Deleted(t *testing.T) {
	t.Run("normal excludes deleted", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin(notesDoc, "write", "notes/meeting")
		env.run("rm", "notes/meeting")

		out := env.run("grep", "-r", "TODO")
		if strings.Contains(out, "notes/meeting") {
			t.Error("Grep() matched deleted, want excluded")
		}
	})

	t.Run("-D includes only deleted", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin(notesDoc, "write", "notes/meeting")
		env.run("rm", "notes/meeting")

		out := env.run("grep", "-r", "-D", "TODO")
		env.contains(out, "notes/meeting")
	})

	t.Run("-A includes all", func(t *testing.T) {
		env := newTestEnv(t)
		guide := testGuideContent()
		env.runStdin(guide, "write", "docs/guide")
		env.runStdin("Another document about version control systems", "write", "docs/other")
		env.run("rm", "docs/other")

		out := env.run("grep", "-r", "-A", "version")
		env.contains(out, "docs/guide")
		env.contains(out, "docs/other")
	})
}

func TestGrep_Invert(t *testing.T) {
	t.Run("invert excludes matching lines", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin(notesDoc, "write", "notes/meeting")

		// Normal grep finds TODO lines
		out := env.run("grep", "-r", "TODO")
		env.contains(out, "TODO")
		env.contains(out, "Alice to implement")

		// Inverted grep excludes TODO lines
		out = env.run("grep", "-r", "-v", "TODO")
		if strings.Contains(out, "TODO") {
			t.Error("Grep(-v TODO) contains TODO, want excluded")
		}
		// But still matches the document (other lines)
		env.contains(out, "notes/meeting")
		env.contains(out, "Meeting Notes")
	})

	t.Run("invert with case insensitive", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin(apiDoc, "write", "docs/api")

		// -v -i should exclude lines matching case-insensitively
		out := env.run("grep", "-r", "-v", "-i", "error")
		if strings.Contains(out, "Error") || strings.Contains(out, "error") {
			t.Error("Grep(-v -i error) contains error, want excluded")
		}
		// Should still have other content
		env.contains(out, "docs/api")
		env.contains(out, "Authentication")
	})
}

func TestGrep_Count(t *testing.T) {
	t.Run("count matches", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin(notesDoc, "write", "notes/meeting")

		// notesDoc has 3 TODO lines
		out := env.run("grep", "-r", "-c", "TODO")
		env.contains(out, "notes/meeting:3")
	})

	t.Run("count multiple documents", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin(notesDoc, "write", "notes/meeting")
		env.runStdin(apiDoc, "write", "docs/api")

		out := env.run("grep", "-r", "-c", "-i", "error")
		env.contains(out, "notes/meeting:")
		env.contains(out, "docs/api:")
	})
}

func TestGrep_Context(t *testing.T) {
	const contextDoc = `Line 1
Line 2
MATCH here
Line 4
Line 5`

	t.Run("context lines", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin(contextDoc, "write", "test")

		out := env.run("grep", "-C", "1", "MATCH")
		env.contains(out, "Line 2")     // context before
		env.contains(out, "MATCH here") // match
		env.contains(out, "Line 4")     // context after
	})

	t.Run("context with separator", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin(notesDoc, "write", "notes/meeting")

		// Multiple matches should show context
		out := env.run("grep", "-r", "-C", "1", "TODO")
		env.contains(out, "TODO")
		env.contains(out, "Action Items") // context before first TODO
	})
}

func TestGrep_ContextValidation(t *testing.T) {
	t.Run("negative context rejected", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin(apiDoc, "write", "docs/api")

		_, err := env.runErr("grep", "-C", "-1", "test")
		if err == nil {
			t.Error("Grep(-C -1) should fail")
		}
	})
}

func TestGrep_Regex(t *testing.T) {
	t.Run("alternation", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin(apiDoc, "write", "docs/api")
		env.runStdin(notesDoc, "write", "notes/meeting")

		// Regex alternation should work
		out := env.run("grep", "-r", "API|TODO")
		env.contains(out, "docs/api")
		env.contains(out, "notes/meeting")
	})

	t.Run("dot star", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin(apiDoc, "write", "docs/api")

		// .* should match any characters
		out := env.run("grep", "-r", "HTTP.*codes")
		env.contains(out, "docs/api")
	})

	t.Run("character class", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin(apiDoc, "write", "docs/api")

		// [0-9] should match digits
		out := env.run("grep", "-r", "[0-9][0-9][0-9]")
		env.contains(out, "docs/api") // matches 400, 401, etc
	})

	t.Run("line output format", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin(apiDoc, "write", "docs/api")

		// Output should include path:line:content like real grep
		out := env.run("grep", "-r", "Authentication")
		env.contains(out, "docs/api:")
		env.contains(out, "Authentication")
	})

	t.Run("invalid regex", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin(apiDoc, "write", "docs/api")

		// Invalid regex should return error
		out, err := env.runErr("grep", "[invalid")
		if err == nil {
			t.Error("Grep([invalid) should fail with invalid regex error")
		}
		env.contains(out, "invalid regex")
	})
}
