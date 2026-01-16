// Package grep provides regex-based content search for documents.
//
// While FTS5 (find) handles natural language queries, grep provides precise
// pattern matching with familiar Unix semantics (-i, -v, -l, -c, -C flags).
// This enables exact matches and complex patterns that tokenised search cannot.
package grep

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/jpl-au/llmd/internal/service"
	"github.com/jpl-au/llmd/internal/store"
)

// Options configures a grep operation.
type Options struct {
	Path        string // Scope search to path prefix
	IncludeAll  bool   // Include deleted documents
	DeletedOnly bool   // Search only deleted documents
	PathsOnly   bool   // Only output paths (-l flag)
	IgnoreCase  bool   // Case insensitive search (-i flag)

	// Invert returns non-matching lines. Useful for LLMs filtering out noise
	// (e.g., "show me everything except import statements").
	Invert bool // Invert match (-v flag)

	// Context shows N lines around each match. LLMs need surrounding context
	// to understand matches without reading entire documents. Avoids wasting
	// context window on irrelevant content while still providing enough info
	// to make informed edits.
	Context int // Lines of context around matches (-C flag)

	// CountOnly outputs just the match count per document. Enables LLMs to
	// quickly assess scope ("how many TODOs?", "how many errors?") before
	// deciding whether to dive deeper.
	CountOnly bool // Only show count of matches (-c flag)

	// MaxLineLength is the maximum line length for scanning (0 = default 10MB).
	// Needed for documents with very long lines (minified JS, large JSON).
	MaxLineLength int
}

// Match represents a single line match within a document.
type Match struct {
	Line    int    // 1-indexed line number
	Content string // The matching line content
}

// DocMatch represents all matches within a single document.
type DocMatch struct {
	Document store.Document
	Matches  []Match
}

// Result contains the outcome of a grep operation.
type Result struct {
	Documents []store.Document // For backwards compatibility
	Hits      []DocMatch       // Detailed match info
}

// Run searches documents for a regex pattern and writes output to w.
func Run(ctx context.Context, w io.Writer, svc service.Service, pattern string, opts Options) (Result, error) {
	var result Result

	// Compile regex
	flags := ""
	if opts.IgnoreCase {
		flags = "(?i)"
	}
	re, err := regexp.Compile(flags + pattern)
	if err != nil {
		return result, fmt.Errorf("invalid regex: %w", err)
	}

	// Get all documents (filtered by path prefix)
	docs, err := svc.List(ctx, opts.Path, opts.IncludeAll, opts.DeletedOnly)
	if err != nil {
		return result, err
	}

	// Match each document
	for _, doc := range docs {
		matches, err := matchLines(re, doc.Content, opts.Invert, opts.MaxLineLength)
		if err != nil {
			return result, fmt.Errorf("scanning %s: %w", doc.Path, err)
		}
		if len(matches) > 0 {
			result.Documents = append(result.Documents, doc)
			result.Hits = append(result.Hits, DocMatch{
				Document: doc,
				Matches:  matches,
			})
		}
	}

	// Format output
	if opts.PathsOnly {
		for _, hit := range result.Hits {
			fmt.Fprintln(w, hit.Document.Path)
		}
	} else if opts.CountOnly {
		for _, hit := range result.Hits {
			fmt.Fprintf(w, "%s:%d\n", hit.Document.Path, len(hit.Matches))
		}
	} else if opts.Context > 0 {
		// Context output follows grep convention:
		// - ":" separates path:line:content for matching lines
		// - "-" separates path-line-content for context lines
		// - "--" separates non-contiguous match groups
		// This allows LLMs to distinguish matches from context at a glance.
		for _, hit := range result.Hits {
			lines := strings.Split(hit.Document.Content, "\n")
			printed := make(map[int]bool) // track printed lines to avoid duplicates when matches overlap
			needSep := false

			for _, m := range hit.Matches {
				start := m.Line - opts.Context - 1 // convert to 0-indexed
				if start < 0 {
					start = 0
				}
				end := m.Line + opts.Context // exclusive upper bound for 0-indexed loop
				if end > len(lines) {
					end = len(lines)
				}

				// Print separator if there's a gap from previous context
				if needSep && !printed[start] {
					fmt.Fprintln(w, "--")
				}

				for i := start; i < end; i++ {
					if printed[i] {
						continue
					}
					printed[i] = true
					lineNum := i + 1
					sep := "-" // context line
					if lineNum == m.Line {
						sep = ":" // matching line
					}
					fmt.Fprintf(w, "%s%s%d%s%s\n", hit.Document.Path, sep, lineNum, sep, lines[i])
				}
				needSep = true
			}
		}
	} else {
		for _, hit := range result.Hits {
			for _, m := range hit.Matches {
				fmt.Fprintf(w, "%s:%d:%s\n", hit.Document.Path, m.Line, m.Content)
			}
		}
	}

	return result, nil
}

// matchLines finds all lines matching the regex and returns Match structs.
// If invert is true, returns lines that do NOT match.
// Uses bufio.Scanner for memory efficiency - avoids allocating a slice of all
// lines upfront. Important when searching many large documents where most won't match.
func matchLines(re *regexp.Regexp, content string, invert bool, maxLineLength int) ([]Match, error) {
	var matches []Match
	if maxLineLength <= 0 {
		maxLineLength = 10 * 1024 * 1024 // 10MB default
	}
	scanner := bufio.NewScanner(strings.NewReader(content))
	scanner.Buffer(make([]byte, 64*1024), maxLineLength)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if re.MatchString(line) != invert {
			matches = append(matches, Match{
				Line:    lineNum,
				Content: line,
			})
		}
	}
	if err := scanner.Err(); err != nil {
		return matches, err
	}
	return matches, nil
}
