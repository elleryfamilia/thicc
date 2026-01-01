package filemanager

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
)

// Git status icons (Nerd Font glyphs)
const (
	GitIconModified  = "\uf040" // Pencil - modified
	GitIconStaged    = "\uf00c" // Check - staged
	GitIconUntracked = "\uf059" // Question - untracked
	GitIconDeleted   = "\uf00d" // X mark - deleted
	GitIconRenamed   = "\uf061" // Arrow - renamed
	GitIconConflict  = "\uf071" // Warning - conflict
)

// GitStatus represents the git status of a file
type GitStatus int

const (
	GitStatusNone GitStatus = iota
	GitStatusModified
	GitStatusStaged
	GitStatusUntracked
	GitStatusDeleted
	GitStatusRenamed
	GitStatusConflict
)

// gitCache caches git status for directories
type gitCache struct {
	mu         sync.RWMutex
	repoRoots  map[string]string      // dir -> repo root
	ignoredMap map[string]bool        // path -> is ignored
	gitDirs    map[string]bool        // dir -> has .git
	statusMap  map[string]GitStatus   // path -> git status
}

var cache = &gitCache{
	repoRoots:  make(map[string]string),
	ignoredMap: make(map[string]bool),
	gitDirs:    make(map[string]bool),
	statusMap:  make(map[string]GitStatus),
}

// GetGitStatus returns the git status of a file and the corresponding icon
func (t *Tree) GetGitStatus(path string) (GitStatus, string) {
	// Check cache first
	cache.mu.RLock()
	if status, ok := cache.statusMap[path]; ok {
		cache.mu.RUnlock()
		return status, gitStatusIcon(status)
	}
	cache.mu.RUnlock()

	// Find git repo root
	repoRoot := t.findGitRepo(path)
	if repoRoot == "" {
		cache.mu.Lock()
		cache.statusMap[path] = GitStatusNone
		cache.mu.Unlock()
		return GitStatusNone, ""
	}

	// Get relative path from repo root
	relPath, err := filepath.Rel(repoRoot, path)
	if err != nil {
		return GitStatusNone, ""
	}

	// Run git status for this file
	status := t.checkGitStatus(repoRoot, relPath)

	// Cache result
	cache.mu.Lock()
	cache.statusMap[path] = status
	cache.mu.Unlock()

	return status, gitStatusIcon(status)
}

// checkGitStatus runs git status for a specific file
func (t *Tree) checkGitStatus(repoRoot, relPath string) GitStatus {
	cmd := exec.Command("git", "status", "--porcelain", "--", relPath)
	cmd.Dir = repoRoot

	output, err := cmd.Output()
	if err != nil {
		return GitStatusNone
	}

	line := strings.TrimSpace(string(output))
	if line == "" {
		return GitStatusNone
	}

	// Parse porcelain format: XY filename
	// X = index status, Y = working tree status
	if len(line) < 2 {
		return GitStatusNone
	}

	indexStatus := line[0]
	workTreeStatus := line[1]

	// Check for conflicts first
	if indexStatus == 'U' || workTreeStatus == 'U' ||
		(indexStatus == 'A' && workTreeStatus == 'A') ||
		(indexStatus == 'D' && workTreeStatus == 'D') {
		return GitStatusConflict
	}

	// Check for untracked
	if indexStatus == '?' && workTreeStatus == '?' {
		return GitStatusUntracked
	}

	// Check for staged changes (index has changes)
	if indexStatus == 'A' || indexStatus == 'M' || indexStatus == 'D' || indexStatus == 'R' {
		if indexStatus == 'R' {
			return GitStatusRenamed
		}
		if indexStatus == 'D' {
			return GitStatusDeleted
		}
		return GitStatusStaged
	}

	// Check for modified in working tree
	if workTreeStatus == 'M' {
		return GitStatusModified
	}

	if workTreeStatus == 'D' {
		return GitStatusDeleted
	}

	return GitStatusNone
}

// gitStatusIcon returns the Nerd Font icon for a git status
func gitStatusIcon(status GitStatus) string {
	switch status {
	case GitStatusModified:
		return GitIconModified
	case GitStatusStaged:
		return GitIconStaged
	case GitStatusUntracked:
		return GitIconUntracked
	case GitStatusDeleted:
		return GitIconDeleted
	case GitStatusRenamed:
		return GitIconRenamed
	case GitStatusConflict:
		return GitIconConflict
	default:
		return ""
	}
}

// RefreshGitStatus clears the git status cache (call when files change)
func (t *Tree) RefreshGitStatus() {
	cache.mu.Lock()
	cache.statusMap = make(map[string]GitStatus)
	cache.mu.Unlock()
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
