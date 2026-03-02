package sdk

// MirrorStore syncs documents between the store and filesystem.
type MirrorStore interface {
	// Directory returns the mirror directory for the active store.
	Directory() string

	// Pull writes store documents matching prefix to dir as .md files.
	// Unchanged files are skipped. Stale files are removed.
	Pull(prefix, dir string) (*PullResult, error)

	// Push imports .md files from dir back into the store. New and
	// modified files are written as new versions. Unchanged files
	// are skipped.
	Push(dir string, opts PushOpts) (*PushResult, error)
}

// PullResult contains the counts from a mirror pull.
type PullResult struct {
	Wrote   int
	Skipped int
	Removed int
}

// PushOpts configures a mirror push operation.
type PushOpts struct {
	Prefix string // Target path prefix in store
}

// PushResult contains the counts from a mirror push.
type PushResult struct {
	Created []string
	Updated []string
	Skipped []string
}
