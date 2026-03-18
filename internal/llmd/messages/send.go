package messages

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jpl-au/llmd/internal/llmd/key"
	pkgevents "github.com/jpl-au/llmd/pkg/events"
	"github.com/jpl-au/llmd/pkg/model/message"
)

// SendOptions configures a send operation.
type SendOptions struct {
	Type       string
	Payload    string
	Author     string
	Source     string
	AssignedTo string
	SourceKey  string
}

// Send inserts a message into the queue. When SourceKey is non-empty,
// a duplicate insert is silently ignored (deduplication).
func (m *Messages) Send(ctx context.Context, opts SendOptions) (*message.Message, error) {
	if err := m.ensure(); err != nil {
		return nil, fmt.Errorf("messages: %w", err)
	}

	now := time.Now().UnixMilli()
	k := key.Generate()

	var sourceKey sql.NullString
	if opts.SourceKey != "" {
		sourceKey = sql.NullString{String: opts.SourceKey, Valid: true}
	}
	var assignedTo sql.NullString
	if opts.AssignedTo != "" {
		assignedTo = sql.NullString{String: opts.AssignedTo, Valid: true}
	}

	_, err := m.db.Query(`
		INSERT OR IGNORE INTO messages (key, source_key, type, payload, author, source, assigned_to, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, k, sourceKey, opts.Type, opts.Payload, opts.Author, opts.Source, assignedTo, now).WithContext(ctx).Write()
	if err != nil {
		return nil, fmt.Errorf("inserting message: %w", err)
	}

	msg := &message.Message{
		Key:        k,
		SourceKey:  opts.SourceKey,
		Type:       opts.Type,
		Payload:    opts.Payload,
		AssignedTo: opts.AssignedTo,
		CreatedAt:  now,
	}
	msg.Author = opts.Author
	msg.Source = opts.Source

	if m.bus != nil {
		if err := m.bus.Emit(ctx, pkgevents.Event{
			Type:       pkgevents.MessageSent,
			Key:        k,
			Author:     opts.Author,
			AssignedTo: opts.AssignedTo,
			Timestamp:  now,
			Metadata:   map[string]any{"message_type": opts.Type},
		}); err != nil {
			return nil, fmt.Errorf("emitting event: %w", err)
		}
	}

	return msg, nil
}
