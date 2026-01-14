package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/ellery/thicc/internal/util"
)

const maxLogSize = 10 * 1024 * 1024 // 10MB
const maxRotationsPerMinute = 3     // Circuit breaker threshold

// NullWriter simply sends writes into the void
type NullWriter struct{}

// Write is empty
func (NullWriter) Write(data []byte) (n int, err error) {
	return 0, nil
}

// RotatingWriter wraps a file and rotates it when it exceeds the size limit
type RotatingWriter struct {
	path       string
	file       *os.File
	size       int64
	maxSize    int64
	mu         sync.Mutex
	rotations  []time.Time // Track rotation times for circuit breaker
	disabled   bool        // Circuit breaker tripped
}

// NewRotatingWriter creates a new rotating log writer
func NewRotatingWriter(path string, maxSize int64) (*RotatingWriter, error) {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_APPEND, util.FileMode)
	if err != nil {
		return nil, err
	}

	info, _ := f.Stat()
	var currentSize int64
	if info != nil {
		currentSize = info.Size()
	}

	return &RotatingWriter{
		path:    path,
		file:    f,
		size:    currentSize,
		maxSize: maxSize,
	}, nil
}

// rotate closes the current file, rotates old logs, and opens a fresh file
func (w *RotatingWriter) rotate() error {
	// Circuit breaker: check rotation frequency
	now := time.Now()
	oneMinuteAgo := now.Add(-time.Minute)

	// Remove old rotation timestamps
	recent := w.rotations[:0]
	for _, t := range w.rotations {
		if t.After(oneMinuteAgo) {
			recent = append(recent, t)
		}
	}
	w.rotations = append(recent, now)

	// Trip circuit breaker if rotating too frequently
	if len(w.rotations) > maxRotationsPerMinute {
		w.disabled = true
		w.file.Close()
		return nil
	}

	w.file.Close()

	// Delete old backup if it exists
	backupPath := w.path + ".1"
	os.Remove(backupPath)

	// Rename current to backup - if this fails, truncate instead
	if err := os.Rename(w.path, backupPath); err != nil {
		// Rename failed - truncate the file instead to prevent unbounded growth
		f, err := os.OpenFile(w.path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, util.FileMode)
		if err != nil {
			return err
		}
		w.file = f
		w.size = 0
		return nil
	}

	// Open fresh file
	f, err := os.OpenFile(w.path, os.O_RDWR|os.O_CREATE|os.O_APPEND, util.FileMode)
	if err != nil {
		return err
	}

	w.file = f
	w.size = 0
	return nil
}

// Write implements io.Writer with log rotation
func (w *RotatingWriter) Write(data []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Circuit breaker tripped - silently discard
	if w.disabled {
		return len(data), nil
	}

	// If this write would exceed the limit, rotate the log
	if w.size+int64(len(data)) > w.maxSize {
		if err := w.rotate(); err != nil {
			return 0, err
		}
		// Check if circuit breaker tripped during rotation
		if w.disabled {
			return len(data), nil
		}
	}

	n, err = w.file.Write(data)
	w.size += int64(n)
	return n, err
}

// InitLog sets up the debug log system if enabled by compile-time variables
func InitLog() {
	if util.Debug == "ON" {
		// Write to temp directory - safer than working directory
		logPath := filepath.Join(os.TempDir(), "thicc-debug.log")
		writer, err := NewRotatingWriter(logPath, maxLogSize)
		if err != nil {
			log.Fatalf("error opening log file: %v", err)
		}
		log.SetOutput(writer)
		log.Println("THICC started with logging enabled")
		fmt.Fprintf(os.Stderr, "Debug logging to: %s\n", logPath)
	} else {
		log.SetOutput(NullWriter{})
	}
}
