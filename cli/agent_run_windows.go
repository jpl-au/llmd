//go:build windows

package cli

import (
	"log/slog"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"
)

var (
	kernel32                     = syscall.NewLazyDLL("kernel32.dll")
	procGenerateConsoleCtrlEvent = kernel32.NewProc("GenerateConsoleCtrlEvent")
)

// gracePeriod is the time to wait after sending CTRL_BREAK_EVENT
// before falling back to a hard kill.
const gracePeriod = 5 * time.Second

// setup places the child in its own process group so that Ctrl+C at
// the console does not reach it directly, letting the parent control
// the shutdown sequence.
func setup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}

// forward arranges for interrupt signals to be forwarded to the child
// process. On Windows we send CTRL_BREAK_EVENT to the child's process
// group (whose ID equals the child PID because it was started with
// CREATE_NEW_PROCESS_GROUP), then allow a grace period before falling
// back to a hard kill.
func forward(cmd *exec.Cmd) func() {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch, os.Interrupt)
	go func() {
		_, ok := <-ch
		if !ok {
			return
		}
		ctrlBreak(cmd.Process.Pid)

		// Allow a grace period for the child to exit cleanly. If it
		// is still running after the deadline, kill it outright.
		time.AfterFunc(gracePeriod, func() {
			if err := cmd.Process.Kill(); err != nil {
				slog.Debug("kill after grace period", "error", err)
			}
		})
	}()
	return func() {
		signal.Stop(ch)
		close(ch)
	}
}

// ctrlBreak sends CTRL_BREAK_EVENT to the process group identified by
// pid. The target must have been started with CREATE_NEW_PROCESS_GROUP.
func ctrlBreak(pid int) {
	r, _, err := procGenerateConsoleCtrlEvent.Call(
		uintptr(syscall.CTRL_BREAK_EVENT),
		uintptr(pid),
	)
	if r == 0 {
		slog.Debug("GenerateConsoleCtrlEvent failed", "pid", pid, "error", err)
	}
}
