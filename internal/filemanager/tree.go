package filemanager

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// TreeNode represents a file or directory in the tree
type TreeNode struct {
	Path     string      // Absolute path
	Name     string      // Basename
	IsDir    bool        // Directory flag
	IsHidden bool        // Dotfile flag
	Indent   int         // Nesting level
	Owner    int         // Parent index in flat list
	Children []*TreeNode // Child nodes (for dirs)
	Expanded bool        // Expansion state
	Info     os.FileInfo // File metadata
}

// Tree manages the file tree state
type Tree struct {
	Root         string               // Root directory
	Nodes        []*TreeNode          // Flat list for rendering
	Index        map[string]*TreeNode // Path → Node lookup
	GitIgnored   map[string]bool      // Ignored files cache
	CurrentDir   string               // Active directory
	SelectedNode *TreeNode            // Currently selected node
	SelectedIdx  int                  // Selected index in Nodes
	Width        int                  // Tree pane width

	// Expansion state - stored by path so it persists across rescans
	ExpandedPaths map[string]bool

	// Configuration
	ShowDotfiles    bool
	ShowIgnored     bool
	FoldersFirst    bool
	ShowParent      bool
	AutoRefresh     bool
	RefreshInterval time.Duration

	// Synchronization
	mu sync.RWMutex
}

// NewTree creates a new file tree
func NewTree(root string) *Tree {
	absRoot, err := filepath.Abs(root)
	if err != nil {
		absRoot = root
	}

	return &Tree{
		Root:            absRoot,
		Nodes:           make([]*TreeNode, 0),
		Index:           make(map[string]*TreeNode),
		GitIgnored:      make(map[string]bool),
		ExpandedPaths:   make(map[string]bool),
		CurrentDir:      absRoot,
		Width:           30,
		ShowDotfiles:    true,
		ShowIgnored:     true,
		FoldersFirst:    true,
		ShowParent:      false,
		AutoRefresh:     true,
		RefreshInterval: 2 * time.Second,
	}
}

// Scan recursively scans a directory and builds the tree
func (t *Tree) Scan(dir string) error {
	log.Printf("THOCK Tree: Scan() called for root: %s", dir)
	t.mu.Lock()
	defer t.mu.Unlock()

	// Clear existing nodes
	t.Nodes = make([]*TreeNode, 0)
	t.Index = make(map[string]*TreeNode)

	log.Printf("THOCK Tree: Starting scanDir from root")
	// Scan from root
	err := t.scanDir(dir, 0, -1)
	log.Printf("THOCK Tree: Scan() complete, err=%v, total nodes=%d", err, len(t.Nodes))
	return err
}

// Skip list for directories that commonly cause issues
var skipDirs = map[string]bool{
	".git":         true,
	"node_modules": true,
	".build":       true,
	"vendor":       true,
	".cache":       true,
	"dist":         true,
	"build":        true,
	"target":       true,
	".venv":        true,
	"venv":         true,
	"__pycache__":  true,
}

// scanDir recursively scans a directory (must be called with lock held)
func (t *Tree) scanDir(dir string, indent int, parentIdx int) error {
	// THOCK: Reduce max depth to 2 for safety
	if indent > 2 {
		return nil
	}

	log.Printf("THOCK Tree: Scanning dir=%s indent=%d", dir, indent)

	entries, err := os.ReadDir(dir)
	if err != nil {
		log.Printf("THOCK Tree: ReadDir failed for %s: %v", dir, err)
		return err
	}

	log.Printf("THOCK Tree: ReadDir succeeded for %s, found %d entries", dir, len(entries))

	// Filter and sort entries
	var nodes []*TreeNode
	for i, entry := range entries {
		name := entry.Name()

		// THOCK: Skip ALL hidden files/directories for safety (even if ShowDotfiles=true)
		if strings.HasPrefix(name, ".") {
			log.Printf("THOCK Tree: Skipping hidden: %s", name)
			continue
		}

		// Skip common problematic directories
		if entry.IsDir() && skipDirs[name] {
			log.Printf("THOCK Tree: Skipping skipDir: %s", name)
			continue
		}

		// THOCK: Safety limit - max 100 files per directory
		if i > 100 {
			log.Printf("THOCK Tree: Hit 100 file limit in %s, stopping", dir)
			break
		}

		path := filepath.Join(dir, name)

		// THOCK: Skip git ignore checking for now (can be slow)
		isHidden := false

		// Get file info
		info, err := entry.Info()
		if err != nil {
			continue // Skip files we can't stat
		}

		node := &TreeNode{
			Path:     path,
			Name:     name,
			IsDir:    entry.IsDir(),
			IsHidden: isHidden,
			Indent:   indent,
			Owner:    parentIdx,
			Children: nil,
			Expanded: t.ExpandedPaths[path], // Use persistent expansion state
			Info:     info,
		}

		nodes = append(nodes, node)
	}

	log.Printf("THOCK Tree: Sorting %d nodes from %s", len(nodes), dir)
	// Sort nodes
	t.sortNodes(nodes)

	// Add nodes to flat list
	for _, node := range nodes {
		idx := len(t.Nodes)
		t.Nodes = append(t.Nodes, node)
		t.Index[node.Path] = node

		// If directory is expanded, scan children
		if node.IsDir && node.Expanded {
			log.Printf("THOCK Tree: Recursing into expanded dir: %s", node.Path)
			t.scanDir(node.Path, indent+1, idx)
		}
	}

	log.Printf("THOCK Tree: Completed scanning dir=%s, total nodes now=%d", dir, len(t.Nodes))
	return nil
}

// Expand expands a directory node
func (t *Tree) Expand(node *TreeNode) error {
	if !node.IsDir {
		return nil
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	// Check if already expanded
	if t.ExpandedPaths[node.Path] {
		return nil
	}

	log.Printf("THOCK Tree: Expanding directory: %s", node.Path)

	// Mark path as expanded in persistent state
	t.ExpandedPaths[node.Path] = true

	// Rebuild tree (scanDir will use ExpandedPaths)
	t.Nodes = make([]*TreeNode, 0)
	t.Index = make(map[string]*TreeNode)
	return t.scanDir(t.Root, 0, -1)
}

// Collapse collapses a directory node
func (t *Tree) Collapse(node *TreeNode) {
	if !node.IsDir {
		return
	}

	t.mu.Lock()
	defer t.mu.Unlock()

	// Check if already collapsed
	if !t.ExpandedPaths[node.Path] {
		return
	}

	log.Printf("THOCK Tree: Collapsing directory: %s", node.Path)

	// Remove from expanded paths
	delete(t.ExpandedPaths, node.Path)

	// Rebuild tree (scanDir will use ExpandedPaths)
	t.Nodes = make([]*TreeNode, 0)
	t.Index = make(map[string]*TreeNode)
	t.scanDir(t.Root, 0, -1)
}

// Toggle toggles a directory's expansion state
func (t *Tree) Toggle(node *TreeNode) error {
	if !node.IsDir {
		return nil
	}

	// Use ExpandedPaths to check state (not node.Expanded which may be stale)
	if t.ExpandedPaths[node.Path] {
		t.Collapse(node)
		return nil
	}

	return t.Expand(node)
}

// Refresh rescans the tree preserving state
func (t *Tree) Refresh() error {
	log.Println("THOCK Tree: Refresh() starting")
	t.mu.Lock()
	defer t.mu.Unlock()

	// Save selected path
	selectedPath := ""
	if t.SelectedNode != nil {
		selectedPath = t.SelectedNode.Path
	}

	// Clear and re-scan (ExpandedPaths is persistent, scanDir will use it)
	t.Nodes = make([]*TreeNode, 0)
	t.Index = make(map[string]*TreeNode)

	log.Println("THOCK Tree: Refresh() calling scanDir")
	err := t.scanDir(t.Root, 0, -1)
	if err != nil {
		log.Printf("THOCK Tree: Refresh() scanDir failed: %v", err)
		return err
	}

	log.Printf("THOCK Tree: Refresh() complete, %d nodes loaded", len(t.Nodes))

	// Restore selection
	if selectedPath != "" {
		// Need to call selectPathLocked since we hold the lock
		t.selectPathLocked(selectedPath)
	}

	return nil
}

// SelectPath selects a node by path
func (t *Tree) SelectPath(path string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.selectPathLocked(path)
}

// selectPathLocked selects a node by path (must be called with lock held)
func (t *Tree) selectPathLocked(path string) bool {
	node, ok := t.Index[path]
	if !ok {
		return false
	}

	// Find node index
	for i, n := range t.Nodes {
		if n.Path == path {
			t.SelectedNode = node
			t.SelectedIdx = i
			return true
		}
	}

	return false
}

// SelectIndex selects a node by index
func (t *Tree) SelectIndex(idx int) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	if idx < 0 || idx >= len(t.Nodes) {
		return false
	}

	t.SelectedNode = t.Nodes[idx]
	t.SelectedIdx = idx
	return true
}

// MoveUp moves selection up one node
func (t *Tree) MoveUp() bool {
	if t.SelectedIdx <= 0 {
		return false
	}

	return t.SelectIndex(t.SelectedIdx - 1)
}

// MoveDown moves selection down one node
func (t *Tree) MoveDown() bool {
	if t.SelectedIdx >= len(t.Nodes)-1 {
		return false
	}

	return t.SelectIndex(t.SelectedIdx + 1)
}

// GetNodes returns a copy of the current nodes (thread-safe)
func (t *Tree) GetNodes() []*TreeNode {
	t.mu.RLock()
	defer t.mu.RUnlock()

	nodes := make([]*TreeNode, len(t.Nodes))
	copy(nodes, t.Nodes)
	return nodes
}

// GetSelected returns the currently selected node (thread-safe)
func (t *Tree) GetSelected() *TreeNode {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.SelectedNode
}

// sortNodes sorts nodes based on configuration
func (t *Tree) sortNodes(nodes []*TreeNode) {
	if t.FoldersFirst {
		// Stable sort: folders first, then alphabetically
		for i := 0; i < len(nodes); i++ {
			for j := i + 1; j < len(nodes); j++ {
				// Folders before files
				if nodes[i].IsDir != nodes[j].IsDir {
					if nodes[j].IsDir {
						nodes[i], nodes[j] = nodes[j], nodes[i]
					}
					continue
				}

				// Alphabetical within same type
				if strings.ToLower(nodes[i].Name) > strings.ToLower(nodes[j].Name) {
					nodes[i], nodes[j] = nodes[j], nodes[i]
				}
			}
		}
	} else {
		// Just alphabetical
		for i := 0; i < len(nodes); i++ {
			for j := i + 1; j < len(nodes); j++ {
				if strings.ToLower(nodes[i].Name) > strings.ToLower(nodes[j].Name) {
					nodes[i], nodes[j] = nodes[j], nodes[i]
				}
			}
		}
	}
}
