// Package sample demonstrates the plugin pattern for external authors.
//
// This plugin shows:
// - sdk.Plugin interface implementation
// - Flag parsing patterns
// - sdk.API calls (Read, List, Exists, History)
// - Returning sdk.Rich with text and structured data
// - Error handling
//
// Commands:
//   - sample stat <path>     Show document metadata
//   - sample recent [-n N]   Show recently modified documents
//   - sample wc <path>       Count lines/words/chars
package sample

import (
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode"

	"github.com/jpl-au/llmd/sdk"
)

// Sample is a sample plugin demonstrating the plugin pattern.
type Sample struct{}

// New creates a new sample plugin.
func New() *Sample { return &Sample{} }

// Name returns the plugin name.
func (s *Sample) Name() string { return "sample" }

// Commands returns the plugin's commands.
func (s *Sample) Commands() []sdk.Command {
	return []sdk.Command{
		{
			Name:  "stat",
			Desc:  "Show document metadata",
			Usage: "stat <path>",
		},
		{
			Name:  "recent",
			Desc:  "Show recently modified documents",
			Usage: "recent [-n limit]",
			Flags: []sdk.Flag{
				{Name: "n", Type: "int", Desc: "Maximum documents to show (default 10)"},
			},
		},
		{
			Name:  "wc",
			Desc:  "Count lines, words, and characters",
			Usage: "wc <path>",
		},
	}
}

// Exec executes a command.
func (s *Sample) Exec(ctx sdk.Context, cmd string, args []string) (sdk.Result, error) {
	switch cmd {
	case "stat":
		return s.stat(args)
	case "recent":
		return s.recent(args)
	case "wc":
		return s.wc(args)
	default:
		return nil, fmt.Errorf("unknown command: %s", cmd)
	}
}

// stat shows document metadata using Exists() and History().
func (s *Sample) stat(args []string) (sdk.Result, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("stat: missing path argument")
	}
	path := args[0]

	exists, err := sdk.API.Exists(path)
	if err != nil {
		return nil, fmt.Errorf("stat: %w", err)
	}

	if !exists {
		return sdk.Rich{
			Text: fmt.Sprintf("%s: not found", path),
			Data: map[string]any{"path": path, "exists": false},
		}, nil
	}

	versions, err := sdk.API.History(path, 0)
	if err != nil {
		return nil, fmt.Errorf("stat: %w", err)
	}

	var latestAuthor string
	var latestDate string
	if len(versions) > 0 {
		latestAuthor = versions[0].Author
		latestDate = time.UnixMilli(versions[0].CreatedAt).Format("2006-01-02")
	}

	text := fmt.Sprintf("exists: true, versions: %d, latest: %s @ %s",
		len(versions), latestAuthor, latestDate)

	data := map[string]any{
		"path":          path,
		"exists":        true,
		"versions":      len(versions),
		"latest_author": latestAuthor,
		"latest_date":   latestDate,
	}

	return sdk.Rich{Text: text, Data: data}, nil
}

// recent shows recently modified documents using List() with time sort.
func (s *Sample) recent(args []string) (sdk.Result, error) {
	limit := 10

	for i := 0; i < len(args); i++ {
		if args[i] == "-n" && i+1 < len(args) {
			i++
			limit, _ = strconv.Atoi(args[i])
		} else if strings.HasPrefix(args[i], "-n") {
			limit, _ = strconv.Atoi(args[i][2:])
		}
	}

	docs, err := sdk.API.List("", sdk.ListOpts{Sort: "time"})
	if err != nil {
		return nil, fmt.Errorf("recent: %w", err)
	}

	if len(docs) > limit {
		docs = docs[:limit]
	}

	if len(docs) == 0 {
		return sdk.Rich{Text: "No documents found", Data: []any{}}, nil
	}

	var sb strings.Builder
	for _, d := range docs {
		date := time.UnixMilli(d.CreatedAt).Format("2006-01-02")
		fmt.Fprintf(&sb, "%-30s %s\n", d.Path, date)
	}

	data := make([]map[string]any, len(docs))
	for i, d := range docs {
		data[i] = map[string]any{
			"path": d.Path,
			"date": time.UnixMilli(d.CreatedAt).Format("2006-01-02"),
		}
	}

	return sdk.Rich{Text: strings.TrimSuffix(sb.String(), "\n"), Data: data}, nil
}

// wc counts lines, words, and characters using Read().
func (s *Sample) wc(args []string) (sdk.Result, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("wc: missing path argument")
	}
	path := args[0]

	content, err := sdk.API.Read(path, 0)
	if err != nil {
		return nil, fmt.Errorf("wc: %w", err)
	}

	text := string(content)
	lines := strings.Count(text, "\n")
	if len(text) > 0 && !strings.HasSuffix(text, "\n") {
		lines++
	}

	words := 0
	inWord := false
	for _, r := range text {
		if unicode.IsSpace(r) {
			inWord = false
		} else if !inWord {
			inWord = true
			words++
		}
	}

	chars := len([]rune(text))

	result := fmt.Sprintf("%d lines, %d words, %d chars", lines, words, chars)
	data := map[string]any{
		"path":  path,
		"lines": lines,
		"words": words,
		"chars": chars,
	}

	return sdk.Rich{Text: result, Data: data}, nil
}
