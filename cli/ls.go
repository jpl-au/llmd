package cli

// ls lists documents in the store, optionally filtered by path.
//
// Short flags can be combined ("-lat" for long + all + time-sorted),
// following standard Unix conventions.
//
// Default output is one path per line. Long format (-l) adds version
// number, author, and date in a lipgloss table. Tree format (--tree)
// renders paths as a directory hierarchy.

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/tree"
	"github.com/jpl-au/llmd/sdk"
)

var lsSpec = sdk.Command{
	Name: "ls", Desc: `List documents in the store

Shows all documents, or those under a given <path>. Without flags,
prints one document path per line. Use -l for detailed output with
version, author, and date. Use --tree for a directory hierarchy.`, Usage: "ls [path]", MCP: true, Flags: []sdk.Flag{
		{Name: "l", Type: "bool", Desc: "Long format with details"},
		{Name: "a", Type: "bool", Desc: "Include deleted documents"},
		{Name: "r", Type: "bool", Desc: "Reverse sort order"},
		{Name: "t", Type: "bool", Desc: "Sort by time (newest first)"},
		{Name: "tree", Type: "bool", Desc: "Render as directory tree"},
	},
}

func ls(ctx sdk.Context, args []string) (sdk.Response, error) {
	var long, all, reverse, sortByTime, asTree bool
	var prefix string

	for _, arg := range args {
		if arg == "--tree" {
			asTree = true
			continue
		}
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

	s := "path"
	if sortByTime {
		s = "time"
	}

	docs, err := ctx.Documents.List(prefix, sdk.ListOpts{
		Deleted: all,
		Sort:    s,
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
	switch {
	case asTree && isTTY():
		text = buildTree(docs)
	case long:
		text = formatTable(docs)
	default:
		paths := make([]string, len(docs))
		for i, d := range docs {
			paths[i] = d.Path
		}
		text = strings.Join(paths, "\n")
	}

	return sdk.Result{Text: text, Data: data}, nil
}

// formatTable renders docs as a lipgloss table with styled path cells.
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

// Tree styles for directory hierarchy rendering.
var (
	treeDir = lipgloss.NewStyle().
		Foreground(lipgloss.Color("12")).
		Bold(true)

	treeEnum = lipgloss.NewStyle().
			Foreground(lipgloss.Color("8"))
)

// buildTree renders document paths as a styled directory tree.
func buildTree(docs []sdk.Doc) string {
	// Collect paths sorted alphabetically for stable output.
	paths := make([]string, len(docs))
	deleted := make(map[string]bool)
	for i, d := range docs {
		paths[i] = d.Path
		if d.Deleted {
			deleted[d.Path] = true
		}
	}
	sort.Strings(paths)

	// Build a nested map of path segments → children.
	type node struct {
		children map[string]*node
		order    []string
		isLeaf   bool
		path     string // full path for deleted lookup
	}
	root := &node{children: make(map[string]*node)}

	for _, p := range paths {
		parts := strings.Split(p, "/")
		cur := root
		for _, seg := range parts {
			if cur.children[seg] == nil {
				cur.children[seg] = &node{children: make(map[string]*node)}
				cur.order = append(cur.order, seg)
			}
			cur = cur.children[seg]
		}
		cur.isLeaf = true
		cur.path = p
	}

	// Recursively convert to lipgloss tree nodes.
	var build func(n *node) []any
	build = func(n *node) []any {
		var items []any
		for _, name := range n.order {
			child := n.children[name]
			if child.isLeaf && len(child.children) == 0 {
				label := name
				if deleted[child.path] {
					label += " (deleted)"
				}
				items = append(items, label)
			} else {
				sub := tree.Root(treeDir.Render(name))
				for _, c := range build(child) {
					sub.Child(c)
				}
				// If this directory is also a leaf document, mark it.
				if child.isLeaf && deleted[child.path] {
					sub.Root(treeDir.Render(name) + " (deleted)")
				}
				items = append(items, sub)
			}
		}
		return items
	}

	t := tree.Root(".").
		EnumeratorStyle(treeEnum)
	for _, c := range build(root) {
		t.Child(c)
	}

	return t.String()
}
