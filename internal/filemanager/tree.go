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

	// File system watcher
	watcher   *FileWatcher
	onRefresh func() // Callback when tree is refreshed (for UI update)

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
	log.Printf("THICC Tree: Scan() called for root: %s", dir)
	t.mu.Lock()
	defer t.mu.Unlock()

	// Clear existing nodes
	t.Nodes = make([]*TreeNode, 0)
	t.Index = make(map[string]*TreeNode)

	log.Printf("THICC Tree: Starting scanDir from root")
	// Scan from root
	err := t.scanDir(dir, 0, -1)
	log.Printf("THICC Tree: Scan() complete, err=%v, total nodes=%d", err, len(t.Nodes))
	return err
}

// SkipDirs is the list of directories to skip during scanning and watching
// Exported so it can be reused by FileWatcher and FileIndex
// TODO: Make this configurable via settings UI (filebrowser.skipdirs)
var SkipDirs = map[string]bool{
	// ===================
	// VERSION CONTROL
	// ===================
	".git": true, ".svn": true, ".hg": true, ".bzr": true,
	".fossil": true, "_darcs": true,

	// ===================
	// JAVASCRIPT / NODE.JS
	// ===================
	// Package managers
	"node_modules": true, ".npm": true, ".yarn": true, ".pnpm": true,
	"bower_components": true, ".bun": true,
	// Frameworks
	".next": true, ".nuxt": true, ".turbo": true, ".vite": true,
	".svelte-kit": true, ".angular": true, ".remix": true,
	// Build/Bundle
	".parcel-cache": true, ".webpack": true, ".rollup.cache": true,
	".esbuild": true, "storybook-static": true,
	// Testing
	".jest": true, "jest_cache": true,
	"cypress": true, "playwright-report": true, "test-results": true,

	// ===================
	// PYTHON
	// ===================
	"__pycache__": true, ".pytest_cache": true, ".tox": true,
	".venv": true, "venv": true, "env": true, ".env": true,
	".eggs": true, ".mypy_cache": true, ".ruff_cache": true,
	".hypothesis": true, ".nox": true, ".pytype": true,
	"site-packages": true, ".python-version": true,
	".pdm": true, ".hatch": true, "htmlcov": true,

	// ===================
	// RUST
	// ===================
	"target": true, ".cargo": true,

	// ===================
	// GO
	// ===================
	// vendor intentionally NOT skipped - many projects need it

	// ===================
	// JAVA / KOTLIN / SCALA
	// ===================
	".gradle": true, ".m2": true, "out": true,
	".bsp": true, ".bloop": true, ".metals": true,
	".sbt": true,

	// ===================
	// .NET / C#
	// ===================
	"bin": true, "obj": true, "packages": true,
	".nuget": true, "TestResults": true,

	// ===================
	// RUBY
	// ===================
	".bundle": true,

	// ===================
	// PHP
	// ===================
	// Note: "vendor" intentionally NOT skipped - Go projects need it visible
	".composer": true, ".phpunit.cache": true,

	// ===================
	// ELIXIR / ERLANG
	// ===================
	"_build": true, "deps": true, ".elixir_ls": true,
	".fetch": true, "ebin": true,

	// ===================
	// HASKELL
	// ===================
	".stack-work": true, "dist-newstyle": true, ".cabal-sandbox": true,

	// ===================
	// CLOJURE
	// ===================
	".cpcache": true, ".lsp": true, ".clj-kondo": true,

	// ===================
	// OCAML / REASON
	// ===================
	"_opam": true, "_esy": true, ".merlin": true,

	// ===================
	// ZIG
	// ===================
	"zig-cache": true, "zig-out": true,

	// ===================
	// NIM
	// ===================
	"nimcache": true, ".nimble": true,

	// ===================
	// DART / FLUTTER
	// ===================
	".dart_tool": true, ".pub-cache": true, ".pub": true,
	".flutter-plugins": true, ".flutter-plugins-dependencies": true,

	// ===================
	// MOBILE - iOS / SWIFT
	// ===================
	"DerivedData": true, "Pods": true, ".swiftpm": true,
	"xcuserdata": true, "SourcePackages": true,

	// ===================
	// MOBILE - ANDROID
	// ===================
	".android": true, ".cxx": true, ".externalNativeBuild": true,
	"intermediates": true, "generated": true,

	// ===================
	// MOBILE - REACT NATIVE
	// ===================
	".expo": true, ".metro": true, "metro-bundler-cache": true,
	"haste-map-metro": true,

	// ===================
	// C / C++ / CMAKE
	// ===================
	"CMakeFiles": true, "cmake-build-debug": true, "cmake-build-release": true,
	"cmake-build-relwithdebinfo": true, "cmake-build-minsizerel": true,
	".cmake": true, "_deps": true,

	// ===================
	// IDE / EDITORS
	// ===================
	".idea": true, ".vscode": true, ".vs": true,
	".eclipse": true, ".settings": true, ".metadata": true,
	"nbproject": true, ".fleet": true, ".atom": true,
	".zed": true, ".sublime-workspace": true,

	// ===================
	// BUILD / DIST (GENERIC)
	// ===================
	"dist": true, "build": true, ".cache": true, "cache": true,
	"coverage": true, ".build": true, "Release": true, "Debug": true,
	"lib": true, "libs": true, "output": true, "outputs": true,

	// ===================
	// DOCUMENTATION
	// ===================
	"_site": true, "site": true, ".docusaurus": true,
	"_book": true, ".vuepress": true, ".vitepress": true,
	"book": true,

	// ===================
	// CLOUD / DEVOPS
	// ===================
	".terraform": true, ".serverless": true,
	".aws-sam": true, "cdk.out": true, ".amplify": true,
	".vercel": true, ".netlify": true,
	".vagrant": true, ".pulumi": true,
	".azure": true, ".gcloud": true,

	// ===================
	// CONTAINERS
	// ===================
	".docker": true, ".devcontainer": true,

	// ===================
	// DATABASE / ORM
	// ===================
	".prisma": true, "migrations": true,

	// ===================
	// LOGS / TEMP
	// ===================
	"logs": true, "log": true, "tmp": true, "temp": true,

	// ===================
	// MISC
	// ===================
	".nx": true, ".bazel-cache": true, "bazel-out": true,
	".pants.d": true, "buck-out": true,
	".dub": true, ".crystal": true,
}

// scanDir recursively scans a directory (must be called with lock held)
func (t *Tree) scanDir(dir string, indent int, parentIdx int) error {
	// THICC: Reduce max depth to 2 for safety
	if indent > 2 {
		return nil
	}

	log.Printf("THICC Tree: Scanning dir=%s indent=%d", dir, indent)

	entries, err := os.ReadDir(dir)
	if err != nil {
		log.Printf("THICC Tree: ReadDir failed for %s: %v", dir, err)
		return err
	}

	log.Printf("THICC Tree: ReadDir succeeded for %s, found %d entries", dir, len(entries))

	// Filter and sort entries
	var nodes []*TreeNode
	for i, entry := range entries {
		name := entry.Name()

		// Skip hidden files/directories unless ShowDotfiles is enabled
		// Note: .git and other problematic dirs are handled separately by SkipDirs
		if strings.HasPrefix(name, ".") && !t.ShowDotfiles {
			continue
		}

		// Skip common problematic directories
		if entry.IsDir() && SkipDirs[name] {
			log.Printf("THICC Tree: Skipping skipDir: %s", name)
			continue
		}

		// THICC: Safety limit - max 100 files per directory
		if i > 100 {
			log.Printf("THICC Tree: Hit 100 file limit in %s, stopping", dir)
			break
		}

		path := filepath.Join(dir, name)

		// THICC: Skip git ignore checking for now (can be slow)
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

	log.Printf("THICC Tree: Sorting %d nodes from %s", len(nodes), dir)
	// Sort nodes
	t.sortNodes(nodes)

	// Add nodes to flat list
	for _, node := range nodes {
		idx := len(t.Nodes)
		t.Nodes = append(t.Nodes, node)
		t.Index[node.Path] = node

		// If directory is expanded, scan children
		if node.IsDir && node.Expanded {
			log.Printf("THICC Tree: Recursing into expanded dir: %s", node.Path)
			t.scanDir(node.Path, indent+1, idx)
		}
	}

	log.Printf("THICC Tree: Completed scanning dir=%s, total nodes now=%d", dir, len(t.Nodes))
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

	log.Printf("THICC Tree: Expanding directory: %s", node.Path)

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

	log.Printf("THICC Tree: Collapsing directory: %s", node.Path)

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
	log.Println("THICC Tree: Refresh() starting")
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

	log.Println("THICC Tree: Refresh() calling scanDir")
	err := t.scanDir(t.Root, 0, -1)
	if err != nil {
		log.Printf("THICC Tree: Refresh() scanDir failed: %v", err)
		return err
	}

	log.Printf("THICC Tree: Refresh() complete, %d nodes loaded", len(t.Nodes))

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

// SetOnRefresh sets a callback to be called after the tree is refreshed
func (t *Tree) SetOnRefresh(callback func()) {
	t.onRefresh = callback
}

// EnableWatching starts file system watching for this tree
func (t *Tree) EnableWatching() error {
	if t.watcher != nil {
		return nil // Already watching
	}

	watcher, err := NewFileWatcher(t.Root, SkipDirs, func() {
		// Refresh tree and notify UI
		if err := t.Refresh(); err != nil {
			log.Printf("THICC Tree: Refresh after watch event failed: %v", err)
		}
		if t.onRefresh != nil {
			t.onRefresh()
		}
	})
	if err != nil {
		return err
	}

	t.watcher = watcher
	go t.watcher.Start()
	log.Printf("THICC Tree: Watching enabled for %s", t.Root)
	return nil
}

// DisableWatching stops file system watching (can be re-enabled later)
func (t *Tree) DisableWatching() {
	if t.watcher != nil {
		t.watcher.Stop()
		t.watcher = nil
		log.Printf("THICC Tree: Watching disabled for %s", t.Root)
	}
}

// Close stops watching and cleans up resources
func (t *Tree) Close() {
	if t.watcher != nil {
		t.watcher.Stop()
		t.watcher = nil
	}
}
