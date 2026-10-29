package cli

import (
	"errors"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jpl-au/llmd/internal/clock/clocktest"
	"github.com/jpl-au/llmd/sdk"
)

// queueClock installs a deterministic clock so every message gets a
// distinct, increasing timestamp on a whole-second boundary: the queue
// orders by created_at only, and --since accepts RFC 3339 at second
// precision. Timestamps start ten minutes ago so relative --since
// values ("1h") still include them.
func queueClock(t *testing.T) {
	t.Helper()
	start := time.Now().Truncate(time.Second).Add(-10 * time.Minute).UnixMilli()
	clocktest.Sequence(t, start, 1000)
}

// queueMsgOf returns the *sdk.Message carried by a send / peek response.
func queueMsgOf(t *testing.T, resp sdk.Response) sdk.Message {
	t.Helper()
	m, ok := data(resp).(*sdk.Message)
	if !ok || m == nil {
		t.Fatalf("queue response Data = %T, want *sdk.Message", data(resp))
	}
	return *m
}

// queueMsgsOf returns the []sdk.Message carried by an ls / history response.
func queueMsgsOf(t *testing.T, resp sdk.Response) []sdk.Message {
	t.Helper()
	mm, ok := data(resp).([]sdk.Message)
	if !ok {
		t.Fatalf("queue response Data = %T, want []sdk.Message", data(resp))
	}
	return mm
}

// queuePost sends content as author and returns the message.
func queuePost(t *testing.T, c sdk.Context, author, content string, args ...string) sdk.Message {
	t.Helper()
	c.Author = author
	all := append([]string{"send", content}, args...)
	return queueMsgOf(t, run(t, c, "queue", nil, all...))
}

// queueAs returns c acting as author.
func queueAs(c sdk.Context, author string) sdk.Context {
	c.Author = author
	return c
}

// queueFixture seeds the smoke scenario: human broadcasts "deployment
// complete" ({b}), then directs "review task x" to claude ({d}).
func queueFixture(t *testing.T, c sdk.Context) (b, d sdk.Message) {
	t.Helper()
	b = queuePost(t, c, "human", "deployment complete")
	d = queuePost(t, c, "human", "review task x", "--assign", "claude")
	return b, d
}

// queueExpand replaces {b} and {d} in args.
func queueExpand(args []string, b, d sdk.Message) []string {
	repl := strings.NewReplacer("{b}", b.Key, "{d}", d.Key)
	out := make([]string, len(args))
	for i, a := range args {
		out[i] = repl.Replace(a)
	}
	return out
}

// TestQueueCmd covers the dispatcher: a subcommand is required, every
// subcommand requires an author, and unknown subcommands are rejected.
func TestQueueCmd(t *testing.T) {
	tests := []struct {
		name    string
		author  string
		args    []string
		wantErr error
		wantMsg string
	}{
		{name: "no subcommand", author: "claude", wantErr: sdk.ErrMissingArg},
		{name: "unknown subcommand", author: "claude", args: []string{"bogus"}, wantMsg: "unknown subcommand"},
		{name: "send needs author", args: []string{"send", "hi"}, wantMsg: "an author is required."},
		{name: "ls needs author", args: []string{"ls"}, wantMsg: "an author is required."},
		{name: "peek needs author", args: []string{"peek"}, wantMsg: "an author is required."},
		{name: "ack needs author", args: []string{"ack", "abc"}, wantMsg: "an author is required."},
		{name: "history needs author", args: []string{"history"}, wantMsg: "an author is required."},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newCLI(t, tt.author)
			err := runErr(t, c, "queue", nil, tt.args...)
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Errorf("queue %v error = %v, want %v", tt.args, err, tt.wantErr)
			}
			if tt.wantMsg != "" && (err == nil || !strings.Contains(err.Error(), tt.wantMsg)) {
				t.Errorf("queue %v error = %v, want to contain %q", tt.args, err, tt.wantMsg)
			}
		})
	}
}

// TestQueueSend covers broadcast and directed sends, content from
// arguments or stdin, and the argument errors.
func TestQueueSend(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		stdin      string
		wantSuffix string // text after "Sent <key>"
		want       sdk.Message
		wantErr    error
	}{
		{
			name: "broadcast",
			args: []string{"send", "deployment complete"},
			want: sdk.Message{Type: "direct", Payload: "deployment complete", Author: "human", Source: "cli"},
		},
		{
			name:       "directed",
			args:       []string{"send", "review task x", "--assign", "claude"},
			wantSuffix: " -> claude",
			want:       sdk.Message{Payload: "review task x", AssignedTo: "claude"},
		},
		{
			name: "positional words are joined",
			args: []string{"send", "two", "words"},
			want: sdk.Message{Payload: "two words"},
		},
		{
			name:       "content from stdin",
			args:       []string{"send", "--assign", "gemini"},
			stdin:      "detailed instructions",
			wantSuffix: " -> gemini",
			want:       sdk.Message{Payload: "detailed instructions", AssignedTo: "gemini"},
		},
		{
			name:  "positional beats stdin",
			args:  []string{"send", "inline"},
			stdin: "from stdin",
			want:  sdk.Message{Payload: "inline"},
		},
		{
			name:    "no content",
			args:    []string{"send"},
			wantErr: sdk.ErrMissingArg,
		},
		{
			name:    "unknown flag",
			args:    []string{"send", "hi", "--limit", "1"},
			wantErr: sdk.ErrInvalidArg,
		},
		{
			name:    "assign without value",
			args:    []string{"send", "hi", "--assign"},
			wantErr: sdk.ErrInvalidArg,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newCLI(t, "human")
			var stdin []byte
			if tt.stdin != "" {
				stdin = []byte(tt.stdin)
			}
			if tt.wantErr != nil {
				if err := runErr(t, c, "queue", stdin, tt.args...); !errors.Is(err, tt.wantErr) {
					t.Errorf("queue %v error = %v, want %v", tt.args, err, tt.wantErr)
				}
				return
			}

			resp := run(t, c, "queue", stdin, tt.args...)
			msg := queueMsgOf(t, resp)
			if len(msg.Key) != 10 {
				t.Errorf("queue %v Key = %q, want a 10-char key", tt.args, msg.Key)
			}
			if got, want := text(resp), "Sent "+msg.Key+tt.wantSuffix; got != want {
				t.Errorf("queue %v = %q, want %q", tt.args, got, want)
			}
			if tt.want.Type != "" && msg.Type != tt.want.Type {
				t.Errorf("queue %v Type = %q, want %q", tt.args, msg.Type, tt.want.Type)
			}
			if tt.want.Source != "" && msg.Source != tt.want.Source {
				t.Errorf("queue %v Source = %q, want %q", tt.args, msg.Source, tt.want.Source)
			}
			if tt.want.Author != "" && msg.Author != tt.want.Author {
				t.Errorf("queue %v Author = %q, want %q", tt.args, msg.Author, tt.want.Author)
			}
			if msg.Payload != tt.want.Payload {
				t.Errorf("queue %v Payload = %q, want %q", tt.args, msg.Payload, tt.want.Payload)
			}
			if msg.AssignedTo != tt.want.AssignedTo {
				t.Errorf("queue %v AssignedTo = %q, want %q", tt.args, msg.AssignedTo, tt.want.AssignedTo)
			}
			// The message is visible to its recipient (or everyone).
			reader := msg.AssignedTo
			if reader == "" {
				reader = "anyone"
			}
			if got := text(run(t, queueAs(c, reader), "queue", nil, "ls")); !strings.Contains(got, msg.Key) {
				t.Errorf("queue ls as %s = %q, want to contain %q", reader, got, msg.Key)
			}
		})
	}
}

// TestQueueLs covers per-consumer visibility of broadcast and directed
// messages, ordering, --limit, independent acknowledgement and the
// empty queue. Fixture: {b} broadcast then {d} directed to claude;
// acks are performed by claude before the call.
func TestQueueLs(t *testing.T) {
	tests := []struct {
		name       string
		author     string
		acks       []string
		args       []string
		wantKeys   []string
		wantAbsent []string
		wantEmpty  bool
		wantErr    error
	}{
		{
			name:     "claude sees broadcast and directed, oldest first",
			author:   "claude",
			args:     []string{"ls"},
			wantKeys: []string{"{b}", "{d}"},
		},
		{
			name:     "list alias",
			author:   "claude",
			args:     []string{"list"},
			wantKeys: []string{"{b}", "{d}"},
		},
		{
			name:       "gemini only sees the broadcast",
			author:     "gemini",
			args:       []string{"ls"},
			wantKeys:   []string{"{b}"},
			wantAbsent: []string{"{d}"},
		},
		{
			name:       "limit",
			author:     "claude",
			args:       []string{"ls", "--limit", "1"},
			wantKeys:   []string{"{b}"},
			wantAbsent: []string{"{d}"},
		},
		{
			name:       "acked messages disappear",
			author:     "claude",
			acks:       []string{"{b}"},
			args:       []string{"ls"},
			wantKeys:   []string{"{d}"},
			wantAbsent: []string{"{b}"},
		},
		{
			name:      "empty after acking all",
			author:    "claude",
			acks:      []string{"{b}", "{d}"},
			args:      []string{"ls"},
			wantEmpty: true,
		},
		{
			name:     "claude's acks do not affect gemini",
			author:   "gemini",
			acks:     []string{"{b}", "{d}"},
			args:     []string{"ls"},
			wantKeys: []string{"{b}"},
		},
		{
			name:    "unknown flag",
			author:  "claude",
			args:    []string{"ls", "--since", "1h"},
			wantErr: sdk.ErrInvalidArg,
		},
		{
			name:    "limit must be an integer",
			author:  "claude",
			args:    []string{"ls", "--limit", "many"},
			wantErr: sdk.ErrInvalidArg,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queueClock(t)
			c := newCLI(t, "human")
			b, d := queueFixture(t, c)
			for _, k := range queueExpand(tt.acks, b, d) {
				run(t, queueAs(c, "claude"), "queue", nil, "ack", k)
			}
			args := queueExpand(tt.args, b, d)
			me := queueAs(c, tt.author)

			if tt.wantErr != nil {
				if err := runErr(t, me, "queue", nil, args...); !errors.Is(err, tt.wantErr) {
					t.Errorf("queue %v error = %v, want %v", args, err, tt.wantErr)
				}
				return
			}

			resp := run(t, me, "queue", nil, args...)
			got := text(resp)
			msgs := queueMsgsOf(t, resp)
			if tt.wantEmpty {
				if got != "No pending messages" || len(msgs) != 0 {
					t.Errorf("queue %v = %q (%d messages), want %q and none", args, got, len(msgs), "No pending messages")
				}
				return
			}
			wantKeys := queueExpand(tt.wantKeys, b, d)
			if len(msgs) != len(wantKeys) {
				t.Errorf("queue %v returned %d messages, want %d", args, len(msgs), len(wantKeys))
			}
			for i, k := range wantKeys {
				if !strings.Contains(got, k) {
					t.Errorf("queue %v = %q, want to contain %q", args, got, k)
				}
				if i < len(msgs) && msgs[i].Key != k {
					t.Errorf("queue %v Data[%d].Key = %q, want %q", args, i, msgs[i].Key, k)
				}
			}
			for _, k := range queueExpand(tt.wantAbsent, b, d) {
				if strings.Contains(got, k) {
					t.Errorf("queue %v = %q, must not contain %q", args, got, k)
				}
			}
			// Each line shows type and author.
			for _, m := range msgs {
				if !strings.Contains(got, m.Type) || !strings.Contains(got, m.Author) {
					t.Errorf("queue %v = %q, want to contain %q and %q", args, got, m.Type, m.Author)
				}
			}
		})
	}

	t.Run("fresh queue is empty", func(t *testing.T) {
		c := newCLI(t, "claude")
		resp := run(t, c, "queue", nil, "ls")
		if got := text(resp); got != "No pending messages" {
			t.Errorf("queue ls = %q, want %q", got, "No pending messages")
		}
		if msgs := queueMsgsOf(t, resp); len(msgs) != 0 {
			t.Errorf("queue ls Data = %v, want none", msgs)
		}
	})
}

// TestQueueEvents ports the event bus smoke: once the work database is
// open, domain events (a document write, an audit) land in every
// consumer's queue with source "bus", while the queue's own
// message.sent events do not feed back.
func TestQueueEvents(t *testing.T) {
	queueClock(t)
	c := newCLI(t, "smoke")
	// Opening the queue opens work.db; only then does the bridge queue events.
	run(t, c, "queue", nil, "ls")

	writeDoc(t, c, "docs/test", "# Test doc")
	run(t, c, "audit", nil, "add", "docs/test", "check this")
	queuePost(t, c, "smoke", "direct one")

	resp := run(t, queueAs(c, "observer"), "queue", nil, "ls")
	got := text(resp)
	msgs := queueMsgsOf(t, resp)

	byType := map[string]sdk.Message{}
	for _, m := range msgs {
		byType[m.Type] = m
	}
	for _, typ := range []string{"document.written", "audit.created", "direct"} {
		if !strings.Contains(got, typ) {
			t.Errorf("queue ls = %q, want to contain %q", got, typ)
		}
		if _, ok := byType[typ]; !ok {
			t.Errorf("queue ls Data has no %q message: %v", typ, msgs)
		}
	}
	if strings.Contains(got, "message.sent") {
		t.Errorf("queue ls = %q, must not contain the queue's own events", got)
	}
	if m := byType["document.written"]; m.Source != "bus" || m.Author != "smoke" || !strings.Contains(m.Payload, "docs/test") {
		t.Errorf("document.written message = %+v, want source bus, author smoke and the path in the payload", m)
	}
	if m := byType["direct"]; m.Source != "cli" {
		t.Errorf("direct message Source = %q, want cli", m.Source)
	}
	if len(msgs) != 3 {
		t.Errorf("queue ls returned %d messages, want 3: %v", len(msgs), msgs)
	}
}

// TestQueuePeek covers the oldest pending message with its payload, the
// per-consumer view, and the empty queue.
func TestQueuePeek(t *testing.T) {
	tests := []struct {
		name     string
		author   string
		acks     []string // acked by author before the call
		wantKey  string
		wantText []string
		wantErr  error
	}{
		{
			name:     "claude peeks the oldest",
			author:   "claude",
			wantKey:  "{b}",
			wantText: []string{"{b}", "direct", "human", "deployment complete"},
		},
		{
			name:     "after acking the oldest, the directed message is next",
			author:   "claude",
			acks:     []string{"{b}"},
			wantKey:  "{d}",
			wantText: []string{"review task x"},
		},
		{
			name:    "gemini does not see the directed message",
			author:  "gemini",
			wantKey: "{b}",
		},
		{
			name:    "gemini's queue is empty once the broadcast is acked",
			author:  "gemini",
			acks:    []string{"{b}"},
			wantErr: sdk.ErrNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queueClock(t)
			c := newCLI(t, "human")
			b, d := queueFixture(t, c)
			me := queueAs(c, tt.author)
			for _, k := range queueExpand(tt.acks, b, d) {
				run(t, me, "queue", nil, "ack", k)
			}

			if tt.wantErr != nil {
				if err := runErr(t, me, "queue", nil, "peek"); !errors.Is(err, tt.wantErr) {
					t.Errorf("queue peek error = %v, want %v", err, tt.wantErr)
				}
				return
			}

			resp := run(t, me, "queue", nil, "peek")
			got := text(resp)
			for _, want := range queueExpand(tt.wantText, b, d) {
				if !strings.Contains(got, want) {
					t.Errorf("queue peek = %q, want to contain %q", got, want)
				}
			}
			msg := queueMsgOf(t, resp)
			if want := queueExpand([]string{tt.wantKey}, b, d)[0]; msg.Key != want {
				t.Errorf("queue peek Key = %q, want %q", msg.Key, want)
			}
			if !strings.Contains(got, msg.Payload) {
				t.Errorf("queue peek = %q, want to contain the payload %q", got, msg.Payload)
			}
		})
	}

	t.Run("fresh queue", func(t *testing.T) {
		c := newCLI(t, "claude")
		if err := runErr(t, c, "queue", nil, "peek"); !errors.Is(err, sdk.ErrNotFound) {
			t.Errorf("queue peek error = %v, want %v", err, sdk.ErrNotFound)
		}
	})
}

// TestQueueAck covers strict ordering: only the oldest pending message
// can be acknowledged, acks are idempotent per consumer only through
// the ordering rule, and an empty queue reports not found.
func TestQueueAck(t *testing.T) {
	tests := []struct {
		name    string
		acks    []string // successful acks before the call
		args    []string
		wantErr error
	}{
		{name: "out of order", args: []string{"ack", "{d}"}, wantErr: sdk.ErrOrderViolation},
		{name: "oldest succeeds", args: []string{"ack", "{b}"}},
		{name: "second in order succeeds", acks: []string{"{b}"}, args: []string{"ack", "{d}"}},
		{name: "acking the oldest twice is an order violation", acks: []string{"{b}"}, args: []string{"ack", "{b}"}, wantErr: sdk.ErrOrderViolation},
		{name: "unknown key with pending messages", args: []string{"ack", "zzzzzzzzz"}, wantErr: sdk.ErrOrderViolation},
		{name: "empty queue", acks: []string{"{b}", "{d}"}, args: []string{"ack", "{b}"}, wantErr: sdk.ErrNotFound},
		{name: "missing key", args: []string{"ack"}, wantErr: sdk.ErrMissingArg},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queueClock(t)
			c := newCLI(t, "human")
			b, d := queueFixture(t, c)
			claude := queueAs(c, "claude")
			for _, k := range queueExpand(tt.acks, b, d) {
				run(t, claude, "queue", nil, "ack", k)
			}
			args := queueExpand(tt.args, b, d)

			if tt.wantErr != nil {
				if err := runErr(t, claude, "queue", nil, args...); !errors.Is(err, tt.wantErr) {
					t.Errorf("queue %v error = %v, want %v", args, err, tt.wantErr)
				}
				return
			}

			resp := run(t, claude, "queue", nil, args...)
			if got, want := text(resp), "Acknowledged "+args[1]; got != want {
				t.Errorf("queue %v = %q, want %q", args, got, want)
			}
			if _, ok := resp.(sdk.Text); !ok {
				t.Errorf("queue %v response type = %T, want sdk.Text", args, resp)
			}
			if got := text(run(t, claude, "queue", nil, "ls")); strings.Contains(got, args[1]) {
				t.Errorf("queue ls after ack = %q, must not contain %q", got, args[1])
			}
			// Acknowledged messages stay in history.
			if got := text(run(t, claude, "queue", nil, "history")); !strings.Contains(got, args[1]) {
				t.Errorf("queue history after ack = %q, want to contain %q", got, args[1])
			}
		})
	}
}

// TestQueueHistory covers the full log including acknowledged messages,
// per-consumer visibility, the default cap of twenty with -n and --all,
// and --since. Fixture: {b} broadcast, {d} directed to claude, plus
// extra broadcasts when the case asks for them.
func TestQueueHistory(t *testing.T) {
	tests := []struct {
		name       string
		author     string
		extra      int // additional broadcasts sent after the fixture
		ackAll     bool
		args       []string
		since      string // "b" or "d": --since the fixture message's timestamp
		wantLen    int
		wantKeys   []string
		wantAbsent []string
		wantEmpty  bool
		wantErr    error
	}{
		{
			name:     "claude sees everything in order",
			author:   "claude",
			args:     []string{"history"},
			wantLen:  2,
			wantKeys: []string{"{b}", "{d}"},
		},
		{
			name:       "gemini does not see the directed message",
			author:     "gemini",
			args:       []string{"history"},
			wantLen:    1,
			wantKeys:   []string{"{b}"},
			wantAbsent: []string{"{d}"},
		},
		{
			name:     "acknowledged messages remain",
			author:   "claude",
			ackAll:   true,
			args:     []string{"history"},
			wantLen:  2,
			wantKeys: []string{"{b}", "{d}"},
		},
		{
			name:       "default cap keeps the most recent twenty",
			author:     "claude",
			extra:      23,
			args:       []string{"history"},
			wantLen:    20,
			wantAbsent: []string{"{b}", "{d}"},
		},
		{
			name:       "--limit caps",
			author:     "claude",
			extra:      3,
			args:       []string{"history", "--limit", "2"},
			wantLen:    2,
			wantAbsent: []string{"{b}", "{d}"},
		},
		{
			name:     "--all lifts the cap",
			author:   "claude",
			extra:    23,
			args:     []string{"history", "--all"},
			wantLen:  25,
			wantKeys: []string{"{b}", "{d}"},
		},
		{
			name:       "since a timestamp excludes older messages",
			author:     "claude",
			args:       []string{"history", "--since", "{since}"},
			since:      "b",
			wantLen:    1,
			wantKeys:   []string{"{d}"},
			wantAbsent: []string{"{b}"},
		},
		{
			name:      "since the newest message is empty",
			author:    "claude",
			args:      []string{"history", "--since", "{since}"},
			since:     "d",
			wantEmpty: true,
		},
		{
			name:    "invalid since",
			author:  "claude",
			args:    []string{"history", "--since", "lately"},
			wantErr: errAuditAny,
		},
		{
			name:    "unknown flag",
			author:  "claude",
			args:    []string{"history", "--bogus", "1"},
			wantErr: sdk.ErrInvalidArg,
		},
		{
			name:    "-n (short for --limit) must be an integer",
			author:  "claude",
			args:    []string{"history", "-n", "few"},
			wantErr: sdk.ErrInvalidArg,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			queueClock(t)
			c := newCLI(t, "human")
			b, d := queueFixture(t, c)
			for i := range tt.extra {
				queuePost(t, c, "human", "extra "+strconv.Itoa(i))
			}
			me := queueAs(c, tt.author)
			if tt.ackAll {
				run(t, me, "queue", nil, "ack", b.Key)
				run(t, me, "queue", nil, "ack", d.Key)
			}
			args := queueExpand(tt.args, b, d)
			if tt.since != "" {
				at := b.CreatedAt
				if tt.since == "d" {
					at = d.CreatedAt
				}
				stamp := time.UnixMilli(at).UTC().Format(time.RFC3339)
				for i := range args {
					args[i] = strings.ReplaceAll(args[i], "{since}", stamp)
				}
			}

			if tt.wantErr != nil {
				err := runErr(t, me, "queue", nil, args...)
				if tt.wantErr == errAuditAny && err == nil {
					t.Errorf("queue %v error = nil, want an error", args)
				} else if tt.wantErr != errAuditAny && !errors.Is(err, tt.wantErr) {
					t.Errorf("queue %v error = %v, want %v", args, err, tt.wantErr)
				}
				return
			}

			resp := run(t, me, "queue", nil, args...)
			got := text(resp)
			msgs := queueMsgsOf(t, resp)
			if tt.wantEmpty {
				if got != "No messages" || len(msgs) != 0 {
					t.Errorf("queue %v = %q (%d messages), want %q and none", args, got, len(msgs), "No messages")
				}
				return
			}
			if len(msgs) != tt.wantLen {
				t.Errorf("queue %v returned %d messages, want %d", args, len(msgs), tt.wantLen)
			}
			for i := 1; i < len(msgs); i++ {
				if msgs[i].CreatedAt < msgs[i-1].CreatedAt {
					t.Errorf("queue %v Data is not oldest first at index %d", args, i)
				}
			}
			for _, k := range queueExpand(tt.wantKeys, b, d) {
				if !strings.Contains(got, k) {
					t.Errorf("queue %v = %q, want to contain %q", args, got, k)
				}
			}
			for _, k := range queueExpand(tt.wantAbsent, b, d) {
				if strings.Contains(got, k) {
					t.Errorf("queue %v = %q, must not contain %q", args, got, k)
				}
			}
		})
	}

	t.Run("fresh queue", func(t *testing.T) {
		c := newCLI(t, "claude")
		resp := run(t, c, "queue", nil, "history")
		if got := text(resp); got != "No messages" {
			t.Errorf("queue history = %q, want %q", got, "No messages")
		}
		if msgs := queueMsgsOf(t, resp); len(msgs) != 0 {
			t.Errorf("queue history Data = %v, want none", msgs)
		}
	})
}
