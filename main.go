package main

import (
"flag"
"fmt"
"log/slog"
"os"
"os/signal"
"syscall"
)

func main() {
root := flag.String("root", "", "Directory to watch for changes")
build := flag.String("build", "", "Build command to run")
exec := flag.String("exec", "", "Executable command to run after build")
flag.Parse()

if *root == "" || *build == "" || *exec == "" {
fmt.Fprintln(os.Stderr, "Usage: hotreload --root <dir> --build <cmd> --exec <cmd>")
fmt.Fprintln(os.Stderr, "All flags are required:")
flag.PrintDefaults()
os.Exit(1)
}

logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
slog.SetDefault(logger)

slog.Info("hotreload starting",
"root", *root,
"build", *build,
"exec", *exec,
)

_ = *root
_ = *build
_ = *exec

// Graceful shutdown on SIGINT / SIGTERM
sigCh := make(chan os.Signal, 1)
signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
<-sigCh
slog.Info("shutting down")
}
