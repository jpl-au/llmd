package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLLM_ComprehensiveEditingWorkflow tests a realistic LLM editing workflow
// with multiple large edits, version verification, and diff validation.
// This is NOT a unit test - it's an integration test that verifies the entire
// system works correctly under realistic LLM usage patterns.
func TestLLM_ComprehensiveEditingWorkflow(t *testing.T) {
	env := newTestEnv(t)

	// =========================================================================
	// PHASE 1: Initial document creation
	// =========================================================================
	t.Log("Phase 1: Creating initial document")

	env.runStdin(LLMTestDoc_V1, "write", "docs/api", "-a", "human", "-m", "Initial API documentation")

	// Verify V1 content
	v1Content := env.run("cat", "docs/api")
	env.contains(v1Content, "# API Documentation")
	env.contains(v1Content, "## Authentication")
	env.contains(v1Content, "Bearer tokens")
	env.contains(v1Content, "## Endpoints")
	env.contains(v1Content, "Rate limit headers")

	// =========================================================================
	// PHASE 2: LLM replaces Quick Start section (large edit via line range)
	// =========================================================================
	t.Log("Phase 2: LLM replaces entire Quick Start section")

	// First, let's add a Quick Start section to the doc
	docWithQuickStart := LLMTestDoc_V1 + "\n## Quick Start\n\nBasic quick start info here.\n"
	env.runStdin(docWithQuickStart, "write", "docs/api", "-a", "human", "-m", "Added Quick Start")

	// Now LLM completely rewrites the Quick Start section
	// The section starts around line 75 in the combined doc
	env.runStdin(LLMTestDoc_V2_QuickStartReplacement, "edit", "docs/api", "-l", "75:78", "-a", "claude-opus", "-m", "Completely rewrote Quick Start section")

	v3Content := env.run("cat", "docs/api")
	env.contains(v3Content, "## Quick Start Guide")
	env.contains(v3Content, "### Prerequisites")
	env.contains(v3Content, "Go 1.21 or later")
	env.contains(v3Content, "### Your First Document Store")
	// Original content still there
	env.contains(v3Content, "## Authentication")
	env.contains(v3Content, "Bearer tokens")

	// =========================================================================
	// PHASE 3: LLM replaces Commands section with expanded version
	// =========================================================================
	t.Log("Phase 3: LLM expands Commands section")

	env.run("edit", "docs/api", "## Endpoints", "## Endpoints (Legacy)", "-a", "claude-opus")

	// Add the new commands section
	currentContent := env.run("cat", "docs/api")
	newContent := currentContent + "\n" + LLMTestDoc_V3_CommandsTableReplacement
	env.runStdin(newContent, "write", "docs/api", "-a", "claude-opus", "-m", "Added expanded commands reference")

	v5Content := env.run("cat", "docs/api")
	env.contains(v5Content, "## Available Commands")
	env.contains(v5Content, "### Core Commands")
	env.contains(v5Content, "### Search Commands")
	env.contains(v5Content, "### History Commands")
	env.contains(v5Content, "### Sync Commands")
	env.contains(v5Content, "| `grep` |")

	// =========================================================================
	// PHASE 4: LLM adds entirely new Advanced section
	// =========================================================================
	t.Log("Phase 4: LLM adds new Advanced section")

	currentContent = env.run("cat", "docs/api")
	newContent = currentContent + "\n" + LLMTestDoc_V4_NewSection
	env.runStdin(newContent, "write", "docs/api", "-a", "claude-opus", "-m", "Added advanced usage section")

	v6Content := env.run("cat", "docs/api")
	env.contains(v6Content, "## Advanced Usage")
	env.contains(v6Content, "### Working with Large Documents")
	env.contains(v6Content, "### Automation and Scripting")
	env.contains(v6Content, "### Integration with AI Assistants")
	env.contains(v6Content, "set -euo pipefail")

	// =========================================================================
	// PHASE 5: Complete document rewrite
	// =========================================================================
	t.Log("Phase 5: Complete document rewrite")

	env.runStdin(LLMTestDoc_V5_CompleteRewrite, "write", "docs/api", "-a", "claude-opus", "-m", "Complete documentation rewrite v2.0")

	v7Content := env.run("cat", "docs/api")
	env.contains(v7Content, "# LLMD Reference Manual")
	env.contains(v7Content, "**Version 2.0 - Complete Rewrite**")
	env.contains(v7Content, "## Table of Contents")
	env.contains(v7Content, "## Core Concepts")
	env.contains(v7Content, "## Best Practices")

	// =========================================================================
	// PHASE 6: Verify version history integrity
	// =========================================================================
	t.Log("Phase 6: Verifying version history")

	history := env.run("history", "docs/api")
	env.contains(history, "v1")
	env.contains(history, "v7")
	env.contains(history, "human")
	env.contains(history, "claude-opus")
	env.contains(history, "Initial API documentation")
	env.contains(history, "Complete documentation rewrite")

	// =========================================================================
	// PHASE 7: Verify all versions are retrievable and correct
	// =========================================================================
	t.Log("Phase 7: Verifying all versions are retrievable")

	// V1 should be original
	v1Check := env.run("cat", "-v", "1", "docs/api")
	env.contains(v1Check, "# API Documentation")
	env.contains(v1Check, "Bearer tokens")
	if strings.Contains(v1Check, "Quick Start Guide") {
		t.Error("V1 should not contain Quick Start Guide")
	}

	// V7 should be complete rewrite
	v7Check := env.run("cat", "-v", "7", "docs/api")
	env.contains(v7Check, "# LLMD Reference Manual")
	env.contains(v7Check, "Version 2.0")
	if strings.Contains(v7Check, "# API Documentation") {
		t.Error("V7 should not contain API Documentation header")
	}

	// =========================================================================
	// PHASE 8: Verify diff between versions
	// =========================================================================
	t.Log("Phase 8: Verifying diff functionality")

	// Diff V1 to V7 - should show major changes
	diff1to7 := env.run("diff", "docs/api", "-v", "1:7")
	env.contains(diff1to7, "API Documentation")
	env.contains(diff1to7, "LLMD Reference Manual")

	// Diff V6 to V7 - should show the rewrite
	diff6to7 := env.run("diff", "docs/api", "-v", "6:7")
	env.contains(diff6to7, "v6")
	env.contains(diff6to7, "v7")

	// =========================================================================
	// PHASE 9: Test grep across this complex document
	// =========================================================================
	t.Log("Phase 9: Testing search functionality")

	grepResult := env.run("grep", "authentication", "docs/api")
	env.contains(grepResult, "docs/api")

	grepResult = env.run("grep", "-i", "version", "docs/api")
	env.contains(grepResult, "docs/api")

	// =========================================================================
	// PHASE 10: Test find (full-text search)
	// =========================================================================
	t.Log("Phase 10: Testing full-text search")

	findResult := env.run("find", "Reference Manual")
	env.contains(findResult, "docs/api")

	// =========================================================================
	// Summary verification
	// =========================================================================
	t.Log("All phases completed successfully")
}

// TestLLM_FilesystemDiffIntegration tests that diff works correctly
// against the actual filesystem guide.md
func TestLLM_FilesystemDiffIntegration(t *testing.T) {
	env := newTestEnv(t)
	guide := testGuideContent()

	// =========================================================================
	// PHASE 1: Import the actual guide.md from filesystem
	// =========================================================================
	t.Log("Phase 1: Importing guide.md from filesystem")

	env.runStdin(guide, "write", "docs/guide", "-a", "import", "-m", "Imported from guide/guide.md")

	// Verify it matches
	imported := env.run("cat", "docs/guide")
	if imported != guide {
		t.Errorf("Imported content doesn't match guide.md exactly.\nExpected length: %d\nGot length: %d",
			len(guide), len(imported))
	}

	// =========================================================================
	// PHASE 2: Make a series of large LLM-style edits
	// =========================================================================
	t.Log("Phase 2: Making large LLM-style edits")

	// Edit 1: Replace the entire Quick Start section (lines 5-12)
	env.runStdin(LLMTestDoc_V2_QuickStartReplacement, "edit", "docs/guide", "-l", "5:12", "-a", "claude", "-m", "Rewrote Quick Start")

	// Edit 2: Replace the Commands section (lines 14-41)
	env.runStdin(LLMTestDoc_V3_CommandsTableReplacement, "edit", "docs/guide", "-l", "14:41", "-a", "claude", "-m", "Expanded Commands")

	// Edit 3: Add Advanced section at the end
	v3Content := env.run("cat", "docs/guide")
	env.runStdin(v3Content+"\n"+LLMTestDoc_V4_NewSection, "write", "docs/guide", "-a", "claude", "-m", "Added Advanced section")

	// =========================================================================
	// PHASE 3: Verify we can diff back to original
	// =========================================================================
	t.Log("Phase 3: Verifying diff to original version")

	// Should have 4 versions now
	history := env.run("history", "docs/guide")
	env.contains(history, "v4")

	// Diff V1 (original) to V4 (current)
	diff1to4 := env.run("diff", "docs/guide", "-v", "1:4")

	// Original guide has "## Quick Start", new has "## Quick Start Guide"
	env.contains(diff1to4, "Quick Start")

	// =========================================================================
	// PHASE 4: Verify original is still exactly the filesystem version
	// =========================================================================
	t.Log("Phase 4: Verifying V1 matches filesystem exactly")

	v1Content := env.run("cat", "-v", "1", "docs/guide")
	if v1Content != guide {
		t.Errorf("V1 doesn't match original guide.md.\nExpected length: %d\nGot length: %d",
			len(guide), len(v1Content))
	}

	// =========================================================================
	// PHASE 5: Export and verify against filesystem
	// =========================================================================
	t.Log("Phase 5: Testing export functionality")

	exportDir := filepath.Join(env.dir, "exported")
	env.run("export", "docs/", exportDir)

	// Read the exported file
	exportedPath := filepath.Join(exportDir, "guide.md")
	exportedContent, err := os.ReadFile(exportedPath)
	if err != nil {
		t.Fatalf("Failed to read exported file: %v", err)
	}

	// Current version should match exported
	currentContent := env.run("cat", "docs/guide")
	if string(exportedContent) != currentContent {
		t.Error("Exported content doesn't match current version")
	}
}

// TestLLM_MassiveVersionHistory tests creating and accessing many versions
func TestLLM_MassiveVersionHistory(t *testing.T) {
	env := newTestEnv(t)

	// Create initial document
	env.runStdin(LLMTestDoc_V1, "write", "docs/massive", "-a", "init")

	// Make 20 large edits, each replacing significant content
	sections := []string{
		LLMTestDoc_V2_QuickStartReplacement,
		LLMTestDoc_V3_CommandsTableReplacement,
		LLMTestDoc_V4_NewSection,
		LLMTestDoc_V5_CompleteRewrite,
	}

	for i := range 20 {
		section := sections[i%len(sections)]
		// Append the section to create a growing document
		current := env.run("cat", "docs/massive")
		marker := strings.Repeat("=", 40) + "\n## Iteration " + string(rune('A'+i)) + "\n"
		env.runStdin(current+"\n"+marker+section, "write", "docs/massive", "-a", "llm-iteration-"+string(rune('A'+i)))
	}

	// Should have 21 versions now
	history := env.run("history", "docs/massive")
	env.contains(history, "v21")
	env.contains(history, "llm-iteration-A")
	env.contains(history, "llm-iteration-T")

	// Verify we can access any version
	v1 := env.run("cat", "-v", "1", "docs/massive")
	env.contains(v1, "# API Documentation")

	v10 := env.run("cat", "-v", "10", "docs/massive")
	env.contains(v10, "## Iteration")

	v21 := env.run("cat", "-v", "21", "docs/massive")
	env.contains(v21, "## Iteration T")

	// Diff across many versions
	diff1to21 := env.run("diff", "docs/massive", "-v", "1:21")
	env.contains(diff1to21, "v1")
	env.contains(diff1to21, "v21")

	// Search should work across this large document
	grepResult := env.run("grep", "authentication", "docs/massive")
	env.contains(grepResult, "docs/massive")
}

// TestLLM_MultipleDocumentsSimultaneous tests editing multiple documents
// in a realistic LLM workflow where it's working on several files
func TestLLM_MultipleDocumentsSimultaneous(t *testing.T) {
	env := newTestEnv(t)

	// Create multiple documents that an LLM might work on
	docs := map[string]string{
		"docs/api/overview":      LLMTestDoc_V1,
		"docs/api/auth":          LLMTestDoc_V2_QuickStartReplacement,
		"docs/guides/quickstart": LLMTestDoc_V3_CommandsTableReplacement,
		"docs/guides/advanced":   LLMTestDoc_V4_NewSection,
		"docs/reference/manual":  LLMTestDoc_V5_CompleteRewrite,
	}

	// Create all documents
	for path, content := range docs {
		env.runStdin(content, "write", path, "-a", "claude-init")
	}

	// Edit each document multiple times
	for path := range docs {
		for i := range 3 {
			current := env.run("cat", path)
			env.runStdin(current+"\n\n## Update "+string(rune('1'+i))+"\n\nAdditional content.\n",
				"write", path, "-a", "claude-update-"+string(rune('1'+i)))
		}
	}

	// Verify each document has 4 versions
	for path := range docs {
		history := env.run("history", path)
		env.contains(history, "v4")
		env.contains(history, "claude-init")
		env.contains(history, "claude-update-3")
	}

	// List all documents
	lsResult := env.run("ls", "docs/")
	env.contains(lsResult, "docs/api/overview")
	env.contains(lsResult, "docs/api/auth")
	env.contains(lsResult, "docs/guides/quickstart")
	env.contains(lsResult, "docs/guides/advanced")
	env.contains(lsResult, "docs/reference/manual")

	// Grep across all documents
	grepResult := env.run("grep", "-r", "content", "docs/")
	env.contains(grepResult, "docs/api")
	env.contains(grepResult, "docs/guides")
	env.contains(grepResult, "docs/reference")

	// Verify we can access any version of any document
	v1Overview := env.run("cat", "-v", "1", "docs/api/overview")
	env.contains(v1Overview, "# API Documentation")

	v4Manual := env.run("cat", "-v", "4", "docs/reference/manual")
	env.contains(v4Manual, "## Update 3")
}

// TestLLM_SearchAndReplaceChain tests a chain of search-and-replace
// operations that an LLM might perform across a document
func TestLLM_SearchAndReplaceChain(t *testing.T) {
	env := newTestEnv(t)
	guide := testGuideContent()

	env.runStdin(guide, "write", "docs/guide", "-a", "import")

	// Chain of realistic LLM edits - each building on the previous
	edits := []struct {
		old, new, author, message string
	}{
		// Terminology changes
		{"document store", "documentation system", "claude", "Updated terminology"},
		{"documentation system", "document management platform", "claude", "Refined terminology"},

		// Branding changes
		{"LLMD Guide", "LLMD User Manual", "claude", "Updated title"},
		{"LLMD User Manual", "LLMD Documentation Portal", "claude", "Rebranded"},

		// Technical updates
		{"llmd init", "llmd initialise", "claude", "Standardised command names"},
		{"llmd cat", "llmd read", "claude", "Made command more intuitive"},

		// Section renames
		{"Quick Start", "Getting Started", "claude", "More descriptive heading"},
		{"For LLMs", "AI Assistant Integration", "claude", "Professional section name"},
	}

	for _, e := range edits {
		env.run("edit", "docs/guide", e.old, e.new, "-a", e.author, "-m", e.message)
	}

	// Verify final state has all changes
	final := env.run("cat", "docs/guide")
	env.contains(final, "document management platform")
	env.contains(final, "LLMD Documentation Portal")
	env.contains(final, "llmd initialise")
	env.contains(final, "Getting Started")
	env.contains(final, "AI Assistant Integration")

	// Verify we have all versions
	history := env.run("history", "docs/guide")
	env.contains(history, "v9") // 1 original + 8 edits

	// Verify original is unchanged
	v1 := env.run("cat", "-v", "1", "docs/guide")
	env.contains(v1, "document store")
	env.contains(v1, "LLMD Guide")
	env.contains(v1, "llmd init")
	env.contains(v1, "Quick Start")
	env.contains(v1, "For LLMs")

	// Diff should show the progression
	diff := env.run("diff", "docs/guide", "-v", "1:9")
	env.contains(diff, "Guide")                // Original title word
	env.contains(diff, "Documentation Portal") // New title
	env.contains(diff, "v1")
	env.contains(diff, "v9")
}

// TestLLM_LineRangeLargeReplacement tests replacing large sections via line ranges
func TestLLM_LineRangeLargeReplacement(t *testing.T) {
	env := newTestEnv(t)
	guide := testGuideContent()

	env.runStdin(guide, "write", "docs/guide", "-a", "import")

	// Replace lines 5-42 (Quick Start through Commands table) with new content
	env.runStdin(LLMTestDoc_V2_QuickStartReplacement+"\n\n"+LLMTestDoc_V3_CommandsTableReplacement,
		"edit", "docs/guide", "-l", "5:42", "-a", "claude", "-m", "Rewrote quick start and commands sections")

	v2Content := env.run("cat", "docs/guide")
	env.contains(v2Content, "## Quick Start Guide")
	env.contains(v2Content, "### Prerequisites")
	env.contains(v2Content, "## Available Commands")
	env.contains(v2Content, "### Core Commands")
	// Header should still be there
	env.contains(v2Content, "# LLMD Guide")
	// End sections should still be there
	env.contains(v2Content, "## Document Paths")

	// Now replace the end section entirely
	env.runStdin(LLMTestDoc_V4_NewSection, "edit", "docs/guide", "-l", "100:133", "-a", "claude", "-m", "Replaced end with Advanced section")

	v3Content := env.run("cat", "docs/guide")
	env.contains(v3Content, "## Advanced Usage")
	env.contains(v3Content, "### Working with Large Documents")

	// Verify all versions accessible
	v1 := env.run("cat", "-v", "1", "docs/guide")
	if v1 != guide {
		t.Error("V1 should match original guide.md exactly")
	}

	// Diff should show massive changes
	diff := env.run("diff", "docs/guide", "-v", "1:3")
	env.contains(diff, "v1")
	env.contains(diff, "v3")
}
