//go:build !telemetry

package telemetry

// Enabled reports whether telemetry is compiled into this build.
const Enabled = false

// Init is a no-op when telemetry is not compiled in.
func Init() {}

// Close is a no-op when telemetry is not compiled in.
func Close() {}

// Emit is a no-op when telemetry is not compiled in.
func Emit(e Entry) {}
