package cli

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/jpl-au/llmd/sdk"
)

// TestGrep covers full-text search output: the section-bounded
// default, --lines with -n and -C, --full, -l, -c, the path prefix,
// literal hyphenated terms, the structured []GrepHit payload that
// --json, MCP and HTTP consumers see, and the error cases. Each case
// runs against a fresh store holding the documents written in the
// loop below.
func TestGrep(t *testing.T) {
	const apiSpec = `# API Spec

## Overview

This is the overview section.

## Authentication

OAuth2 with PKCE.

## Errors

RFC 7807 problem+json.
`

	tests := []struct {
		name         string
		args         []string
		wantText     []string // substrings that must appear
		wantAbsent   []string // substrings that must not appear
		wantExact    string   // full response text, when deterministic
		wantMarkdown bool     // response is sdk.Markdown (content modes)
		wantHits     int      // number of GrepHit entries in Data
		wantHit      *sdk.GrepHit
		wantErr      error
		wantAnyErr   bool   // any error is acceptable (no sentinel is defined)
		skip         string // t.Skip reason for cases that pin a known bug
	}{
		{
			name:         "default returns the matching section only",
			args:         []string{"OAuth2"},
			wantText:     []string{"specs/api:\n", "## Authentication", "OAuth2 with PKCE."},
			wantAbsent:   []string{"RFC 7807", "## Overview", "## Errors"},
			wantMarkdown: true,
			wantHits:     1,
			wantHit:      &sdk.GrepHit{Path: "specs/api", Line: 7, Text: "## Authentication\n\nOAuth2 with PKCE.\n", Section: "Authentication"},
		},
		{
			name:         "path on its own line followed by a colon",
			args:         []string{"Updated"},
			wantExact:    "docs/hello:\n# Updated",
			wantMarkdown: true,
			wantHits:     1,
		},
		{
			name:         "sections flag is the default spelled out",
			args:         []string{"--sections", "OAuth2"},
			wantText:     []string{"## Authentication"},
			wantAbsent:   []string{"RFC 7807"},
			wantMarkdown: true,
			wantHits:     1,
		},
		{
			name:         "lines returns the matching line",
			args:         []string{"--lines", "OAuth2"},
			wantExact:    "specs/api:\nOAuth2 with PKCE.",
			wantMarkdown: true,
			wantHits:     1,
			wantHit:      &sdk.GrepHit{Path: "specs/api", Line: 9, Text: "OAuth2 with PKCE."},
		},
		{
			name:         "lines with line numbers",
			args:         []string{"--lines", "-n", "OAuth2"},
			wantExact:    "specs/api:\n9: OAuth2 with PKCE.",
			wantMarkdown: true,
			wantHits:     1,
		},
		{
			name:         "lines with context",
			args:         []string{"--lines", "-C", "2", "OAuth2"},
			wantText:     []string{"## Authentication", "OAuth2 with PKCE.", "## Errors"},
			wantAbsent:   []string{"RFC 7807", "overview"},
			wantMarkdown: true,
			wantHits:     1,
		},
		{
			name:         "lines with numbered context",
			args:         []string{"--lines", "-n", "-C2", "OAuth2"},
			wantText:     []string{"7: ## Authentication", "8: ", "9: OAuth2 with PKCE.", "10: ", "11: ## Errors"},
			wantMarkdown: true,
			wantHits:     1,
		},
		{
			name:         "line numbers are ignored outside lines mode",
			args:         []string{"-n", "OAuth2"},
			wantAbsent:   []string{"9: "},
			wantText:     []string{"## Authentication"},
			wantMarkdown: true,
			wantHits:     1,
		},
		{
			name:         "full returns the whole document",
			args:         []string{"--full", "OAuth2"},
			wantText:     []string{"specs/api:\n", "## Overview", "OAuth2 with PKCE.", "RFC 7807"},
			wantMarkdown: true,
			wantHits:     1,
		},
		{
			name:       "l returns paths only",
			args:       []string{"-l", "OAuth2"},
			wantExact:  "specs/api",
			wantAbsent: []string{"OAuth2"},
			wantHits:   1,
		},
		{
			name:      "l lists a document once however many sections match",
			args:      []string{"-l", "alpha"},
			wantExact: "multi",
			wantHits:  1,
		},
		{
			name:      "c returns path and count",
			args:      []string{"-c", "OAuth2"},
			wantExact: "specs/api:1",
			wantHits:  1,
		},
		{
			name:     "c across documents",
			args:     []string{"-c", "OAuth2 OR Updated"},
			wantText: []string{"specs/api:1", "docs/hello:1"},
			wantHits: 2,
		},
		{
			name:      "c counts every match in a document",
			args:      []string{"-c", "alpha"},
			wantExact: "multi:2",
			wantHits:  2,
		},
		{
			name:         "literal hyphenated term",
			args:         []string{"foo-bar"},
			wantText:     []string{"notes/hyphen:\n", "foo-bar baz"},
			wantMarkdown: true,
			wantHits:     1,
		},
		{
			name:         "literal phrase with punctuation",
			args:         []string{"OAuth2 with PKCE."},
			wantText:     []string{"specs/api:\n"},
			wantMarkdown: true,
			wantHits:     1,
		},
		{
			name:         "path prefix restricts the search",
			args:         []string{"alpha", "specs/"},
			wantExact:    "",
			wantMarkdown: true,
			wantHits:     0,
		},
		{
			name:         "path prefix keeps matches underneath it",
			args:         []string{"OAuth2", "specs/"},
			wantText:     []string{"specs/api:\n"},
			wantMarkdown: true,
			wantHits:     1,
		},
		{
			name:         "multiple matching sections in one document",
			args:         []string{"alpha"},
			wantText:     []string{"multi:\n## One", "multi:\n## Two", "alpha here", "alpha again"},
			wantMarkdown: true,
			wantHits:     2,
		},
		{
			name:         "no match",
			args:         []string{"unicorn"},
			wantExact:    "",
			wantMarkdown: true,
			wantHits:     0,
		},
		{
			name:    "no pattern",
			args:    []string{"--lines"},
			wantErr: sdk.ErrMissingArg,
		},
		{
			name:    "unknown flag",
			args:    []string{"--bogus", "OAuth2"},
			wantErr: sdk.ErrInvalidArg,
		},
		{
			name:    "context must be an integer",
			args:    []string{"--lines", "-C", "two", "OAuth2"},
			wantErr: sdk.ErrInvalidArg,
		},
		{
			name:       "path prefix may not traverse",
			args:       []string{"OAuth2", "../x"},
			wantAnyErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.skip != "" {
				t.Skip(tt.skip)
			}
			c := newCLI(t, "alice")
			writeDoc(t, c, "docs/hello", "# Updated")
			writeDoc(t, c, "specs/api", apiSpec)
			writeDoc(t, c, "notes/hyphen", "# Notes\n\nfoo-bar baz\n")
			writeDoc(t, c, "multi", "# Multi\n\n## One\n\nalpha here\n\n## Two\n\nalpha again\n")

			if tt.wantAnyErr {
				if err := runErr(t, c, "grep", nil, tt.args...); err == nil {
					t.Errorf("grep %v error = nil, want an error", tt.args)
				}
				return
			}
			if tt.wantErr != nil {
				if err := runErr(t, c, "grep", nil, tt.args...); !errors.Is(err, tt.wantErr) {
					t.Errorf("grep %v error = %v, want %v", tt.args, err, tt.wantErr)
				}
				return
			}
			resp := run(t, c, "grep", nil, tt.args...)
			if _, ok := resp.(sdk.Markdown); ok != tt.wantMarkdown {
				t.Errorf("grep %v response type = %T, want markdown %v", tt.args, resp, tt.wantMarkdown)
			}
			got := text(resp)
			if tt.wantExact != "" && got != tt.wantExact {
				t.Errorf("grep %v = %q, want %q", tt.args, got, tt.wantExact)
			}
			for _, want := range tt.wantText {
				if !strings.Contains(got, want) {
					t.Errorf("grep %v = %q, want to contain %q", tt.args, got, want)
				}
			}
			for _, absent := range tt.wantAbsent {
				if strings.Contains(got, absent) {
					t.Errorf("grep %v = %q, must not contain %q", tt.args, got, absent)
				}
			}

			// Data is always []sdk.GrepHit so agents reading JSON get
			// the same shape whichever mode produced the text.
			hits, ok := data(resp).([]sdk.GrepHit)
			if !ok {
				t.Fatalf("grep %v Data = %T, want []sdk.GrepHit", tt.args, data(resp))
			}
			if tt.wantHits != 0 || tt.wantExact != "" || tt.wantMarkdown {
				if len(hits) != tt.wantHits {
					t.Errorf("grep %v Data has %d hits, want %d: %+v", tt.args, len(hits), tt.wantHits, hits)
				}
			}
			for _, h := range hits {
				if h.Path == "" {
					t.Errorf("grep %v hit without a path: %+v", tt.args, h)
				}
			}
			if tt.wantHit != nil && len(hits) > 0 {
				h := hits[0]
				if h.Path != tt.wantHit.Path || h.Line != tt.wantHit.Line || h.Text != tt.wantHit.Text || h.Section != tt.wantHit.Section {
					t.Errorf("grep %v hit = %+v, want %+v", tt.args, h, *tt.wantHit)
				}
			}
			if slices.Contains(tt.args, "-C") || slices.Contains(tt.args, "-C2") {
				if len(hits) > 0 && (len(hits[0].Before) != 2 || len(hits[0].After) != 2) {
					t.Errorf("grep %v hit context = %+v, want 2 lines before and after", tt.args, hits[0])
				}
			}
		})
	}
}
