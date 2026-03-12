//go:build !windows

package runner

import (
	"os/exec"
	"syscall"
)

// setProcGroup places the child in its own process group via Setpgid.
// Without this, the child inherits hotreload's PGID and a kill(-pgid)
// would take down the watcher itself.
func setProcGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killProcessGroup sends SIGKILL to the entire process group by negating
// the PID. This ensures child processes (e.g., background goroutines
// spawned via os/exec inside the server) are reaped, preventing orphans
// from holding onto ports or file locks.
func killProcessGroup(cmd *exec.Cmd) {
	if cmd.Process != nil {
		_ = syscall.Kill (-cmd.Process.Pid, syscall.SIGKILL)
	}
}
