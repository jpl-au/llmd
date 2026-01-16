// Package sed provides sed-style text substitution for documents.
//
// Supports the familiar s/old/new/ syntax with optional 'g' flag for global
// replacement. Alternate delimiters (s|old|new|) work too. Only substitution
// commands are supported - other sed features are out of scope.
package sed

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/jpl-au/llmd/internal/service"
)

var (
	// ErrInvalidExpr is returned when a sed expression is malformed.
	ErrInvalidExpr = errors.New("invalid sed expression")
	// ErrUnsupportedCommand is returned for non-substitution commands.
	ErrUnsupportedCommand = errors.New("only substitution (s) commands are supported")
	// ErrTextNotFound is returned when the search text is not in the document.
	ErrTextNotFound = errors.New("text not found")
)

// Options configures a sed operation.
type Options struct {
	Author  string // Author attribution
	Message string // Version message
}

// Result contains the outcome of a sed operation.
type Result struct {
	Path string `json:"path"`
}

// Expr represents a parsed sed expression.
type Expr struct {
	Old    string
	New    string
	Global bool // 'g' flag - replace all occurrences
}

// Run executes a sed substitution on a document.
// path can be a document path or a key.
func Run(ctx context.Context, w io.Writer, svc service.Service, path, expr string, opts Options) (Result, error) {
	result := Result{Path: path}

	parsed, err := ParseExpr(expr)
	if err != nil {
		return result, err
	}

	// Resolve path or key to get the document
	doc, _, err := svc.Resolve(ctx, path, false)
	if err != nil {
		return result, err
	}
	path = doc.Path // Use resolved path
	result.Path = path

	// Perform substitution
	if !strings.Contains(doc.Content, parsed.Old) {
		return result, fmt.Errorf("%w: %q", ErrTextNotFound, parsed.Old)
	}

	var newContent string
	if parsed.Global {
		newContent = strings.ReplaceAll(doc.Content, parsed.Old, parsed.New)
	} else {
		newContent = strings.Replace(doc.Content, parsed.Old, parsed.New, 1)
	}

	// Write new version
	if err := svc.Write(ctx, path, newContent, opts.Author, opts.Message); err != nil {
		return result, err
	}

	fmt.Fprintf(w, "Edited %s\n", path)
	return result, nil
}

// ParseExpr parses a sed substitution expression like s/old/new/ or s|old|new|g.
func ParseExpr(expr string) (Expr, error) {
	if len(expr) < 4 {
		return Expr{}, ErrInvalidExpr
	}

	if expr[0] != 's' {
		return Expr{}, ErrUnsupportedCommand
	}

	delim := expr[1]
	rest := expr[2:]

	parts := splitByDelim(rest, delim)
	if len(parts) < 2 {
		return Expr{}, fmt.Errorf("%w: expected s%cold%cnew%c", ErrInvalidExpr, delim, delim, delim)
	}

	result := Expr{
		Old: parts[0],
		New: parts[1],
	}

	// Check for flags (third part after final delimiter)
	if len(parts) >= 3 {
		flags := parts[2]
		if strings.Contains(flags, "g") {
			result.Global = true
		}
	}

	return result, nil
}

// splitByDelim splits a string by delimiter, respecting escaped delimiters.
func splitByDelim(s string, delim byte) []string {
	var parts []string
	var current strings.Builder
	escaped := false

	for i := 0; i < len(s); i++ {
		c := s[i]
		if escaped {
			current.WriteByte(c)
			escaped = false
			continue
		}
		if c == '\\' {
			escaped = true
			continue
		}
		if c == delim {
			parts = append(parts, current.String())
			current.Reset()
			continue
		}
		current.WriteByte(c)
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}
