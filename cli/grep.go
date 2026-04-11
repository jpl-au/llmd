package cli

// grep searches document content using FTS5 full-text search.
//
// llmd is an AI-first tool, and grep is one of the workhorse commands
// agents use to navigate documents. The defaults are tuned for that
// workflow: each match returns the markdown SECTION that contains it,
// not the whole document, so an agent searching a 5000-line spec gets
// back the relevant section bounded by markdown headings - never the
// entire file. Sections are the natural unit of llmd content (the M
// in llmd is markdown), so they give the agent semantically complete,
// context-window-friendly chunks to act on.
//
// Modes (mutually exclusive, --sections is the default):
//
//	--sections (default)  matching markdown section per hit
//	--lines               matching line per hit, with optional -C context
//	--full                full document content per hit (use sparingly)
//	-l                    paths only, deduplicated
//	-c                    "path:count" per document
//
// The structured Data field always carries []sdk.GrepHit with Path,
// Line, Column, Text, Before, After, and Section, regardless of mode -
// so agents that consume the JSON or MCP path always get everything
// they need to act on the result.

import (
	"fmt"
	"strings"

	"github.com/jpl-au/llmd/sdk"
)

var grepSpec = sdk.Command{
	Name: "grep", Desc: `Search document content using full-text search

Returns the matching markdown section per hit by default - never the
full document, so agent context windows stay bounded. Use --lines for
ripgrep-style line snippets, --full for whole documents (rare),
-l for paths only, -c for counts.

Search syntax is FTS5: bare words match implicit AND; uppercase
AND/OR/NOT and NEAR() are operators; "exact phrase" for phrases;
foo* for prefix matching. Literal punctuation searches like
foo-bar or "Authentication: OAuth2" are auto-quoted by the host.`, Usage: "grep [options] <pattern> [path]", MCP: true, MCPName: "llmd_grep", Flags: []sdk.Flag{
		{Name: "sections", Type: "bool", Desc: "Return matching markdown sections (default)"},
		{Name: "lines", Type: "bool", Desc: "Return matching lines with optional context"},
		{Name: "full", Type: "bool", Desc: "Return full document content per hit"},
		{Name: "l", Type: "bool", Desc: "Show only matching paths"},
		{Name: "c", Type: "bool", Desc: "Show match count per document"},
		{Name: "n", Type: "bool", Desc: "Show line numbers (--lines mode)"},
		{Name: "C", Type: "int", Desc: "Context lines around each match (--lines mode)"},
	},
}

func grep(ctx sdk.Context, args []string) (sdk.Response, error) {
	flags, positional, err := sdk.ParseArgs(grepSpec.Flags, args)
	if err != nil {
		return nil, fmt.Errorf("grep: %w", err)
	}

	var pattern, pathPrefix string
	if len(positional) > 0 {
		pattern = positional[0]
	}
	if len(positional) > 1 {
		pathPrefix = positional[1]
	}
	if pattern == "" {
		return nil, fmt.Errorf("grep: %w", sdk.ErrMissingArg)
	}

	// Resolve mode. Mutually exclusive flags; --sections is the
	// default. -l and -c short-circuit before mode selection because
	// they don't return any content per match.
	filesOnly := flags.Bool("l")
	countOnly := flags.Bool("c")
	mode := sdk.GrepSections
	switch {
	case flags.Bool("lines"):
		mode = sdk.GrepLines
	case flags.Bool("full"):
		mode = sdk.GrepFull
	case flags.Bool("sections"):
		mode = sdk.GrepSections
	}

	opts := sdk.GrepOpts{
		Path:    pathPrefix,
		Mode:    mode,
		Context: flags.Int("C"),
	}
	// -l only needs paths back from the index; ask for the cheapest
	// shape so we don't pull document content we'll throw away.
	if filesOnly || countOnly {
		opts.Mode = sdk.GrepPaths
	}

	results, err := ctx.Documents.Grep(pattern, opts)
	if err != nil {
		return nil, fmt.Errorf("grep: %w", err)
	}

	if len(results) == 0 {
		return sdk.Markdown{Text: "", Data: []sdk.GrepHit{}}, nil
	}

	// -l and -c are explicitly machine-friendly path-list modes.
	// Plain Result, Unix-grep shape, no glamour rendering.
	if countOnly {
		counts := make(map[string]int)
		for _, r := range results {
			counts[r.Path]++
		}
		var out strings.Builder
		for path, count := range counts {
			fmt.Fprintf(&out, "%s:%d\n", path, count)
		}
		return sdk.Result{Text: strings.TrimSuffix(out.String(), "\n"), Data: results}, nil
	}
	if filesOnly {
		seen := make(map[string]bool)
		var paths []string
		for _, r := range results {
			if !seen[r.Path] {
				seen[r.Path] = true
				paths = append(paths, r.Path)
			}
		}
		return sdk.Result{Text: strings.Join(paths, "\n"), Data: results}, nil
	}

	// Content modes: format each hit as "path:" header followed by
	// the match content. Sections and full mode print the matched
	// content directly. Lines mode includes any context lines from
	// Before/After and prefixes line numbers when -n is set. The
	// Data field always carries the full []GrepHit so agents
	// reading via --json or MCP get every field including line,
	// column, section heading, and context arrays.
	showLineNums := flags.Bool("n") && mode == sdk.GrepLines
	var out strings.Builder
	for i, r := range results {
		if i > 0 {
			out.WriteByte('\n')
		}
		fmt.Fprintf(&out, "%s:\n", r.Path)

		if mode == sdk.GrepLines {
			// Render before-context, matched line, after-context.
			// Line numbers are computed relative to the matched line
			// when -n is set so the agent can locate every printed
			// line in the source document.
			startLine := r.Line - len(r.Before)
			for j, line := range r.Before {
				if showLineNums {
					fmt.Fprintf(&out, "%d: %s\n", startLine+j, line)
				} else {
					fmt.Fprintf(&out, "%s\n", line)
				}
			}
			if showLineNums {
				fmt.Fprintf(&out, "%d: %s\n", r.Line, r.Text)
			} else {
				fmt.Fprintf(&out, "%s\n", r.Text)
			}
			for j, line := range r.After {
				if showLineNums {
					fmt.Fprintf(&out, "%d: %s\n", r.Line+1+j, line)
				} else {
					fmt.Fprintf(&out, "%s\n", line)
				}
			}
		} else {
			// Sections and full: the Text field already contains
			// the complete chunk; print as-is.
			fmt.Fprintf(&out, "%s\n", r.Text)
		}
	}
	return sdk.Markdown{Text: strings.TrimSuffix(out.String(), "\n"), Data: results}, nil
}
