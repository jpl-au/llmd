//go:build !telemetry

package telemetry

// Init is a no-op when telemetry is not compiled in.
func Init() {}

// Close is a no-op when telemetry is not compiled in.
func Close() {}

// Emit is a no-op when telemetry is not compiled in.
func Emit(e Entry) {}
