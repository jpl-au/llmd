package cmd

import (
	"strconv"
	"strings"
	"testing"
)

const editDoc = `# User Guide

## Introduction

Welcome to the application. This guide will help you get started.

## Installation

Follow these steps to install:
1. Download the package
2. Run the installer
3. Configure settings

## Usage

Run the following command:
` + "```" + `bash
app start --config config.yaml
` + "```" + `

## Troubleshooting

If you encounter issues, check the logs.
`

func TestEdit(t *testing.T) {
	t.Run("search and replace", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin(editDoc, "write", "docs/guide")

		env.run("edit", "docs/guide", "application", "software")

		out := env.run("cat", "docs/guide")
		env.contains(out, "software")
	})

	t.Run("replace first only", func(t *testing.T) {
		env := newTestEnv(t)
		content := "foo bar foo baz foo"
		env.runStdin(content, "write", "docs/test")

		env.run("edit", "docs/test", "foo", "qux")

		out := env.run("cat", "docs/test")
		env.equals(out, "qux bar foo baz foo")
	})

	t.Run("preserves structure", func(t *testing.T) {
		env := newTestEnv(t)
		env.runStdin(editDoc, "write", "docs/guide")

		env.run("edit", "docs/guide", "User Guide", "Administrator Guide")

		out := env.run("cat", "docs/guide")
		env.contains(out, "# Administrator Guide")
		env.contains(out, "## Introduction")
		env.contains(out, "## Installation")
		env.contains(out, "## Usage")
		env.contains(out, "## Troubleshooting")
	})

	t.Run("JSON output", func(t *testing.T) {
		env := newTestEnv(t)
		guide := testGuideContent()
		env.runStdin(guide, "write", "docs/guide")

		out := env.run("edit", "docs/guide", testEditOld, testEditNew, "-o", "json")
		env.contains(out, `"path"`)
		env.contains(out, `"docs/guide"`)

		content := env.run("cat", "docs/guide")
		env.contains(content, testEditNew)
	})
}

func TestEdit_LineRange(t *testing.T) {
	t.Run("basic line range", func(t *testing.T) {
		env := newTestEnv(t)
		content := "line 1\nline 2\nline 3\nline 4\nline 5"
		env.runStdin(content, "write", "docs/lines")

		env.runStdin("new line 2\nnew line 3", "edit", "docs/lines", "-l", "2:3")

		out := env.run("cat", "docs/lines")
		env.contains(out, "line 1")
		env.contains(out, "new line 2")
		env.contains(out, "new line 3")
		env.contains(out, "line 4")
	})

	t.Run("JSON output", func(t *testing.T) {
		env := newTestEnv(t)
		guide := testGuideContent()
		env.runStdin(guide, "write", "docs/guide")

		out := env.runStdin(testLineRangeReplacement, "edit", "docs/guide", "-l", "5:7", "-o", "json")
		env.contains(out, `"path"`)
		env.contains(out, `"docs/guide"`)

		content := env.run("cat", "docs/guide")
		env.contains(content, "Getting Started")
		env.contains(content, "completely rewritten")
	})

	t.Run("on large document", func(t *testing.T) {
		env := newTestEnv(t)
		guide := testGuideContent()
		env.runStdin(guide, "write", "docs/guide")

		newSection := `## Replaced Section

This entire section was replaced using line range editing.
It demonstrates that large documents can be edited precisely.

### Subsection

More content in the replaced section.`

		env.runStdin(newSection, "edit", "docs/guide", "-l", "10:20")

		result := env.run("cat", "docs/guide")
		env.contains(result, "## Replaced Section")
		env.contains(result, "line range editing")
		env.contains(result, "LLMD Guide")
	})
}

func TestEdit_WithAuthor(t *testing.T) {
	env := newTestEnv(t)
	env.runStdin(editDoc, "write", "docs/guide")

	env.run("edit", "docs/guide", "Introduction", "Overview", "-a", "editor")

	out := env.run("history", "docs/guide")
	env.contains(out, "editor")
}

func TestEdit_NotFound(t *testing.T) {
	env := newTestEnv(t)
	env.runStdin(editDoc, "write", "docs/guide")

	_, err := env.runErr("edit", "docs/guide", "NONEXISTENT_TEXT", "replacement")
	if err == nil {
		t.Error("Edit(not found) = nil, want error")
	}
}

func TestEdit_MultipleEdits(t *testing.T) {
	env := newTestEnv(t)
	env.runStdin("hello world", "write", "docs/test")

	env.run("edit", "docs/test", "hello", "hi")
	env.run("edit", "docs/test", "world", "there")

	out := env.run("cat", "docs/test")
	env.equals(out, "hi there")

	out = env.run("history", "docs/test")
	env.contains(out, "v3")
}

func TestEdit_ComplexMulti(t *testing.T) {
	env := newTestEnv(t)
	guide := testGuideContent()
	env.runStdin(guide, "write", "docs/guide")

	env.run("edit", "docs/guide", "llmd", "LLMD-CLI", "-a", "editor1")
	env.run("edit", "docs/guide", "document", "file", "-a", "editor2")
	env.run("edit", "docs/guide", "version", "revision", "-a", "editor3")

	content := env.run("cat", "docs/guide")
	env.contains(content, "LLMD-CLI")
	env.contains(content, "file")
	env.contains(content, "revision")

	history := env.run("history", "docs/guide")
	env.contains(history, "v4")
	env.contains(history, "editor1")
	env.contains(history, "editor2")
	env.contains(history, "editor3")

	v1 := env.run("cat", "-v", "1", "docs/guide")
	env.contains(v1, "llmd")
}

func TestSed_ComplexMulti(t *testing.T) {
	env := newTestEnv(t)
	guide := testGuideContent()
	env.runStdin(guide, "write", "docs/guide")

	env.run("sed", "-i", "s/LLMD/LLMD-TOOL/g", "docs/guide", "-a", "sed-user1")
	env.run("sed", "-i", "s/Guide/Manual/g", "docs/guide", "-a", "sed-user2")
	env.run("sed", "-i", "s/Quick Start/Getting Started/", "docs/guide", "-a", "sed-user3")

	content := env.run("cat", "docs/guide")
	env.contains(content, "LLMD-TOOL")
	env.contains(content, "Manual")
	env.contains(content, "Getting Started")

	history := env.run("history", "docs/guide")
	env.contains(history, "v4")
	env.contains(history, "sed-user1")
	env.contains(history, "sed-user2")
	env.contains(history, "sed-user3")
}

func TestEdit_MixedEditsAndSeds(t *testing.T) {
	env := newTestEnv(t)
	guide := testGuideContent()
	env.runStdin(guide, "write", "docs/guide")

	env.run("edit", "docs/guide", "LLMD Guide", "Documentation Guide")
	env.run("sed", "-i", "s/llmd/llmd-cli/g", "docs/guide")
	env.run("edit", "docs/guide", "Quick Start", "Getting Started")
	env.run("sed", "-i", "s/version/ver/", "docs/guide")

	content := env.run("cat", "docs/guide")
	env.contains(content, "Documentation Guide")
	env.contains(content, "llmd-cli")
	env.contains(content, "Getting Started")

	history := env.run("history", "docs/guide")
	env.contains(history, "v5")
}

// Advanced LLM-style editing tests
// These tests simulate realistic editing patterns that LLMs produce

func TestEdit_LLM_LargeSectionReplacement(t *testing.T) {
	env := newTestEnv(t)
	guide := testGuideContent()
	env.runStdin(guide, "write", "docs/guide")

	// Simulate LLM replacing an entire section (lines 42-53 is "Command Usage" section)
	newSection := `## Command Usage

### Reading Documents

Use these commands to read and explore your documents:

` + "```bash" + `
# Read a document
llmd cat docs/readme

# Read a specific version
llmd cat docs/readme -v 3

# List all documents in tree format
llmd ls -t

# Search with glob patterns
llmd glob "docs/**/*.md"
` + "```" + `

### Writing Documents

Create and modify documents with these commands:`

	env.runStdin(newSection, "edit", "docs/guide", "-l", "42:53", "-a", "claude-opus")

	content := env.run("cat", "docs/guide")
	env.contains(content, "### Reading Documents")
	env.contains(content, "### Writing Documents")
	env.contains(content, "llmd cat docs/readme -v 3")
	// Verify other sections still intact
	env.contains(content, "# LLMD Guide")
	env.contains(content, "## Commands")
	env.contains(content, "## For LLMs")

	history := env.run("history", "docs/guide")
	env.contains(history, "claude-opus")
}

func TestEdit_LLM_MultipleSequentialLineRanges(t *testing.T) {
	env := newTestEnv(t)
	guide := testGuideContent()
	env.runStdin(guide, "write", "docs/guide")

	// First edit: modify header (line 1)
	env.runStdin("# LLMD Documentation", "edit", "docs/guide", "-l", "1:1", "-a", "llm-edit-1")

	// Second edit: modify Quick Start section intro (line 5)
	env.runStdin("## Getting Started\n", "edit", "docs/guide", "-l", "5:5", "-a", "llm-edit-2")

	// Third edit: add content at end (last few lines)
	newEnding := `## Additional Resources

- [GitHub Repository](https://github.com/example/llmd)
- [API Documentation](https://docs.example.com/llmd)
- [Community Forum](https://forum.example.com/llmd)
`
	env.runStdin(newEnding, "edit", "docs/guide", "-l", "128:133", "-a", "llm-edit-3")

	content := env.run("cat", "docs/guide")
	env.contains(content, "# LLMD Documentation")
	env.contains(content, "## Getting Started")
	env.contains(content, "## Additional Resources")
	env.contains(content, "GitHub Repository")

	// Verify version history
	history := env.run("history", "docs/guide")
	env.contains(history, "v4")
	env.contains(history, "llm-edit-1")
	env.contains(history, "llm-edit-2")
	env.contains(history, "llm-edit-3")

	// Verify we can retrieve original version
	v1 := env.run("cat", "-v", "1", "docs/guide")
	env.contains(v1, "# LLMD Guide")
	env.contains(v1, "## Quick Start")
}

func TestEdit_LLM_VersionIntegrityAfterManyEdits(t *testing.T) {
	env := newTestEnv(t)
	guide := testGuideContent()
	env.runStdin(guide, "write", "docs/guide")

	// Perform 10 sequential edits
	edits := []struct {
		old, new, author string
	}{
		{"LLMD Guide", "LLMD Manual", "edit-1"},
		{"Quick Start", "Getting Started", "edit-2"},
		{"document store", "documentation system", "edit-3"},
		{"filesystem", "file system", "edit-4"},
		{"versioning", "version control", "edit-5"},
		{"version control", "revision history", "edit-6"},
		{"Commands", "Available Commands", "edit-7"},
		{"Run `llmd", "Execute `llmd", "edit-8"},
		{"For LLMs", "For AI Assistants", "edit-9"},
		{"Global Flags", "Command-Line Flags", "edit-10"},
	}

	for _, e := range edits {
		env.run("edit", "docs/guide", e.old, e.new, "-a", e.author)
	}

	// Verify final content has all changes
	final := env.run("cat", "docs/guide")
	env.contains(final, "LLMD Manual")
	env.contains(final, "Getting Started")
	env.contains(final, "documentation system")
	env.contains(final, "For AI Assistants")
	env.contains(final, "Command-Line Flags")

	// Verify we have 11 versions (1 original + 10 edits)
	history := env.run("history", "docs/guide")
	env.contains(history, "v11")

	// Verify intermediate versions are intact
	v1 := env.run("cat", "-v", "1", "docs/guide")
	env.contains(v1, "LLMD Guide")
	env.contains(v1, "Quick Start")
	env.contains(v1, "For LLMs")

	v5 := env.run("cat", "-v", "5", "docs/guide")
	env.contains(v5, "LLMD Manual")          // changed in edit-1
	env.contains(v5, "Getting Started")      // changed in edit-2
	env.contains(v5, "documentation system") // changed in edit-3
	env.contains(v5, "file system")          // changed in edit-4
	env.contains(v5, "For LLMs")             // not yet changed

	// Diff between versions shows changes
	diff := env.run("diff", "docs/guide", "-v", "1:11")
	env.contains(diff, "Guide")  // removed
	env.contains(diff, "Manual") // added
	env.contains(diff, "v1")
	env.contains(diff, "v11")
}

func TestEdit_LLM_PreservesMarkdownStructure(t *testing.T) {
	env := newTestEnv(t)
	guide := testGuideContent()
	env.runStdin(guide, "write", "docs/guide")

	// Edit content inside a code block region - should preserve block
	env.run("edit", "docs/guide", "llmd init", "llmd initialise")

	content := env.run("cat", "docs/guide")
	// Verify code blocks still have proper fencing
	env.contains(content, "```bash")
	env.contains(content, "```")
	env.contains(content, "llmd initialise")

	// Verify table structure preserved
	env.contains(content, "| Command | Description |")
	env.contains(content, "|---------|-------------|")
	env.contains(content, "| `init` |")

	// Edit table content
	env.run("edit", "docs/guide", "Initialise a new store", "Create a new document store")

	content = env.run("cat", "docs/guide")
	env.contains(content, "Create a new document store")
	// Table structure still intact
	env.contains(content, "| `init` | Create a new document store |")
}

func TestEdit_LLM_CompleteDocumentRewrite(t *testing.T) {
	env := newTestEnv(t)
	guide := testGuideContent()
	env.runStdin(guide, "write", "docs/guide")

	// Simulate LLM completely rewriting the document
	newDoc := `# LLMD User Manual

## Overview

LLMD is a command-line document management system with built-in versioning.

## Installation

` + "```bash" + `
go install github.com/example/llmd@latest
` + "```" + `

## Basic Usage

### Creating Documents

` + "```bash" + `
echo "# My Document" | llmd write docs/readme
` + "```" + `

### Reading Documents

` + "```bash" + `
llmd cat docs/readme
` + "```" + `

## Version History

Every change creates a new version. View history with:

` + "```bash" + `
llmd history docs/readme
` + "```" + `
`

	env.runStdin(newDoc, "write", "docs/guide", "-a", "claude-rewrite")

	content := env.run("cat", "docs/guide")
	env.contains(content, "# LLMD User Manual")
	env.contains(content, "## Overview")
	env.contains(content, "command-line document management")

	// Original still accessible
	v1 := env.run("cat", "-v", "1", "docs/guide")
	env.contains(v1, "# LLMD Guide")
	env.contains(v1, "## Quick Start")

	// History shows both versions
	history := env.run("history", "docs/guide")
	env.contains(history, "v2")
	env.contains(history, "claude-rewrite")
}

func TestEdit_LLM_EdgeCaseLineRanges(t *testing.T) {
	t.Run("first line only", func(t *testing.T) {
		env := newTestEnv(t)
		guide := testGuideContent()
		env.runStdin(guide, "write", "docs/guide")

		env.runStdin("# LLMD Reference Manual", "edit", "docs/guide", "-l", "1:1")

		content := env.run("cat", "docs/guide")
		env.contains(content, "# LLMD Reference Manual")
		env.contains(content, "## Quick Start") // rest preserved
	})

	t.Run("single line in middle", func(t *testing.T) {
		env := newTestEnv(t)
		guide := testGuideContent()
		env.runStdin(guide, "write", "docs/guide")

		env.runStdin("Run `llmd guide <command>` for comprehensive help.", "edit", "docs/guide", "-l", "16:16")

		content := env.run("cat", "docs/guide")
		env.contains(content, "comprehensive help")
	})

	t.Run("expand single line to multiple", func(t *testing.T) {
		env := newTestEnv(t)
		guide := testGuideContent()
		env.runStdin(guide, "write", "docs/guide")

		expanded := `Run one of these commands for detailed help:

- ` + "`llmd guide init`" + ` - Setup instructions
- ` + "`llmd guide write`" + ` - Writing documents
- ` + "`llmd guide edit`" + ` - Editing documents`

		env.runStdin(expanded, "edit", "docs/guide", "-l", "16:16")

		content := env.run("cat", "docs/guide")
		env.contains(content, "Setup instructions")
		env.contains(content, "Writing documents")
		env.contains(content, "Editing documents")
	})
}

func TestEdit_LLM_RealisticWorkflow(t *testing.T) {
	env := newTestEnv(t)
	guide := testGuideContent()

	// Step 1: User creates initial document
	env.runStdin(guide, "write", "docs/guide", "-a", "user")

	// Step 2: LLM reads and searches the document
	content := env.run("cat", "docs/guide")
	env.contains(content, "LLMD Guide")

	grepOut := env.run("grep", "init", "docs/guide")
	env.contains(grepOut, "docs/guide")

	// Step 3: LLM makes targeted fix
	env.run("edit", "docs/guide", "declutters your filesystem", "organises your documents", "-a", "claude", "-m", "Improved description")

	// Step 4: LLM adds new section via line range
	newSection := `
## Tips for LLM Usage

1. Always use the ` + "`-a`" + ` flag to identify yourself
2. Use ` + "`-o json`" + ` for structured output parsing
3. Check history before making changes
4. Use specific paths to avoid conflicts
`
	env.runStdin(newSection, "edit", "docs/guide", "-l", "118:118", "-a", "claude", "-m", "Added LLM tips")

	// Step 5: LLM uses sed for global replacement
	env.run("sed", "-i", "s/llmd/LLMD/g", "docs/guide", "-a", "claude", "-m", "Consistent casing")

	// Step 6: Verify final state
	final := env.run("cat", "docs/guide")
	env.contains(final, "organises your documents")
	env.contains(final, "## Tips for LLM Usage")
	env.contains(final, "LLMD init")

	// Step 7: Verify complete history
	history := env.run("history", "docs/guide")
	env.contains(history, "v4")
	env.contains(history, "user")
	env.contains(history, "claude")
	env.contains(history, "Improved description")
	env.contains(history, "Added LLM tips")
	env.contains(history, "Consistent casing")

	// Step 8: Verify diff capability
	diff := env.run("diff", "docs/guide", "-v", "1:4")
	env.contains(diff, "declutters")
	env.contains(diff, "organises")
}

// Stress tests and edge cases for production reliability

func TestEdit_LLM_VeryLargeDocument(t *testing.T) {
	env := newTestEnv(t)

	// Generate a 500-line document with complex structure
	var b strings.Builder
	b.WriteString("# Large Document Test\n\n")
	b.WriteString("This document tests handling of large content with many sections.\n\n")

	for section := range 20 {
		b.WriteString("## Section ")
		b.WriteString(strconv.Itoa(section + 1))
		b.WriteString("\n\n")
		b.WriteString("Introduction to section ")
		b.WriteString(strconv.Itoa(section + 1))
		b.WriteString(".\n\n")

		// Add code block
		b.WriteString("```python\n")
		b.WriteString("def function_")
		b.WriteString(strconv.Itoa(section))
		b.WriteString("():\n")
		b.WriteString("    # Process section ")
		b.WriteString(strconv.Itoa(section))
		b.WriteString("\n")
		b.WriteString("    return ")
		b.WriteString(strconv.Itoa(section * 10))
		b.WriteString("\n```\n\n")

		// Add list
		for item := range 5 {
			b.WriteString("- Item ")
			b.WriteString(strconv.Itoa(item + 1))
			b.WriteString(" in section ")
			b.WriteString(strconv.Itoa(section + 1))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	content := b.String()
	env.runStdin(content, "write", "docs/large")

	// Verify document was stored correctly
	out := env.run("cat", "docs/large")
	env.contains(out, "# Large Document Test")
	env.contains(out, "## Section 1")
	env.contains(out, "## Section 20")
	env.contains(out, "def function_19")

	// Edit in the middle of the large document
	env.run("edit", "docs/large", "## Section 10", "## Chapter 10 - Modified", "-a", "llm")

	out = env.run("cat", "docs/large")
	env.contains(out, "## Chapter 10 - Modified")
	env.contains(out, "## Section 9")  // Before - unchanged
	env.contains(out, "## Section 11") // After - unchanged

	// Line range edit on large document
	env.runStdin("## New Section\n\nThis replaced multiple lines.\n", "edit", "docs/large", "-l", "50:60", "-a", "llm")

	out = env.run("cat", "docs/large")
	env.contains(out, "## New Section")
	env.contains(out, "This replaced multiple lines")

	// Verify history
	history := env.run("history", "docs/large")
	env.contains(history, "v3")
}

func TestEdit_LLM_ComplexMarkdown(t *testing.T) {
	env := newTestEnv(t)

	// Document with all markdown features
	complexDoc := `# Complex Markdown Document

## Introduction

This document contains **bold**, *italic*, and ` + "`inline code`" + `.

## Nested Lists

1. First ordered item
   - Nested unordered
   - Another nested
     - Deep nested
     - More deep
   - Back to second level
2. Second ordered item
   1. Nested ordered
   2. Another nested ordered

## Blockquotes

> This is a blockquote.
> It spans multiple lines.
>
> > This is a nested blockquote.
> > With more content.

## Code Blocks

` + "```go" + `
package main

import "fmt"

func main() {
    fmt.Println("Hello, World!")
}
` + "```" + `

` + "```json" + `
{
    "name": "test",
    "value": 123,
    "nested": {
        "key": "value"
    }
}
` + "```" + `

## Tables

| Header 1 | Header 2 | Header 3 |
|----------|:--------:|---------:|
| Left     | Centre   | Right    |
| Data 1   | Data 2   | Data 3   |
| More     | Data     | Here     |

## Links and Images

[Link to example](https://example.com)
![Image alt text](https://example.com/image.png)

## Horizontal Rules

---

***

___

## Task Lists

- [x] Completed task
- [ ] Incomplete task
- [x] Another completed

## Footnotes

Here is a footnote reference[^1].

[^1]: This is the footnote content.

## End

The end of the document.
`

	env.runStdin(complexDoc, "write", "docs/complex")

	// Edit preserving markdown structure
	env.run("edit", "docs/complex", "This document contains", "This documentation includes", "-a", "llm")

	out := env.run("cat", "docs/complex")
	env.contains(out, "This documentation includes")
	env.contains(out, "**bold**")
	env.contains(out, "*italic*")

	// Edit inside code block
	env.run("edit", "docs/complex", "Hello, World!", "Hello, LLMD!", "-a", "llm")

	out = env.run("cat", "docs/complex")
	env.contains(out, "Hello, LLMD!")
	env.contains(out, "```go") // Code fence preserved

	// Edit table content
	env.run("edit", "docs/complex", "| Data 1   | Data 2   | Data 3   |", "| Updated  | Content  | Here     |", "-a", "llm")

	out = env.run("cat", "docs/complex")
	env.contains(out, "| Updated  | Content  | Here     |")
	env.contains(out, "| Header 1 | Header 2 | Header 3 |") // Header preserved

	// Verify nested blockquote preserved
	env.contains(out, "> > This is a nested blockquote.")

	// Verify task lists preserved
	env.contains(out, "- [x] Completed task")
	env.contains(out, "- [ ] Incomplete task")

	// Verify JSON in code block preserved
	env.contains(out, `"nested": {`)

	history := env.run("history", "docs/complex")
	env.contains(history, "v4")
}

func TestEdit_LLM_WhitespacePreservation(t *testing.T) {
	env := newTestEnv(t)

	// Document with significant whitespace
	wsDoc := "# Document\n\n" +
		"Line with trailing spaces   \n" +
		"Line with\ttabs\tin\tit\n" +
		"\n" +
		"\n" +
		"\n" +
		"After multiple blank lines\n" +
		"    Indented with spaces\n" +
		"\tIndented with tab\n" +
		"Normal line\n"

	env.runStdin(wsDoc, "write", "docs/whitespace")

	out := env.run("cat", "docs/whitespace")
	env.contains(out, "trailing spaces   \n") // Trailing spaces preserved
	env.contains(out, "with\ttabs\tin")       // Tabs preserved
	env.contains(out, "\n\n\n")               // Multiple blank lines
	env.contains(out, "    Indented")         // Space indentation
	env.contains(out, "\tIndented with tab")  // Tab indentation

	// Edit that doesn't touch whitespace
	env.run("edit", "docs/whitespace", "Normal line", "Modified line", "-a", "llm")

	out = env.run("cat", "docs/whitespace")
	env.contains(out, "Modified line")
	env.contains(out, "trailing spaces   \n") // Still preserved
	env.contains(out, "\tIndented with tab")  // Still preserved
}

func TestEdit_LLM_VeryLongLines(t *testing.T) {
	env := newTestEnv(t)

	// Create document with very long lines
	var longLine strings.Builder
	for range 100 {
		longLine.WriteString("This is a very long line segment that repeats. ")
	}
	longLineStr := longLine.String()

	doc := "# Long Lines Test\n\n" +
		"Short line.\n\n" +
		longLineStr + "\n\n" +
		"Another short line.\n"

	env.runStdin(doc, "write", "docs/longlines")

	out := env.run("cat", "docs/longlines")
	env.contains(out, "very long line segment")

	// Edit within the long line
	env.run("edit", "docs/longlines", "very long line segment", "extremely lengthy text segment", "-a", "llm")

	out = env.run("cat", "docs/longlines")
	env.contains(out, "extremely lengthy text segment")
	// Should have replaced all occurrences in that line
	if strings.Count(out, "extremely lengthy text segment") < 50 {
		// Only first occurrence replaced
		env.contains(out, "extremely lengthy text segment")
	}
}

func TestEdit_LLM_SpecialCharactersInPatterns(t *testing.T) {
	env := newTestEnv(t)

	doc := `# Special Characters

Search for: [brackets] and (parens) and {braces}
Also: $dollar and ^caret and .period
Regex chars: * + ? | \
Quotes: "double" and 'single' and ` + "`backtick`" + `
Unicode: émoji 🎉 and 中文 and العربية
Symbols: © ® ™ § ¶ † ‡ • ° ± × ÷
`

	env.runStdin(doc, "write", "docs/special")

	// Edit with brackets
	env.run("edit", "docs/special", "[brackets]", "[REPLACED]", "-a", "llm")
	out := env.run("cat", "docs/special")
	env.contains(out, "[REPLACED]")

	// Edit with parens
	env.run("edit", "docs/special", "(parens)", "(CHANGED)", "-a", "llm")
	out = env.run("cat", "docs/special")
	env.contains(out, "(CHANGED)")

	// Edit with unicode
	env.run("edit", "docs/special", "émoji 🎉", "emoji replaced", "-a", "llm")
	out = env.run("cat", "docs/special")
	env.contains(out, "emoji replaced")

	// Edit with quotes
	env.run("edit", "docs/special", `"double"`, `"DOUBLE"`, "-a", "llm")
	out = env.run("cat", "docs/special")
	env.contains(out, `"DOUBLE"`)

	// Verify other special chars preserved
	env.contains(out, "{braces}")
	env.contains(out, "$dollar")
	env.contains(out, "中文")
	env.contains(out, "© ® ™")

	history := env.run("history", "docs/special")
	env.contains(history, "v5")
}

func TestEdit_LLM_RapidSequentialEdits(t *testing.T) {
	env := newTestEnv(t)
	guide := testGuideContent()
	env.runStdin(guide, "write", "docs/guide")

	// Perform 25 rapid sequential edits
	edits := []struct{ old, new string }{
		{"LLMD", "LLMD-V1"},
		{"LLMD-V1", "LLMD-V2"},
		{"LLMD-V2", "LLMD-V3"},
		{"LLMD-V3", "LLMD-V4"},
		{"LLMD-V4", "LLMD-V5"},
		{"Guide", "Manual"},
		{"Manual", "Reference"},
		{"Reference", "Documentation"},
		{"Quick Start", "Getting Started"},
		{"Getting Started", "Quickstart Guide"},
		{"Quickstart Guide", "Setup Instructions"},
		{"document", "file"},
		{"file store", "document store"}, // back
		{"Commands", "Available Commands"},
		{"Available Commands", "Command Reference"},
		{"Command Reference", "CLI Commands"},
		{"initialise", "configure"},
		{"configure", "setup"},
		{"versioning", "version control"},
		{"version control", "revision tracking"},
		{"filesystem", "file system"},
		{"file system", "disk storage"},
		{"search", "find"},
		{"history", "changelog"},
		{"LLMs", "AI assistants"},
	}

	for i, e := range edits {
		env.run("edit", "docs/guide", e.old, e.new, "-a", "rapid-"+strconv.Itoa(i+1))
	}

	// Verify we have 26 versions
	history := env.run("history", "docs/guide")
	env.contains(history, "v26")
	env.contains(history, "rapid-1")
	env.contains(history, "rapid-25")

	// Verify final state
	final := env.run("cat", "docs/guide")
	env.contains(final, "LLMD-V5")
	env.contains(final, "Documentation")
	env.contains(final, "AI assistants")

	// Verify we can still access early versions
	v1 := env.run("cat", "-v", "1", "docs/guide")
	env.contains(v1, "LLMD Guide")
	env.contains(v1, "Quick Start")

	v10 := env.run("cat", "-v", "10", "docs/guide")
	env.contains(v10, "LLMD-V5")
	env.contains(v10, "Getting Started") // Changed in edit 9
}

func TestEdit_LLM_ByteForByteIntegrity(t *testing.T) {
	env := newTestEnv(t)

	// Create document with exact known content
	original := "Line 1: exact content here\n" +
		"Line 2: more precise text\n" +
		"Line 3: final line no newline"

	env.runStdin(original, "write", "docs/exact")

	// Single targeted edit
	env.run("edit", "docs/exact", "exact content", "modified content", "-a", "llm")

	expected := "Line 1: modified content here\n" +
		"Line 2: more precise text\n" +
		"Line 3: final line no newline"

	out := env.run("cat", "docs/exact")
	env.equals(out, expected)

	// Verify original still accessible
	v1 := env.run("cat", "-v", "1", "docs/exact")
	env.equals(v1, original)
}

func TestEdit_LLM_EntireDocumentLineRange(t *testing.T) {
	env := newTestEnv(t)
	guide := testGuideContent()
	env.runStdin(guide, "write", "docs/guide")

	// Replace entire document via line range
	newDoc := `# Completely New Document

All previous content has been replaced.

## New Section 1

New content here.

## New Section 2

More new content.
`

	env.runStdin(newDoc, "edit", "docs/guide", "-l", "1:133", "-a", "full-rewrite")

	out := env.run("cat", "docs/guide")
	env.contains(out, "# Completely New Document")
	env.contains(out, "All previous content has been replaced")
	env.contains(out, "## New Section 1")

	// Original still accessible
	v1 := env.run("cat", "-v", "1", "docs/guide")
	env.contains(v1, "# LLMD Guide")
	env.contains(v1, "## Quick Start")

	history := env.run("history", "docs/guide")
	env.contains(history, "full-rewrite")
}

func TestEdit_LLM_EmptyLineRangeReplacement(t *testing.T) {
	env := newTestEnv(t)

	doc := "Line 1\nLine 2\nLine 3\nLine 4\nLine 5\n"
	env.runStdin(doc, "write", "docs/lines")

	// Replace lines 2-4 with nothing (delete)
	env.runStdin("", "edit", "docs/lines", "-l", "2:4")

	out := env.run("cat", "docs/lines")
	env.contains(out, "Line 1")
	env.contains(out, "Line 5")
	if strings.Contains(out, "Line 2") || strings.Contains(out, "Line 3") || strings.Contains(out, "Line 4") {
		t.Error("Edit(-l 2:4 empty) did not delete lines")
	}
}

func TestEdit_LLM_InsertViaLineRange(t *testing.T) {
	env := newTestEnv(t)

	doc := "# Header\n\nExisting content.\n"
	env.runStdin(doc, "write", "docs/insert")

	// Insert after line 2 by replacing line 3 with more content
	newContent := "Existing content.\n\n## New Section\n\nInserted via line range.\n"
	env.runStdin(newContent, "edit", "docs/insert", "-l", "3:3")

	out := env.run("cat", "docs/insert")
	env.contains(out, "# Header")
	env.contains(out, "Existing content")
	env.contains(out, "## New Section")
	env.contains(out, "Inserted via line range")
}
