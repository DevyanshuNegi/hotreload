//go:build windows

package runner

import (
	"log/slog"
	"os/exec"
	"strconv"
	"syscall"
)

// setProcGroup creates a new process group on Windows via
// CREATE_NEW_PROCESS_GROUP. This is the Windows equivalent of Unix Setpgid
// and is required for taskkill /T to reliably discover the full process tree.
func setProcGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}

// killProcessGroup uses taskkill /T /F to forcefully terminate the process
// tree. /T walks the tree by PID; /F forces termination since we cannot
// rely on the server handling WM_CLOSE or CTRL_BREAK gracefully mid-reload.
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
