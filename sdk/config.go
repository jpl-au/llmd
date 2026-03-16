package sdk

// ConfigStore reads and writes llmd configuration and manages
// .llmd/.gitignore rules.
type ConfigStore interface {
	// Read returns the merged configuration (global + local).
	// Local values override global ones. Returns partial config
	// alongside any errors so callers can fall back to defaults
	// when a file is unreadable.
	Read() (map[string]string, error)

	// Write sets a key=value pair in a config file.
	Write(key, value string, opts WriteOpts) error

	// GitRules returns all patterns from .llmd/.gitignore.
	GitRules() ([]string, error)

	// GitAllow adds a whitelist entry (!pattern) to .llmd/.gitignore
	// so the file is committed. Safe to call multiple times.
	GitAllow(pattern string) error

	// GitDeny removes a whitelist entry (!pattern) from
	// .llmd/.gitignore so the file is no longer committed.
	GitDeny(pattern string) error
}

// WriteOpts controls config write behaviour.
type WriteOpts struct {
	// Global writes to ~/.llmd/config instead of .llmd/config.
	Global bool
}
