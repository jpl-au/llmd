// activity.go provides a unified activity feed across all store domains.

package llmd

import (
	"context"
	"database/sql"
	"encoding/json"
	"sort"
	"sync"
)

// ActivityEvent is a unified event from any domain.
type ActivityEvent struct {
	Type      string // "document", "tag", "link", "task"
	Action    string // "written", "deleted", "tagged", etc.
	Subject   string // path or task key
	Author    string
	Detail    string
	Timestamp int64
}

// RecentActivity returns the most recent events across documents,
// entities, and tasks. Each source is queried in parallel, then
// results are merged and sorted by timestamp.
func (s *Store) RecentActivity(ctx context.Context, limit int) ([]ActivityEvent, error) {
	var (
		wg    sync.WaitGroup
		mu    sync.Mutex
		items []ActivityEvent
	)

	collect := func(fn func() []ActivityEvent) {
		defer wg.Done()
		if res := fn(); len(res) > 0 {
			mu.Lock()
			items = append(items, res...)
			mu.Unlock()
		}
	}

	wg.Add(3)
	go collect(func() []ActivityEvent { return s.docActivity(ctx, limit) })
	go collect(func() []ActivityEvent { return s.entityActivity(ctx, limit) })
	go collect(func() []ActivityEvent { return s.taskActivity(ctx, limit) })
	wg.Wait()

	sort.Slice(items, func(i, j int) bool {
		return items[i].Timestamp > items[j].Timestamp
	})

	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

// docActivity fetches recent document writes and deletes from the content table.
func (s *Store) docActivity(ctx context.Context, limit int) []ActivityEvent {
	rows, err := s.db.QueryContext(ctx, `
		SELECT path, author, COALESCE(message, ''), created_at, deleted_at
		FROM content
		WHERE namespace = 'core:document'
		ORDER BY created_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var out []ActivityEvent
	for rows.Next() {
		var path, author, msg string
		var createdAt int64
		var deletedAt sql.NullInt64
		if err := rows.Scan(&path, &author, &msg, &createdAt, &deletedAt); err != nil {
			continue
		}
		action := "written"
		ts := createdAt
		if deletedAt.Valid {
			action = "deleted"
			ts = deletedAt.Int64
		}
		out = append(out, ActivityEvent{
			Type:      "document",
			Action:    action,
			Subject:   path,
			Author:    author,
			Detail:    msg,
			Timestamp: ts,
		})
	}
	return out
}

// entityActivity fetches recent tag and link changes from the entities table.
func (s *Store) entityActivity(ctx context.Context, limit int) []ActivityEvent {
	rows, err := s.db.QueryContext(ctx, `
		SELECT namespace, relation, value, author, created_at, deleted_at
		FROM entities
		WHERE namespace IN ('core:tag', 'core:link')
		ORDER BY created_at DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil
	}
	defer rows.Close()

	var out []ActivityEvent
	for rows.Next() {
		var ns, relation, value, author string
		var createdAt int64
		var deletedAt sql.NullInt64
		if err := rows.Scan(&ns, &relation, &value, &author, &createdAt, &deletedAt); err != nil {
			continue
		}

		var typ, action, detail string
		ts := createdAt
		switch ns {
		case "core:tag":
			typ = "tag"
			action = "tagged"
			detail = jsonField(value, "tag")
			if deletedAt.Valid {
				action = "untagged"
				ts = deletedAt.Int64
			}
		case "core:link":
			typ = "link"
			action = "linked"
			detail = jsonField(value, "to")
			if deletedAt.Valid {
				action = "unlinked"
				ts = deletedAt.Int64
			}
		}

		out = append(out, ActivityEvent{
			Type:      typ,
			Action:    action,
			Subject:   relation,
			Author:    author,
			Detail:    detail,
			Timestamp: ts,
		})
	}
	return out
}

// taskActivity fetches recent task events from the audit history table.
func (s *Store) taskActivity(ctx context.Context, limit int) []ActivityEvent {
	events, err := s.Audit.Query(ctx, "", limit)
	if err != nil {
		return nil
	}
	var out []ActivityEvent
	for _, e := range events {
		detail := e.NewValue
		if e.OldValue != "" && e.NewValue != "" {
			detail = e.OldValue + " → " + e.NewValue
		}
		out = append(out, ActivityEvent{
			Type:      "task",
			Action:    e.Action,
			Subject:   e.Subject,
			Author:    e.Actor,
			Detail:    detail,
			Timestamp: e.Timestamp,
		})
	}
	return out
}

// jsonField extracts a string field from a JSON object.
func jsonField(raw, field string) string {
	var m map[string]string
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return raw
	}
	if v, ok := m[field]; ok {
		return v
	}
	return raw
}
