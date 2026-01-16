package cmd

import "testing"

const sampleDoc = `# API Documentation

This document describes the REST API endpoints.

## Authentication

All requests require a Bearer token in the Authorisation header.
The token should be obtained from the /auth/login endpoint.

## Endpoints

### GET /users
Returns a list of users. Requires admin privileges.

### POST /users
Creates a new user. Request body should contain:
- name: string (required)
- email: string (required)
- role: string (optional, defaults to "user")
`

func TestSed(t *testing.T) {
	t.Run("replace first", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin(sampleDoc, "write", "docs/api")

		env.run("sed", "-i", "s/Bearer/JWT Bearer/", "docs/api")

		out := env.run("cat", "docs/api")
		env.contains(out, "JWT Bearer token")
	})

	t.Run("replace global", func(t *testing.T) {
		env := newTestEnv(t)
		content := "TODO: fix this\nTODO: and this\nTODO: also this"
		expected := "DONE: fix this\nDONE: and this\nDONE: also this"
		env.runStdin(content, "write", "docs/tasks")

		env.run("sed", "-i", "s/TODO/DONE/g", "docs/tasks")

		out := env.run("cat", "docs/tasks")
		env.equals(out, expected)
	})

	t.Run("alternate delimiter", func(t *testing.T) {
		env := newTestEnv(t)
		content := "Visit http://example.com for more info.\nAlso see http://docs.example.com"
		env.runStdin(content, "write", "docs/links")

		env.run("sed", "-i", "s|http://|https://|g", "docs/links")

		out := env.run("cat", "docs/links")
		env.contains(out, "https://example.com")
		env.contains(out, "https://docs.example.com")
	})

	t.Run("preserves multiline", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin(sampleDoc, "write", "docs/api")

		env.run("sed", "-i", "s/Authentication/Auth/", "docs/api")

		out := env.run("cat", "docs/api")
		env.contains(out, "## Auth")
		env.contains(out, "## Endpoints")
		env.contains(out, "### GET /users")
	})

	t.Run("JSON output", func(t *testing.T) {
		env := newTestEnv(t)
		guide := testGuideContent()
		env.runStdin(guide, "write", "docs/guide")

		out := env.run("sed", "-i", testSedExpr, "docs/guide", "-o", "json")
		env.contains(out, `"path"`)
		env.contains(out, `"docs/guide"`)

		content := env.run("cat", "docs/guide")
		env.contains(content, testSedExpectContains)
	})
}

func TestSed_Errors(t *testing.T) {
	t.Run("requires -i flag", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin("hello world", "write", "docs/test")

		_, err := env.runErr("sed", "s/hello/goodbye/", "docs/test")
		if err == nil {
			t.Error("Sed(without -i) = nil, want error")
		}
	})

	t.Run("pattern not found", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin(sampleDoc, "write", "docs/api")

		_, err := env.runErr("sed", "-i", "s/NONEXISTENT_STRING/replacement/", "docs/api")
		if err == nil {
			t.Error("Sed(not found) = nil, want error")
		}
	})
}

func TestSed_WithAuthor(t *testing.T) {
	env := newTestEnv(t)
	env.runStdin(sampleDoc, "write", "docs/api")

	env.run("sed", "-i", "s/REST/GraphQL/", "docs/api", "-a", "claude")

	out := env.run("history", "docs/api")
	env.contains(out, "claude")
}
