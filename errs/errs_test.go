package errs

import (
	"errors"
	"testing"
)

var errKind = errors.New("not found")

func TestNew(t *testing.T) {
	cause := errors.New("disk full")
	tests := []struct {
		name      string
		err       error
		wantMsg   string
		wantIs    []error
		wantNotIs []error
	}{
		{
			name:    "message is the sentence alone",
			err:     New(errKind, "%s does not exist.", "p"),
			wantMsg: "p does not exist.",
			wantIs:  []error{errKind},
		},
		{
			name:    "a wrapped cause keeps its text",
			err:     New(errKind, "reading p: %w", cause),
			wantMsg: "reading p: disk full",
			wantIs:  []error{errKind, cause},
		},
		{
			name:    "mark adds a kind, not text",
			err:     Mark(errKind, cause),
			wantMsg: "disk full",
			wantIs:  []error{errKind, cause},
		},
		{
			name:    "context leaves a reader's message alone",
			err:     Context(New(errKind, "p does not exist."), "checking %s", "p"),
			wantMsg: "p does not exist.",
			wantIs:  []error{errKind},
		},
		{
			name:      "context describes an unexpected error",
			err:       Context(cause, "checking %s", "p"),
			wantMsg:   "checking p: disk full",
			wantIs:    []error{cause},
			wantNotIs: []error{errKind},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.err.Error(); got != tt.wantMsg {
				t.Errorf("Error() = %q, want %q", got, tt.wantMsg)
			}
			for _, k := range tt.wantIs {
				if !errors.Is(tt.err, k) {
					t.Errorf("errors.Is(err, %v) = false, want true", k)
				}
			}
			for _, k := range tt.wantNotIs {
				if errors.Is(tt.err, k) {
					t.Errorf("errors.Is(err, %v) = true, want false", k)
				}
			}
		})
	}
	if Mark(errKind, nil) != nil || Context(nil, "x") != nil {
		t.Error("Mark(nil) and Context(nil) must return nil")
	}
}
