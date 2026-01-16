// Package edit provides text editing utilities for document manipulation.
package edit

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

var (
	// ErrTextNotFound is returned when search text is not in the document.
	ErrTextNotFound = errors.New("text not found")
	// ErrInvalidLineRange is returned when a line range is malformed.
	ErrInvalidLineRange = errors.New("invalid line range")
)

// Options configures a search/replace edit operation.
type Options struct {
	Old             string // Text to find
	New             string // Text to replace with
	CaseInsensitive bool   // Case-insensitive matching
	Author          string // Author attribution
	Message         string // Version message
}

// LineRangeOptions configures a line-range edit operation.
type LineRangeOptions struct {
	Start   int    // Start line (1-indexed)
	End     int    // End line (inclusive)
	Author  string // Author attribution
	Message string // Version message
}

// Result contains the outcome of an edit operation.
type Result struct {
	Path string `json:"path"`
}

// Editor is the interface for search/replace edit operations.
type Editor interface {
	Edit(ctx context.Context, path string, opts Options) error
}

// LineRangeEditor is the interface for line-range edit operations.
type LineRangeEditor interface {
	EditLineRange(ctx context.Context, path string, opts LineRangeOptions, replacement string) error
}

// Run executes an edit operation (search/replace).
func Run(ctx context.Context, w io.Writer, svc Editor, path string, opts Options) (Result, error) {
	r := Result{Path: path}

	if err := svc.Edit(ctx, path, opts); err != nil {
		return r, err
	}

	fmt.Fprintf(w, "Edited %s\n", path)
	return r, nil
}

// RunLineRange executes a line-range edit operation, replacing specified lines.
func RunLineRange(ctx context.Context, w io.Writer, svc LineRangeEditor, path string, opts LineRangeOptions, replacement string) (Result, error) {
	r := Result{Path: path}

	if err := svc.EditLineRange(ctx, path, opts, replacement); err != nil {
		return r, err
	}

	fmt.Fprintf(w, "Edited %s\n", path)
	return r, nil
}

// Replace performs a search/replace operation on content.
// It replaces the first occurrence of old with new.
// If caseInsensitive is true, matching ignores case but preserves the replacement text as-is.
// Returns an error if old is not found in content.
func Replace(content, old, newStr string, caseInsensitive bool) (string, error) {
	if caseInsensitive {
		// Find position case-insensitively
		idx := strings.Index(strings.ToLower(content), strings.ToLower(old))
		if idx == -1 {
			return "", fmt.Errorf("%w: %q", ErrTextNotFound, old)
		}
		// Replace at the found position
		return content[:idx] + newStr + content[idx+len(old):], nil
	}

	if !strings.Contains(content, old) {
		return "", fmt.Errorf("%w: %q", ErrTextNotFound, old)
	}
	return strings.Replace(content, old, newStr, 1), nil
}

// ReplaceLines replaces a range of lines with new content.
// Lines are 1-indexed (first line is 1, not 0).
// The range is inclusive: start:end replaces lines start through end.
//
// Boundary behaviour:
//   - start < 1: returns error
//   - start > document length: returns error
//   - end < start: returns error (also covers end < 1 when start >= 1)
//   - end > document length: silently clamped to document length (permissive,
//     consistent with cat behaviour - allows "edit lines 5 to end" without
//     knowing exact line count)
func ReplaceLines(content string, start, end int, replacement string) (string, error) {
	if start < 1 {
		return "", fmt.Errorf("start line must be >= 1, got %d", start)
	}
	if end < start {
		return "", fmt.Errorf("end line %d cannot be less than start line %d", end, start)
	}

	lines := strings.Split(content, "\n")
	if start > len(lines) {
		return "", fmt.Errorf("start line %d exceeds document length %d", start, len(lines))
	}
	if end > len(lines) {
		end = len(lines)
	}

	// Build new content: lines before + replacement + lines after
	var result []string
	result = append(result, lines[:start-1]...)

	// Add replacement (trimming trailing newline if present)
	replacement = strings.TrimSuffix(replacement, "\n")
	if replacement != "" {
		result = append(result, strings.Split(replacement, "\n")...)
	}

	result = append(result, lines[end:]...)

	return strings.Join(result, "\n"), nil
}

// ParseLineRange parses a line range string like "5:10" into start and end integers.
func ParseLineRange(s string) (start, end int, err error) {
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("%w: %q (expected start:end)", ErrInvalidLineRange, s)
	}

	start, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("%w: invalid start line: %w", ErrInvalidLineRange, err)
	}

	end, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("%w: invalid end line: %w", ErrInvalidLineRange, err)
	}

	return start, end, nil
}
