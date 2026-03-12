// Package main is the entry point for the hotreload CLI.
//
// hotreload watches a Go project directory for file changes, rebuilds it,
// and restarts the server binary — forming a watch → debounce → build → exec
// pipeline. Each stage communicates via channels, keeping the main loop
// select-driven and free of polling.
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

	// Bootstrap the pipeline: watcher → debouncer → builder → executor.
	// Each component owns a goroutine; teardown propagates via channel closure.
	w, err := watcher.New(*root)
	if err != nil {
		slog.Error("failed to create watcher", "err", err)
		os.Exit(1)
	}
	defer w.Close()

	// 500ms debounce window prevents rapid-fire rebuilds when editors
	// perform multi-file saves or atomic rename-write cycles.
	deb := debounce.New(w.Events, 500*time.Millisecond)
	defer deb.Close()

	builder := runner.NewBuilder(*buildCmd)
	executor := runner.NewExecutor(*execCmd)

	// signal.NotifyContext ties OS signal handling to context cancellation,
	// so the main select cleanly exits on SIGINT/SIGTERM.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
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

			// Build() cancels any in-flight compilation via its internal context.
			// context.Canceled means a newer trigger superseded this build — not
			// a real failure, so we loop and let the next trigger take over.
			if err := builder.Build(); err != nil {
				if err == context.Canceled {
					continue
				}
				slog.Error("build failed, waiting for next change", "err", err)
				continue
			}

			// Kill the old binary and launch the freshly compiled one.
			if err := executor.Start(); err != nil {
				slog.Error("failed to start server", "err", err)
			}
		}
	}
}
