// Agent telemetry records raw LLM responses for post-hoc diagnostic
// analysis. The full JSON response from each agent run is preserved
// so that token usage, model behaviour, and response quality can be
// investigated after the fact without re-running the agent. Entries
// are written as append-only markdown with region-delimited blocks,
// using fenced JSON code blocks, so that an LLM can parse and
// summarise the file directly.

package telemetry

// AgentEntry captures the context needed to correlate a raw LLM
// response back to the run and task that produced it. RawJSON is
// the complete JSON output from the agent tool (e.g. Claude Code's
// --output-format json response), stored verbatim without parsing
// so nothing is lost.
type AgentEntry struct {
	RunKey  string
	TaskKey string
	Agent   string
	RawJSON string
}
