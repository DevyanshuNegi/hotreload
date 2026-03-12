// Package debounce collapses rapid filesystem events into a single trigger
// signal. Editors like vim and VS Code often write multiple files atomically
// (rename + create) or do save-all operations; without debouncing, each
// event would kick off a redundant build.
package debounce

import (
	"log/slog"
	"sync"
	"time"

	"hotreload/watcher"
)

// Debouncer collapses rapid file-change events into a single trigger.
// Trigger is buffered at 1 so the producer (timer callback) never blocks;
// a pending trigger already in the channel means a build is queued, so
// additional signals are safely dropped.
type Debouncer struct {
	input    <-chan watcher.Event
	Trigger  chan struct{}
	interval time.Duration
	done     chan struct{}
	once     sync.Once
}

// New creates a Debouncer that reads from the watcher event channel
// and emits on Trigger after `interval` of quiet. The trailing-edge
// strategy (fire after silence) is deliberate — it ensures we compile
// only after the editor finishes its batch of writes, not on the first
// event of a burst. A synthetic trigger fires immediately so the user's
// project is built on startup without requiring a file change.
func New(events <-chan watcher.Event, interval time.Duration) *Debouncer {
	d := &Debouncer{
		input:    events,
		Trigger:  make(chan struct{}, 1),
		interval: interval,
		done:     make(chan struct{}),
	}

	go d.loop()
	return d
}

// Close stops the debouncer.
func (d *Debouncer) Close() {
	d.once.Do(func() { close(d.done) })
}

func (d *Debouncer) loop() {
	defer close(d.Trigger)

	// Synthetic initial trigger for the first build
	slog.Info("firing initial build trigger")
	d.fire()

	var timer *time.Timer

	for {
		select {
		case <-d.done:
			if timer != nil {
				timer.Stop()
			}
			return

		case _, ok := <-d.input:
			if !ok {
				return
			}

			// Each new event resets the timer. time.AfterFunc runs the
			// callback in its own goroutine, so fire() is safe to call
			// without blocking this select loop.
			if timer != nil {
				timer.Stop()
			}
			timer = time.AfterFunc(d.interval, func() {
				slog.Info("debounced trigger fired")
				d.fire()
			})
		}
	}
}

// fire sends a non-blocking trigger signal. The default branch is
// intentional — if a trigger is already pending, the upcoming build
// will pick up whatever changes landed since the last trigger.
func (d *Debouncer) fire() {
	select {
	case d.Trigger <- struct{}{}:
	default:
	}
}
