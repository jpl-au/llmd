package cli

// ls lists documents in the store, optionally filtered by path prefix.
//
// Short flags can be combined ("-lat" for long + all + time-sorted),
// following standard Unix conventions.
//
// Default output is one path per line. Long format (-l) adds version
// number, author, and date in a dynamically-sized table — column widths
// expand to fit the longest value rather than truncating.

import (
	"fmt"
	"strings"
	"time"

	"github.com/jpl-au/llmd/sdk"
)

func ls(ctx sdk.Context, args []string) (sdk.Response, error) {
	var long, all, reverse, sortByTime bool
	var prefix string

	for _, arg := range args {
		if strings.HasPrefix(arg, "-") && !strings.HasPrefix(arg, "--") {
			for _, c := range arg[1:] {
				switch c {
				case 'l':
					long = true
				case 'a':
					all = true
				case 'r':
					reverse = true
				case 't':
					sortByTime = true
				}
			}
		} else if !strings.HasPrefix(arg, "-") {
			prefix = arg
		}
	}

	sort := "path"
	if sortByTime {
		sort = "time"
	}

	docs, err := sdk.API.List(prefix, sdk.ListOpts{
		Deleted: all,
		Sort:    sort,
		Reverse: reverse,
	})
	if err != nil {
		return nil, err
	}

	if len(docs) == 0 {
		return sdk.Result{Text: "", Data: []sdk.Doc{}}, nil
	}

	// Structured data uses maps for stable JSON field names regardless of
	// any future changes to the sdk.Doc struct.
	data := make([]map[string]any, len(docs))
	for i, d := range docs {
		data[i] = map[string]any{
			"path":       d.Path,
			"version":    d.Version,
			"author":     d.Author,
			"message":    d.Message,
			"created_at": d.CreatedAt,
			"deleted":    d.Deleted,
		}
	}

	var text string
	if long {
		text = formatTable(docs)
	} else {
		paths := make([]string, len(docs))
		for i, d := range docs {
			paths[i] = d.Path
		}
		text = strings.Join(paths, "\n")
	}

	return sdk.Result{Text: text, Data: data}, nil
}

// formatTable renders docs as a fixed-width table with dynamic column
// widths. Minimum widths (3 for version, 6 for author) prevent the
// header labels from being clipped when all values are short.
func formatTable(docs []sdk.Doc) string {
	if len(docs) == 0 {
		return ""
	}

	verWidth := 3
	authorWidth := 6
	for _, d := range docs {
		if w := len(fmt.Sprintf("%d", d.Version)); w > verWidth {
			verWidth = w
		}
		if w := len(d.Author); w > authorWidth {
			authorWidth = w
		}
	}

	var b strings.Builder
	fmt.Fprintf(&b, "%*s  %-*s  %-10s  %s\n", verWidth, "VER", authorWidth, "AUTHOR", "DATE", "PATH")

	for _, d := range docs {
		date := time.UnixMilli(d.CreatedAt).Format("2006-01-02")
		path := d.Path
		if d.Deleted {
			path = d.Path + " (deleted)"
		}
		fmt.Fprintf(&b, "%*d  %-*s  %-10s  %s\n", verWidth, d.Version, authorWidth, d.Author, date, path)
	}

	return strings.TrimSuffix(b.String(), "\n")
}
