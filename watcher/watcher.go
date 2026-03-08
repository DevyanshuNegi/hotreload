package watcher

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
)

// Event represents a validated file-change event.
type Event struct {
	Path string
	Op   fsnotify.Op
}

// Watcher recursively watches a directory tree and emits filtered events.
type Watcher struct {
	fsw    *fsnotify.Watcher
	root   string
	Events chan Event
	Errors chan error
	done   chan struct{}
}

// ignoredDirs that should never be watched.
var ignoredDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	"vendor":       true,
}

// New creates a Watcher rooted at the given directory.
func New(root string) (*Watcher, error) {
	fsw, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	w := &Watcher{
		fsw:    fsw,
		root:   root,
		Events: make(chan Event, 64),
		Errors: make(chan error, 8),
		done:   make(chan struct{}),
	}

	if err := w.addRecursive(root); err != nil {
		fsw.Close()
		return nil, err
	}

	go w.loop()
	return w, nil
}

// Close shuts down the watcher.
func (w *Watcher) Close() error {
	close(w.done)
	return w.fsw.Close()
}

// addRecursive walks the directory tree and adds every eligible dir.
func (w *Watcher) addRecursive(root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			if shouldIgnoreDir(info.Name()) && path != root {
				return filepath.SkipDir
			}
			slog.Debug("watching directory", "path", path)
			return w.fsw.Add(path)
		}
		return nil
	})
}

// loop reads raw fsnotify events, filters them, and forwards valid ones.
func (w *Watcher) loop() {
	defer close(w.Events)
	defer close(w.Errors)

	for {
		select {
		case <-w.done:
			return

		case ev, ok := <-w.fsw.Events:
			if !ok {
				return
			}
			w.handleEvent(ev)

		case err, ok := <-w.fsw.Errors:
			if !ok {
				return
			}
			slog.Error("watcher error", "err", err)
			w.Errors <- err
		}
	}
}

func (w *Watcher) handleEvent(ev fsnotify.Event) {
	// Dynamic directory handling
	if ev.Has(fsnotify.Create) {
		if info, err := os.Stat(ev.Name); err == nil && info.IsDir() {
			if !shouldIgnoreDir(info.Name()) {
				slog.Info("new directory detected, adding to watch", "path", ev.Name)
				_ = w.addRecursive(ev.Name)
			}
			return
		}
	}

	if ev.Has(fsnotify.Remove) || ev.Has(fsnotify.Rename) {
		// fsnotify auto-removes deleted dirs; just log it
		slog.Debug("path removed/renamed", "path", ev.Name)
	}

	if shouldIgnoreFile(ev.Name) {
		return
	}

	slog.Debug("file event", "op", ev.Op.String(), "path", ev.Name)
	w.Events <- Event{Path: ev.Name, Op: ev.Op}
}

// shouldIgnoreDir returns true for directories we never want to watch.
func shouldIgnoreDir(name string) bool {
	return ignoredDirs[name] || strings.HasPrefix(name, ".")
}

// shouldIgnoreFile returns true for files we should not trigger rebuilds on.
func shouldIgnoreFile(path string) bool {
	base := filepath.Base(path)

	// Temp editor files
	if strings.HasSuffix(base, "~") || strings.HasSuffix(base, ".swp") || strings.HasSuffix(base, ".swo") {
		return true
	}

	// Only care about .go files
	if filepath.Ext(base) != ".go" {
		return true
	}

	// Check if any path component is an ignored dir
	for _, part := range strings.Split(filepath.ToSlash(path), "/") {
		if ignoredDirs[part] {
			return true
		}
	}

	return false
}
