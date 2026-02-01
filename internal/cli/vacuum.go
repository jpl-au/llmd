package cli

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/jpl-au/llmd/internal/llmd"
)

// runVacuum executes the vacuum command.
func (c *CLI) runVacuum(ctx context.Context, result *ParseResult, store *llmd.Store) int {
	vacuumResult, err := store.Vacuum(ctx)
	if err != nil {
		c.writeError(err, result.Output)
		return ExitError
	}

	switch result.Output {
	case OutputJSON:
		return c.vacuumJSON(vacuumResult)
	default:
		return c.vacuumText(vacuumResult)
	}
}

// vacuumInfo holds vacuum data for JSON output.
type vacuumInfo struct {
	Documents int64 `json:"documents"`
	Tags      int64 `json:"tags"`
	Links     int64 `json:"links"`
	Total     int64 `json:"total"`
}

// vacuumJSON outputs vacuum result as JSON.
func (c *CLI) vacuumJSON(result *llmd.VacuumResult) int {
	info := vacuumInfo{
		Documents: result.Documents,
		Tags:      result.Tags,
		Links:     result.Links,
		Total:     result.Total(),
	}

	out := map[string]vacuumInfo{"vacuum": info}
	enc := json.NewEncoder(c.stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(out); err != nil {
		c.writeError(err, OutputJSON)
		return ExitError
	}
	return ExitSuccess
}

// vacuumText outputs vacuum result as text.
func (c *CLI) vacuumText(result *llmd.VacuumResult) int {
	fmt.Fprintf(c.stdout, "Vacuumed: %d documents, %d tags, %d links\n",
		result.Documents, result.Tags, result.Links)
	return ExitSuccess
}
