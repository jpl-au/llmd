//go:build wasip1

// This file implements the grep command for searching documents.
package commands

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/jpl-au/llmd/sdk"
)

// Grep defines the grep command for searching documents.
//
// The grep command searches document content using FTS5 full-text search.
// It returns matching documents with their path, line number, and content.
//
// MCPName is set to "llmd_grep" to avoid conflicts with existing grep tools
// that AI assistants may have access to.
var Grep = sdk.Command{
	Name:        "grep",
	Description: "Search documents with full-text search",
	Usage:       "grep [options] <pattern> [path]",
	MCPEnabled:  true,
	MCPName:     "llmd_grep",
	Flags: []sdk.Flag{
		{Name: "n", Type: "bool", Description: "Show line numbers"},
		{Name: "i", Type: "bool", Description: "Ignore case"},
		{Name: "l", Type: "bool", Description: "Show only filenames"},
		{Name: "c", Type: "bool", Description: "Show match count only"},
		{Name: "C", Type: "int", Description: "Lines of context"},
		{Name: "A", Type: "int", Description: "Lines after match"},
		{Name: "B", Type: "int", Description: "Lines before match"},
	},
}

// ExecGrep executes the grep command.
//
// Searches all documents for content matching the FTS5 query.
func ExecGrep(ctx sdk.Context, args []string, flags map[string]any) (sdk.Result, error) {
	// Parse args
	var pattern, pathPrefix string
	var showLineNums, ignoreCase, filesOnly, countOnly bool
	var contextLines, afterLines, beforeLines int

	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "-n":
			showLineNums = true
		case arg == "-i":
			ignoreCase = true
		case arg == "-l":
			filesOnly = true
		case arg == "-c":
			countOnly = true
		case arg == "-C" && i+1 < len(args):
			i++
			contextLines, _ = strconv.Atoi(args[i])
		case strings.HasPrefix(arg, "-C"):
			contextLines, _ = strconv.Atoi(arg[2:])
		case arg == "-A" && i+1 < len(args):
			i++
			afterLines, _ = strconv.Atoi(args[i])
		case strings.HasPrefix(arg, "-A"):
			afterLines, _ = strconv.Atoi(arg[2:])
		case arg == "-B" && i+1 < len(args):
			i++
			beforeLines, _ = strconv.Atoi(args[i])
		case strings.HasPrefix(arg, "-B"):
			beforeLines, _ = strconv.Atoi(arg[2:])
		case !strings.HasPrefix(arg, "-"):
			if pattern == "" {
				pattern = arg
			} else {
				pathPrefix = arg
			}
		}
	}

	if pattern == "" {
		return nil, fmt.Errorf("grep: missing pattern argument")
	}

	// Use -C if -A/-B not specified
	if contextLines > 0 {
		if afterLines == 0 {
			afterLines = contextLines
		}
		if beforeLines == 0 {
			beforeLines = contextLines
		}
	}
	maxContext := afterLines
	if beforeLines > maxContext {
		maxContext = beforeLines
	}

	results, err := sdk.Host.Grep(pattern, sdk.GrepOptions{
		Path:         pathPrefix,
		IgnoreCase:   ignoreCase,
		ContextLines: maxContext,
	})
	if err != nil {
		return nil, fmt.Errorf("grep: %w", err)
	}

	if len(results) == 0 {
		return sdk.RichResult{Text: "", Data: []sdk.GrepResult{}}, nil
	}

	// Build output based on flags
	var text string
	if countOnly {
		text = formatCount(results)
	} else if filesOnly {
		text = formatFilesOnly(results)
	} else {
		text = formatMatches(results, showLineNums, beforeLines, afterLines)
	}

	return sdk.RichResult{Text: text, Data: results}, nil
}

func formatCount(results []sdk.GrepResult) string {
	// Count per file
	counts := make(map[string]int)
	for _, r := range results {
		counts[r.Path]++
	}
	var out strings.Builder
	for path, count := range counts {
		fmt.Fprintf(&out, "%s:%d\n", path, count)
	}
	return strings.TrimSuffix(out.String(), "\n")
}

func formatFilesOnly(results []sdk.GrepResult) string {
	seen := make(map[string]bool)
	var paths []string
	for _, r := range results {
		if !seen[r.Path] {
			seen[r.Path] = true
			paths = append(paths, r.Path)
		}
	}
	return strings.Join(paths, "\n")
}

func formatMatches(results []sdk.GrepResult, showLineNums bool, beforeLines, afterLines int) string {
	var out strings.Builder
	for i, r := range results {
		// Context before
		if beforeLines > 0 && len(r.ContextBefore) > 0 {
			start := len(r.ContextBefore) - beforeLines
			if start < 0 {
				start = 0
			}
			for j := start; j < len(r.ContextBefore); j++ {
				lineNum := r.Line - (len(r.ContextBefore) - j)
				if showLineNums {
					fmt.Fprintf(&out, "%s-%d-%s\n", r.Path, lineNum, r.ContextBefore[j])
				} else {
					fmt.Fprintf(&out, "%s-%s\n", r.Path, r.ContextBefore[j])
				}
			}
		}

		// Match line
		if showLineNums {
			fmt.Fprintf(&out, "%s:%d:%s\n", r.Path, r.Line, r.Content)
		} else {
			fmt.Fprintf(&out, "%s:%s\n", r.Path, r.Content)
		}

		// Context after
		if afterLines > 0 && len(r.ContextAfter) > 0 {
			end := afterLines
			if end > len(r.ContextAfter) {
				end = len(r.ContextAfter)
			}
			for j := 0; j < end; j++ {
				lineNum := r.Line + j + 1
				if showLineNums {
					fmt.Fprintf(&out, "%s-%d-%s\n", r.Path, lineNum, r.ContextAfter[j])
				} else {
					fmt.Fprintf(&out, "%s-%s\n", r.Path, r.ContextAfter[j])
				}
			}
		}

		// Separator between matches (if context shown)
		if (beforeLines > 0 || afterLines > 0) && i < len(results)-1 {
			out.WriteString("--\n")
		}
	}
	return strings.TrimSuffix(out.String(), "\n")
}
