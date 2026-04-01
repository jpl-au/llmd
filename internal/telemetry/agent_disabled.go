//go:build !telemetry

package telemetry

// EmitAgent is a no-op when telemetry is not compiled in.
func EmitAgent(e AgentEntry) {}
