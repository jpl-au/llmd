// Package cat provides document reading with line range support.
//
// The StartLine/EndLine options are critical for LLM workflows - they can read
// just the relevant section (e.g., lines 50-70) without consuming context on
// the full document. Combined with grep output (which shows line numbers), this
// enables the workflow: grep -> find line -> cat -l to read context -> edit.
package cat

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/jpl-au/llmd/internal/service"
	"github.com/jpl-au/llmd/internal/store"
)

// minLineNumWidth is the minimum column width for line numbers.
// Ensures consistent formatting for typical documents.
const minLineNumWidth = 6

// Options configures a cat operation.
type Options struct {
	Version        int  // Specific version to read (0 = latest)
	IncludeDeleted bool // Allow reading deleted documents
	LineNumbers    bool // Show line numbers (-n flag)

	// StartLine and EndLine enable reading specific sections of large documents.
	// This is critical for LLMs working with large files - they can read just the
	// relevant section (e.g., lines 50-70) without consuming context on the full doc.
	// Combined with grep output (which shows line numbers), this enables the workflow:
	// grep -> find line -> cat -l to read context -> edit.
	StartLine int // First line to show (1-indexed, 0 = start)
	EndLine   int // Last line to show (1-indexed, 0 = end)

	// MaxLineLength is the maximum line length for scanning (0 = default 10MB).
	// Needed for documents with very long lines (minified JS, large JSON).
	MaxLineLength int
}

// Result contains the outcome of a cat operation.
type Result struct {
	Document *store.Document
}

// Run reads a document and writes its content to w.
func Run(ctx context.Context, w io.Writer, svc service.Service, path string, opts Options) (Result, error) {
	var result Result
	var doc *store.Document
	var err error

	if opts.Version > 0 {
		doc, err = svc.Version(ctx, path, opts.Version)
	} else {
		// Use Resolve to handle both paths and keys
		doc, _, err = svc.Resolve(ctx, path, opts.IncludeDeleted)
	}
	if err != nil {
		return result, err
	}

	result.Document = doc

	// Fast path: no line range and no line numbers - output content as-is
	if opts.StartLine == 0 && opts.EndLine == 0 && !opts.LineNumbers {
		fmt.Fprint(w, doc.Content)
		return result, nil
	}

	// Calculate line number width for alignment.
	// We need to know the max line number that will be displayed.
	// Count total lines (cheap O(n) scan) to size the width properly.
	totalLines := strings.Count(doc.Content, "\n") + 1
	if strings.HasSuffix(doc.Content, "\n") {
		totalLines-- // trailing newline doesn't add a line
	}

	maxLineNum := totalLines
	if opts.EndLine > 0 && opts.EndLine < maxLineNum {
		maxLineNum = opts.EndLine
	}
	lineNumWidth := len(strconv.Itoa(maxLineNum))
	if lineNumWidth < minLineNumWidth {
		lineNumWidth = minLineNumWidth
	}

	// Determine line range (1-indexed in opts, we track 1-indexed lineNum)
	start := 1
	end := totalLines
	if opts.StartLine > 0 {
		start = opts.StartLine
	}
	if opts.EndLine > 0 && opts.EndLine < end {
		end = opts.EndLine
	}

	// Use bufio.Scanner to avoid allocating a slice of all lines.
	// This is important for large documents - we skip lines we don't need
	// without creating string copies for every line.
	maxLine := opts.MaxLineLength
	if maxLine <= 0 {
		maxLine = 10 * 1024 * 1024 // 10MB default
	}
	scanner := bufio.NewScanner(strings.NewReader(doc.Content))
	scanner.Buffer(make([]byte, 64*1024), maxLine)
	lineNum := 0
	hasTrailingNewline := strings.HasSuffix(doc.Content, "\n")

	for scanner.Scan() {
		lineNum++
		if lineNum < start {
			continue // skip lines before range
		}
		if lineNum > end {
			break // done with range
		}

		line := scanner.Text()
		if opts.LineNumbers {
			fmt.Fprintf(w, "%*d\t%s", lineNumWidth, lineNum, line)
		} else {
			fmt.Fprint(w, line)
		}

		// Add newline: always between lines, and at end if original had trailing newline
		if lineNum < end {
			fmt.Fprintln(w)
		} else if hasTrailingNewline {
			fmt.Fprintln(w)
		}
	}

	// Check for scanner errors (e.g., lines exceeding 64KB buffer limit)
	if err := scanner.Err(); err != nil {
		return result, fmt.Errorf("reading content: %w", err)
	}

	return result, nil
}
