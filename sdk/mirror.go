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
	// are skipped. A document whose file was removed since the last
	// pull or push is soft-deleted, unless it changed in the store
	// since then.
	Push(dir string, opts PushOpts) (*PushResult, error)
}

// PullResult contains the counts from a mirror pull.
type PullResult struct {
	Wrote   int `json:"wrote"`
	Skipped int `json:"skipped"`
	Removed int `json:"removed"`
}

// PushOpts configures a mirror push operation.
type PushOpts struct {
	Prefix string // Target path prefix in store
}

// PushResult contains the counts from a mirror push.
type PushResult struct {
	Created []string `json:"created"`
	Updated []string `json:"updated"`
	Skipped []string `json:"skipped"`
	Deleted []string `json:"deleted"`
}
