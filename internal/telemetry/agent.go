// Agent telemetry records raw LLM responses for diagnostic analysis.
// Entries are append-only markdown with region-delimited blocks
// designed for LLM consumption.

package telemetry

// AgentEntry is a single agent telemetry record.
type AgentEntry struct {
	RunKey  string
	TaskKey string
	Agent   string
	RawJSON string
}
