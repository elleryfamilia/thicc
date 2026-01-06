package filemanager

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"

	"github.com/sahilm/fuzzy"
)

// IndexedFile represents a file in the quick-find index
type IndexedFile struct {
	Path    string // Absolute path
	Name    string // Basename (for fuzzy matching)
	RelPath string // Relative path from root (for display)
	IsDir   bool   // Directory flag
}

// SearchResult represents a fuzzy search match
type SearchResult struct {
	File       IndexedFile
	Score      int   // Match score (higher = better)
	MatchedIdx []int // Character indices that matched (for highlighting)
}

// FileIndex maintains a flat list of all files for quick-find
type FileIndex struct {
	Root     string
	Files    []IndexedFile
	ready    int32        // Atomic: 1 = index built
	building int32        // Atomic: 1 = currently building
	mu       sync.RWMutex // Protects Files slice

	// Limits
	MaxDepth int
	MaxFiles int
}

// NewFileIndex creates a new file index
func NewFileIndex(root string) *FileIndex {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		absRoot = root
	}

	return &FileIndex{
		Root:     absRoot,
		Files:    make([]IndexedFile, 0, 1000),
		MaxDepth: 10,
		MaxFiles: 10000,
	}
}

// IsReady returns true if the index is built
func (idx *FileIndex) IsReady() bool {
	return atomic.LoadInt32(&idx.ready) == 1
}

// IsBuilding returns true if the index is currently being built
func (idx *FileIndex) IsBuilding() bool {
	return atomic.LoadInt32(&idx.building) == 1
}

// Build indexes all files in the root directory
// Safe to call from a goroutine
func (idx *FileIndex) Build() error {
	// Check if already building
	if !atomic.CompareAndSwapInt32(&idx.building, 0, 1) {
		return nil // Already building
	}
	defer atomic.StoreInt32(&idx.building, 0)

	log.Printf("FileIndex: Starting build for root: %s", idx.Root)

	// Collect files
	files := make([]IndexedFile, 0, 1000)
	count := 0

	err := idx.walkDir(idx.Root, 0, &files, &count)
	if err != nil {
		log.Printf("FileIndex: Build failed: %v", err)
		return err
	}

	// Update the index atomically
	idx.mu.Lock()
	idx.Files = files
	idx.mu.Unlock()

	// Mark as ready
	atomic.StoreInt32(&idx.ready, 1)

	log.Printf("FileIndex: Build complete, indexed %d files", len(files))
	return nil
}

// walkDir recursively walks a directory and collects files
func (idx *FileIndex) walkDir(dir string, depth int, files *[]IndexedFile, count *int) error {
	if depth > idx.MaxDepth {
		return nil
	}

	if *count >= idx.MaxFiles {
		return nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil // Skip unreadable directories
	}

	for _, entry := range entries {
		if *count >= idx.MaxFiles {
			break
		}

		name := entry.Name()

		// Skip hidden files/directories
		if strings.HasPrefix(name, ".") {
			continue
		}

		// Skip common problematic directories (reuse skipDirs from tree.go)
		if entry.IsDir() && skipDirs[name] {
			continue
		}

		fullPath := filepath.Join(dir, name)
		relPath, _ := filepath.Rel(idx.Root, fullPath)

		// Add to index
		*files = append(*files, IndexedFile{
			Path:    fullPath,
			Name:    name,
			RelPath: relPath,
			IsDir:   entry.IsDir(),
		})
		*count++

		// Recurse into directories
		if entry.IsDir() {
			idx.walkDir(fullPath, depth+1, files, count)
		}
	}

	return nil
}

// Search performs fuzzy search on filenames and returns matching results
func (idx *FileIndex) Search(query string, limit int) []SearchResult {
	if !idx.IsReady() {
		return nil
	}

	idx.mu.RLock()
	files := idx.Files
	idx.mu.RUnlock()

	if query == "" {
		// Return first N files if no query
		results := make([]SearchResult, 0, limit)
		for i := 0; i < len(files) && i < limit; i++ {
			results = append(results, SearchResult{
				File:       files[i],
				Score:      0,
				MatchedIdx: nil,
			})
		}
		return results
	}

	// Create source for fuzzy matching (just filenames)
	source := make([]string, len(files))
	for i, f := range files {
		source[i] = f.Name
	}

	// Perform fuzzy search
	matches := fuzzy.Find(query, source)

	// Convert to SearchResult
	results := make([]SearchResult, 0, limit)
	for i := 0; i < len(matches) && i < limit; i++ {
		m := matches[i]
		results = append(results, SearchResult{
			File:       files[m.Index],
			Score:      m.Score,
			MatchedIdx: m.MatchedIndexes,
		})
	}

	return results
}

// Refresh rebuilds the index
func (idx *FileIndex) Refresh() {
	atomic.StoreInt32(&idx.ready, 0)
	go idx.Build()
}

// Count returns the number of indexed files
func (idx *FileIndex) Count() int {
	idx.mu.RLock()
	defer idx.mu.RUnlock()
	return len(idx.Files)
}
