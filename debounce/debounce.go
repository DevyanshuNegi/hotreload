package debounce

import (
	"log/slog"
	"sync"
	"time"

	"hotreload/watcher"
)

// Debouncer collapses rapid file-change events into a single trigger.
type Debouncer struct {
	input    <-chan watcher.Event
	Trigger  chan struct{}
	interval time.Duration
	done     chan struct{}
	once     sync.Once
}

// New creates a Debouncer that reads from the watcher event channel
// and emits on Trigger after `interval` of inactivity.
// It fires an immediate synthetic trigger on startup for the first build.
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

			// Reset or start debounce timer
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

// fire sends a non-blocking trigger signal.
func (d *Debouncer) fire() {
	select {
	case d.Trigger <- struct{}{}:
	default:
		// Already a pending trigger, skip
	}
}
