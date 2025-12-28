package filemanager

import (
	"os"
	"os/exec"
	"path/filepath"
	"sync"
)

// gitCache caches git status for directories
type gitCache struct {
	mu         sync.RWMutex
	repoRoots  map[string]string      // dir -> repo root
	ignoredMap map[string]bool        // path -> is ignored
	gitDirs    map[string]bool        // dir -> has .git
}

var cache = &gitCache{
	repoRoots:  make(map[string]string),
	ignoredMap: make(map[string]bool),
	gitDirs:    make(map[string]bool),
}

// IsGitIgnored checks if a path is gitignored
func (t *Tree) IsGitIgnored(path string) bool {
	// Check cache first
	cache.mu.RLock()
	if ignored, ok := cache.ignoredMap[path]; ok {
		cache.mu.RUnlock()
		return ignored
	}
	cache.mu.RUnlock()

	// Find git repo root
	repoRoot := t.findGitRepo(path)
	if repoRoot == "" {
		// Not in a git repo
		cache.mu.Lock()
		cache.ignoredMap[path] = false
		cache.mu.Unlock()
		return false
	}

	// Check with git check-ignore
	ignored := t.checkGitIgnore(repoRoot, path)

	// Cache result
	cache.mu.Lock()
	cache.ignoredMap[path] = ignored
	cache.mu.Unlock()

	return ignored
}

// findGitRepo finds the git repository root for a path
func (t *Tree) findGitRepo(path string) string {
	dir := path
	if !isDir(path) {
		dir = filepath.Dir(path)
	}

	// Check cache
	cache.mu.RLock()
	if root, ok := cache.repoRoots[dir]; ok {
		cache.mu.RUnlock()
		return root
	}
	cache.mu.RUnlock()

	// Walk up directory tree looking for .git
	current := dir
	for {
		gitDir := filepath.Join(current, ".git")
		if exists(gitDir) {
			// Found git repo
			cache.mu.Lock()
			cache.repoRoots[dir] = current
			cache.gitDirs[current] = true
			cache.mu.Unlock()
			return current
		}

		// Move up one directory
		parent := filepath.Dir(current)
		if parent == current {
			// Reached root, no git repo
			cache.mu.Lock()
			cache.repoRoots[dir] = ""
			cache.mu.Unlock()
			return ""
		}
		current = parent
	}
}

// checkGitIgnore uses git check-ignore to determine if path is ignored
func (t *Tree) checkGitIgnore(repoRoot, path string) bool {
	// Run git check-ignore
	cmd := exec.Command("git", "check-ignore", "-q", path)
	cmd.Dir = repoRoot

	err := cmd.Run()
	if err != nil {
		// Exit code 1 means not ignored
		// Exit code 0 means ignored
		if exitErr, ok := err.(*exec.ExitError); ok {
			return exitErr.ExitCode() == 0
		}
		// Command failed, assume not ignored
		return false
	}

	// Exit code 0 means ignored
	return true
}

// ClearGitCache clears the git ignore cache
func (t *Tree) ClearGitCache() {
	cache.mu.Lock()
	defer cache.mu.Unlock()

	cache.ignoredMap = make(map[string]bool)
	cache.repoRoots = make(map[string]string)
}

// Helper functions

func isDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
