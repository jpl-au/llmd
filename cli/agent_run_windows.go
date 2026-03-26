//go:build windows

package cli

import (
	"os"
	"os/exec"
	"os/signal"
)

// forward arranges for interrupt signals to be forwarded to
// the child process. On Windows only os.Interrupt is available, and
// Process.Signal does not support it for other processes, so we fall
// back to killing the child outright.
func forward(cmd *exec.Cmd) func() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	go func() {
		_, ok := <-ch
		if !ok {
			return
		}
		cmd.Process.Kill()
	}()
	return func() {
		signal.Stop(ch)
		close(ch)
	}
}
