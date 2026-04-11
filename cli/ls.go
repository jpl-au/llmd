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

// defaultListLimit caps ls/find/glob output. A large store can have
// thousands of documents, and an agent calling any of these tools
// without a filter should not get them all dumped into its context.
// 500 is generous enough for typical stores and bounded enough to
// stay under ~10k tokens.
const defaultListLimit = 500

var lsSpec = sdk.Command{
	Name: "ls", Desc: `List documents in the store

Shows all documents, or those under a given <path>. Without flags,
prints one document path per line. Use -l for detailed output with
version, author, and date. Use --tree for a directory hierarchy.

Defaults to the first 500 matching documents so an agent on a large
store doesn't get the full catalogue dumped. Pass --limit N for a
different cap or --all to disable it.`, Usage: "ls [flags] [path]", MCP: true, Flags: []sdk.Flag{
		{Name: "l", Type: "bool", Desc: "Long format with details"},
		{Name: "a", Type: "bool", Desc: "Include deleted documents"},
		{Name: "r", Type: "bool", Desc: "Reverse sort order"},
		{Name: "t", Type: "bool", Desc: "Sort by time (newest first)"},
		{Name: "tree", Type: "bool", Desc: "Render as directory tree"},
		{Name: "limit", Type: "int", Desc: "Maximum documents to return (default 500)"},
		{Name: "all", Type: "bool", Desc: "Show every document, no limit"},
		{Name: "since", Type: "string", Desc: "Show documents updated after (e.g. 5m, 1h, RFC 3339)"},
	},
}

func ls(ctx sdk.Context, args []string) (sdk.Response, error) {
	flags, positional, err := sdk.ParseArgs(lsSpec.Flags, args)
	if err != nil {
		return nil, fmt.Errorf("ls: %w", err)
	}
	long := flags.Bool("l")
	all := flags.Bool("a")
	reverse := flags.Bool("r")
	sortByTime := flags.Bool("t")
	asTree := flags.Bool("tree")

	var prefix string
	if len(positional) > 0 {
		prefix = positional[0]
	}

	s := "path"
	if sortByTime {
		s = "time"
	}

	since, err := sdk.ParseSince(flags.String("since"))
	if err != nil {
		return nil, fmt.Errorf("ls: %w", err)
	}
	docs, err := ctx.Documents.List(prefix, sdk.ListOpts{
		Deleted: all,
		Sort:    s,
		Reverse: reverse,
		Since:   since,
	})
	if err != nil {
		return nil, err
	}

	// Resolve the cap: --all beats --limit beats the default.
	limit := flags.Int("limit")
	if flags.Bool("all") {
		limit = 0
	} else if limit == 0 {
		limit = defaultListLimit
	}
	if limit > 0 && len(docs) > limit {
		docs = docs[:limit]
	}

	if len(docs) == 0 {
		return sdk.Result{Text: "", Data: []sdk.Doc{}}, nil
	}

	// Structured data uses maps for stable JSON field names regardless of
	// any future changes to the sdk.Doc struct.
	data := make([]map[string]any, len(docs))
	for i, d := range docs {
		data[i] = map[string]any{
			"key":        d.Key,
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
		lines := make([]string, len(docs))
		for i, d := range docs {
			lines[i] = d.Key + " " + d.Path
		}
		text = strings.Join(lines, "\n")
	}

	return sdk.Result{Text: text, Data: data}, nil
}

// formatTable renders docs as a lipgloss table with styled path cells.
func formatTable(docs []sdk.Doc) string {
	if len(docs) == 0 {
		return ""
	}

	t := newTable("KEY", "VER", "AUTHOR", "DATE", "PATH")

	for _, d := range docs {
		date := time.UnixMilli(d.CreatedAt).Format("2006-01-02")
		path := d.Path
		if d.Deleted {
			path = d.Path + " (deleted)"
		}
		t.Row(d.Key, fmt.Sprintf("%d", d.Version), d.Author, date, path)
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
