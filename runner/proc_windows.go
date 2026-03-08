//go:build windows

package runner

import (
	"log/slog"
	"os/exec"
	"strconv"
	"syscall"
)

// setProcGroup configures the command to create a new process group (Windows).
func setProcGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}

// killProcessGroup kills the process and all children via taskkill /T (Windows).
func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	pid := cmd.Process.Pid
	slog.Debug("killing process tree", "pid", pid)

	kill := exec.Command("taskkill", "/T", "/F", "/PID", strconv.Itoa(pid))
	if out, err := kill.CombinedOutput(); err != nil {
		slog.Warn("taskkill failed", "err", err, "output", string(out))
	}
}
