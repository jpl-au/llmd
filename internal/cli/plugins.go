package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"text/tabwriter"

	"github.com/jpl-au/llmd/internal/host"
)

// runPlugins executes the plugins command.
func (c *CLI) runPlugins(ctx context.Context, result *ParseResult, h *host.Host) int {
	plugins := h.Plugins()

	switch result.Output {
	case OutputJSON:
		return c.pluginsJSON(plugins)
	default:
		return c.pluginsText(plugins)
	}
}

// pluginInfo holds plugin data for JSON output.
type pluginInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
	Source  string `json:"source"`
	Status  string `json:"status"`
}

// pluginsJSON outputs plugins as JSON.
func (c *CLI) pluginsJSON(plugins []*host.LoadedPlugin) int {
	infos := make([]pluginInfo, len(plugins))
	for i, p := range plugins {
		infos[i] = pluginInfo{
			Name:    p.Name,
			Version: p.Version,
			Source:  p.Source,
			Status:  "loaded",
		}
	}

	out := map[string][]pluginInfo{"plugins": infos}
	if err := json.NewEncoder(c.stdout).Encode(out); err != nil {
		c.writeError(err, OutputJSON)
		return ExitError
	}
	return ExitSuccess
}

// pluginsText outputs plugins as a table.
func (c *CLI) pluginsText(plugins []*host.LoadedPlugin) int {
	w := tabwriter.NewWriter(c.stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tVERSION\tSOURCE\tSTATUS")

	for _, p := range plugins {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", p.Name, p.Version, p.Source, "loaded")
	}

	w.Flush()
	return ExitSuccess
}
