// Package errs builds errors whose message is written for the reader
// and whose kind is for code.
//
// Wrapping a sentinel with fmt.Errorf("%w: ...") puts the sentinel's
// own text in the message, so each layer that classified an error
// added a phrase: "mv: already exists: destination already exists: p".
// An error from New carries its kind for errors.Is (and so for HTTP
// status codes) but its message is only the sentence it was given.
//
// Messages follow one style: a sentence that says what is wrong and
// names the thing once, then, where there is one, a sentence with the
// next step. "p is deleted. Run llmd restore p to bring it back." The
// command adds its name in front: "cat: p is deleted. ..."
package errs

import (
	"errors"
	"fmt"
)

// kinded is an error with a kind that does not appear in its message.
type kinded struct {
	kind error
	err  error
}

func (e *kinded) Error() string   { return e.err.Error() }
func (e *kinded) Unwrap() []error { return []error{e.kind, e.err} }

// New returns an error of the given kind whose message is the
// formatted text alone. A %w verb in format wraps a cause as
// fmt.Errorf does, cause text included.
func New(kind error, format string, args ...any) error {
	return &kinded{kind: kind, err: fmt.Errorf(format, args...)}
}

// Mark gives err another kind without changing its message: the host
// uses it to classify an internal error as an SDK one.
func Mark(kind, err error) error {
	if err == nil {
		return nil
	}
	return &kinded{kind: kind, err: err}
}

// Context adds what was being done to an unexpected error, such as a
// database failure. An error made by New or Mark already says what is
// wrong in the reader's terms and is returned unchanged, so callers can
// wrap every error the same way without stacking phrases on the ones a
// person will read.
func Context(err error, format string, args ...any) error {
	if err == nil {
		return nil
	}
	if _, ok := errors.AsType[*kinded](err); ok {
		return err
	}
	return fmt.Errorf(format+": %w", append(args, err)...)
}
