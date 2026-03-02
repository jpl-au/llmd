package sdk

// ConfigStore reads and writes llmd configuration and manages
// .llmd/.gitignore patterns.
type ConfigStore interface {
	// Read returns the merged configuration (global + local).
	// Local values override global ones.
	Read() map[string]string

	// Write sets a key=value pair in a config file.
	Write(key, value string, opts WriteOpts) error

	// IgnorePatterns returns all patterns from .llmd/.gitignore.
	IgnorePatterns() ([]string, error)

	// AddIgnore appends a pattern to .llmd/.gitignore if not already
	// present. Safe to call multiple times with the same pattern.
	AddIgnore(pattern string) error

	// RemoveIgnore removes a pattern from .llmd/.gitignore.
	RemoveIgnore(pattern string) error
}

// WriteOpts controls config write behaviour.
type WriteOpts struct {
	// Global writes to ~/.llmd/config instead of .llmd/config.
	Global bool
}
