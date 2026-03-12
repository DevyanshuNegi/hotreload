package runner

import (
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// Executor manages the running server process. The done channel is used
// as a completion barrier — killProcess releases the mutex while waiting
// on it to avoid deadlocking with the Wait goroutine.
type Executor struct {
	command string
	mu      sync.Mutex
	cmd     *exec.Cmd
	done    chan struct{}
}

// NewExecutor creates an Executor for the given exec command string.
func NewExecutor(command string) *Executor {
	return &Executor{command: command}
}

// Start launches the server process. If one is already running, it is
// killed first — including all child processes — via process-group signals
// so that leaked goroutines or child workers don't linger as zombies.
func (e *Executor) Start() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.cmd != nil && e.cmd.Process != nil {
		e.killProcess()
	}

	parts := parseCommand(e.command)
	if len(parts) == 0 {
		slog.Error("empty exec command")
		return nil
	}

	slog.Info("starting server", "cmd", e.command)
	cmd := exec.Command(parts[0], parts[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// Each child gets its own process group so killProcessGroup can send
	// SIGKILL to the entire tree without hitting hotreload itself.
	setProcGroup(cmd)

	startTime := time.Now()
	if err := cmd.Start(); err != nil {
		slog.Error("failed to start server", "err", err)
		return err
	}

	e.cmd = cmd
	e.done = make(chan struct{})

	// Monitor in background. The 2s crash-guard prevents tight restart
	// loops when the binary panics on startup (e.g., missing config).
	//
	// TODO: Add exponential backoff for crash loops instead of a fixed 2s
	// threshold. Track consecutive sub-2s exits and increase the wait
	// before the next restart, capping at ~30s.
	go func() {
		err := cmd.Wait()
		elapsed := time.Since(startTime)
		defer close(e.done)

		if err != nil {
			slog.Error("server exited with error",
				"err", err,
				"elapsed", elapsed.Round(time.Millisecond),
			)
			if elapsed < 2*time.Second {
				slog.Warn("server crashed within 2s, not auto-restarting — waiting for next file change")
			}
		} else {
			slog.Info("server exited cleanly", "elapsed", elapsed.Round(time.Millisecond))
		}
	}()

	return nil
}

// Stop kills the running server process and waits for it to exit.
func (e *Executor) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.cmd != nil && e.cmd.Process != nil {
		e.killProcess()
	}
}

// killProcess kills the server and all its children, then waits for exit.
// Must be called with e.mu held. The lock is temporarily released while
// waiting on the done channel — this is necessary because the Wait
// goroutine may be trying to log under the same lock on some error paths.
func (e *Executor) killProcess() {
	slog.Info("killing server process", "pid", e.cmd.Process.Pid)
	killProcessGroup(e.cmd)

	if e.done != nil {
		done := e.done
		e.mu.Unlock()
		<-done
		e.mu.Lock()
	}
	e.cmd = nil
}

// CrashedWithin2s checks if the server can be restarted.
// Returns the done channel so the caller can detect early crashes.
func (e *Executor) Done() <-chan struct{} {
	e.mu.Lock()
	defer e.mu.Unlock()
	if e.done == nil {
		ch := make(chan struct{})
		close(ch)
		return ch
	}
	return e.done
}

// splitCommand is a convenience re-export; parseCommand lives in builder.go.
func splitCommand(cmd string) []string {
	return strings.Fields(cmd)
}
