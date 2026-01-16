// Package progress provides CLI progress indicators. Output goes to stderr
// to keep stdout clean for piping, and TTY detection ensures proper formatting
// in both interactive and scripted usage.
package progress

import (
	"fmt"
	"io"
	"os"

	"golang.org/x/term"
)

// minItems is the minimum number of items before showing progress.
// For small operations, progress adds noise without benefit.
const minItems = 5

// Progress tracks and displays operation progress.
type Progress struct {
	w       io.Writer
	label   string
	total   int
	current int
	isTTY   bool
}

// New creates a progress reporter that writes to stderr.
// If total is less than minItems, progress updates are suppressed.
func New(label string, total int) *Progress {
	return &Progress{
		w:     os.Stderr,
		label: label,
		total: total,
		isTTY: term.IsTerminal(int(os.Stderr.Fd())),
	}
}

// Increment advances the progress counter by one.
func (p *Progress) Increment() {
	p.current++
}

// Print writes the current progress to stderr.
// On TTY, it uses carriage return to update in place.
// For non-TTY or small totals, this is a no-op.
func (p *Progress) Print() {
	if p.total < minItems {
		return
	}

	pct := 0
	if p.total > 0 {
		pct = (p.current * 100) / p.total
	}

	if p.isTTY {
		// Overwrite line on TTY
		fmt.Fprintf(p.w, "\r%s... %d/%d (%d%%)", p.label, p.current, p.total, pct)
	}
}

// Done clears the progress line (on TTY) to make way for final output.
func (p *Progress) Done() {
	if p.total < minItems {
		return
	}

	if p.isTTY {
		// Clear the line
		fmt.Fprintf(p.w, "\r%s\r", "                                        ")
	}
}

// Spinner provides visual feedback for indeterminate operations, showing
// users that work is in progress even when completion time is unknown.
type Spinner struct {
	w       io.Writer
	label   string
	frame   int
	isTTY   bool
	frames  []string
	running bool
}

// NewSpinner creates a spinner that writes to stderr.
func NewSpinner(label string) *Spinner {
	return &Spinner{
		w:      os.Stderr,
		label:  label,
		isTTY:  term.IsTerminal(int(os.Stderr.Fd())),
		frames: []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
	}
}

// Start displays the spinner.
func (s *Spinner) Start() {
	if !s.isTTY {
		return
	}
	s.running = true
	fmt.Fprintf(s.w, "%s %s...", s.frames[0], s.label)
}

// Tick advances the spinner animation by one frame.
func (s *Spinner) Tick() {
	if !s.isTTY || !s.running {
		return
	}
	s.frame = (s.frame + 1) % len(s.frames)
	fmt.Fprintf(s.w, "\r%s %s...", s.frames[s.frame], s.label)
}

// Stop clears the spinner line.
func (s *Spinner) Stop() {
	if !s.isTTY || !s.running {
		return
	}
	s.running = false
	fmt.Fprintf(s.w, "\r%s\r", "                                        ")
}
