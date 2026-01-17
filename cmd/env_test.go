// Testing Strategy Design Decision:
//
// The cmd/ package contains CLI integration tests that exercise the full stack:
// command parsing -> service layer -> store layer -> SQLite.
//
// Many internal packages show "[no test files]" - this is intentional.
// These packages are covered by the CLI integration tests:
//   - internal/validate: covered by write/tag/link tests (invalid inputs fail)
//   - internal/store: covered by all CRUD tests (data persists correctly)
//   - internal/edit, internal/find, etc: covered by their respective cmd tests
//
// Unit tests for these packages would duplicate coverage without adding value.
// If underlying functionality breaks, the CLI tests fail - proving the stack works.

package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
)

var (
	binaryPath string
	buildOnce  sync.Once
	buildErr   error
)

// buildBinary compiles the llmd binary once for all tests.
func buildBinary(t *testing.T) string {
	t.Helper()

	buildOnce.Do(func() {
		// Build to a temp location
		tmpDir, err := os.MkdirTemp("", "llmd-test-bin-*")
		if err != nil {
			buildErr = err
			return
		}

		binaryName := "llmd"
		if os.PathSeparator == '\\' {
			binaryName = "llmd.exe"
		}
		binaryPath = filepath.Join(tmpDir, binaryName)

		// Find project root (parent of cmd/)
		wd := mustGetwd()
		projectRoot := filepath.Dir(wd)

		cmd := exec.Command("go", "build", "-o", binaryPath, ".")
		cmd.Dir = projectRoot
		if out, err := cmd.CombinedOutput(); err != nil {
			buildErr = &buildError{err: err, output: string(out)}
			return
		}
	})

	if buildErr != nil {
		t.Fatalf("failed to build binary: %v", buildErr)
	}
	return binaryPath
}

type buildError struct {
	err    error
	output string
}

func (e *buildError) Error() string {
	return e.err.Error() + "\n" + e.output
}

func mustGetwd() string {
	dir, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	return dir
}

// testEnv holds test environment state.
type testEnv struct {
	t      *testing.T
	dir    string
	binary string
}

// newTestEnv creates a temporary directory with an initialised llmd store.
//
// Note: init no longer creates config. Config is managed separately via
// "llmd config". This follows the git model where init just creates the
// repository structure.
func newTestEnv(t *testing.T) *testEnv {
	t.Helper()

	binary := buildBinary(t)
	dir := t.TempDir()

	env := &testEnv{t: t, dir: dir, binary: binary}

	// Initialise the store - note: no longer sets author here
	env.run("init")

	return env
}

// run executes llmd with the given args and returns stdout.
func (e *testEnv) run(args ...string) string {
	e.t.Helper()
	out, err := e.runErr(args...)
	if err != nil {
		e.t.Fatalf("llmd %v failed: %v\noutput: %s", args, err, out)
	}
	return out
}

// runErr executes llmd and returns stdout and any error.
func (e *testEnv) runErr(args ...string) (string, error) {
	e.t.Helper()

	cmd := exec.Command(e.binary, args...)
	cmd.Dir = e.dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// runStdin executes llmd with stdin input.
func (e *testEnv) runStdin(input string, args ...string) string {
	e.t.Helper()
	out, err := e.runStdinErr(input, args...)
	if err != nil {
		e.t.Fatalf("llmd %v failed: %v\noutput: %s", args, err, out)
	}
	return out
}

// runStdinErr executes llmd with stdin input and returns any error.
func (e *testEnv) runStdinErr(input string, args ...string) (string, error) {
	e.t.Helper()

	cmd := exec.Command(e.binary, args...)
	cmd.Dir = e.dir
	cmd.Stdin = strings.NewReader(input)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// contains checks if output contains expected string.
func (e *testEnv) contains(output, expected string) {
	e.t.Helper()
	assert.Contains(e.t, output, expected)
}

// equals checks if output equals expected string (trimmed).
func (e *testEnv) equals(output, expected string) {
	e.t.Helper()
	assert.Equal(e.t, strings.TrimSpace(expected), strings.TrimSpace(output))
}

// testGuideContent returns the guide.md content for testing.
// Uses the actual project documentation as realistic test data.
func testGuideContent() string {
	// Read from guide/guide.md relative to project root
	wd := mustGetwd()
	projectRoot := filepath.Dir(wd)
	content, err := os.ReadFile(filepath.Join(projectRoot, "guide", "guide.md"))
	if err != nil {
		panic("failed to read guide/guide.md for tests: " + err.Error())
	}
	return string(content)
}

// Test edit operations - what we search for and replace with
const (
	// Edit: change "declutters your filesystem" to "organises your documents"
	testEditOld = "declutters your filesystem"
	testEditNew = "organises your documents"

	// Sed: change "llmd init" to "llmd initialise"
	testSedExpr           = "s/llmd init/llmd initialise/"
	testSedExpectContains = "llmd initialise"

	// Line range replacement content
	testLineRangeReplacement = `## Getting Started

This section has been completely rewritten with new content.
Follow these updated instructions to begin using llmd.`
)

// Large content blocks for LLM-style editing tests.
// These simulate real LLM outputs - entire sections, not small edits.

// LLMTestDoc_V1 is a complete document an LLM might create
const LLMTestDoc_V1 = `# API Documentation

## Overview

This API provides access to the document management system. All endpoints
require authentication via Bearer tokens obtained from the /auth endpoint.

## Authentication

To authenticate, send a POST request to /auth/login with your credentials:

` + "```json" + `
{
    "username": "your-username",
    "password": "your-password"
}
` + "```" + `

The response will contain a JWT token valid for 24 hours.

## Endpoints

### GET /documents

Returns a list of all documents the authenticated user can access.

**Parameters:**
- ` + "`limit`" + ` (optional): Maximum number of results (default: 100)
- ` + "`offset`" + ` (optional): Pagination offset (default: 0)
- ` + "`sort`" + ` (optional): Sort field (default: created_at)

**Response:**
` + "```json" + `
{
    "documents": [
        {
            "id": "doc-123",
            "title": "Getting Started",
            "created_at": "2024-01-15T10:30:00Z",
            "updated_at": "2024-01-15T14:22:00Z"
        }
    ],
    "total": 42,
    "limit": 100,
    "offset": 0
}
` + "```" + `

### POST /documents

Creates a new document.

**Request Body:**
` + "```json" + `
{
    "title": "Document Title",
    "content": "Document content in markdown format",
    "tags": ["tag1", "tag2"]
}
` + "```" + `

### GET /documents/{id}

Returns a specific document by ID.

### PUT /documents/{id}

Updates an existing document.

### DELETE /documents/{id}

Soft-deletes a document (can be restored within 30 days).

## Error Handling

All errors follow this format:

` + "```json" + `
{
    "error": {
        "code": "DOCUMENT_NOT_FOUND",
        "message": "The requested document does not exist",
        "details": {}
    }
}
` + "```" + `

## Rate Limiting

- Standard users: 100 requests per minute
- Premium users: 1000 requests per minute

Rate limit headers are included in all responses.
`

// LLMTestDoc_V2_QuickStartReplacement is what an LLM would replace
// the "Quick Start" section with - a completely rewritten section
const LLMTestDoc_V2_QuickStartReplacement = `## Quick Start Guide

Welcome to LLMD! This guide will get you up and running in minutes.

### Prerequisites

Before you begin, ensure you have:
- Go 1.21 or later installed
- A terminal with UTF-8 support
- Basic familiarity with command-line tools

### Installation

Install LLMD using Go:

` + "```bash" + `
go install github.com/jpl-au/llmd@latest
` + "```" + `

Or download a pre-built binary from the releases page.

### Your First Document Store

1. **Initialise a new store:**

` + "```bash" + `
mkdir my-docs && cd my-docs
llmd init --name "Your Name" --email "you@example.com"
` + "```" + `

2. **Create your first document:**

` + "```bash" + `
echo "# Welcome

This is my first document in LLMD." | llmd write docs/welcome
` + "```" + `

3. **View your document:**

` + "```bash" + `
llmd cat docs/welcome
` + "```" + `

4. **List all documents:**

` + "```bash" + `
llmd ls
` + "```" + `

### Next Steps

- Learn about [editing documents](#editing)
- Explore [version history](#history)
- Set up [file synchronisation](#sync)
`

// LLMTestDoc_V3_CommandsTableReplacement replaces the commands table
// with an expanded, detailed version
const LLMTestDoc_V3_CommandsTableReplacement = `## Available Commands

LLMD provides a comprehensive set of commands for document management.
Each command supports ` + "`--help`" + ` for detailed usage information.

### Core Commands

| Command | Description | Example |
|---------|-------------|---------|
| ` + "`init`" + ` | Initialise a new document store | ` + "`llmd init`" + ` |
| ` + "`write`" + ` | Create or update a document | ` + "`echo \"content\" \\| llmd write path`" + ` |
| ` + "`cat`" + ` | Display document contents | ` + "`llmd cat docs/readme`" + ` |
| ` + "`edit`" + ` | Modify document via search/replace | ` + "`llmd edit path \"old\" \"new\"`" + ` |
| ` + "`rm`" + ` | Soft-delete a document | ` + "`llmd rm docs/old`" + ` |

### Search Commands

| Command | Description | Example |
|---------|-------------|---------|
| ` + "`ls`" + ` | List documents | ` + "`llmd ls docs/`" + ` |
| ` + "`grep`" + ` | Search document contents | ` + "`llmd grep \"TODO\"`" + ` |
| ` + "`find`" + ` | Full-text search | ` + "`llmd find \"authentication\"`" + ` |
| ` + "`glob`" + ` | Pattern-based path matching | ` + "`llmd glob \"docs/**\"`" + ` |

### History Commands

| Command | Description | Example |
|---------|-------------|---------|
| ` + "`history`" + ` | View version history | ` + "`llmd history docs/api`" + ` |
| ` + "`diff`" + ` | Compare versions | ` + "`llmd diff docs/api -v 1:3`" + ` |
| ` + "`restore`" + ` | Restore deleted document | ` + "`llmd restore docs/old`" + ` |

### Sync Commands

| Command | Description | Example |
|---------|-------------|---------|
| ` + "`import`" + ` | Import from filesystem | ` + "`llmd import ./docs/`" + ` |
| ` + "`export`" + ` | Export to filesystem | ` + "`llmd export docs/ ./out/`" + ` |
| ` + "`sync`" + ` | Sync filesystem changes | ` + "`llmd sync`" + ` |
`

// LLMTestDoc_V4_NewSection is an entirely new section an LLM adds
const LLMTestDoc_V4_NewSection = `## Advanced Usage

### Working with Large Documents

When working with documents over 1000 lines, consider these best practices:

1. **Use line-range editing** for targeted changes:
` + "```bash" + `
# Replace lines 50-75 with new content
llmd edit docs/large -l 50:75 < new-section.md
` + "```" + `

2. **Leverage grep for navigation:**
` + "```bash" + `
# Find all TODO items with line numbers
llmd grep -n "TODO" docs/
` + "```" + `

3. **Use structured paths** for organisation:
` + "```" + `
docs/
├── api/
│   ├── authentication
│   ├── endpoints
│   └── errors
├── guides/
│   ├── quickstart
│   └── advanced
└── reference/
    ├── commands
    └── config
` + "```" + `

### Automation and Scripting

LLMD works great in scripts and CI/CD pipelines:

` + "```bash" + `
#!/bin/bash
set -euo pipefail

# Update version in all docs
for doc in $(llmd glob "docs/**"); do
    llmd sed -i "s/v1.0.0/v1.1.0/g" "$doc" -a "release-bot"
done

# Export for static site generator
llmd export docs/ ./public/docs/

# Verify no broken links
llmd grep -l "](docs/" docs/ | while read doc; do
    echo "Checking links in $doc..."
done
` + "```" + `

### Integration with AI Assistants

When using LLMD with AI assistants like Claude:

1. **Always identify yourself:**
` + "```bash" + `
llmd write docs/feature -a "claude-code" -m "Added authentication docs"
` + "```" + `

2. **Use JSON output for parsing:**
` + "```bash" + `
llmd ls -o json | jq '.[] | select(.path | startswith("docs/api"))'
` + "```" + `

3. **Check history before editing:**
` + "```bash" + `
llmd history docs/api -n 5  # See recent changes first
` + "```" + `
`

// LLMTestDoc_V5_CompleteRewrite is a complete document rewrite
const LLMTestDoc_V5_CompleteRewrite = `# LLMD Reference Manual

**Version 2.0 - Complete Rewrite**

This document has been completely rewritten to provide clearer,
more comprehensive documentation for the LLMD document management system.

## Table of Contents

1. [Introduction](#introduction)
2. [Installation](#installation)
3. [Core Concepts](#core-concepts)
4. [Command Reference](#command-reference)
5. [Configuration](#configuration)
6. [Best Practices](#best-practices)

## Introduction

LLMD (Lightweight Local Markdown Documents) is a command-line document
management system designed for developers and technical writers. It provides:

- **Version Control**: Every change creates a new version
- **Full-Text Search**: Find content across all documents
- **Filesystem Sync**: Optional bidirectional sync with .md files
- **AI-Friendly**: JSON output and author attribution for LLM integration

## Installation

### Using Go

` + "```bash" + `
go install github.com/jpl-au/llmd@latest
` + "```" + `

### From Source

` + "```bash" + `
git clone https://github.com/jpl-au/llmd.git
cd llmd
go build -o llmd .
sudo mv llmd /usr/local/bin/
` + "```" + `

## Core Concepts

### Document Paths

Documents are identified by paths without file extensions:

- ✓ ` + "`docs/api/authentication`" + `
- ✗ ` + "`docs/api/authentication.md`" + `

### Versions

Every write operation creates a new version. Versions are numbered
sequentially starting from 1. You can access any version:

` + "```bash" + `
llmd cat docs/readme -v 3    # Read version 3
llmd diff docs/readme -v 1:5 # Compare versions 1 and 5
` + "```" + `

### Authors

Track who made changes with the ` + "`-a`" + ` flag:

` + "```bash" + `
llmd edit docs/api "old" "new" -a "claude" -m "Fixed typo"
` + "```" + `

## Command Reference

### Document Operations

#### write
` + "```bash" + `
echo "content" | llmd write <path> [-a author] [-m message]
` + "```" + `

#### cat
` + "```bash" + `
llmd cat <path> [-v version] [-o json]
` + "```" + `

#### edit
` + "```bash" + `
llmd edit <path> <old> <new>     # Search and replace
llmd edit <path> -l <start:end>  # Line range (stdin)
` + "```" + `

### Search Operations

#### grep
` + "```bash" + `
llmd grep <pattern> [path] [-i] [-l] [-n]
` + "```" + `

#### find
` + "```bash" + `
llmd find <query> [-p path]
` + "```" + `

## Configuration

Configuration is stored in ` + "`.llmd/config.yaml`" + `:

` + "```yaml" + `
author:
  name: Your Name
  email: you@example.com
sync:
  files: true  # Enable filesystem sync
` + "```" + `

## Best Practices

1. **Use meaningful paths**: ` + "`docs/api/auth`" + ` not ` + "`doc1`" + `
2. **Always attribute changes**: Use ` + "`-a`" + ` flag consistently
3. **Write descriptive messages**: Use ` + "`-m`" + ` for important changes
4. **Regular exports**: Back up with ` + "`llmd export`" + `
`

