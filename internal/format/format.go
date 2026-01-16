// Package format provides output formatting utilities for CLI display.
//
// Centralises formatting logic so that command implementations focus on
// business logic while this package handles presentation concerns like
// column alignment, tree rendering, and colourised output.
package format

import (
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/jpl-au/llmd/internal/diff"
	"github.com/jpl-au/llmd/internal/store"
)

// humanSize formats a byte count as human-readable (e.g., "1.2K", "3.4M").
func humanSize(bytes int64) string {
	const (
		_        = iota
		KB int64 = 1 << (10 * iota)
		MB
		GB
	)
	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.1fG", float64(bytes)/float64(GB))
	case bytes >= MB:
		return fmt.Sprintf("%.1fM", float64(bytes)/float64(MB))
	case bytes >= KB:
		return fmt.Sprintf("%.1fK", float64(bytes)/float64(KB))
	default:
		return fmt.Sprintf("%dB", bytes)
	}
}

// List prints documents in simple list format.
func List(w io.Writer, docs []store.Document) error {
	for _, doc := range docs {
		prefix := ""
		if doc.DeletedAt != nil {
			prefix = "[deleted] "
		}
		fmt.Fprintf(w, "%s  %s%s\n", doc.Key, prefix, doc.Path)
	}
	return nil
}

// Long prints documents in long format with key, version, date, and author.
func Long(w io.Writer, docs []store.Document) error {
	if len(docs) == 0 {
		return nil
	}

	// Find max path length for alignment
	maxPath := 4 // minimum "PATH"
	for _, doc := range docs {
		if len(doc.Path) > maxPath {
			maxPath = len(doc.Path)
		}
	}

	// Print header
	fmt.Fprintf(w, "%-8s  %-*s  %4s  %-10s  %s\n", "KEY", maxPath, "PATH", "VER", "UPDATED", "AUTHOR")

	for _, doc := range docs {
		date := time.Unix(doc.CreatedAt, 0).Format("2006-01-02")
		author := doc.Author
		if author == "" {
			author = "-"
		}
		deleted := ""
		if doc.DeletedAt != nil {
			deleted = " [deleted]"
		}
		fmt.Fprintf(w, "%s  %-*s  v%-3d  %s  %s%s\n", doc.Key, maxPath, doc.Path, doc.Version, date, author, deleted)
	}
	return nil
}

// LongMeta prints document metadata in long format.
//
// Column order is VER, KEY, SIZE, UPDATED, AUTHOR, PATH. Fixed-width columns
// come first so they align properly. Variable-length fields like AUTHOR and
// PATH are placed at the end where their varying widths do not disrupt the
// alignment of other columns.
func LongMeta(w io.Writer, metas []store.DocumentMeta) error {
	if len(metas) == 0 {
		return nil
	}

	// Find max author length for alignment
	maxAuthor := 6 // minimum "AUTHOR"
	for _, m := range metas {
		author := m.Author
		if author == "" {
			author = "-"
		}
		if len(author) > maxAuthor {
			maxAuthor = len(author)
		}
	}

	// Print header
	fmt.Fprintf(w, "%4s  %-8s  %6s  %-16s  %-*s  %s\n", "VER", "KEY", "SIZE", "UPDATED", maxAuthor, "AUTHOR", "PATH")

	for _, m := range metas {
		updated := time.Unix(m.CreatedAt, 0).Format("2006-01-02 15:04")
		author := m.Author
		if author == "" {
			author = "-"
		}
		size := humanSize(m.Size)
		deleted := ""
		if m.DeletedAt != nil {
			deleted = " [deleted]"
		}
		fmt.Fprintf(w, "%4d  %s  %6s  %s  %-*s  %s%s\n", m.Version, m.Key, size, updated, maxAuthor, author, m.Path, deleted)
	}
	return nil
}

// Tree prints documents as a directory tree.
func Tree(w io.Writer, docs []store.Document) error {
	if len(docs) == 0 {
		return nil
	}

	// Build tree structure
	type node struct {
		name     string
		children map[string]*node
		isDoc    bool
		deleted  bool
	}

	root := &node{children: make(map[string]*node)}

	for _, doc := range docs {
		parts := strings.Split(doc.Path, "/")
		current := root

		for i, part := range parts {
			if current.children[part] == nil {
				current.children[part] = &node{
					name:     part,
					children: make(map[string]*node),
				}
			}
			current = current.children[part]
			if i == len(parts)-1 {
				current.isDoc = true
				current.deleted = doc.DeletedAt != nil
			}
		}
	}

	// Print tree
	var printNode func(n *node, prefix string)
	printNode = func(n *node, prefix string) {
		// Get sorted children
		names := make([]string, 0, len(n.children))
		for name := range n.children {
			names = append(names, name)
		}
		sort.Strings(names)

		for i, name := range names {
			child := n.children[name]
			last := i == len(names)-1

			connector := "├── "
			if last {
				connector = "└── "
			}

			suffix := ""
			if !child.isDoc && len(child.children) > 0 {
				suffix = "/"
			}
			if child.deleted {
				suffix += " [deleted]"
			}

			fmt.Fprintf(w, "%s%s%s%s\n", prefix, connector, name, suffix)

			pfx := prefix
			if last {
				pfx += "    "
			} else {
				pfx += "│   "
			}

			if len(child.children) > 0 {
				printNode(child, pfx)
			}
		}
	}

	printNode(root, "")
	return nil
}

// History prints version history in list format.
func History(w io.Writer, docs []store.Document) error {
	for _, doc := range docs {
		t := time.Unix(doc.CreatedAt, 0)
		msg := "-"
		if doc.Message != "" {
			msg = fmt.Sprintf("%q", doc.Message)
		}
		fmt.Fprintf(w, "%s  v%-3d  %s  %-16s  %s\n",
			doc.Key,
			doc.Version,
			t.Format("2006-01-02 15:04"),
			doc.Author,
			msg,
		)
	}
	return nil
}

// HistoryDiff prints version history with diffs between versions.
func HistoryDiff(w io.Writer, docs []store.Document, colour bool) error {
	// Docs are in descending order (newest first)
	for i := 0; i < len(docs)-1; i++ {
		newer := docs[i]
		older := docs[i+1]

		t := time.Unix(newer.CreatedAt, 0)
		fmt.Fprintf(w, "=== v%d -> v%d (%s by %s) ===\n",
			older.Version, newer.Version,
			t.Format("2006-01-02 15:04"),
			newer.Author,
		)

		if newer.Message != "" {
			fmt.Fprintf(w, "Message: %s\n", newer.Message)
		}

		ol := "v" + strconv.Itoa(older.Version)
		nl := "v" + strconv.Itoa(newer.Version)
		r := diff.Compute(older.Content, newer.Content, ol, nl)
		fmt.Fprint(w, r.Format(colour))
		fmt.Fprintln(w)
	}
	return nil
}

// SearchResults prints search results with matching lines.
func SearchResults(w io.Writer, docs []store.Document, query string) error {
	qLower := strings.ToLower(strings.TrimSuffix(query, "*"))
	for _, doc := range docs {
		lines := strings.Split(doc.Content, "\n")
		for i, line := range lines {
			if strings.Contains(strings.ToLower(line), qLower) {
				display := line
				if len(display) > 80 {
					display = display[:77] + "..."
				}
				fmt.Fprintf(w, "%s:%d: %s\n", doc.Path, i+1, display)
			}
		}
	}
	return nil
}

// Paths prints just document paths, one per line.
func Paths(w io.Writer, docs []store.Document) error {
	for _, doc := range docs {
		fmt.Fprintln(w, doc.Path)
	}
	return nil
}
