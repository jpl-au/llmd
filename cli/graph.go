package cli

// graph visualises the evolution of the document store.
//
// Without arguments it collects every version event across all
// documents, sorts them chronologically, and clusters them into
// bursts of activity rendered as a nested tree. With a path argument
// it shows the version lineage of a single document.

import (
	"fmt"
	"slices"
	"sort"
	"strings"
	"time"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/tree"
	"github.com/jpl-au/llmd/sdk"
)

var graphSpec = sdk.Command{
	Name: "graph",
	Desc: `Visualise the evolution of the document store

Shows how the database grew over time. Events are clustered into
bursts of activity - siblings happened close together, depth shows
progression through time. With a path argument, shows the version
lineage of a single document.`,
	Usage: "graph [path]",
	MCP:   true,
}

// graphEvent holds a single version event for sorting and display.
type graphEvent struct {
	Path      string `json:"path"`
	Version   int    `json:"version"`
	Author    string `json:"author"`
	Message   string `json:"message"`
	CreatedAt int64  `json:"created_at"`
}

// Styles for graph output.
var (
	graphPath    = lipgloss.NewStyle().Foreground(lipgloss.Color("14"))
	graphVersion = lipgloss.NewStyle().Foreground(lipgloss.Color("3"))
)

func graph(ctx sdk.Context, args []string) (sdk.Response, error) {
	_, positional, err := sdk.ParseArgs(graphSpec.Flags, args)
	if err != nil {
		return nil, fmt.Errorf("graph: %w", err)
	}

	if len(positional) > 0 {
		return graphLineage(ctx, positional[0])
	}
	return graphTimeline(ctx)
}

// graphTimeline renders every version event across the store as a
// burst-clustered tree, showing how the database evolved over time.
func graphTimeline(ctx sdk.Context) (sdk.Response, error) {
	docs, err := ctx.Documents.List("", sdk.ListOpts{})
	if err != nil {
		return nil, fmt.Errorf("graph: %w", err)
	}

	var events []graphEvent
	for _, d := range docs {
		versions, err := ctx.Documents.History(d.Path, 0)
		if err != nil {
			return nil, fmt.Errorf("graph: history %s: %w", d.Path, err)
		}
		for _, v := range versions {
			events = append(events, graphEvent{
				Path:      d.Path,
				Version:   v.Number,
				Author:    v.Author,
				Message:   v.Message,
				CreatedAt: v.CreatedAt,
			})
		}
	}

	sort.Slice(events, func(i, j int) bool {
		return events[i].CreatedAt < events[j].CreatedAt
	})

	if len(events) == 0 {
		return sdk.Result{Text: "No events found", Data: []graphEvent{}}, nil
	}

	// Build structured data for JSON output.
	data := make([]map[string]any, len(events))
	for i, e := range events {
		data[i] = map[string]any{
			"path":       e.Path,
			"version":    e.Version,
			"author":     e.Author,
			"message":    e.Message,
			"created_at": e.CreatedAt,
		}
	}

	if !isTTY() {
		lines := make([]string, len(events))
		for i, e := range events {
			lines[i] = fmt.Sprintf("%s (v%d)", e.Path, e.Version)
		}
		return sdk.Result{Text: strings.Join(lines, "\n"), Data: data}, nil
	}

	bursts := clusterBursts(events)
	text := renderBurstTree(bursts, events)
	return sdk.Result{Text: text, Data: data}, nil
}

// graphLineage renders the version history of a single document as a
// simple tree with the path as root and versions as children.
func graphLineage(ctx sdk.Context, path string) (sdk.Response, error) {
	versions, err := ctx.Documents.History(path, 0)
	if err != nil {
		return nil, fmt.Errorf("graph: %w", err)
	}

	if len(versions) == 0 {
		return sdk.Result{Text: "No history found", Data: []sdk.Version{}}, nil
	}

	// Reverse to oldest-first for chronological display.
	for i, j := 0, len(versions)-1; i < j; i, j = i+1, j-1 {
		versions[i], versions[j] = versions[j], versions[i]
	}

	data := make([]map[string]any, len(versions))
	for i, v := range versions {
		data[i] = map[string]any{
			"path":       path,
			"version":    v.Number,
			"author":     v.Author,
			"message":    v.Message,
			"created_at": v.CreatedAt,
		}
	}

	if !isTTY() {
		lines := make([]string, len(versions))
		for i, v := range versions {
			ts := time.UnixMilli(v.CreatedAt).Format("02 Jan 2006 15:04")
			lines[i] = fmt.Sprintf("(v%d) %s %s", v.Number, ts, v.Author)
		}
		return sdk.Result{Text: strings.Join(lines, "\n"), Data: data}, nil
	}

	t := tree.Root(graphPath.Render(path)).
		EnumeratorStyle(treeEnum)

	for _, v := range versions {
		ts := time.UnixMilli(v.CreatedAt).Format("02 Jan 2006 15:04")
		label := fmt.Sprintf("%s  %s  %s",
			graphVersion.Render(fmt.Sprintf("(v%d)", v.Number)),
			ts, v.Author)
		t.Child(label)
	}

	return sdk.Result{Text: t.String(), Data: data}, nil
}

// clusterBursts groups chronologically sorted events into bursts based
// on time gaps. It returns a slice of indices where each new burst
// begins. The first burst always starts at index 0.
func clusterBursts(events []graphEvent) []int {
	if len(events) < 3 {
		return []int{0}
	}

	// Collect gaps between consecutive events.
	gaps := make([]int64, len(events)-1)
	for i := 1; i < len(events); i++ {
		gaps[i-1] = events[i].CreatedAt - events[i-1].CreatedAt
	}

	// Compute the median gap and use 2x as the burst boundary.
	sorted := make([]int64, len(gaps))
	copy(sorted, gaps)
	slices.Sort(sorted)
	median := sorted[len(sorted)/2]
	threshold := median * 2

	// Walk through gaps and mark burst boundaries.
	bursts := []int{0}
	for i, g := range gaps {
		if g > threshold {
			bursts = append(bursts, i+1)
		}
	}
	return bursts
}

// renderBurstTree builds a lipgloss tree where each burst's events are
// siblings and successive bursts nest deeper.
func renderBurstTree(bursts []int, events []graphEvent) string {
	// Format an event line.
	formatEvent := func(e graphEvent) string {
		return graphPath.Render(e.Path) + " " + graphVersion.Render(fmt.Sprintf("(v%d)", e.Version))
	}

	// Build nested tree from innermost burst outward. The last burst
	// becomes the deepest level, and we wrap each preceding burst
	// around it as a parent.
	var inner *tree.Tree
	for b := len(bursts) - 1; b >= 0; b-- {
		start := bursts[b]
		end := len(events)
		if b < len(bursts)-1 {
			end = bursts[b+1]
		}

		t := tree.Root(formatEvent(events[start])).
			EnumeratorStyle(treeEnum)
		for i := start + 1; i < end; i++ {
			t.Child(formatEvent(events[i]))
		}
		if inner != nil {
			t.Child(inner)
		}
		inner = t
	}

	if inner == nil {
		return ""
	}
	return inner.String()
}
