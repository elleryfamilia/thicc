package sourcecontrol

import (
	"log"
	"strings"
	"sync"
)

// Region represents a rectangular area on screen
type Region struct {
	X      int
	Y      int
	Width  int
	Height int
}

// FileStatus represents the git status of a file
type FileStatus struct {
	Path   string
	Status string // M=modified, A=added, D=deleted, R=renamed, ?=untracked, U=conflict
}

// Section represents which section of the panel is active
type Section int

const (
	SectionUnstaged    Section = iota
	SectionStaged
	SectionCommitInput // Text input for commit message
	SectionCommitBtn   // Commit button
	SectionPushBtn     // Push button
	SectionPullBtn     // Pull button
)

// Panel is the Source Control panel for git operations
type Panel struct {
	Region   Region
	Focus    bool
	RepoRoot string

	// Git state
	StagedFiles   []FileStatus
	UnstagedFiles []FileStatus
	AheadCount    int // Commits ahead of remote
	BehindCount   int // Commits behind remote

	// UI state
	Section       Section // Current section
	Selected      int     // Index within current section
	TopLine       int     // Scroll position for file list
	CommitMsg     string  // Commit message being typed
	CommitCursor  int     // Cursor position in commit message

	// Branch dialog state
	ShowBranchDialog bool
	LocalBranches    []string
	BranchSelected   int
	BranchTopLine    int

	// Mutex for thread safety
	mu sync.RWMutex

	// Click position tracking (set during render, used by mouse handler)
	unstagedHeaderY int   // Y position of unstaged section header
	stagedHeaderY   int   // Y position of staged section header
	commitSectionY  int   // Y position of commit section
	buttonsY        int   // Y position of buttons row
	unstagedFileYs  []int // Y positions of unstaged files
	stagedFileYs    []int // Y positions of staged files

	// Callbacks
	OnFileSelect func(path string, isStaged bool) // Called when user selects a file (for diff view)
	OnRefresh    func()                           // Called when UI needs refresh
}

// NewPanel creates a new Source Control panel
func NewPanel(x, y, w, h int, repoRoot string) *Panel {
	p := &Panel{
		Region:   Region{X: x, Y: y, Width: w, Height: h},
		RepoRoot: repoRoot,
		Section:  SectionUnstaged,
		Selected: 0,
		TopLine:  0,
		Focus:    false,
	}

	// Initial status load in background
	go func() {
		log.Println("THOCK SourceControl: Loading initial git status")
		p.RefreshStatus()
		if p.OnRefresh != nil {
			p.OnRefresh()
		}
	}()

	return p
}

// GetCurrentSectionFiles returns files for the current section
func (p *Panel) GetCurrentSectionFiles() []FileStatus {
	p.mu.RLock()
	defer p.mu.RUnlock()

	switch p.Section {
	case SectionUnstaged:
		return p.UnstagedFiles
	case SectionStaged:
		return p.StagedFiles
	default:
		return nil
	}
}

// GetSelectedFile returns the currently selected file, or nil if none
func (p *Panel) GetSelectedFile() *FileStatus {
	files := p.GetCurrentSectionFiles()
	if p.Selected < 0 || p.Selected >= len(files) {
		return nil
	}
	return &files[p.Selected]
}

// SetRegion updates the panel's region
func (p *Panel) SetRegion(x, y, w, h int) {
	p.Region = Region{X: x, Y: y, Width: w, Height: h}
}

// Close cleans up resources
func (p *Panel) Close() {
	// Nothing to clean up for now
}

// ensureSelectedVisible adjusts scrolling to make the selected item visible
func (p *Panel) ensureSelectedVisible() {
	// Layout: border (1) + header (1) + section header (1) + content + buttons (2) + border (1)
	// We'll calculate actual content height in render
	contentHeight := p.Region.Height - 8
	if contentHeight < 1 {
		contentHeight = 1
	}

	files := p.GetCurrentSectionFiles()
	if len(files) == 0 {
		p.Selected = 0
		p.TopLine = 0
		return
	}

	// Clamp selected to valid range
	if p.Selected >= len(files) {
		p.Selected = len(files) - 1
	}
	if p.Selected < 0 {
		p.Selected = 0
	}

	// Adjust scroll position
	if p.Selected < p.TopLine {
		p.TopLine = p.Selected
	}
	if p.Selected >= p.TopLine+contentHeight {
		p.TopLine = p.Selected - contentHeight + 1
	}
}

// MoveUp moves selection up
func (p *Panel) MoveUp() {
	switch p.Section {
	case SectionPullBtn:
		p.Section = SectionPushBtn
	case SectionPushBtn:
		p.Section = SectionCommitBtn
	case SectionCommitBtn:
		p.Section = SectionCommitInput
	case SectionCommitInput:
		p.Section = SectionStaged
		p.Selected = len(p.StagedFiles) - 1
		if p.Selected < 0 {
			p.Selected = 0
		}
	case SectionStaged:
		if p.Selected > 0 {
			p.Selected--
		} else {
			p.Section = SectionUnstaged
			p.Selected = len(p.UnstagedFiles) - 1
			if p.Selected < 0 {
				p.Selected = 0
			}
		}
	case SectionUnstaged:
		if p.Selected > 0 {
			p.Selected--
		}
	}
	p.ensureSelectedVisible()
}

// MoveDown moves selection down
func (p *Panel) MoveDown() {
	switch p.Section {
	case SectionUnstaged:
		if p.Selected < len(p.UnstagedFiles)-1 {
			p.Selected++
		} else {
			p.Section = SectionStaged
			p.Selected = 0
		}
	case SectionStaged:
		if p.Selected < len(p.StagedFiles)-1 {
			p.Selected++
		} else {
			p.Section = SectionCommitInput
		}
	case SectionCommitInput:
		p.Section = SectionCommitBtn
	case SectionCommitBtn:
		p.Section = SectionPushBtn
	case SectionPushBtn:
		p.Section = SectionPullBtn
	case SectionPullBtn:
		// Already at bottom
	}
	p.ensureSelectedVisible()
}

// NextSection moves to the next section
func (p *Panel) NextSection() {
	switch p.Section {
	case SectionUnstaged:
		p.Section = SectionStaged
	case SectionStaged:
		p.Section = SectionCommitInput
	case SectionCommitInput:
		p.Section = SectionCommitBtn
	case SectionCommitBtn:
		p.Section = SectionPushBtn
	case SectionPushBtn:
		p.Section = SectionPullBtn
	case SectionPullBtn:
		p.Section = SectionUnstaged
	}
	p.Selected = 0
	p.TopLine = 0
}

// TriggerFileSelect triggers the OnFileSelect callback for the selected file
func (p *Panel) TriggerFileSelect() {
	file := p.GetSelectedFile()
	if file != nil && p.OnFileSelect != nil {
		isStaged := p.Section == SectionStaged
		p.OnFileSelect(file.Path, isStaged)
	}
}

// ToggleStageSelected stages or unstages the selected file
func (p *Panel) ToggleStageSelected() {
	file := p.GetSelectedFile()
	if file == nil {
		return
	}

	var err error
	if p.Section == SectionUnstaged {
		err = p.StageFile(file.Path)
	} else if p.Section == SectionStaged {
		err = p.UnstageFile(file.Path)
	}

	if err != nil {
		log.Printf("THOCK SourceControl: Failed to toggle stage: %v", err)
	} else {
		p.RefreshStatus()
		if p.OnRefresh != nil {
			p.OnRefresh()
		}
	}
}

// DoCommit commits staged changes with the current message
func (p *Panel) DoCommit() {
	if p.CommitMsg == "" {
		log.Println("THOCK SourceControl: Cannot commit with empty message")
		return
	}

	if len(p.StagedFiles) == 0 {
		log.Println("THOCK SourceControl: Nothing staged to commit")
		return
	}

	err := p.Commit(p.CommitMsg)
	if err != nil {
		log.Printf("THOCK SourceControl: Commit failed: %v", err)
	} else {
		log.Println("THOCK SourceControl: Commit successful")
		p.CommitMsg = ""
		p.CommitCursor = 0
		p.RefreshStatus()
		if p.OnRefresh != nil {
			p.OnRefresh()
		}
	}
}

// DoPush pushes to remote
func (p *Panel) DoPush() {
	err := p.Push()
	if err != nil {
		log.Printf("THOCK SourceControl: Push failed: %v", err)
	} else {
		log.Println("THOCK SourceControl: Push successful")
		if p.OnRefresh != nil {
			p.OnRefresh()
		}
	}
}

// AppendToCommitMsg adds a character to the commit message
func (p *Panel) AppendToCommitMsg(r rune) {
	p.CommitMsg = p.CommitMsg[:p.CommitCursor] + string(r) + p.CommitMsg[p.CommitCursor:]
	p.CommitCursor++
}

// BackspaceCommitMsg removes character before cursor
func (p *Panel) BackspaceCommitMsg() {
	if p.CommitCursor > 0 {
		p.CommitMsg = p.CommitMsg[:p.CommitCursor-1] + p.CommitMsg[p.CommitCursor:]
		p.CommitCursor--
	}
}

// DeleteCommitMsg removes character at cursor
func (p *Panel) DeleteCommitMsg() {
	if p.CommitCursor < len(p.CommitMsg) {
		p.CommitMsg = p.CommitMsg[:p.CommitCursor] + p.CommitMsg[p.CommitCursor+1:]
	}
}

// MoveCursorLeft moves commit message cursor left
func (p *Panel) MoveCursorLeft() {
	if p.CommitCursor > 0 {
		p.CommitCursor--
	}
}

// MoveCursorRight moves commit message cursor right
func (p *Panel) MoveCursorRight() {
	if p.CommitCursor < len(p.CommitMsg) {
		p.CommitCursor++
	}
}

// PasteToCommitMsg pastes text into commit message at cursor position
func (p *Panel) PasteToCommitMsg(text string) {
	// Replace newlines with spaces for single-line commit messages
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.ReplaceAll(text, "\r", "")
	p.CommitMsg = p.CommitMsg[:p.CommitCursor] + text + p.CommitMsg[p.CommitCursor:]
	p.CommitCursor += len(text)
}

// CanCommit returns true if commit is possible
func (p *Panel) CanCommit() bool {
	return len(p.StagedFiles) > 0
}

// CanPush returns true if there are commits to push
func (p *Panel) CanPush() bool {
	return p.AheadCount > 0
}

// CanPull returns true if there are commits to pull
func (p *Panel) CanPull() bool {
	return p.BehindCount > 0
}

// DoPull pulls from remote
func (p *Panel) DoPull() {
	err := p.Pull()
	if err != nil {
		log.Printf("THOCK SourceControl: Pull failed: %v", err)
	} else {
		log.Println("THOCK SourceControl: Pull successful")
		p.RefreshStatus()
		if p.OnRefresh != nil {
			p.OnRefresh()
		}
	}
}

// ShowBranchSwitcher opens the branch switching dialog
func (p *Panel) ShowBranchSwitcher() {
	branches, err := p.GetLocalBranches()
	if err != nil {
		log.Printf("THOCK SourceControl: Failed to get branches: %v", err)
		return
	}
	p.LocalBranches = branches
	p.BranchSelected = 0
	p.BranchTopLine = 0

	// Select current branch if found
	currentBranch := p.GetBranchName()
	for i, branch := range branches {
		if branch == currentBranch {
			p.BranchSelected = i
			break
		}
	}

	p.ShowBranchDialog = true
}

// HideBranchSwitcher closes the branch dialog
func (p *Panel) HideBranchSwitcher() {
	p.ShowBranchDialog = false
}

// SwitchToSelectedBranch switches to the selected branch
func (p *Panel) SwitchToSelectedBranch() {
	if p.BranchSelected < 0 || p.BranchSelected >= len(p.LocalBranches) {
		return
	}
	branchName := p.LocalBranches[p.BranchSelected]
	err := p.CheckoutBranch(branchName)
	if err != nil {
		log.Printf("THOCK SourceControl: Failed to checkout branch: %v", err)
	} else {
		log.Printf("THOCK SourceControl: Switched to branch: %s", branchName)
		p.HideBranchSwitcher()
		p.RefreshStatus()
		if p.OnRefresh != nil {
			p.OnRefresh()
		}
	}
}

// EnsureBranchVisible adjusts scroll to make selected branch visible
func (p *Panel) EnsureBranchVisible() {
	maxVisible := 8
	if p.BranchSelected < p.BranchTopLine {
		p.BranchTopLine = p.BranchSelected
	}
	if p.BranchSelected >= p.BranchTopLine+maxVisible {
		p.BranchTopLine = p.BranchSelected - maxVisible + 1
	}
}
