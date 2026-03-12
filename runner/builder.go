// Package runner implements the build-and-execute pipeline for hotreload.
// Builder handles compilation with context-based cancellation so that a
// slow build can be aborted the instant a new file change arrives.
// Executor manages the child server process with process-group isolation.
package runner

import (
	"context"
	"log/slog"
	"os"
	"os/exec"
	"strings"
	"sync"
)

// Builder runs the build command with context-based cancellation.
// buildID is a monotonic counter used to avoid a race where an old build's
// defer clears the cancel func that belongs to a newer build.
type Builder struct {
	command string
	mu      sync.Mutex
	cancel  context.CancelFunc
	buildID uint64
}

// NewBuilder creates a Builder for the given build command string.
func NewBuilder(command string) *Builder {
	return &Builder{command: command}
}

// Build runs the build command. If a previous build is still running,
// it is cancelled first via its context. The mutex is held only to swap
// the cancel func and bump the buildID — the actual compilation runs
// lock-free so concurrent triggers can cancel it without deadlocking.
func (b *Builder) Build() error {
	b.mu.Lock()
	if b.cancel != nil {
		slog.Info("cancelling previous build")
		b.cancel()
	}

	ctx, cancel := context.WithCancel(context.Background())
	b.cancel = cancel
	b.buildID++
	id := b.buildID
	b.mu.Unlock()

	// Only nil-out cancel if no newer build has started (guard via buildID).
	defer func() {
		b.mu.Lock()
		if b.buildID == id {
			b.cancel = nil
		}
		b.mu.Unlock()
	}()

	parts := parseCommand(b.command)
	if len(parts) == 0 {
		slog.Error("empty build command")
		return nil
	}

	slog.Info("starting build", "cmd", b.command)
	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	err := cmd.Run()
	// Check context first: exec.CommandContext kills the process on
	// cancellation, which surfaces as an ExitError — we want to
	// distinguish "cancelled by us" from "genuine compile failure".
	if ctx.Err() == context.Canceled {
		slog.Warn("build cancelled")
		return ctx.Err()
	}
	if err != nil {
		slog.Error("build failed", "err", err)
		return err
	}

	slog.Info("build succeeded")
	return nil
}

// Cancel aborts any currently running build.
func (b *Builder) Cancel() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.cancel != nil {
		b.cancel()
	}
}

// parseCommand splits a command string into executable + args.
// We do our own tokeniser rather than invoking a shell so the build
// process inherits hotreload's process group cleanly and we can kill
// it via context cancellation without orphan shell wrappers.
func parseCommand(cmd string) []string {
	var parts []string
	var current strings.Builder
	inQuote := false
	quoteChar := byte(0)

	for i := 0; i < len(cmd); i++ {
		c := cmd[i]
		switch {
		case inQuote:
			if c == quoteChar {
				inQuote = false
			} else {
				current.WriteByte(c)
			}
		case c == '"' || c == '\'':
			inQuote = true
			quoteChar = c
		case c == ' ' || c == '\t':
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		default:
			current.WriteByte(c)
		}
	}
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}
	return parts
}
