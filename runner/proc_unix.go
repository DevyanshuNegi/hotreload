//go:build !windows

package runner

import (
	"os/exec"
	"syscall"
)

// setProcGroup configures the command to run in its own process group (Unix).
func setProcGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killProcessGroup kills the entire process group (Unix).
func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		// Send SIGKILL to the negative PID to kill the whole process group
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
	}
}
