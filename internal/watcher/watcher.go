package watcher

import (
	"fmt"
	"log"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/pperesbr/conduit/internal/config"
	"github.com/pperesbr/conduit/internal/manager"
)

// Watcher monitors filesystem changes to the configuration file and manages its lifecycle with the associated Manager.
type Watcher struct {
	configPath string
	configDir  string
	configName string
	manager    *manager.Manager
	fsWatcher  *fsnotify.Watcher
	done       chan struct{}
}

// New creates a new Watcher instance configured to monitor the specified `configPath` and interact with the given Manager.
func New(configPath string, mgr *manager.Manager) (*Watcher, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("failed to create watcher: %w", err)
	}

	return &Watcher{
		configPath: configPath,
		configDir:  filepath.Dir(configPath),
		configName: filepath.Base(configPath),
		manager:    mgr,
		fsWatcher:  fsWatcher,
		done:       make(chan struct{}),
	}, nil
}

// Start begins monitoring the specified directory for changes and launches the file watcher in a separate goroutine.
func (w *Watcher) Start() error {
	if err := w.fsWatcher.Add(w.configDir); err != nil {
		return fmt.Errorf("failed to watch directory: %w", err)
	}

	go w.watch()

	return nil
}

// Stop gracefully stops the file watch process and releases associated resources.
func (w *Watcher) Stop() error {
	close(w.done)
	return w.fsWatcher.Close()
}

// watch monitors filesystem events, processes relevant changes, and triggers reloads or handles errors accordingly.
func (w *Watcher) watch() {
	for {
		select {
		case event, ok := <-w.fsWatcher.Events:
			if !ok {
				return
			}

			if w.isRelevantEvent(event) {
				log.Printf("watcher: config changed (%s: %s), reloading...", event.Op, event.Name)
				w.reload()
			}

		case err, ok := <-w.fsWatcher.Errors:
			if !ok {
				return
			}
			log.Printf("watcher: error: %v", err)

		case <-w.done:
			return
		}
	}
}

// isRelevantEvent determines if a filesystem event is relevant, such as a write or create operation on the config file or symlink updates.
func (w *Watcher) isRelevantEvent(event fsnotify.Event) bool {
	name := filepath.Base(event.Name)

	isWriteOrCreate := event.Op&fsnotify.Write == fsnotify.Write ||
		event.Op&fsnotify.Create == fsnotify.Create

	if !isWriteOrCreate {
		return false
	}

	if name == w.configName {
		return true
	}

	if strings.HasPrefix(name, "..") {
		return true
	}

	return false
}

// reload reloads the configuration by reading the file, parsing its contents, and reconciling with the Manager state.
func (w *Watcher) reload() {
	newConfig, err := config.Load(w.configPath)
	if err != nil {
		log.Printf("watcher: invalid config, keeping current state: %v", err)
		return
	}

	if err := w.manager.Reconcile(newConfig); err != nil {
		log.Printf("watcher: failed to reconcile: %v", err)
	}
}
