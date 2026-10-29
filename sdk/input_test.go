package sdk

import (
	"context"
	"errors"
	"io"
	"os"
	"strings"
	"testing"
	"time"
)

// TestReadInput covers required and optional input against writers of
// different speeds. Stdin once had 50 ms to finish, so a writer slower
// than that lost its input. Each case writes parts to a pipe with a
// pause before each part; closeAfter false leaves the pipe open, as an
// agent harness does.
func TestReadInput(t *testing.T) {
	tests := []struct {
		name       string
		wait       time.Duration
		parts      []string
		pause      time.Duration
		closeAfter bool
		want       string
	}{
		{
			name:       "required input waits for a slow writer",
			parts:      []string{"slow"},
			pause:      200 * time.Millisecond,
			closeAfter: true,
			want:       "slow",
		},
		{
			name:       "optional input reads a writer that starts at once",
			wait:       firstByteWait,
			parts:      []string{"quick"},
			closeAfter: true,
			want:       "quick",
		},
		{
			name:       "optional input keeps reading once input has begun",
			wait:       firstByteWait,
			parts:      []string{"begun", " and", " finished"},
			pause:      100 * time.Millisecond,
			closeAfter: true,
			want:       "begun and finished",
		},
		{
			name: "optional input gives up on an open pipe with nothing in it",
			wait: firstByteWait,
		},
		{
			name:       "empty pipe is no input",
			closeAfter: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, w, err := os.Pipe()
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { r.Close(); w.Close() })

			go func() {
				for i, p := range tt.parts {
					// The first part of "keeps reading" goes at once
					// so the input begins inside the wait.
					if i > 0 || tt.wait == 0 {
						time.Sleep(tt.pause)
					}
					if _, err := io.WriteString(w, p); err != nil {
						t.Errorf("writing to the pipe: %v", err)
						return
					}
				}
				if tt.closeAfter {
					w.Close()
				}
			}()

			got, err := readInput(t.Context(), r, tt.wait)
			if err != nil {
				t.Fatalf("readInput() error = %v", err)
			}
			if string(got) != tt.want {
				t.Errorf("readInput() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestReadInputNil checks that a context with no input source reads
// as no input.
func TestReadInputNil(t *testing.T) {
	got, err := readInput(t.Context(), nil, 0)
	if got != nil || err != nil {
		t.Errorf("readInput(nil) = %q, %v, want nil, nil", got, err)
	}
}

// TestReadInputCancel checks that cancelling the context (Ctrl-C)
// ends a required read on a pipe that never closes.
func TestReadInputCancel(t *testing.T) {
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { r.Close(); w.Close() })

	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	if _, err := readInput(ctx, r, 0); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("readInput() error = %v, want %v", err, context.DeadlineExceeded)
	}
}

// TestContextInput checks the two methods read the context's Stdin.
func TestContextInput(t *testing.T) {
	c := Context{Context: t.Context(), Stdin: strings.NewReader("body")}
	if got, err := c.Input(); string(got) != "body" || err != nil {
		t.Errorf("Input() = %q, %v, want %q, nil", got, err, "body")
	}
	c.Stdin = strings.NewReader("body")
	if got, err := c.OptionalInput(); string(got) != "body" || err != nil {
		t.Errorf("OptionalInput() = %q, %v, want %q, nil", got, err, "body")
	}
}
