// watcher.go implements a file system watcher using fsnotify for hot-reloading
// job configurations. It monitors the schedules directory for Write, Create,
// and Remove events on YAML files, debounces changes for 500ms, and triggers
// Daemon.Reload to re-register all jobs atomically. Stop uses sync.Once to
// prevent double-close panics.
package daemon

import (
	"log"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Watcher watches the schedules directory for changes and triggers hot-reload.
type Watcher struct {
	fsWatcher *fsnotify.Watcher
	daemon    *Daemon
	done      chan struct{}
	stopOnce  sync.Once
}

// NewWatcher creates a file watcher for the schedules directory.
func NewWatcher(dir string, d *Daemon) (*Watcher, error) {
	fsW, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	if err := fsW.Add(dir); err != nil {
		fsW.Close()
		return nil, err
	}

	return &Watcher{
		fsWatcher: fsW,
		daemon:    d,
		done:      make(chan struct{}),
	}, nil
}

// Start begins watching for file changes with debouncing.
func (w *Watcher) Start() {
	var debounceTimer *time.Timer

	for {
		select {
		case event, ok := <-w.fsWatcher.Events:
			if !ok {
				return
			}

			// Only react to write/create/remove events on YAML files
			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Remove) == 0 {
				continue
			}

			// Debounce: wait 500ms after last event before reloading
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			debounceTimer = time.AfterFunc(500*time.Millisecond, func() {
				w.daemon.Reload()
			})

		case err, ok := <-w.fsWatcher.Errors:
			if !ok {
				return
			}
			log.Printf("Watcher error: %v", err)

		case <-w.done:
			return
		}
	}
}

// Stop stops the file watcher. Safe to call multiple times.
func (w *Watcher) Stop() {
	w.stopOnce.Do(func() {
		close(w.done)
		w.fsWatcher.Close()
	})
}
