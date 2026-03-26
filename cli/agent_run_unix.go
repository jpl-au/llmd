//go:build !windows

package cli

import (
	"os"
	"os/exec"
	"os/signal"
	"syscall"
)

// forward arranges for SIGTERM and SIGINT to be forwarded to
// the child process. Returns a function that stops the forwarding.
func forward(cmd *exec.Cmd) func() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGTERM, syscall.SIGINT)
	go func() {
		sig, ok := <-ch
		if !ok {
			return
		}
		cmd.Process.Signal(sig)
	}()
	return func() {
		signal.Stop(ch)
		close(ch)
	}
}
