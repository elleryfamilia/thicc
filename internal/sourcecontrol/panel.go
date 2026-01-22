package sourcecontrol

import (
	"log"
	"strings"
	"sync"
	"time"
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

// CommitEntry represents a commit in the graph
type CommitEntry struct {
	Hash       string
	ShortHash  string
	Subject    string
	Author     string
	Files      []FileStatus // Modified files in this commit
	Expanded   bool         // Whether file list is shown
	IsMerge    bool         // True if merge commit (has 2+ parents)
	FromBranch bool         // True if commit came from a merged branch
	Parents    []string     // Parent hashes for ancestry tracking
}

// Section represents which section of the panel is active
type Section int

const (
	SectionHeader      Section = iota // Branch header (clickable for branch switch)
	SectionUnstaged
	SectionStaged
	SectionCommitInput // Text input for commit message
	SectionCommitBtn   // Commit button
	SectionPushBtn     // Push button
	SectionPullBtn     // Pull button
	SectionCommitGraph // Commit history graph
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

	// Commit graph state
	CommitGraph   []CommitEntry
	GraphSelected int  // Index of selected row in graph view (can be commit or file)
	GraphTopLine  int  // Scroll offset for graph
	GraphHasMore  bool // True if more commits available to load

	// Operation progress state
	OperationInProgress string // "Committing", "Pushing", "Pulling", or ""

	// Mutex for thread safety
	mu sync.RWMutex

	// Polling for auto-refresh
	pollTicker *time.Ticker
	pollStop   chan struct{}
	polling    bool

	// Discard confirmation dialog state
	ShowDiscardConfirm bool   // Whether discard confirmation is shown
	DiscardTarget      string // Path of file to discard
	DiscardIsUntracked bool   // Whether the file is untracked (deletion warning)

	// Click position tracking (set during render, used by mouse handler)
	headerY         int   // Y position of branch header (clickable)
	unstagedHeaderY int   // Y position of unstaged section header
	stagedHeaderY   int   // Y position of staged section header
	commitSectionY  int   // Y position of commit section
	buttonsY        int   // Y position of buttons row
	unstagedFileYs  []int // Y positions of unstaged files
	stagedFileYs    []int // Y positions of staged files
	graphSectionY   int         // Y position of graph section
	graphRowYs      []int       // Y positions of graph rows (first line of each)
	graphYToRow     map[int]int // Maps Y position to logical row index (for multi-line commits)

	// Callbacks
	OnFileSelect   func(path string, isStaged bool)   // Called when user selects a file (for diff view)
	OnCommitSelect func(commitHash string, path string) // Called when user selects a file in a commit
	OnRefresh      func()                             // Called when UI needs refresh
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
		log.Println("THICC SourceControl: Loading initial git status")
		p.RefreshStatus()
		p.RefreshCommitGraph()
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
	p.StopPolling()
}

// StartPolling begins periodic git status refresh (every 5 seconds)
func (p *Panel) StartPolling() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.polling {
		return // Already polling
	}

	p.polling = true
	p.pollTicker = time.NewTicker(5 * time.Second)
	p.pollStop = make(chan struct{})

	go func() {
		for {
			select {
			case <-p.pollStop:
				return
			case <-p.pollTicker.C:
				p.RefreshStatus()
				p.RefreshCommitGraph()
				if p.OnRefresh != nil {
					p.OnRefresh()
				}
			}
		}
	}()
}

// StopPolling stops the periodic git status refresh
func (p *Panel) StopPolling() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.polling {
		return
	}

	p.polling = false
	if p.pollTicker != nil {
		p.pollTicker.Stop()
		p.pollTicker = nil
	}
	if p.pollStop != nil {
		close(p.pollStop)
		p.pollStop = nil
	}
}

// ensureSelectedVisible adjusts scrolling to make the selected item visible
func (p *Panel) ensureSelectedVisible() {
	// Layout: border (1) + header (1) + section header (1) + content + commit section (9)
	// We'll calculate actual content height in render
	contentHeight := p.Region.Height - 9
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
	case SectionCommitGraph:
		// Move from graph to pull button
		p.Section = SectionPullBtn
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
		} else {
			p.Section = SectionHeader
		}
	case SectionHeader:
		// Already at top
	}
	p.ensureSelectedVisible()
}

// MoveDown moves selection down
func (p *Panel) MoveDown() {
	switch p.Section {
	case SectionHeader:
		p.Section = SectionUnstaged
		p.Selected = 0
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
		// Move to commit graph
		p.Section = SectionCommitGraph
		p.GraphSelected = 0
		p.GraphTopLine = 0
	case SectionCommitGraph:
		// Already at bottom
	}
	p.ensureSelectedVisible()
}

// NextSection moves to the next section
func (p *Panel) NextSection() {
	switch p.Section {
	case SectionHeader:
		p.Section = SectionUnstaged
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
		p.Section = SectionCommitGraph
		p.GraphSelected = 0
		p.GraphTopLine = 0
	case SectionCommitGraph:
		p.Section = SectionHeader
	}
	p.Selected = 0
	p.TopLine = 0
}

// GraphMoveUp moves selection up within the graph section
func (p *Panel) GraphMoveUp() {
	if p.GraphSelected > 0 {
		p.GraphSelected--
	}
}

// GraphMoveDown moves selection down within the graph section
func (p *Panel) GraphMoveDown() {
	totalRows := p.getGraphTotalRows()
	if p.GraphSelected < totalRows-1 {
		p.GraphSelected++
		// Check if we need to load more commits
		if p.GraphSelected >= totalRows-5 && p.GraphHasMore {
			go func() {
				p.LoadMoreCommits()
				if p.OnRefresh != nil {
					p.OnRefresh()
				}
			}()
		}
	}
}

// getGraphTotalRows returns the total number of rows in the graph (commits + expanded files)
func (p *Panel) getGraphTotalRows() int {
	total := 0
	for _, commit := range p.CommitGraph {
		total++ // Commit row
		if commit.Expanded {
			total += len(commit.Files)
		}
	}
	return total
}

// getGraphRowInfo returns info about what's at the given row index
// Returns (isCommit, commitIdx, fileIdx) - fileIdx is -1 for commit rows
func (p *Panel) getGraphRowInfo(rowIdx int) (isCommit bool, commitIdx int, fileIdx int) {
	currentRow := 0
	for i, commit := range p.CommitGraph {
		if currentRow == rowIdx {
			return true, i, -1
		}
		currentRow++
		if commit.Expanded {
			for j := range commit.Files {
				if currentRow == rowIdx {
					return false, i, j
				}
				currentRow++
			}
		}
	}
	return false, -1, -1
}

// handleGraphEnter handles Enter key in the graph section
func (p *Panel) handleGraphEnter() {
	isCommit, commitIdx, fileIdx := p.getGraphRowInfo(p.GraphSelected)
	log.Printf("THICC SourceControl: handleGraphEnter - isCommit=%v, commitIdx=%d, fileIdx=%d, GraphSelected=%d",
		isCommit, commitIdx, fileIdx, p.GraphSelected)

	if isCommit && commitIdx >= 0 && commitIdx < len(p.CommitGraph) {
		// Toggle expanded state for commits
		p.CommitGraph[commitIdx].Expanded = !p.CommitGraph[commitIdx].Expanded
		log.Printf("THICC SourceControl: Toggled expand for commit %d, now expanded=%v",
			commitIdx, p.CommitGraph[commitIdx].Expanded)
		// Trigger refresh so Y-to-row map gets updated
		if p.OnRefresh != nil {
			p.OnRefresh()
		}
	} else if !isCommit && commitIdx >= 0 && fileIdx >= 0 {
		// Trigger diff view for this file in this commit
		commit := &p.CommitGraph[commitIdx]
		if fileIdx < len(commit.Files) {
			file := &commit.Files[fileIdx]
			log.Printf("THICC SourceControl: File selected - hash=%s, path=%s", commit.Hash, file.Path)
			if p.OnCommitSelect != nil {
				p.OnCommitSelect(commit.Hash, file.Path)
			} else {
				log.Printf("THICC SourceControl: OnCommitSelect callback is nil!")
			}
		}
	}
}

// handleGraphExpand expands the selected commit (right arrow)
func (p *Panel) handleGraphExpand() {
	isCommit, commitIdx, _ := p.getGraphRowInfo(p.GraphSelected)

	if isCommit && commitIdx >= 0 && commitIdx < len(p.CommitGraph) {
		if !p.CommitGraph[commitIdx].Expanded {
			p.CommitGraph[commitIdx].Expanded = true
			// Trigger refresh so Y-to-row map gets updated
			if p.OnRefresh != nil {
				p.OnRefresh()
			}
		}
	}
}

// handleGraphCollapse collapses the selected commit (left arrow)
func (p *Panel) handleGraphCollapse() {
	isCommit, commitIdx, _ := p.getGraphRowInfo(p.GraphSelected)

	if isCommit && commitIdx >= 0 && commitIdx < len(p.CommitGraph) {
		if p.CommitGraph[commitIdx].Expanded {
			p.CommitGraph[commitIdx].Expanded = false
			// Trigger refresh so Y-to-row map gets updated
			if p.OnRefresh != nil {
				p.OnRefresh()
			}
		}
	} else if !isCommit && commitIdx >= 0 {
		// If on a file row, collapse the parent commit and move selection to it
		p.CommitGraph[commitIdx].Expanded = false
		// Find the row index of the parent commit
		newRowIdx := 0
		for i := 0; i < commitIdx; i++ {
			newRowIdx++ // commit row
			if p.CommitGraph[i].Expanded {
				newRowIdx += len(p.CommitGraph[i].Files)
			}
		}
		p.GraphSelected = newRowIdx
		// Trigger refresh so Y-to-row map gets updated
		if p.OnRefresh != nil {
			p.OnRefresh()
		}
	}
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
		log.Printf("THICC SourceControl: Failed to toggle stage: %v", err)
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
		log.Println("THICC SourceControl: Cannot commit with empty message")
		return
	}

	if len(p.StagedFiles) == 0 {
		log.Println("THICC SourceControl: Nothing staged to commit")
		return
	}

	// Already in progress?
	if p.OperationInProgress != "" {
		return
	}

	msg := p.CommitMsg
	p.OperationInProgress = "Committing"
	go p.spinnerLoop()

	go func() {
		err := p.Commit(msg)

		p.mu.Lock()
		p.OperationInProgress = ""
		if err != nil {
			log.Printf("THICC SourceControl: Commit failed: %v", err)
		} else {
			log.Println("THICC SourceControl: Commit successful")
			p.CommitMsg = ""
			p.CommitCursor = 0
		}
		p.mu.Unlock()

		p.RefreshStatus()
		p.RefreshCommitGraph()
		if p.OnRefresh != nil {
			p.OnRefresh()
		}
	}()
}

// DoPush pushes to remote
func (p *Panel) DoPush() {
	// Already in progress?
	if p.OperationInProgress != "" {
		return
	}

	p.OperationInProgress = "Pushing"
	go p.spinnerLoop()

	go func() {
		err := p.Push()

		p.mu.Lock()
		p.OperationInProgress = ""
		if err != nil {
			log.Printf("THICC SourceControl: Push failed: %v", err)
		} else {
			log.Println("THICC SourceControl: Push successful")
		}
		p.mu.Unlock()

		p.RefreshStatus()
		p.RefreshCommitGraph()
		if p.OnRefresh != nil {
			p.OnRefresh()
		}
	}()
}

// spinnerLoop triggers UI redraws while an operation is in progress
func (p *Panel) spinnerLoop() {
	ticker := time.NewTicker(80 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		p.mu.RLock()
		done := p.OperationInProgress == ""
		p.mu.RUnlock()

		if done {
			return
		}

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

// MoveCursorUp moves cursor up by one visual line
func (p *Panel) MoveCursorUp(lineWidth int) {
	if lineWidth <= 0 {
		lineWidth = 40 // fallback
	}
	if p.CommitCursor >= lineWidth {
		p.CommitCursor -= lineWidth
	} else {
		p.CommitCursor = 0
	}
}

// MoveCursorDown moves cursor down by one visual line
func (p *Panel) MoveCursorDown(lineWidth int) {
	if lineWidth <= 0 {
		lineWidth = 40 // fallback
	}
	if p.CommitCursor+lineWidth <= len(p.CommitMsg) {
		p.CommitCursor += lineWidth
	} else {
		p.CommitCursor = len(p.CommitMsg)
	}
}

// InsertNewline inserts a newline at cursor position
func (p *Panel) InsertNewline() {
	p.CommitMsg = p.CommitMsg[:p.CommitCursor] + "\n" + p.CommitMsg[p.CommitCursor:]
	p.CommitCursor++
}

// PasteToCommitMsg pastes text into commit message at cursor position
func (p *Panel) PasteToCommitMsg(text string) {
	// Replace newlines and carriage returns with spaces
	text = strings.ReplaceAll(text, "\r\n", " ")
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.ReplaceAll(text, "\r", " ")

	// Collapse multiple spaces into single space (handles terminal line padding)
	for strings.Contains(text, "  ") {
		text = strings.ReplaceAll(text, "  ", " ")
	}

	// Trim leading/trailing whitespace
	text = strings.TrimSpace(text)

	p.CommitMsg = p.CommitMsg[:p.CommitCursor] + text + p.CommitMsg[p.CommitCursor:]
	p.CommitCursor += len(text)
}

// ClearCommitMsg clears the commit message
func (p *Panel) ClearCommitMsg() {
	p.CommitMsg = ""
	p.CommitCursor = 0
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
	// Already in progress?
	if p.OperationInProgress != "" {
		return
	}

	p.OperationInProgress = "Pulling"
	go p.spinnerLoop()

	go func() {
		err := p.Pull()

		p.mu.Lock()
		p.OperationInProgress = ""
		if err != nil {
			log.Printf("THICC SourceControl: Pull failed: %v", err)
		} else {
			log.Println("THICC SourceControl: Pull successful")
		}
		p.mu.Unlock()

		p.RefreshStatus()
		p.RefreshCommitGraph()
		if p.OnRefresh != nil {
			p.OnRefresh()
		}
	}()
}

// ShowBranchSwitcher opens the branch switching dialog
func (p *Panel) ShowBranchSwitcher() {
	branches, err := p.GetLocalBranches()
	if err != nil {
		log.Printf("THICC SourceControl: Failed to get branches: %v", err)
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
	if p.OnRefresh != nil {
		p.OnRefresh()
	}
}

// SwitchToSelectedBranch switches to the selected branch
func (p *Panel) SwitchToSelectedBranch() {
	if p.BranchSelected < 0 || p.BranchSelected >= len(p.LocalBranches) {
		return
	}
	branchName := p.LocalBranches[p.BranchSelected]
	err := p.CheckoutBranch(branchName)
	if err != nil {
		log.Printf("THICC SourceControl: Failed to checkout branch: %v", err)
	} else {
		log.Printf("THICC SourceControl: Switched to branch: %s", branchName)
		p.HideBranchSwitcher()
		p.RefreshStatus()
		p.RefreshCommitGraph()
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
