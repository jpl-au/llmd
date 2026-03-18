package messages

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/jpl-au/llmd/pkg/model/message"
)

// Pending returns unacknowledged messages for the consumer, ordered by
// created_at ascending. Only messages assigned to the consumer or
// broadcast (assigned_to IS NULL) are included. Limit of zero means all.
func (m *Messages) Pending(ctx context.Context, consumer string, limit int) ([]message.Message, error) {
	if err := m.ensure(); err != nil {
		return nil, fmt.Errorf("messages: %w", err)
	}

	query := `
		SELECT id, key, source_key, type, payload, author, source, assigned_to, created_at
		FROM messages
		WHERE (assigned_to IS NULL OR assigned_to = ?)
		AND NOT EXISTS (
			SELECT 1 FROM message_acks a
			WHERE a.message_key = messages.key AND a.author = ?
		)
		ORDER BY created_at
	`
	args := []any{consumer, consumer}

	if limit > 0 {
		query += ` LIMIT ?`
		args = append(args, limit)
	}

	rows, err := m.db.Query(query, args...).WithContext(ctx).Read()
	if err != nil {
		return nil, fmt.Errorf("listing pending: %w", err)
	}
	defer rows.Close()

	var msgs []message.Message
	for rows.Next() {
		msg, err := scanMessage(rows)
		if err != nil {
			return nil, err
		}
		msgs = append(msgs, msg)
	}
	return msgs, rows.Err()
}

// Peek returns the oldest unacknowledged message for the consumer.
func (m *Messages) Peek(ctx context.Context, consumer string) (*message.Message, error) {
	msgs, err := m.Pending(ctx, consumer, 1)
	if err != nil {
		return nil, err
	}
	if len(msgs) == 0 {
		return nil, ErrNotFound
	}
	return &msgs[0], nil
}

// History returns all messages with optional filters.
func (m *Messages) History(ctx context.Context, consumer string, since int64) ([]message.Message, error) {
	if err := m.ensure(); err != nil {
		return nil, fmt.Errorf("messages: %w", err)
	}

	query := `
		SELECT id, key, source_key, type, payload, author, source, assigned_to, created_at
		FROM messages
		WHERE 1=1
	`
	var args []any

	if consumer != "" {
		query += ` AND (assigned_to IS NULL OR assigned_to = ?)`
		args = append(args, consumer)
	}
	if since > 0 {
		query += ` AND created_at > ?`
		args = append(args, since)
	}

	query += ` ORDER BY created_at`

	rows, err := m.db.Query(query, args...).WithContext(ctx).Read()
	if err != nil {
		return nil, fmt.Errorf("listing history: %w", err)
	}
	defer rows.Close()

	var msgs []message.Message
	for rows.Next() {
		msg, err := scanMessage(rows)
		if err != nil {
			return nil, err
		}
		msgs = append(msgs, msg)
	}
	return msgs, rows.Err()
}

// scanMessage scans a message row into a Message struct.
func scanMessage(rows *sql.Rows) (message.Message, error) {
	var msg message.Message
	var sourceKey, assignedTo sql.NullString

	err := rows.Scan(
		&msg.ID,
		&msg.Key,
		&sourceKey,
		&msg.Type,
		&msg.Payload,
		&msg.Author,
		&msg.Source,
		&assignedTo,
		&msg.CreatedAt,
	)
	if err != nil {
		return msg, err
	}

	if sourceKey.Valid {
		msg.SourceKey = sourceKey.String
	}
	if assignedTo.Valid {
		msg.AssignedTo = assignedTo.String
	}
	return msg, nil
}
