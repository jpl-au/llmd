package cli

// ls lists documents in the store, optionally filtered by path prefix.
//
// Short flags can be combined ("-lat" for long + all + time-sorted),
// following standard Unix conventions.
//
// Default output is one path per line. Long format (-l) adds version
// number, author, and date in a lipgloss table.

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

	docs, err := sdk.Documents.List(prefix, sdk.ListOpts{
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

// formatTable renders docs as a lipgloss table.
func formatTable(docs []sdk.Doc) string {
	if len(docs) == 0 {
		return ""
	}

	t := newTable("VER", "AUTHOR", "DATE", "PATH")

	for _, d := range docs {
		date := time.UnixMilli(d.CreatedAt).Format("2006-01-02")
		path := d.Path
		if d.Deleted {
			path = d.Path + " (deleted)"
		}
		t.Row(fmt.Sprintf("%d", d.Version), d.Author, date, path)
	}

	return t.String()
}
