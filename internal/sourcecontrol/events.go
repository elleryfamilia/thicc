package sourcecontrol

import (
	"log"

	"github.com/micro-editor/tcell/v2"
)

// HandleEvent processes keyboard and mouse events
func (p *Panel) HandleEvent(event tcell.Event) bool {
	switch ev := event.(type) {
	case *tcell.EventKey:
		// Keyboard only works when focused
		if !p.Focus {
			return false
		}
		return p.handleKey(ev)
	case *tcell.EventMouse:
		// Mouse always works (clicking focuses the panel)
		return p.handleMouse(ev)
	case *tcell.EventPaste:
		// Handle paste into commit message input
		if p.Focus && p.Section == SectionCommitInput && len(p.StagedFiles) > 0 {
			p.PasteToCommitMsg(ev.Text())
			if p.OnRefresh != nil {
				p.OnRefresh()
			}
			return true
		}
	}

	return false
}

// handleKey processes keyboard events
func (p *Panel) handleKey(ev *tcell.EventKey) bool {
	// Modal: Branch dialog takes priority when visible
	if p.ShowBranchDialog {
		return p.handleBranchDialogKey(ev)
	}

	// Special handling for commit input section - captures text input
	// But only if there are staged files (commit is enabled)
	if p.Section == SectionCommitInput {
		if len(p.StagedFiles) > 0 {
			return p.handleCommitInputKey(ev)
		}
		// When disabled, only allow navigation keys
		return p.handleDisabledCommitKey(ev)
	}

	// Handle button sections
	if p.Section == SectionCommitBtn {
		return p.handleCommitBtnKey(ev)
	}
	if p.Section == SectionPushBtn {
		return p.handlePushBtnKey(ev)
	}
	if p.Section == SectionPullBtn {
		return p.handlePullBtnKey(ev)
	}

	switch ev.Key() {
	case tcell.KeyUp:
		p.MoveUp()
		return true

	case tcell.KeyDown:
		p.MoveDown()
		return true

	case tcell.KeyEnter:
		return p.handleEnter()

	case tcell.KeyTab:
		p.NextSection()
		return true

	case tcell.KeyPgUp:
		return p.pageUp()

	case tcell.KeyPgDn:
		return p.pageDown()

	default:
		// Handle character keys
		switch ev.Rune() {
		case 'k':
			p.MoveUp()
			return true
		case 'j':
			p.MoveDown()
			return true
		case 's':
			// Stage selected file
			p.stageSelected()
			return true
		case 'u':
			// Unstage selected file
			p.unstageSelected()
			return true
		case 'c':
			// Focus commit message input
			p.Section = SectionCommitInput
			return true
		case 'p':
			// Push (only if enabled)
			if p.CanPush() {
				p.DoPush()
			}
			return true
		case 'l':
			// Pull (only if enabled)
			if p.CanPull() {
				p.DoPull()
			}
			return true
		case 'b':
			// Open branch switcher
			p.ShowBranchSwitcher()
			return true
		case 'r':
			// Refresh
			p.RefreshStatus()
			if p.OnRefresh != nil {
				p.OnRefresh()
			}
			return true
		case 'a':
			// Stage all
			p.stageAll()
			return true
		case 'A':
			// Unstage all
			p.unstageAll()
			return true
		}
	}

	return false
}

// handleCommitInputKey handles keyboard events in the commit input section
func (p *Panel) handleCommitInputKey(ev *tcell.EventKey) bool {
	switch ev.Key() {
	case tcell.KeyUp:
		p.MoveUp()
		return true

	case tcell.KeyDown:
		p.MoveDown()
		return true

	case tcell.KeyTab:
		p.NextSection()
		return true

	case tcell.KeyEnter:
		// Move to commit button (don't commit directly from input)
		p.Section = SectionCommitBtn
		return true

	case tcell.KeyEsc:
		// Exit commit section
		p.Section = SectionUnstaged
		p.Selected = 0
		return true

	case tcell.KeyBackspace, tcell.KeyBackspace2:
		p.BackspaceCommitMsg()
		return true

	case tcell.KeyDelete:
		p.DeleteCommitMsg()
		return true

	case tcell.KeyLeft:
		p.MoveCursorLeft()
		return true

	case tcell.KeyRight:
		p.MoveCursorRight()
		return true

	case tcell.KeyHome:
		p.CommitCursor = 0
		return true

	case tcell.KeyEnd:
		p.CommitCursor = len(p.CommitMsg)
		return true

	default:
		// Handle text input
		r := ev.Rune()
		if r != 0 && r >= 32 { // Printable characters
			p.AppendToCommitMsg(r)
			return true
		}
	}

	return false
}

// handleCommitBtnKey handles keyboard events when commit button is focused
func (p *Panel) handleCommitBtnKey(ev *tcell.EventKey) bool {
	switch ev.Key() {
	case tcell.KeyUp:
		p.MoveUp()
		return true
	case tcell.KeyDown:
		p.MoveDown()
		return true
	case tcell.KeyTab:
		p.NextSection()
		return true
	case tcell.KeyEnter:
		// Commit
		p.DoCommit()
		return true
	case tcell.KeyEsc:
		p.Section = SectionUnstaged
		p.Selected = 0
		return true
	}
	return false
}

// handlePushBtnKey handles keyboard events when push button is focused
func (p *Panel) handlePushBtnKey(ev *tcell.EventKey) bool {
	switch ev.Key() {
	case tcell.KeyUp:
		p.MoveUp()
		return true
	case tcell.KeyDown:
		p.MoveDown()
		return true
	case tcell.KeyTab:
		p.NextSection()
		return true
	case tcell.KeyEnter:
		// Push (only if enabled)
		if p.CanPush() {
			p.DoPush()
		}
		return true
	case tcell.KeyEsc:
		p.Section = SectionUnstaged
		p.Selected = 0
		return true
	}
	return false
}

// handlePullBtnKey handles keyboard events when pull button is focused
func (p *Panel) handlePullBtnKey(ev *tcell.EventKey) bool {
	switch ev.Key() {
	case tcell.KeyUp:
		p.MoveUp()
		return true
	case tcell.KeyDown:
		p.MoveDown()
		return true
	case tcell.KeyTab:
		p.NextSection()
		return true
	case tcell.KeyEnter:
		// Pull (only if enabled)
		if p.CanPull() {
			p.DoPull()
		}
		return true
	case tcell.KeyEsc:
		p.Section = SectionUnstaged
		p.Selected = 0
		return true
	}
	return false
}

// handleDisabledCommitKey handles keyboard events when commit input is disabled
func (p *Panel) handleDisabledCommitKey(ev *tcell.EventKey) bool {
	// Only allow navigation when commit is disabled
	switch ev.Key() {
	case tcell.KeyUp:
		p.MoveUp()
		return true
	case tcell.KeyDown:
		p.MoveDown()
		return true
	case tcell.KeyTab:
		p.NextSection()
		return true
	case tcell.KeyEsc:
		p.Section = SectionUnstaged
		p.Selected = 0
		return true
	}
	// Block all other input including typing
	return true
}

// handleBranchDialogKey handles keyboard events for the branch dialog
func (p *Panel) handleBranchDialogKey(ev *tcell.EventKey) bool {
	switch ev.Key() {
	case tcell.KeyUp:
		if p.BranchSelected > 0 {
			p.BranchSelected--
			p.EnsureBranchVisible()
		}
		return true

	case tcell.KeyDown:
		if p.BranchSelected < len(p.LocalBranches)-1 {
			p.BranchSelected++
			p.EnsureBranchVisible()
		}
		return true

	case tcell.KeyEnter:
		p.SwitchToSelectedBranch()
		return true

	case tcell.KeyEsc:
		p.HideBranchSwitcher()
		return true

	default:
		// Handle j/k for navigation
		switch ev.Rune() {
		case 'k':
			if p.BranchSelected > 0 {
				p.BranchSelected--
				p.EnsureBranchVisible()
			}
			return true
		case 'j':
			if p.BranchSelected < len(p.LocalBranches)-1 {
				p.BranchSelected++
				p.EnsureBranchVisible()
			}
			return true
		}
	}

	// Consume all events when dialog is open
	return true
}

// handleEnter handles the Enter key for file sections
func (p *Panel) handleEnter() bool {
	switch p.Section {
	case SectionUnstaged:
		// Trigger diff view for selected file
		file := p.GetSelectedFile()
		if file != nil {
			if p.OnFileSelect != nil {
				p.OnFileSelect(file.Path, false)
			}
		}
		return true

	case SectionStaged:
		// Trigger diff view for selected file
		file := p.GetSelectedFile()
		if file != nil {
			if p.OnFileSelect != nil {
				p.OnFileSelect(file.Path, true)
			}
		}
		return true
	}

	return false
}

// handleMouse processes mouse events
func (p *Panel) handleMouse(ev *tcell.EventMouse) bool {
	x, y := ev.Position()

	// Check if click is within our region
	if !p.Region.Contains(x, y) {
		return false
	}

	// Handle mouse wheel scrolling
	if ev.Buttons() == tcell.WheelUp {
		p.MoveUp()
		p.MoveUp()
		p.MoveUp()
		return true
	}

	if ev.Buttons() == tcell.WheelDown {
		p.MoveDown()
		p.MoveDown()
		p.MoveDown()
		return true
	}

	// Handle left click
	if ev.Buttons() == tcell.Button1 {
		localY := y - p.Region.Y
		localX := x - p.Region.X

		// Check if clicking on buttons row
		if localY == p.buttonsY {
			// Button layout: [c]Commit (x=2-10), [p]Push (x=11-20), [l]Pull (x=21-30)
			if localX >= 2 && localX < 11 {
				p.Section = SectionCommitBtn
				log.Printf("THOCK SourceControl: Clicked commit button")
				return true
			} else if localX >= 11 && localX < 21 {
				p.Section = SectionPushBtn
				log.Printf("THOCK SourceControl: Clicked push button")
				return true
			} else if localX >= 21 {
				p.Section = SectionPullBtn
				log.Printf("THOCK SourceControl: Clicked pull button")
				return true
			}
		}

		// Check if clicking in commit section (input box area)
		if localY >= p.commitSectionY && localY < p.buttonsY {
			p.Section = SectionCommitInput
			log.Printf("THOCK SourceControl: Focused commit input")
			return true
		}

		// Check if clicking on an unstaged file
		for i, fileY := range p.unstagedFileYs {
			if localY == fileY {
				p.Section = SectionUnstaged
				p.Selected = i
				log.Printf("THOCK SourceControl: Selected unstaged file %d", i)
				// Trigger diff view for the clicked file
				if i < len(p.UnstagedFiles) && p.OnFileSelect != nil {
					p.OnFileSelect(p.UnstagedFiles[i].Path, false)
				}
				return true
			}
		}

		// Check if clicking on unstaged header
		if localY == p.unstagedHeaderY {
			p.Section = SectionUnstaged
			p.Selected = 0
			log.Printf("THOCK SourceControl: Focused unstaged section")
			return true
		}

		// Check if clicking on a staged file
		for i, fileY := range p.stagedFileYs {
			if localY == fileY {
				p.Section = SectionStaged
				p.Selected = i
				log.Printf("THOCK SourceControl: Selected staged file %d", i)
				// Trigger diff view for the clicked file
				if i < len(p.StagedFiles) && p.OnFileSelect != nil {
					p.OnFileSelect(p.StagedFiles[i].Path, true)
				}
				return true
			}
		}

		// Check if clicking on staged header
		if localY == p.stagedHeaderY {
			p.Section = SectionStaged
			p.Selected = 0
			log.Printf("THOCK SourceControl: Focused staged section")
			return true
		}

		// Click somewhere else in panel - ignore but consume event
		log.Printf("THOCK SourceControl: Click at local y=%d (no target)", localY)
		return true
	}

	return false
}

// pageUp moves up by one page
func (p *Panel) pageUp() bool {
	files := p.GetCurrentSectionFiles()
	if len(files) == 0 {
		return false
	}

	contentHeight := p.Region.Height - 10
	if contentHeight < 1 {
		contentHeight = 1
	}

	for i := 0; i < contentHeight; i++ {
		p.MoveUp()
	}
	return true
}

// pageDown moves down by one page
func (p *Panel) pageDown() bool {
	files := p.GetCurrentSectionFiles()
	if len(files) == 0 {
		return false
	}

	contentHeight := p.Region.Height - 10
	if contentHeight < 1 {
		contentHeight = 1
	}

	for i := 0; i < contentHeight; i++ {
		p.MoveDown()
	}
	return true
}

// stageSelected stages the selected file
func (p *Panel) stageSelected() {
	if p.Section != SectionUnstaged {
		return
	}

	file := p.GetSelectedFile()
	if file == nil {
		return
	}

	err := p.StageFile(file.Path)
	if err != nil {
		log.Printf("THOCK SourceControl: Failed to stage file: %v", err)
		return
	}

	p.RefreshStatus()
	if p.OnRefresh != nil {
		p.OnRefresh()
	}
}

// unstageSelected unstages the selected file
func (p *Panel) unstageSelected() {
	if p.Section != SectionStaged {
		return
	}

	file := p.GetSelectedFile()
	if file == nil {
		return
	}

	err := p.UnstageFile(file.Path)
	if err != nil {
		log.Printf("THOCK SourceControl: Failed to unstage file: %v", err)
		return
	}

	p.RefreshStatus()
	if p.OnRefresh != nil {
		p.OnRefresh()
	}
}

// stageAll stages all unstaged files
func (p *Panel) stageAll() {
	err := p.StageAll()
	if err != nil {
		log.Printf("THOCK SourceControl: Failed to stage all: %v", err)
		return
	}

	p.RefreshStatus()
	if p.OnRefresh != nil {
		p.OnRefresh()
	}
}

// unstageAll unstages all staged files
func (p *Panel) unstageAll() {
	err := p.UnstageAll()
	if err != nil {
		log.Printf("THOCK SourceControl: Failed to unstage all: %v", err)
		return
	}

	p.RefreshStatus()
	if p.OnRefresh != nil {
		p.OnRefresh()
	}
}

// Contains checks if a point is within this region
func (r Region) Contains(x, y int) bool {
	return x >= r.X && x < r.X+r.Width &&
		y >= r.Y && y < r.Y+r.Height
}
