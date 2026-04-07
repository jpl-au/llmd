package documents

import "context"

// KeyToPath translates a key to its document path. Used by the shared
// resolve package to detect key-based identifiers.
func (d *Documents) KeyToPath(ctx context.Context, k string) (string, error) {
	var path string
	row, err := d.db.Query(`
		SELECT path FROM content
		WHERE key = ? AND deleted_at IS NULL
		ORDER BY version DESC LIMIT 1
	`, k).WithContext(ctx).ReadRow()
	if err != nil {
		return "", err
	}
	if err := row.Scan(&path); err != nil {
		return "", ErrNotFound
	}
	return path, nil
}
