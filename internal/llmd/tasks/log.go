// log.go handles task audit history queries.

package tasks

import (
	"context"

	"github.com/jpl-au/llmd/internal/llmd/audit"
)

// Log returns audit events for a task, newest first.
// If key is empty, returns all task history.
func (t *Tasks) Log(ctx context.Context, key string, limit int) ([]audit.Event, error) {
	if key != "" {
		// Verify task exists (including deleted)
		if _, err := t.Read(ctx, key); err != nil {
			if _, err := t.scanDeleted(ctx, key); err != nil {
				return nil, err
			}
		}
	}
	return t.audit.Query(ctx, key, limit)
}
