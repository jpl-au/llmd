package messages

import (
	"context"
	"fmt"

	"github.com/jpl-au/llmd/internal/errs"

	"github.com/jpl-au/llmd/internal/clock"

	pkgevents "github.com/jpl-au/llmd/pkg/events"
)

// Ack marks a message as acknowledged by the consumer. The key must be
// the consumer's oldest pending message; otherwise ErrOrderViolation
// is returned.
func (m *Messages) Ack(ctx context.Context, key, consumer string) error {
	if err := m.ensure(); err != nil {
		return fmt.Errorf("messages: %w", err)
	}

	// Look up the consumer's oldest pending message to enforce ordering.
	oldest, err := m.Peek(ctx, consumer)
	if err != nil {
		return errs.Context(err, "finding the oldest message")
	}
	if oldest.Key != key {
		return errs.New(ErrOrderViolation, "%s is not the oldest pending message. Acknowledge %s first.", key, oldest.Key)
	}

	now := clock.Now()
	_, err = m.db.Query(`
		INSERT OR IGNORE INTO message_acks (message_key, author, created_at)
		VALUES (?, ?, ?)
	`, key, consumer, now).WithContext(ctx).Exec()
	if err != nil {
		return fmt.Errorf("inserting ack: %w", err)
	}

	if m.bus != nil {
		m.bus.Emit(ctx, pkgevents.Event{
			Type:      pkgevents.MessageAcknowledged,
			Key:       key,
			Author:    consumer,
			Timestamp: now,
		})
	}

	return nil
}
