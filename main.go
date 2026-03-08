package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"hotreload/debounce"
	"hotreload/runner"
	"hotreload/watcher"
)

func main() {
	root := flag.String("root", "", "Directory to watch for changes")
	buildCmd := flag.String("build", "", "Build command to run")
	execCmd := flag.String("exec", "", "Executable command to run after build")
	flag.Parse()

	if *root == "" || *buildCmd == "" || *execCmd == "" {
		fmt.Fprintln(os.Stderr, "Usage: hotreload --root <dir> --build <cmd> --exec <cmd>")
		fmt.Fprintln(os.Stderr, "All flags are required:")
		flag.PrintDefaults()
		os.Exit(1)
	}

	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	slog.SetDefault(logger)

	slog.Info("hotreload starting",
		"root", *root,
		"build", *buildCmd,
		"exec", *execCmd,
	)

	// Phase 2: Start file watcher
	w, err := watcher.New(*root)
	if err != nil {
		slog.Error("failed to create watcher", "err", err)
		os.Exit(1)
	}
	defer w.Close()

	// Phase 3: Debounce events (500ms window)
	deb := debounce.New(w.Events, 500*time.Millisecond)
	defer deb.Close()

	// Phase 4 & 5: Build and execution engines
	builder := runner.NewBuilder(*buildCmd)
	executor := runner.NewExecutor(*execCmd)

	// Graceful shutdown
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Main loop: wait for triggers, build, then run
	for {
		select {
		case <-ctx.Done():
			slog.Info("shutting down")
			executor.Stop()
			return

		case _, ok := <-deb.Trigger:
			if !ok {
				slog.Info("trigger channel closed, shutting down")
				executor.Stop()
				return
			}

			// Cancel any in-flight build and rebuild
			if err := builder.Build(); err != nil {
				if err == context.Canceled {
					continue // A newer build superseded this one
				}
				slog.Error("build failed, waiting for next change", "err", err)
				continue
			}

			// Build succeeded — restart the server
			if err := executor.Start(); err != nil {
				slog.Error("failed to start server", "err", err)
			}
		}
	}
}
