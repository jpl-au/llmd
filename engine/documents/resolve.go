package documents

import "context"

// KeyToPath translates a key to its document path, live or deleted, so
// a key reaches the same document and the same errors as its path
// (restore by key, and ErrDeleted rather than ErrNotFound elsewhere).
// Used by the shared resolve package to detect key-based identifiers.
func (d *Documents) KeyToPath(ctx context.Context, k string) (string, error) {
	var path string
	row, err := d.db.Query(`
		SELECT path FROM content
		WHERE namespace = ? AND key = ?
		ORDER BY version DESC LIMIT 1
	`, namespace, k).WithContext(ctx).ReadRow()
	if err != nil {
		return "", err
	}
	if err := row.Scan(&path); err != nil {
		return "", ErrNotFound
	}
	return path, nil
}
