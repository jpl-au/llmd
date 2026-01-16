package cmd

import (
	"strconv"
	"strings"
	"testing"
)

func TestWrite(t *testing.T) {
	t.Run("basic write and read", func(t *testing.T) {
		env := newTestEnv(t)
		content := "# Hello World\n\nThis is a test document."

		env.runStdin(content, "write", "docs/readme")

		out := env.run("cat", "docs/readme")
		env.equals(out, content)
	})

	t.Run("nested path", func(t *testing.T) {
		env := newTestEnv(t)
		content := "Deep nested content"

		env.runStdin(content, "write", "docs/api/v2/endpoints/users")

		out := env.run("cat", "docs/api/v2/endpoints/users")
		env.equals(out, content)

		out = env.run("ls", "docs/api/")
		env.contains(out, "docs/api/v2/endpoints/users")
	})

	t.Run("empty content rejected", func(t *testing.T) {
		env := newTestEnv(t)

		_, err := env.runErr("write", "docs/empty")
		if err == nil {
			t.Error("write with empty content should fail")
		}
	})

	t.Run("special characters", func(t *testing.T) {
		env := newTestEnv(t)
		content := "Special chars: <>&\"' and unicode: 你好 🎉"

		env.runStdin(content, "write", "docs/special")

		out := env.run("cat", "docs/special")
		env.equals(out, content)
	})

	t.Run("JSON output", func(t *testing.T) {
		env := newTestEnv(t)

		out := env.runStdin("content", "write", "docs/json", "-o", "json")
		env.contains(out, `"path"`)
		env.contains(out, `"docs/json"`)
	})
}

func TestWrite_Metadata(t *testing.T) {
	t.Run("with author", func(t *testing.T) {
		env := newTestEnv(t)

		env.runStdin("content", "write", "docs/readme", "-a", "claude")

		out := env.run("history", "docs/readme")
		env.contains(out, "claude")
	})

	t.Run("with message", func(t *testing.T) {
		env := newTestEnv(t)

		env.runStdin("content", "write", "docs/readme", "-m", "Initial commit")

		out := env.run("history", "docs/readme")
		env.contains(out, "Initial commit")
	})
}

func TestWrite_Versions(t *testing.T) {
	t.Run("multiple versions", func(t *testing.T) {
		env := newTestEnv(t)

		env.runStdin("Version 1", "write", "docs/readme")
		env.runStdin("Version 2", "write", "docs/readme")
		env.runStdin("Version 3", "write", "docs/readme")

		out := env.run("cat", "docs/readme")
		env.equals(out, "Version 3")

		out = env.run("history", "docs/readme")
		env.contains(out, "v1")
		env.contains(out, "v2")
		env.contains(out, "v3")
	})

	t.Run("overwrite preserves history", func(t *testing.T) {
		env := newTestEnv(t)

		env.runStdin("Original content", "write", "docs/readme")
		env.runStdin("Completely different content", "write", "docs/readme")

		out := env.run("cat", "docs/readme")
		env.equals(out, "Completely different content")

		out = env.run("cat", "-v", "1", "docs/readme")
		env.equals(out, "Original content")
	})
}

func TestWrite_LargeContent(t *testing.T) {
	env := newTestEnv(t)

	var b strings.Builder
	for i := range 100 {
		b.WriteString("This is line number ")
		b.WriteString(strconv.Itoa(i % 10))
		b.WriteString(" of the document.\n")
	}
	content := b.String()

	env.runStdin(content, "write", "docs/large")

	out := env.run("cat", "docs/large")
	env.contains(out, "This is line number")
}

// Advanced write tests for LLM use cases

func TestWrite_LLM_VeryLargeDocument(t *testing.T) {
	env := newTestEnv(t)

	// Generate a 1000-line document
	var b strings.Builder
	b.WriteString("# Very Large Document\n\n")

	for section := range 50 {
		b.WriteString("## Section ")
		b.WriteString(strconv.Itoa(section + 1))
		b.WriteString("\n\n")

		for para := range 3 {
			b.WriteString("Paragraph ")
			b.WriteString(strconv.Itoa(para + 1))
			b.WriteString(" of section ")
			b.WriteString(strconv.Itoa(section + 1))
			b.WriteString(". This is filler content to make the document larger. ")
			b.WriteString("It contains multiple sentences per paragraph.\n\n")
		}

		b.WriteString("```\n")
		b.WriteString("Code block in section ")
		b.WriteString(strconv.Itoa(section + 1))
		b.WriteString("\n```\n\n")
	}

	content := b.String()
	env.runStdin(content, "write", "docs/verylarge")

	out := env.run("cat", "docs/verylarge")
	env.contains(out, "# Very Large Document")
	env.contains(out, "## Section 1")
	env.contains(out, "## Section 50")
	env.contains(out, "Code block in section 50")

	// Verify we can update it
	env.runStdin(content+"## Final Section\n\nAppended content.\n", "write", "docs/verylarge")

	out = env.run("cat", "docs/verylarge")
	env.contains(out, "## Final Section")
	env.contains(out, "Appended content")

	// Verify history
	history := env.run("history", "docs/verylarge")
	env.contains(history, "v2")
}

func TestWrite_LLM_CompleteRewritePreservesHistory(t *testing.T) {
	env := newTestEnv(t)
	guide := testGuideContent()

	// Write original
	env.runStdin(guide, "write", "docs/guide", "-a", "original-author")

	// Complete rewrite with different structure
	newContent := `# Completely Different Document

This document has entirely different content.

## New Structure

The structure is also different.

## Another Section

More different content here.
`
	env.runStdin(newContent, "write", "docs/guide", "-a", "rewrite-author", "-m", "Complete rewrite")

	// Third version
	env.runStdin(newContent+"## Added Section\n\nMore content.\n", "write", "docs/guide", "-a", "update-author")

	// Verify current version
	out := env.run("cat", "docs/guide")
	env.contains(out, "## Added Section")

	// Verify all versions accessible
	v1 := env.run("cat", "-v", "1", "docs/guide")
	env.contains(v1, "# LLMD Guide")
	env.contains(v1, "## Quick Start")

	v2 := env.run("cat", "-v", "2", "docs/guide")
	env.contains(v2, "# Completely Different Document")
	if strings.Contains(v2, "## Added Section") {
		t.Error("v2 should not contain Added Section")
	}

	// Verify history
	history := env.run("history", "docs/guide")
	env.contains(history, "original-author")
	env.contains(history, "rewrite-author")
	env.contains(history, "update-author")
	env.contains(history, "Complete rewrite")
}

func TestWrite_LLM_BinaryLikeContent(t *testing.T) {
	env := newTestEnv(t)

	// Content with various special bytes (but valid UTF-8)
	content := "Normal text\n" +
		"Line with null: \x00 (should be preserved)\n" +
		"Line with bell: \x07\n" +
		"Line with backspace: \x08\n" +
		"Line with tab: \t (tab)\n" +
		"Line with vertical tab: \x0b\n" +
		"Line with form feed: \x0c\n" +
		"Line with carriage return: \r\n" +
		"End of content"

	env.runStdin(content, "write", "docs/binary")

	out := env.run("cat", "docs/binary")
	env.contains(out, "Normal text")
	env.contains(out, "End of content")
}

func TestWrite_LLM_ManyVersions(t *testing.T) {
	env := newTestEnv(t)

	// Create 30 versions
	for i := range 30 {
		content := "# Document Version " + strconv.Itoa(i+1) + "\n\n" +
			"This is version " + strconv.Itoa(i+1) + " of the document.\n" +
			"Created in iteration " + strconv.Itoa(i+1) + ".\n"
		env.runStdin(content, "write", "docs/versions", "-a", "author-"+strconv.Itoa(i+1))
	}

	// Verify latest
	out := env.run("cat", "docs/versions")
	env.contains(out, "# Document Version 30")

	// Verify history shows all versions
	history := env.run("history", "docs/versions")
	env.contains(history, "v30")
	env.contains(history, "author-1")
	env.contains(history, "author-30")

	// Verify random access to versions
	v1 := env.run("cat", "-v", "1", "docs/versions")
	env.contains(v1, "# Document Version 1")

	v15 := env.run("cat", "-v", "15", "docs/versions")
	env.contains(v15, "# Document Version 15")

	v25 := env.run("cat", "-v", "25", "docs/versions")
	env.contains(v25, "# Document Version 25")
}

func TestWrite_LLM_UnicodeContent(t *testing.T) {
	env := newTestEnv(t)

	content := `# Unicode Document

## Languages

English: Hello, World!
Chinese: 你好世界
Japanese: こんにちは世界
Korean: 안녕하세요 세계
Arabic: مرحبا بالعالم
Hebrew: שלום עולם
Russian: Привет мир
Greek: Γειά σου κόσμε
Thai: สวัสดีชาวโลก

## Emoji

🎉 Party 🎊 Celebration 🥳
👨‍💻 Developer 👩‍💻 Coder
🚀 Rocket 🌟 Star ⭐
❤️ Heart 💙 Blue Heart 💚 Green Heart

## Mathematical Symbols

∑ (sum) ∏ (product) ∫ (integral)
√ (sqrt) ∞ (infinity) ≈ (approximately)
≤ ≥ ≠ ± × ÷

## Currency

$ € £ ¥ ₹ ₽ ₿

## Special Characters

© ® ™ § ¶ † ‡ • ° ± × ÷
`

	env.runStdin(content, "write", "docs/unicode")

	out := env.run("cat", "docs/unicode")
	env.contains(out, "你好世界")
	env.contains(out, "こんにちは世界")
	env.contains(out, "مرحبا بالعالم")
	env.contains(out, "🎉 Party")
	env.contains(out, "∑ (sum)")
	env.contains(out, "₿")

	// Verify searchable
	grepOut := env.run("grep", "Party", "docs/unicode")
	env.contains(grepOut, "docs/unicode")
}

func TestWrite_LLM_ExactContentPreservation(t *testing.T) {
	env := newTestEnv(t)

	// Exact content that must be preserved byte-for-byte
	exact := "First line\n" +
		"Second line with trailing space \n" +
		"Third line with\ttab\n" +
		"    Fourth line indented\n" +
		"\tFifth line tab indented\n" +
		"\n" +
		"\n" +
		"After blank lines\n" +
		"Final line without newline"

	env.runStdin(exact, "write", "docs/exact")

	out := env.run("cat", "docs/exact")
	env.equals(out, exact)

	// Write again and verify
	env.runStdin(exact, "write", "docs/exact")
	out = env.run("cat", "docs/exact")
	env.equals(out, exact)
}

func TestWrite_LLM_CodeHeavyDocument(t *testing.T) {
	env := newTestEnv(t)

	// Document with lots of code blocks in various languages
	content := "# Code Examples\n\n" +
		"## Go\n\n" +
		"```go\n" +
		"package main\n\n" +
		"import (\n" +
		"\t\"fmt\"\n" +
		"\t\"os\"\n" +
		")\n\n" +
		"func main() {\n" +
		"\tfmt.Println(\"Hello, World!\")\n" +
		"\tos.Exit(0)\n" +
		"}\n" +
		"```\n\n" +
		"## Python\n\n" +
		"```python\n" +
		"#!/usr/bin/env python3\n" +
		"import sys\n\n" +
		"def main():\n" +
		"    print(\"Hello, World!\")\n" +
		"    sys.exit(0)\n\n" +
		"if __name__ == \"__main__\":\n" +
		"    main()\n" +
		"```\n\n" +
		"## JavaScript\n\n" +
		"```javascript\n" +
		"const greet = (name) => {\n" +
		"    console.log(`Hello, ${name}!`);\n" +
		"};\n\n" +
		"greet('World');\n" +
		"```\n\n" +
		"## SQL\n\n" +
		"```sql\n" +
		"SELECT * FROM users\n" +
		"WHERE active = true\n" +
		"ORDER BY created_at DESC\n" +
		"LIMIT 10;\n" +
		"```\n\n" +
		"## Shell\n\n" +
		"```bash\n" +
		"#!/bin/bash\n" +
		"set -euo pipefail\n\n" +
		"echo \"Hello, World!\"\n" +
		"exit 0\n" +
		"```\n"

	env.runStdin(content, "write", "docs/code")

	out := env.run("cat", "docs/code")
	env.contains(out, "```go")
	env.contains(out, "```python")
	env.contains(out, "```javascript")
	env.contains(out, "```sql")
	env.contains(out, "```bash")
	env.contains(out, "fmt.Println")
	env.contains(out, "print(\"Hello")
	env.contains(out, "console.log")
	env.contains(out, "SELECT * FROM")
	env.contains(out, "set -euo pipefail")

	// Verify grep finds code
	grepOut := env.run("grep", "pipefail", "docs/code")
	env.contains(grepOut, "docs/code")
}
