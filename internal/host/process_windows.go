//go:build windows

package host

import (
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"strconv"
	"syscall"
)

var (
	kernel32                     = syscall.NewLazyDLL("kernel32.dll")
	procGenerateConsoleCtrlEvent = kernel32.NewProc("GenerateConsoleCtrlEvent")
)

// detach places the child in its own process group so that Ctrl+C does
// not propagate directly, and so that Stop can send CTRL_BREAK_EVENT
// to its group ID (which equals the child PID).
func detach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}

// terminate attempts a graceful shutdown via CTRL_BREAK_EVENT. If that
// fails (common under Git Bash / mintty which lack a real Windows
// console), it falls back to taskkill /T to kill the entire process
// tree so that child processes are not orphaned.
func terminate(proc *os.Process) error {
	r, _, err := procGenerateConsoleCtrlEvent.Call(
		uintptr(syscall.CTRL_BREAK_EVENT),
		uintptr(proc.Pid),
	)
	if r != 0 {
		return nil
	}
	slog.Debug("CTRL_BREAK failed, killing process tree", "pid", proc.Pid, "error", err)
	out, killErr := exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(proc.Pid)).CombinedOutput()
	if killErr != nil {
		return fmt.Errorf("taskkill: %s: %w", out, killErr)
	}
	return nil
}
