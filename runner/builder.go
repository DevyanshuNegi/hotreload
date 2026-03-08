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
// it is cancelled first. Returns nil on success, error on failure.
func (b *Builder) Build() error {
	b.mu.Lock()
	// Cancel any in-flight build
	if b.cancel != nil {
		slog.Info("cancelling previous build")
		b.cancel()
	}

	ctx, cancel := context.WithCancel(context.Background())
	b.cancel = cancel
	b.buildID++
	id := b.buildID
	b.mu.Unlock()

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
// Handles simple quoting but not shell-level features.
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
