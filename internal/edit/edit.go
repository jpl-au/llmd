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
	EditLineRange(ctx context.Context, path, replacement string, opts LineRangeOptions) error
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
func RunLineRange(ctx context.Context, w io.Writer, svc LineRangeEditor, path, replacement string, opts LineRangeOptions) (Result, error) {
	r := Result{Path: path}

	if err := svc.EditLineRange(ctx, path, replacement, opts); err != nil {
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
//   - start == 0: treated as 1 (start of document)
//   - start > document length: returns error
//   - end == 0: treated as document length (end of document)
//   - end < start (when both > 0): returns error
//   - end > document length: silently clamped to document length
func ReplaceLines(content string, start, end int, replacement string) (string, error) {
	lines := strings.Split(content, "\n")

	// Handle 0 values (unspecified in open-ended ranges)
	if start == 0 {
		start = 1
	}
	if end == 0 {
		end = len(lines)
	}

	if start < 1 {
		return "", fmt.Errorf("start line must be >= 1, got %d", start)
	}
	if end < start {
		return "", fmt.Errorf("end line %d cannot be less than start line %d", end, start)
	}
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

// ParseLineRange parses a line range string like "5:10", "5:", or ":10".
// Returns start and end line numbers (1-indexed), where 0 means unspecified.
// Matches cat's parseLineRange behaviour for consistency.
func ParseLineRange(s string) (start, end int, err error) {
	parts := strings.Split(s, ":")
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("%w: %q (expected start:end)", ErrInvalidLineRange, s)
	}

	if parts[0] == "" && parts[1] == "" {
		return 0, 0, fmt.Errorf("%w: %q (at least start or end line required)", ErrInvalidLineRange, s)
	}

	if parts[0] != "" {
		start, err = strconv.Atoi(parts[0])
		if err != nil {
			return 0, 0, fmt.Errorf("%w: invalid start line %q", ErrInvalidLineRange, parts[0])
		}
		if start < 1 {
			return 0, 0, fmt.Errorf("%w: start line must be >= 1, got %d", ErrInvalidLineRange, start)
		}
	}

	if parts[1] != "" {
		end, err = strconv.Atoi(parts[1])
		if err != nil {
			return 0, 0, fmt.Errorf("%w: invalid end line %q", ErrInvalidLineRange, parts[1])
		}
		if end < 1 {
			return 0, 0, fmt.Errorf("%w: end line must be >= 1, got %d", ErrInvalidLineRange, end)
		}
	}

	if start > 0 && end > 0 && start > end {
		return 0, 0, fmt.Errorf("%w: start line %d is greater than end line %d", ErrInvalidLineRange, start, end)
	}

	return start, end, nil
}
