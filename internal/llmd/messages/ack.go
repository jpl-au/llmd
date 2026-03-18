package messages

import (
	"context"
	"fmt"
	"time"

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
		return fmt.Errorf("ack: %w", err)
	}
	if oldest.Key != key {
		return fmt.Errorf("ack %s: expected %s: %w", key, oldest.Key, ErrOrderViolation)
	}

	now := time.Now().UnixMilli()
	_, err = m.db.Query(`
		INSERT OR IGNORE INTO message_acks (message_key, author, created_at)
		VALUES (?, ?, ?)
	`, key, consumer, now).WithContext(ctx).Write()
	if err != nil {
		return fmt.Errorf("inserting ack: %w", err)
	}

	if m.bus != nil {
		if err := m.bus.Emit(ctx, pkgevents.Event{
			Type:      pkgevents.MessageAcknowledged,
			Key:       key,
			Author:    consumer,
			Timestamp: now,
		}); err != nil {
			return fmt.Errorf("emitting event: %w", err)
		}
	}

	return nil
}
