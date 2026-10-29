package sdk

import (
	"bytes"
	"context"
	"io"
	"time"
)

// firstByteWait is how long OptionalInput waits for piped input to
// start. Agent harnesses often run commands with stdin as a pipe they
// never write to or close; a command whose input is optional must not
// wait on it forever.
const firstByteWait = 50 * time.Millisecond

// Input reads all of Stdin. It is for commands that require input
// (write, hook): it waits for the writer to finish however long that
// takes, and stops when the context is cancelled. Returns nil when
// there is no input source. Stdin can be read once.
func (c Context) Input() ([]byte, error) {
	return readInput(c, c.Stdin, 0)
}

// OptionalInput reads Stdin for commands where input is one of several
// sources (an argument, --file, stdin). It returns nil when no input
// starts within a short wait, and otherwise reads to the end with no
// limit, so a slow writer is never cut off once it has begun. Stdin
// can be read once.
func (c Context) OptionalInput() ([]byte, error) {
	return readInput(c, c.Stdin, firstByteWait)
}

// readInput reads r to the end. With wait > 0 it gives up, returning
// nil, if no byte arrives within wait. The read runs in a goroutine so
// cancellation can interrupt a pipe that never closes; the goroutine
// ends when the pipe does, which for the CLI is at process exit.
func readInput(ctx context.Context, r io.Reader, wait time.Duration) ([]byte, error) {
	if r == nil {
		return nil, nil
	}

	type result struct {
		data []byte
		err  error
	}
	started := make(chan struct{})
	done := make(chan result, 1)
	go func() {
		var buf bytes.Buffer
		p := make([]byte, 32*1024)
		signalled := false
		for {
			n, err := r.Read(p)
			if n > 0 {
				buf.Write(p[:n])
				if !signalled {
					close(started)
					signalled = true
				}
			}
			if err == io.EOF {
				done <- result{data: buf.Bytes()}
				return
			}
			if err != nil {
				done <- result{data: buf.Bytes(), err: err}
				return
			}
		}
	}()

	var timeout <-chan time.Time
	if wait > 0 {
		t := time.NewTimer(wait)
		defer t.Stop()
		timeout = t.C
	}
	first := started
	for {
		select {
		case res := <-done:
			return res.data, res.err
		case <-first:
			// Input has begun: read it all, however long it takes.
			first, timeout = nil, nil
		case <-timeout:
			return nil, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
}
