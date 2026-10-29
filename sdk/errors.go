package sdk

import (
	"fmt"

	"github.com/jpl-au/llmd/internal/errs"
)

// Errorf returns an error of the given kind (ErrNotFound, ErrMissingArg,
// ...) whose message is the formatted text alone. errors.Is(err, kind)
// holds, and the transports map the kind to a status, but the kind's
// own text is not added to the message.
//
// Write the message as a sentence that says what is wrong and names the
// thing once, then, where there is one, a sentence with the next step:
//
//	sdk.Errorf(sdk.ErrNotFound, "tag: %s has no tag %s.", path, name)
//	sdk.Errorf(sdk.ErrMissingArg, "write: no content. Pipe the document to llmd write %s.", path)
func Errorf(kind error, format string, args ...any) error {
	return errs.New(kind, format, args...)
}

// AuthorRequired is the error for a change attempted without an
// author, the same sentence from every command and transport. cmd
// leads the message; pass "" where the caller's handler adds it.
func AuthorRequired(cmd string) error {
	const msg = "an author is required. Pass --author, the MCP author field or the HTTP Author header."
	if cmd == "" {
		return Errorf(ErrMissingArg, msg)
	}
	return Errorf(ErrMissingArg, "%s: "+msg, cmd)
}

// ExitError ends a command with a process exit code instead of a
// message, for a command whose result is another program's exit
// status (agent run). The CLI exits with Code and prints nothing; MCP
// and HTTP callers see Error's sentence.
type ExitError struct {
	Cmd  string
	Code int
}

func (e *ExitError) Error() string {
	return fmt.Sprintf("%s: the process exited with code %d.", e.Cmd, e.Code)
}
