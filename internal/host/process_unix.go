//go:build !windows

package host

import (
	"os"
	"os/exec"
	"syscall"
)

// detach is a no-op on Unix. No special process attributes are needed.
func detach(_ *exec.Cmd) {}

// terminate sends SIGTERM to the process for a graceful shutdown.
func terminate(proc *os.Process) error {
	return proc.Signal(syscall.SIGTERM)
}
