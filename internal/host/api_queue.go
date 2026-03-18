package host

import (
	"context"
	"errors"
	"fmt"

	"github.com/jpl-au/llmd/internal/llmd"
	"github.com/jpl-au/llmd/internal/llmd/messages"
	"github.com/jpl-au/llmd/pkg/model/message"
	"github.com/jpl-au/llmd/sdk"
)

// queueAPI implements [sdk.QueueStore] by delegating to the internal
// messages package.
type queueAPI struct {
	ctx   context.Context
	store *llmd.Store
}

// newQueueAPI creates the SDK-to-internal bridge for queue operations.
func newQueueAPI(store *llmd.Store, ctx context.Context) *queueAPI {
	return &queueAPI{ctx: ctx, store: store}
}

// queueErr translates internal message errors to SDK sentinels.
func queueErr(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, messages.ErrNotFound):
		return fmt.Errorf("%w: %w", sdk.ErrNotFound, err)
	case errors.Is(err, messages.ErrOrderViolation):
		return fmt.Errorf("%w: %w", sdk.ErrOrderViolation, err)
	default:
		return err
	}
}

func (q *queueAPI) Send(opts sdk.SendOpts) (*sdk.Message, error) {
	msg, err := q.store.Messages.Send(q.ctx, messages.SendOptions{
		Type:       opts.Type,
		Payload:    opts.Payload,
		Author:     opts.Author,
		Source:     opts.Source,
		AssignedTo: opts.AssignedTo,
		SourceKey:  opts.SourceKey,
	})
	if err != nil {
		return nil, queueErr(err)
	}
	return msgToSDK(msg), nil
}

func (q *queueAPI) Pending(consumer string, limit int) ([]sdk.Message, error) {
	msgs, err := q.store.Messages.Pending(q.ctx, consumer, limit)
	if err != nil {
		return nil, queueErr(err)
	}
	return msgsToSDK(msgs), nil
}

func (q *queueAPI) Peek(consumer string) (*sdk.Message, error) {
	msg, err := q.store.Messages.Peek(q.ctx, consumer)
	if err != nil {
		return nil, queueErr(err)
	}
	return msgToSDK(msg), nil
}

func (q *queueAPI) Ack(key, consumer string) error {
	return queueErr(q.store.Messages.Ack(q.ctx, key, consumer))
}

func (q *queueAPI) History(opts sdk.HistoryOpts) ([]sdk.Message, error) {
	msgs, err := q.store.Messages.History(q.ctx, opts.Consumer, opts.Since)
	if err != nil {
		return nil, queueErr(err)
	}
	return msgsToSDK(msgs), nil
}

// msgToSDK converts an internal message to the SDK type.
func msgToSDK(m *message.Message) *sdk.Message {
	return &sdk.Message{
		Key:        m.Key,
		Type:       m.Type,
		Payload:    m.Payload,
		Author:     m.Author,
		Source:     m.Source,
		AssignedTo: m.AssignedTo,
		CreatedAt:  m.CreatedAt,
	}
}

// msgsToSDK converts a slice of internal messages to SDK types.
func msgsToSDK(msgs []message.Message) []sdk.Message {
	out := make([]sdk.Message, len(msgs))
	for i := range msgs {
		out[i] = *msgToSDK(&msgs[i])
	}
	return out
}
