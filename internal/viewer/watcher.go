package viewer

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
)

// startFileWatcher recursively watches dir with fsnotify and broadcasts a
// debounced change notification on every relevant filesystem event. It returns
// a stop function that closes the watcher and blocks until the goroutine exits.
func (srv *Server) startFileWatcher(dir string) (func(), error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("create fsnotify watcher: %w", err)
	}
	if err := addWatchTree(watcher, dir); err != nil {
		watcher.Close()
		return nil, err
	}

	relay := newChangeRelay(srv.hub)
	done := make(chan struct{})
	go func() {
		defer close(done)
		watchChanges(watcher, relay)
	}()

	return func() {
		watcher.Close()
		<-done
	}, nil
}

// changeRelay coalesces a burst of filesystem events into a single broadcast.
type changeRelay struct {
	hub     *changeHub
	timer   *time.Timer
	pending bool
}

func newChangeRelay(hub *changeHub) *changeRelay {
	timer := time.NewTimer(time.Hour)
	if !timer.Stop() {
		<-timer.C
	}
	return &changeRelay{hub: hub, timer: timer}
}

func (r *changeRelay) note() {
	r.pending = true
	if !r.timer.Stop() {
		select {
		case <-r.timer.C:
		default:
		}
	}
	r.timer.Reset(150 * time.Millisecond)
}

func (r *changeRelay) flush() {
	if r.pending {
		r.pending = false
		r.hub.broadcast()
	}
}

func watchChanges(watcher *fsnotify.Watcher, relay *changeRelay) {
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				relay.flush()
				return
			}
			handleWatchEvent(watcher, relay, event)
		case err, ok := <-watcher.Errors:
			if !ok {
				relay.flush()
				return
			}
			log.Printf("fsnotify error: %v", err)
		case <-relay.timer.C:
			relay.flush()
		}
	}
}

func handleWatchEvent(watcher *fsnotify.Watcher, relay *changeRelay, event fsnotify.Event) {
	const relevantOps = fsnotify.Write | fsnotify.Create | fsnotify.Remove | fsnotify.Rename | fsnotify.Chmod

	if event.Op&fsnotify.Create != 0 {
		if info, statErr := os.Stat(event.Name); statErr == nil && info.IsDir() {
			_ = addWatchTree(watcher, event.Name)
		}
	}
	if event.Op&relevantOps != 0 {
		relay.note()
	}
}

func addWatchTree(watcher *fsnotify.Watcher, dir string) error {
	walkErr := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return nil
		}
		if err := watcher.Add(path); err != nil {
			return fmt.Errorf("watch %s: %w", path, err)
		}
		return nil
	})
	if walkErr != nil {
		return fmt.Errorf("walk directory tree %s: %w", dir, walkErr)
	}
	return nil
}
