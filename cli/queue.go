// queue.go dispatches queue subcommands.

package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/jpl-au/llmd/sdk"
)

var queueSpec = sdk.Command{
	Name: "queue", Desc: `Message queue for cross-consumer coordination.

Subcommands (passed as first arg):
  send <content>             send a message (broadcast or directed)
  ls [--limit N]             pending messages, oldest first
  peek                       next unacknowledged message
  ack <key>                  acknowledge oldest pending message
  history [--since 5m]       all messages including acknowledged`, Usage: "queue <subcommand> [options]", MCP: true, MCPName: "queue", Flags: []sdk.Flag{
		{Name: "assign", Type: "string", Desc: "Direct message to a specific consumer"},
		{Name: "limit", Type: "int", Desc: "Maximum messages to return"},
		{Name: "since", Type: "string", Desc: "Show messages after (e.g. 5m, 1h, RFC 3339)"},
	},
}

// queueCmd dispatches to queue subcommands. All subcommands require
// --author so the queue can bind acks to an identity.
func queueCmd(ctx sdk.Context, args []string) (sdk.Response, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("queue: %w", sdk.ErrMissingArg)
	}
	if ctx.Author == "" {
		return nil, fmt.Errorf("queue %s: author required", args[0])
	}

	sub := args[0]
	args = args[1:]

	switch sub {
	case "send":
		return queueSend(ctx, args)
	case "ls", "list":
		return queueList(ctx, args)
	case "peek":
		return queuePeek(ctx, args)
	case "ack":
		return queueAck(ctx, args)
	case "history":
		return queueHistory(ctx, args)
	default:
		return nil, fmt.Errorf("queue: unknown subcommand: %s", sub)
	}
}

var queueSendFlags = []sdk.Flag{
	{Name: "assign", Type: "string"},
}

func queueSend(ctx sdk.Context, args []string) (sdk.Response, error) {
	flags, positional, err := sdk.ParseArgs(queueSendFlags, args)
	if err != nil {
		return nil, fmt.Errorf("queue send: %w", err)
	}

	content := strings.Join(positional, " ")
	if content == "" && len(ctx.Stdin) > 0 {
		content = string(ctx.Stdin)
	}
	if content == "" {
		return nil, fmt.Errorf("queue send: %w: content", sdk.ErrMissingArg)
	}

	msg, err := ctx.Queue.Send(sdk.SendOpts{
		Type:       "direct",
		Payload:    content,
		Author:     ctx.Author,
		Source:     "cli",
		AssignedTo: flags.String("assign"),
	})
	if err != nil {
		return nil, fmt.Errorf("queue send: %w", err)
	}

	text := fmt.Sprintf("Sent %s", msg.Key)
	if msg.AssignedTo != "" {
		text += fmt.Sprintf(" -> %s", msg.AssignedTo)
	}
	return sdk.Result{Text: text, Data: msg}, nil
}

var queueListFlags = []sdk.Flag{
	{Name: "limit", Type: "int"},
}

func queueList(ctx sdk.Context, args []string) (sdk.Response, error) {
	flags, _, err := sdk.ParseArgs(queueListFlags, args)
	if err != nil {
		return nil, fmt.Errorf("queue ls: %w", err)
	}

	msgs, err := ctx.Queue.Pending(ctx.Author, flags.Int("limit"))
	if err != nil {
		return nil, fmt.Errorf("queue ls: %w", err)
	}

	if len(msgs) == 0 {
		return sdk.Result{Text: "No pending messages.", Data: msgs}, nil
	}

	var b strings.Builder
	for _, m := range msgs {
		ts := time.UnixMilli(m.CreatedAt).Format("2006-01-02 15:04")
		fmt.Fprintf(&b, "%-14s  %-20s  %-12s  %s\n", m.Key, m.Type, m.Author, ts)
	}
	return sdk.Result{Text: b.String(), Data: msgs}, nil
}

func queuePeek(ctx sdk.Context, args []string) (sdk.Response, error) {
	msg, err := ctx.Queue.Peek(ctx.Author)
	if err != nil {
		return nil, fmt.Errorf("queue peek: %w", err)
	}

	var b strings.Builder
	ts := time.UnixMilli(msg.CreatedAt).Format("2006-01-02 15:04")
	fmt.Fprintf(&b, "%s  %s  %s  %s\n", msg.Key, msg.Type, msg.Author, ts)
	if msg.Payload != "" {
		b.WriteString(msg.Payload)
		b.WriteByte('\n')
	}
	return sdk.Result{Text: b.String(), Data: msg}, nil
}

func queueAck(ctx sdk.Context, args []string) (sdk.Response, error) {
	if len(args) == 0 {
		return nil, fmt.Errorf("queue ack: %w: message key", sdk.ErrMissingArg)
	}

	if err := ctx.Queue.Ack(args[0], ctx.Author); err != nil {
		return nil, fmt.Errorf("queue ack: %w", err)
	}

	return sdk.Text(fmt.Sprintf("Acknowledged %s", args[0])), nil
}

// defaultQueueHistoryLimit caps queue history output. Long-running
// queues accumulate thousands of messages; an agent calling history
// without a filter should not get them all dumped into its context.
// The most recent 20 is a reasonable window for "what's been
// happening lately?"; --all overrides.
const defaultQueueHistoryLimit = 20

var queueHistoryFlags = []sdk.Flag{
	{Name: "since", Type: "string"},
	{Name: "n", Type: "int", Desc: "Maximum messages to show (default 20)"},
	{Name: "all", Type: "bool", Desc: "Show every message, no limit"},
}

func queueHistory(ctx sdk.Context, args []string) (sdk.Response, error) {
	flags, _, err := sdk.ParseArgs(queueHistoryFlags, args)
	if err != nil {
		return nil, fmt.Errorf("queue history: %w", err)
	}

	since, err := sdk.ParseSince(flags.String("since"))
	if err != nil {
		return nil, fmt.Errorf("queue history: %w", err)
	}

	var sinceMS int64
	if !since.IsZero() {
		sinceMS = since.UnixMilli()
	}

	msgs, err := ctx.Queue.History(sdk.HistoryOpts{
		Consumer: ctx.Author,
		Since:    sinceMS,
	})
	if err != nil {
		return nil, fmt.Errorf("queue history: %w", err)
	}

	if len(msgs) == 0 {
		return sdk.Result{Text: "No messages.", Data: msgs}, nil
	}

	// Resolve the cap: --all beats -n beats the default.
	limit := flags.Int("n")
	if flags.Bool("all") {
		limit = 0
	} else if limit == 0 {
		limit = defaultQueueHistoryLimit
	}
	if limit > 0 && len(msgs) > limit {
		// Keep the most recent N. The slice is oldest-first, so the
		// tail is most recent.
		msgs = msgs[len(msgs)-limit:]
	}

	var b strings.Builder
	for _, m := range msgs {
		ts := time.UnixMilli(m.CreatedAt).Format("2006-01-02 15:04")
		fmt.Fprintf(&b, "%-14s  %-20s  %-12s  %s\n", m.Key, m.Type, m.Author, ts)
	}
	return sdk.Result{Text: b.String(), Data: msgs}, nil
}
