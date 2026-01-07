package filemanager

import (
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// FileWatcher watches a directory tree for changes and triggers callbacks
type FileWatcher struct {
	watcher    *fsnotify.Watcher
	root       string
	skipDirs   map[string]bool
	onChange   func()
	debounceMs int
	stop       chan struct{}
	stopped    bool
	mu         sync.Mutex
}

// NewFileWatcher creates a new file watcher for the given root directory
func NewFileWatcher(root string, skipDirs map[string]bool, onChange func()) (*FileWatcher, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	fw := &FileWatcher{
		watcher:    w,
		root:       root,
		skipDirs:   skipDirs,
		onChange:   onChange,
		debounceMs: 100,
		stop:       make(chan struct{}),
	}

	return fw, nil
}

// Start begins watching the directory tree
func (fw *FileWatcher) Start() error {
	// Add watches for all directories
	if err := fw.addDirRecursive(fw.root); err != nil {
		log.Printf("THICC Watcher: Error adding watches: %v", err)
		// Continue anyway - partial watching is better than none
	}

	log.Printf("THICC Watcher: Started watching %s", fw.root)

	// Start event loop
	go fw.eventLoop()

	return nil
}

// Stop stops watching and cleans up resources
func (fw *FileWatcher) Stop() {
	fw.mu.Lock()
	if fw.stopped {
		fw.mu.Unlock()
		return
	}
	fw.stopped = true
	fw.mu.Unlock()

	close(fw.stop)
	fw.watcher.Close()
	log.Printf("THICC Watcher: Stopped watching %s", fw.root)
}

// addDirRecursive adds watches for a directory and all its subdirectories
func (fw *FileWatcher) addDirRecursive(path string) error {
	return filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			// Log but continue - some dirs may be inaccessible
			log.Printf("THICC Watcher: Walk error for %s: %v", p, err)
			return nil
		}

		if !d.IsDir() {
			return nil
		}

		// Check if this directory should be skipped
		name := d.Name()
		if fw.skipDirs[name] {
			log.Printf("THICC Watcher: Skipping watch for %s", p)
			return filepath.SkipDir
		}

		// Skip hidden directories (except root)
		if p != fw.root && len(name) > 0 && name[0] == '.' {
			return filepath.SkipDir
		}

		// Add watch for this directory
		if err := fw.watcher.Add(p); err != nil {
			log.Printf("THICC Watcher: Failed to watch %s: %v", p, err)
			// Continue anyway
		}

		return nil
	})
}

// eventLoop handles fsnotify events with debouncing
func (fw *FileWatcher) eventLoop() {
	var timer *time.Timer
	var timerMu sync.Mutex

	resetTimer := func() {
		timerMu.Lock()
		defer timerMu.Unlock()

		if timer != nil {
			timer.Stop()
		}
		timer = time.AfterFunc(time.Duration(fw.debounceMs)*time.Millisecond, func() {
			fw.mu.Lock()
			stopped := fw.stopped
			fw.mu.Unlock()

			if !stopped && fw.onChange != nil {
				log.Println("THICC Watcher: Triggering refresh")
				fw.onChange()
			}
		})
	}

	for {
		select {
		case <-fw.stop:
			timerMu.Lock()
			if timer != nil {
				timer.Stop()
			}
			timerMu.Unlock()
			return

		case event, ok := <-fw.watcher.Events:
			if !ok {
				return
			}

			// Check if the changed path is in a skipped directory
			if fw.shouldSkipEvent(event.Name) {
				continue
			}

			log.Printf("THICC Watcher: Event %s on %s", event.Op, event.Name)

			// Handle new directories - add watches for them
			if event.Has(fsnotify.Create) {
				if info, err := os.Stat(event.Name); err == nil && info.IsDir() {
					// Check if we should watch this new directory
					name := filepath.Base(event.Name)
					if !fw.skipDirs[name] && (len(name) == 0 || name[0] != '.') {
						if err := fw.addDirRecursive(event.Name); err != nil {
							log.Printf("THICC Watcher: Failed to watch new dir %s: %v", event.Name, err)
						}
					}
				}
			}

			// Debounce the refresh callback
			resetTimer()

		case err, ok := <-fw.watcher.Errors:
			if !ok {
				return
			}
			log.Printf("THICC Watcher: Error: %v", err)
		}
	}
}

// shouldSkipEvent checks if an event should be ignored based on path
func (fw *FileWatcher) shouldSkipEvent(path string) bool {
	// Walk up the path checking each component
	for p := path; p != fw.root && p != "/" && p != "."; p = filepath.Dir(p) {
		name := filepath.Base(p)
		if fw.skipDirs[name] {
			return true
		}
	}
	return false
}
