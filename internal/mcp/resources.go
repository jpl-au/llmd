// resources.go implements MCP resource handlers for document access.
//
// MCP resources provide read-only access to documents via URI schemes,
// enabling LLM clients to reference documents without using tools. This
// is useful for context loading where the LLM needs document content but
// isn't performing an action.
//
// Design: Resource URIs follow the pattern llmd://documents/{path}[/v/{version}].
// Version is optional; omitting it returns the latest version. This mirrors
// the CLI's "cat" command behaviour.

package mcp

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
)

var (
	// ErrInvalidURI indicates a malformed resource URI, helping clients
	// debug URI construction issues.
	ErrInvalidURI = errors.New("invalid URI")
	// ErrEmptyPath indicates a missing document path in a resource URI.
	ErrEmptyPath = errors.New("empty document path")
)

// readDocumentResource reads a document and returns it as resource contents.
func (h *handlers) readDocumentResource(ctx context.Context, uri string) ([]mcp.ResourceContents, error) {
	if h.svc == nil {
		return nil, errors.New(ErrNotInitialised)
	}

	// Parse URI: llmd://documents/{path} or llmd://documents/{path}/v/{version}
	path, version, err := parseDocumentURI(uri)
	if err != nil {
		return nil, err
	}

	var content string
	if version > 0 {
		doc, err := h.svc.Version(ctx, path, version)
		if err != nil {
			return nil, err
		}
		content = doc.Content
	} else {
		// Use Resolve to support both paths and keys
		doc, _, err := h.svc.Resolve(ctx, path, false)
		if err != nil {
			return nil, err
		}
		content = doc.Content
	}

	return []mcp.ResourceContents{
		mcp.TextResourceContents{
			URI:      uri,
			MIMEType: "text/markdown",
			Text:     content,
		},
	}, nil
}

// parseDocumentURI extracts path and version from a document URI.
// Supports: llmd://documents/{path} and llmd://documents/{path}/v/{version}
func parseDocumentURI(uri string) (path string, version int, err error) {
	const prefix = "llmd://documents/"
	if !strings.HasPrefix(uri, prefix) {
		return "", 0, fmt.Errorf("%w: %s", ErrInvalidURI, uri)
	}

	rest := strings.TrimPrefix(uri, prefix)
	if rest == "" {
		return "", 0, ErrEmptyPath
	}

	// Check for version suffix: /v/{version}
	if idx := strings.LastIndex(rest, "/v/"); idx != -1 {
		path = rest[:idx]
		vStr := rest[idx+3:]
		v, err := strconv.Atoi(vStr)
		if err != nil {
			return "", 0, fmt.Errorf("%w: invalid version %s", ErrInvalidURI, vStr)
		}
		return path, v, nil
	}

	return rest, 0, nil
}
