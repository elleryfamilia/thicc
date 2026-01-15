package sourcecontrol

import (
	"log"
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

	// Run git status --porcelain
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = p.RepoRoot
	output, err := cmd.Output()
	if err != nil {
		log.Printf("THOCK SourceControl: git status failed: %v", err)
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

	log.Printf("THOCK SourceControl: Loaded %d staged, %d unstaged files, ahead: %d, behind: %d",
		len(p.StagedFiles), len(p.UnstagedFiles), ahead, behind)
}

// StageFile stages a file using git add
func (p *Panel) StageFile(path string) error {
	cmd := exec.Command("git", "add", "--", path)
	cmd.Dir = p.RepoRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("THOCK SourceControl: git add failed: %v, output: %s", err, string(output))
		return err
	}
	log.Printf("THOCK SourceControl: Staged file: %s", path)
	return nil
}

// UnstageFile unstages a file using git reset HEAD
func (p *Panel) UnstageFile(path string) error {
	cmd := exec.Command("git", "reset", "HEAD", "--", path)
	cmd.Dir = p.RepoRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("THOCK SourceControl: git reset failed: %v, output: %s", err, string(output))
		return err
	}
	log.Printf("THOCK SourceControl: Unstaged file: %s", path)
	return nil
}

// StageAll stages all files
func (p *Panel) StageAll() error {
	cmd := exec.Command("git", "add", "-A")
	cmd.Dir = p.RepoRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("THOCK SourceControl: git add -A failed: %v, output: %s", err, string(output))
		return err
	}
	log.Println("THOCK SourceControl: Staged all files")
	return nil
}

// UnstageAll unstages all files
func (p *Panel) UnstageAll() error {
	cmd := exec.Command("git", "reset", "HEAD")
	cmd.Dir = p.RepoRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("THOCK SourceControl: git reset HEAD failed: %v, output: %s", err, string(output))
		return err
	}
	log.Println("THOCK SourceControl: Unstaged all files")
	return nil
}

// Commit creates a commit with the given message
func (p *Panel) Commit(message string) error {
	cmd := exec.Command("git", "commit", "-m", message)
	cmd.Dir = p.RepoRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("THOCK SourceControl: git commit failed: %v, output: %s", err, string(output))
		return err
	}
	log.Printf("THOCK SourceControl: Committed with message: %s", message)
	return nil
}

// Push pushes to the remote
func (p *Panel) Push() error {
	cmd := exec.Command("git", "push")
	cmd.Dir = p.RepoRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("THOCK SourceControl: git push failed: %v, output: %s", err, string(output))
		return err
	}
	log.Printf("THOCK SourceControl: Push successful, output: %s", string(output))
	return nil
}

// Pull pulls from the remote
func (p *Panel) Pull() error {
	cmd := exec.Command("git", "pull")
	cmd.Dir = p.RepoRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Printf("THOCK SourceControl: git pull failed: %v, output: %s", err, string(output))
		return err
	}
	log.Printf("THOCK SourceControl: Pull successful, output: %s", string(output))
	return nil
}

// GetLocalBranches returns a list of local branch names
func (p *Panel) GetLocalBranches() ([]string, error) {
	cmd := exec.Command("git", "branch", "--format=%(refname:short)")
	cmd.Dir = p.RepoRoot
	output, err := cmd.Output()
	if err != nil {
		log.Printf("THOCK SourceControl: git branch failed: %v", err)
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
		log.Printf("THOCK SourceControl: git checkout failed: %v, output: %s", err, string(output))
		return err
	}
	log.Printf("THOCK SourceControl: Checked out branch: %s", branchName)
	return nil
}

// GetFileContent gets the content of a file at HEAD for diff comparison
func GetFileContentAtHEAD(repoRoot, path string) (string, error) {
	// Use git show to get the file content at HEAD
	cmd := exec.Command("git", "show", "HEAD:./"+path)
	cmd.Dir = repoRoot
	output, err := cmd.Output()
	if err != nil {
		log.Printf("THOCK SourceControl: git show failed for %s: %v", path, err)
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
