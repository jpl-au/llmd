// Package diff provides simple text diff utilities used by llmd to compute
// and format differences between document versions.
package diff

import (
	"context"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/sergi/go-diff/diffmatchpatch"
)

// contextLines is the number of unchanged lines shown before/after changes.
// When equal sections exceed 2*contextLines, they're collapsed with "...".
const contextLines = 3

// Options configures a diff operation.
type Options struct {
	Path2          string // Second document path (for comparing two documents)
	Version1       int    // First version to compare
	Version2       int    // Second version to compare
	IncludeDeleted bool   // Allow diffing deleted documents
	FileContent    string // Filesystem file content (for -f flag)
}

// Differ is the interface for diff operations.
type Differ interface {
	Diff(ctx context.Context, path string, opts Options) (Result, error)
}

// Run executes a diff operation and writes output to w.
func Run(ctx context.Context, w io.Writer, svc Differ, path string, opts Options, colour bool) (Result, error) {
	r, err := svc.Diff(ctx, path, opts)
	if err != nil {
		return r, err
	}

	fmt.Fprint(w, r.Format(colour))
	return r, nil
}

// Result holds diff output.
type Result struct {
	Old  string // old label
	New  string // new label
	Diff string // plain diff text
}

// Compute returns a diff between old and new content.
func Compute(oldContent, newContent, oldLabel, newLabel string) Result {
	dmp := diffmatchpatch.New()
	d := dmp.DiffMain(oldContent, newContent, false)
	d = dmp.DiffCleanupSemantic(d)

	return Result{
		Old:  oldLabel,
		New:  newLabel,
		Diff: format(d),
	}
}

// format converts diffs to unified-style text.
func format(diffs []diffmatchpatch.Diff) string {
	var b strings.Builder
	for _, d := range diffs {
		// Trim trailing newline to avoid artefact empty string from Split
		text := strings.TrimSuffix(d.Text, "\n")
		if text == "" {
			continue
		}
		lines := strings.Split(text, "\n")
		switch d.Type {
		case diffmatchpatch.DiffDelete:
			for _, l := range lines {
				b.WriteString("- " + l + "\n")
			}
		case diffmatchpatch.DiffInsert:
			for _, l := range lines {
				b.WriteString("+ " + l + "\n")
			}
		case diffmatchpatch.DiffEqual:
			if len(lines) > 2*contextLines {
				for i := range contextLines {
					b.WriteString("  " + lines[i] + "\n")
				}
				b.WriteString("  ...\n")
				for i := len(lines) - contextLines; i < len(lines); i++ {
					b.WriteString("  " + lines[i] + "\n")
				}
			} else {
				for _, l := range lines {
					b.WriteString("  " + l + "\n")
				}
			}
		}
	}
	return b.String()
}

// Colourise adds ANSI colours to diff output.
func Colourise(d string) string {
	const (
		red   = "\033[31m"
		green = "\033[32m"
		reset = "\033[0m"
	)

	var b strings.Builder
	for _, line := range strings.Split(d, "\n") {
		if line == "" {
			continue
		}
		switch {
		case strings.HasPrefix(line, "- "):
			b.WriteString(red + line + reset + "\n")
		case strings.HasPrefix(line, "+ "):
			b.WriteString(green + line + reset + "\n")
		default:
			b.WriteString(line + "\n")
		}
	}
	return b.String()
}

// Format returns the full diff with header.
func (r Result) Format(colour bool) string {
	header := fmt.Sprintf("--- %s\n+++ %s\n", r.Old, r.New)
	if colour {
		return header + Colourise(r.Diff)
	}
	return header + r.Diff
}

// ParseVersionRange parses a version range string like "3:5" into two integers.
func ParseVersionRange(s string) (v1, v2 int, err error) {
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid version range %q (expected v1:v2)", s)
	}
	v1, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid start version: %w", err)
	}
	v2, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid end version: %w", err)
	}
	return v1, v2, nil
}
