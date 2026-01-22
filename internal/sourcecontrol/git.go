package sourcecontrol

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// RefreshStatus updates the staged and unstaged file lists from git
func (p *Panel) RefreshStatus() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.StagedFiles = nil
	p.UnstagedFiles = nil

	if p.RepoRoot == "" {
		return
	}

	// Run git status --porcelain -uall (show individual files in untracked directories)
	cmd := exec.Command("git", "status", "--porcelain", "-uall")
	cmd.Dir = p.RepoRoot
	output, err := cmd.Output()
	if err != nil {
		log.Printf("THICC SourceControl: git status failed: %v", err)
		return
	}

	// Parse porcelain output
	// Format: XY PATH (or XY ORIG -> PATH for renames)
	// X = index status, Y = worktree status
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if len(line) < 3 {
			continue
		}

		indexStatus := line[0]
		workTreeStatus := line[1]
		path := strings.TrimSpace(line[3:])

		// Handle renames: "R  old -> new"
		if strings.Contains(path, " -> ") {
			parts := strings.Split(path, " -> ")
			if len(parts) == 2 {
				path = parts[1]
			}
		}

		// Make path relative to repo root for display
		if filepath.IsAbs(path) {
			if rel, err := filepath.Rel(p.RepoRoot, path); err == nil {
				path = rel
			}
		}

		// Add to appropriate list based on status
		// Staged changes (index status is not ' ' or '?')
		if indexStatus != ' ' && indexStatus != '?' {
			p.StagedFiles = append(p.StagedFiles, FileStatus{
				Path:   path,
				Status: string(indexStatus),
			})
		}

		// Unstaged changes (worktree status is not ' ')
		if workTreeStatus != ' ' {
			status := string(workTreeStatus)
			if workTreeStatus == '?' {
				status = "?" // Untracked
			}
			p.UnstagedFiles = append(p.UnstagedFiles, FileStatus{
				Path:   path,
				Status: status,
			})
		}
	}

	// Update ahead/behind counts
	ahead, behind := p.GetAheadBehind()
	p.AheadCount = ahead
	p.BehindCount = behind

	log.Printf("THICC SourceControl: Loaded %d staged, %d unstaged files, ahead: %d, behind: %d",
		len(p.StagedFiles), len(p.UnstagedFiles), ahead, behind)
}

// StageFile stages a file using git add
func (p *Panel) StageFile(path string) error {
	cmd := exec.Command("git", "add", "--", path)
	cmd.Dir = p.RepoRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("THICC SourceControl: git add failed: %v, output: %s", err, string(output))
		return err
	}
	log.Printf("THICC SourceControl: Staged file: %s", path)
	return nil
}

// UnstageFile unstages a file using git reset HEAD
func (p *Panel) UnstageFile(path string) error {
	cmd := exec.Command("git", "reset", "HEAD", "--", path)
	cmd.Dir = p.RepoRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("THICC SourceControl: git reset failed: %v, output: %s", err, string(output))
		return err
	}
	log.Printf("THICC SourceControl: Unstaged file: %s", path)
	return nil
}

// DiscardChanges discards changes to a file, reverting it to the last commit
// For untracked files, this deletes the file
func (p *Panel) DiscardChanges(path string, isUntracked bool) error {
	fullPath := filepath.Join(p.RepoRoot, path)

	if isUntracked {
		// For untracked files, delete the file
		err := os.Remove(fullPath)
		if err != nil {
			log.Printf("THICC SourceControl: failed to delete untracked file: %v", err)
			return err
		}
		log.Printf("THICC SourceControl: Deleted untracked file: %s", path)
	} else {
		// For tracked files, use git checkout to revert
		cmd := exec.Command("git", "checkout", "--", path)
		cmd.Dir = p.RepoRoot
		output, err := cmd.CombinedOutput()
		if err != nil {
			log.Printf("THICC SourceControl: git checkout failed: %v, output: %s", err, string(output))
			return err
		}
		log.Printf("THICC SourceControl: Discarded changes to: %s", path)
	}
	return nil
}

// StageAll stages all files
func (p *Panel) StageAll() error {
	cmd := exec.Command("git", "add", "-A")
	cmd.Dir = p.RepoRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("THICC SourceControl: git add -A failed: %v, output: %s", err, string(output))
		return err
	}
	log.Println("THICC SourceControl: Staged all files")
	return nil
}

// UnstageAll unstages all files
func (p *Panel) UnstageAll() error {
	cmd := exec.Command("git", "reset", "HEAD")
	cmd.Dir = p.RepoRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("THICC SourceControl: git reset HEAD failed: %v, output: %s", err, string(output))
		return err
	}
	log.Println("THICC SourceControl: Unstaged all files")
	return nil
}

// Commit creates a commit with the given message
func (p *Panel) Commit(message string) error {
	cmd := exec.Command("git", "commit", "-m", message)
	cmd.Dir = p.RepoRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("THICC SourceControl: git commit failed: %v, output: %s", err, string(output))
		return err
	}
	log.Printf("THICC SourceControl: Committed with message: %s", message)
	return nil
}

// Push pushes to the remote
func (p *Panel) Push() error {
	cmd := exec.Command("git", "push")
	cmd.Dir = p.RepoRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("THICC SourceControl: git push failed: %v, output: %s", err, string(output))
		return err
	}
	log.Printf("THICC SourceControl: Push successful, output: %s", string(output))
	return nil
}

// Pull pulls from the remote
func (p *Panel) Pull() error {
	cmd := exec.Command("git", "pull")
	cmd.Dir = p.RepoRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("THICC SourceControl: git pull failed: %v, output: %s", err, string(output))
		return err
	}
	log.Printf("THICC SourceControl: Pull successful, output: %s", string(output))
	return nil
}

// GetLocalBranches returns a list of local branch names
func (p *Panel) GetLocalBranches() ([]string, error) {
	cmd := exec.Command("git", "branch", "--format=%(refname:short)")
	cmd.Dir = p.RepoRoot
	output, err := cmd.Output()
	if err != nil {
		log.Printf("THICC SourceControl: git branch failed: %v", err)
		return nil, err
	}

	var branches []string
	for _, line := range strings.Split(strings.TrimSpace(string(output)), "\n") {
		if line != "" {
			branches = append(branches, line)
		}
	}
	return branches, nil
}

// CheckoutBranch switches to the specified branch
func (p *Panel) CheckoutBranch(branchName string) error {
	cmd := exec.Command("git", "checkout", branchName)
	cmd.Dir = p.RepoRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("THICC SourceControl: git checkout failed: %v, output: %s", err, string(output))
		return err
	}
	log.Printf("THICC SourceControl: Checked out branch: %s", branchName)
	return nil
}

// GetFileContent gets the content of a file at HEAD for diff comparison
func GetFileContentAtHEAD(repoRoot, path string) (string, error) {
	// Use git show to get the file content at HEAD
	cmd := exec.Command("git", "show", "HEAD:./"+path)
	cmd.Dir = repoRoot
	output, err := cmd.Output()
	if err != nil {
		log.Printf("THICC SourceControl: git show failed for %s: %v", path, err)
		return "", err
	}
	return string(output), nil
}

// GetBranchName returns the current branch name
func (p *Panel) GetBranchName() string {
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = p.RepoRoot
	output, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(output))
}

// HasRemote checks if there's a configured remote
func (p *Panel) HasRemote() bool {
	cmd := exec.Command("git", "remote")
	cmd.Dir = p.RepoRoot
	output, err := cmd.Output()
	if err != nil {
		return false
	}
	return len(strings.TrimSpace(string(output))) > 0
}

// GetAheadBehind returns how many commits ahead/behind the remote we are
func (p *Panel) GetAheadBehind() (ahead, behind int) {
	cmd := exec.Command("git", "rev-list", "--left-right", "--count", "HEAD...@{upstream}")
	cmd.Dir = p.RepoRoot
	output, err := cmd.Output()
	if err != nil {
		return 0, 0
	}

	parts := strings.Fields(strings.TrimSpace(string(output)))
	if len(parts) >= 2 {
		// Parse ahead and behind counts
		if n, err := parseInt(parts[0]); err == nil {
			ahead = n
		}
		if n, err := parseInt(parts[1]); err == nil {
			behind = n
		}
	}
	return ahead, behind
}

// parseInt is a helper to parse an int from a string
func parseInt(s string) (int, error) {
	var n int
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		} else {
			return 0, nil
		}
	}
	return n, nil
}

// RefreshCommitGraph loads the commit history for the graph display
func (p *Panel) RefreshCommitGraph() {
	if p.RepoRoot == "" {
		return
	}

	// Get commit log with format: hash|short_hash|subject|author|parents
	// --name-status gives us the files changed per commit
	cmd := exec.Command("git", "log", "--format=%H|%h|%s|%an|%P", "--name-status", "-n", "50")
	cmd.Dir = p.RepoRoot
	output, err := cmd.Output()
	if err != nil {
		log.Printf("THICC SourceControl: git log failed: %v", err)
		return
	}

	// Parse the output
	commits := parseCommitLog(string(output))

	p.mu.Lock()
	// Preserve expanded state from existing commits
	expandedMap := make(map[string]bool)
	for _, c := range p.CommitGraph {
		if c.Expanded {
			expandedMap[c.Hash] = true
		}
	}

	p.CommitGraph = commits
	p.GraphHasMore = len(commits) >= 50

	// Restore expanded state
	for i := range p.CommitGraph {
		if expandedMap[p.CommitGraph[i].Hash] {
			p.CommitGraph[i].Expanded = true
		}
	}

	// Compute ancestry to mark commits from merged branches
	p.computeAncestry()
	p.mu.Unlock()

	log.Printf("THICC SourceControl: Loaded %d commits for graph", len(commits))
}

// parseCommitLog parses git log output into CommitEntry slice
func parseCommitLog(output string) []CommitEntry {
	var commits []CommitEntry
	lines := strings.Split(output, "\n")

	var currentCommit *CommitEntry
	for _, line := range lines {
		if line == "" {
			continue
		}

		// Check if this is a commit header line (contains |)
		if strings.Contains(line, "|") && !strings.HasPrefix(line, "M\t") && !strings.HasPrefix(line, "A\t") && !strings.HasPrefix(line, "D\t") && !strings.HasPrefix(line, "R") {
			// Save previous commit
			if currentCommit != nil {
				commits = append(commits, *currentCommit)
			}

			// Parse new commit header: hash|short_hash|subject|author|parents
			parts := strings.SplitN(line, "|", 5)
			if len(parts) < 4 {
				continue
			}

			currentCommit = &CommitEntry{
				Hash:      parts[0],
				ShortHash: parts[1],
				Subject:   parts[2],
				Author:    parts[3],
			}

			// Parse parents (space-separated)
			if len(parts) >= 5 && parts[4] != "" {
				currentCommit.Parents = strings.Fields(parts[4])
				currentCommit.IsMerge = len(currentCommit.Parents) >= 2
			}
		} else if currentCommit != nil {
			// This is a file status line: M\tpath, A\tpath, D\tpath, R###\told\tnew
			if len(line) < 2 {
				continue
			}
			status := string(line[0])
			var path string

			if strings.HasPrefix(line, "R") {
				// Rename: R###\told\tnew
				parts := strings.Split(line, "\t")
				if len(parts) >= 3 {
					path = strings.TrimSpace(parts[2]) // Use new name
					status = "R"
				}
			} else if line[1] == '\t' {
				path = strings.TrimSpace(line[2:])
			}

			if path != "" {
				currentCommit.Files = append(currentCommit.Files, FileStatus{
					Path:   path,
					Status: status,
				})
			}
		}
	}

	// Don't forget the last commit
	if currentCommit != nil {
		commits = append(commits, *currentCommit)
	}

	return commits
}

// computeAncestry marks commits that came from merged branches
func (p *Panel) computeAncestry() {
	// Build hash → index map
	hashToIdx := make(map[string]int)
	for i, c := range p.CommitGraph {
		hashToIdx[c.Hash] = i
	}

	// For each merge commit, mark second parent's lineage as FromBranch
	for i, c := range p.CommitGraph {
		if len(c.Parents) >= 2 {
			p.CommitGraph[i].IsMerge = true
			// Trace back from second parent (the merged branch)
			secondParent := c.Parents[1]
			p.markBranchAncestry(secondParent, hashToIdx)
		}
	}
}

// markBranchAncestry marks all commits reachable from the given hash as FromBranch
func (p *Panel) markBranchAncestry(hash string, hashToIdx map[string]int) {
	visited := make(map[string]bool)
	queue := []string{hash}

	for len(queue) > 0 {
		h := queue[0]
		queue = queue[1:]

		if visited[h] {
			continue
		}
		visited[h] = true

		if idx, ok := hashToIdx[h]; ok {
			// Stop if we hit another merge (that's a different branch point)
			if p.CommitGraph[idx].IsMerge {
				continue
			}
			p.CommitGraph[idx].FromBranch = true
			// Continue to parents
			for _, parent := range p.CommitGraph[idx].Parents {
				queue = append(queue, parent)
			}
		}
	}
}

// LoadMoreCommits loads additional commits beyond what's already loaded
func (p *Panel) LoadMoreCommits() {
	if p.RepoRoot == "" || !p.GraphHasMore {
		return
	}

	p.mu.RLock()
	currentCount := len(p.CommitGraph)
	p.mu.RUnlock()

	// Get more commits, skipping what we already have
	cmd := exec.Command("git", "log", "--format=%H|%h|%s|%an|%P", "--name-status",
		"-n", "50", "--skip", fmt.Sprintf("%d", currentCount))
	cmd.Dir = p.RepoRoot
	output, err := cmd.Output()
	if err != nil {
		log.Printf("THICC SourceControl: git log (more) failed: %v", err)
		return
	}

	newCommits := parseCommitLog(string(output))

	p.mu.Lock()
	p.CommitGraph = append(p.CommitGraph, newCommits...)
	p.GraphHasMore = len(newCommits) >= 50
	p.computeAncestry() // Recompute ancestry with new commits
	p.mu.Unlock()

	log.Printf("THICC SourceControl: Loaded %d more commits, total: %d", len(newCommits), len(p.CommitGraph)+len(newCommits))
}
