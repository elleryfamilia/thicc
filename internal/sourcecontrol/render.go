package sourcecontrol

import (
	"fmt"
	"strings"

	"github.com/ellery/thicc/internal/config"
	"github.com/micro-editor/tcell/v2"
)

// Nerd Font icons for git status
const (
	IconModified  = "\uf040" // Pencil - modified
	IconStaged    = "\uf00c" // Check - staged
	IconUntracked = "\uf059" // Question - untracked
	IconDeleted   = "\uf00d" // X mark - deleted
	IconRenamed   = "\uf061" // Arrow - renamed
	IconConflict  = "\uf071" // Warning - conflict
	IconBranch    = "\ue725" // Git branch
	IconCommit    = "\uf417" // Git commit
	IconPush      = "\uf062" // Arrow up - push
	IconCheck     = "\uf00c" // Checkmark for buttons
)

// Colors - matching diff colors from colorscheme
var (
	colorAdded      = tcell.Color40  // Green (#00AF00)
	colorDeleted    = tcell.Color160 // Red (#D70000)
	colorModified   = tcell.Color214 // Yellow/Orange (#FFAF00)
	colorUntracked  = tcell.Color243 // Gray
	colorConflict   = tcell.Color226 // Bright yellow
	colorBorder     = tcell.Color205 // Hot pink (spiderverse)
	colorHeader     = tcell.Color33  // Blue for headers
	colorButton     = tcell.Color45  // Cyan for buttons
	colorButtonText = tcell.ColorBlack
)

// Render draws the source control panel to the screen
func (p *Panel) Render(screen tcell.Screen) {
	// Don't render if region is too small
	if p.Region.Width < 10 || p.Region.Height < 10 {
		return
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	// Clear the region
	p.clearRegion(screen)

	// Draw content
	y := p.drawHeader(screen)
	y = p.drawUnstagedSection(screen, y)
	y = p.drawStagedSection(screen, y)
	y = p.drawCommitSection(screen, y)
	_ = y // silence unused warning

	// Draw border
	p.drawBorder(screen)

	// Draw branch dialog overlay if visible
	if p.ShowBranchDialog {
		p.drawBranchDialog(screen)
	}
}

// clearRegion clears the panel's screen region
func (p *Panel) clearRegion(screen tcell.Screen) {
	style := config.DefStyle
	for y := 0; y < p.Region.Height; y++ {
		for x := 0; x < p.Region.Width; x++ {
			screen.SetContent(p.Region.X+x, p.Region.Y+y, ' ', nil, style)
		}
	}
}

// drawHeader draws the panel header with branch info
func (p *Panel) drawHeader(screen tcell.Screen) int {
	y := 1 // After top border

	// Header: just branch icon + branch name (compact)
	branchName := p.GetBranchName()
	if branchName == "" {
		branchName = "no branch"
	}

	header := fmt.Sprintf(" %s %s", IconBranch, branchName)
	style := config.DefStyle.Foreground(colorHeader).Bold(true)
	p.drawText(screen, 1, y, header, style)

	return y + 1
}

// drawUnstagedSection draws the unstaged changes section
func (p *Panel) drawUnstagedSection(screen tcell.Screen, startY int) int {
	y := startY

	// Track section header Y for click detection
	p.unstagedHeaderY = y

	// Section header with shortcuts
	headerStyle := config.DefStyle.Foreground(colorModified).Bold(true)
	header := fmt.Sprintf(" %s Unstaged (%d)", IconModified, len(p.UnstagedFiles))
	if p.Section == SectionUnstaged {
		header = "▸" + header[1:] // Active section indicator
	}
	p.drawText(screen, 1, y, header, headerStyle)

	// Draw shortcut hints if there's space
	hintStyle := config.DefStyle.Foreground(tcell.ColorGray)
	hintX := len(header) + 3
	if hintX+10 < p.Region.Width-2 {
		p.drawText(screen, hintX, y, "s:stage", hintStyle)
	}
	y++

	// Reset file Y positions
	p.unstagedFileYs = make([]int, 0, len(p.UnstagedFiles))

	// Calculate bottom limit based on whether commit section is shown
	// When staged files exist: need 8 rows for commit section
	// When no staged files: need 2 rows for buttons only
	bottomLimit := p.Region.Height - 2
	if len(p.StagedFiles) > 0 {
		bottomLimit = p.Region.Height - 8
	}

	// Files
	for i, file := range p.UnstagedFiles {
		if y >= bottomLimit-2 { // Leave room for staged section
			break
		}

		// Track Y position for click detection
		p.unstagedFileYs = append(p.unstagedFileYs, y)

		isSelected := p.Section == SectionUnstaged && i == p.Selected
		p.drawFileEntry(screen, y, file, isSelected, false)
		y++
	}

	// Empty state
	if len(p.UnstagedFiles) == 0 {
		emptyStyle := config.DefStyle.Foreground(tcell.ColorGray)
		p.drawText(screen, 3, y, "No unstaged changes", emptyStyle)
		y++
	}

	y++ // Spacing
	return y
}

// drawStagedSection draws the staged changes section
func (p *Panel) drawStagedSection(screen tcell.Screen, startY int) int {
	y := startY

	// Track section header Y for click detection
	p.stagedHeaderY = y

	// Section header with shortcuts
	headerStyle := config.DefStyle.Foreground(colorAdded).Bold(true)
	header := fmt.Sprintf(" %s Staged (%d)", IconStaged, len(p.StagedFiles))
	if p.Section == SectionStaged {
		header = "▸" + header[1:] // Active section indicator
	}
	p.drawText(screen, 1, y, header, headerStyle)

	// Draw shortcut hints if there's space
	hintStyle := config.DefStyle.Foreground(tcell.ColorGray)
	hintX := len(header) + 3
	if hintX+10 < p.Region.Width-2 {
		p.drawText(screen, hintX, y, "u:unstage", hintStyle)
	}
	y++

	// Reset file Y positions
	p.stagedFileYs = make([]int, 0, len(p.StagedFiles))

	// Calculate bottom limit based on whether commit section is shown
	// When staged files exist: need 8 rows for commit section
	// When no staged files: need 2 rows for buttons only
	bottomLimit := p.Region.Height - 2
	if len(p.StagedFiles) > 0 {
		bottomLimit = p.Region.Height - 8
	}

	// Files
	for i, file := range p.StagedFiles {
		if y >= bottomLimit { // Leave room for commit section
			break
		}

		// Track Y position for click detection
		p.stagedFileYs = append(p.stagedFileYs, y)

		isSelected := p.Section == SectionStaged && i == p.Selected
		p.drawFileEntry(screen, y, file, isSelected, true)
		y++
	}

	// Empty state
	if len(p.StagedFiles) == 0 {
		emptyStyle := config.DefStyle.Foreground(tcell.ColorGray)
		p.drawText(screen, 3, y, "No staged changes", emptyStyle)
		y++
	}

	y++ // Spacing
	return y
}

// drawFileEntry draws a single file entry with status icon
func (p *Panel) drawFileEntry(screen tcell.Screen, y int, file FileStatus, isSelected bool, isStaged bool) {
	// Selection background
	style := config.DefStyle
	if isSelected {
		if p.Focus {
			style = config.DefStyle.Foreground(tcell.ColorBlack).Background(tcell.ColorWhite)
		} else {
			style = config.DefStyle.Background(tcell.Color236) // Dark gray
		}
		// Fill line with selection background
		for x := 1; x < p.Region.Width-1; x++ {
			screen.SetContent(p.Region.X+x, p.Region.Y+y, ' ', nil, style)
		}
	}

	x := 3 // Indentation

	// Status indicator [N] for new/untracked, [M] for modified, [D] for deleted, [R] for renamed
	statusTag, tagStyle := p.getStatusTag(file.Status)
	if isSelected && p.Focus {
		tagStyle = style
	}
	p.drawTextAt(screen, x, y, statusTag, tagStyle)
	x += len(statusTag) + 1

	// Filename
	name := file.Path
	maxWidth := p.Region.Width - x - 2
	if len(name) > maxWidth && maxWidth > 3 {
		name = name[:maxWidth-3] + "..."
	}
	if isSelected && p.Focus {
		p.drawTextAt(screen, x, y, name, style)
	} else {
		p.drawTextAt(screen, x, y, name, config.DefStyle.Foreground(tcell.Color252))
	}
}

// getStatusTag returns the status tag like [N], [M], [D] for a git status
func (p *Panel) getStatusTag(status string) (string, tcell.Style) {
	switch status {
	case "?":
		return "[N]", config.DefStyle.Foreground(colorAdded).Bold(true) // New/untracked - green
	case "A":
		return "[+]", config.DefStyle.Foreground(colorAdded).Bold(true) // Added - green
	case "M":
		return "[M]", config.DefStyle.Foreground(colorModified).Bold(true) // Modified - yellow
	case "D":
		return "[D]", config.DefStyle.Foreground(colorDeleted).Bold(true) // Deleted - red
	case "R":
		return "[R]", config.DefStyle.Foreground(colorModified).Bold(true) // Renamed - yellow
	case "U":
		return "[!]", config.DefStyle.Foreground(colorConflict).Bold(true) // Conflict - bright yellow
	default:
		return "[?]", config.DefStyle.Foreground(colorUntracked) // Unknown - gray
	}
}

// drawCommitSection draws the commit message input and buttons
func (p *Panel) drawCommitSection(screen tcell.Screen, startY int) int {
	// Check if commit section should be shown (has staged files)
	commitEnabled := len(p.StagedFiles) > 0

	// If no staged files, don't draw commit section at all
	if !commitEnabled {
		// Just draw buttons at bottom
		y := p.Region.Height - 2
		p.buttonsY = y
		p.commitSectionY = -1 // Mark as not visible
		p.drawButtons(screen, y)
		return y + 1
	}

	// Position commit section at bottom of panel (3 rows + borders + header + buttons = 8)
	y := p.Region.Height - 8

	// Track commit section Y for click detection
	p.commitSectionY = y

	// Section header with (c) hint
	headerStyle := config.DefStyle.Foreground(colorHeader).Bold(true)
	header := fmt.Sprintf(" %s Commit (c):", IconCommit)
	if p.Section == SectionCommitInput || p.Section == SectionCommitBtn || p.Section == SectionPushBtn || p.Section == SectionPullBtn {
		header = "▸" + header[1:] // Active section indicator
		headerStyle = headerStyle.Foreground(colorBorder) // Hot pink when active
	}
	p.drawText(screen, 1, y, header, headerStyle)
	y++

	// Commit message input box (3 rows)
	boxWidth := p.Region.Width - 4
	if boxWidth < 10 {
		boxWidth = 10
	}
	contentWidth := boxWidth - 2 // Width inside the box
	numRows := 3

	// Draw input box border
	boxStyle := config.DefStyle.Foreground(tcell.ColorGray)
	if p.Section == SectionCommitInput && p.Focus {
		boxStyle = config.DefStyle.Foreground(colorBorder) // Hot pink when focused
	}

	// Top border
	p.drawText(screen, 2, y, "┌"+strings.Repeat("─", contentWidth)+"┐", boxStyle)
	y++

	// Draw 3 content rows
	msgStyle := config.DefStyle.Foreground(tcell.Color252)
	placeholderStyle := config.DefStyle.Foreground(tcell.ColorGray)

	// Calculate which portion of the message to show (scroll if needed)
	totalChars := len(p.CommitMsg)
	charsPerRow := contentWidth
	totalVisibleChars := charsPerRow * numRows

	// Determine scroll offset to keep cursor visible
	scrollOffset := 0
	if p.CommitCursor >= totalVisibleChars {
		// Scroll to keep cursor visible in last row
		scrollOffset = ((p.CommitCursor - totalVisibleChars) / charsPerRow + 1) * charsPerRow
	}

	for row := 0; row < numRows; row++ {
		// Draw side borders
		p.drawText(screen, 2, y, "│", boxStyle)
		p.drawText(screen, 2+boxWidth-1, y, "│", boxStyle)

		// Calculate start/end indices for this row
		startIdx := scrollOffset + row*charsPerRow
		endIdx := startIdx + charsPerRow

		if row == 0 && p.CommitMsg == "" && !(p.Section == SectionCommitInput && p.Focus) {
			// Show placeholder on first row when empty and not focused
			p.drawText(screen, 3, y, "Enter commit message...", placeholderStyle)
		} else if startIdx < totalChars {
			// Draw portion of message for this row
			if endIdx > totalChars {
				endIdx = totalChars
			}
			rowText := p.CommitMsg[startIdx:endIdx]
			p.drawText(screen, 3, y, rowText, msgStyle)
		}

		// Draw cursor if in this row
		if p.Section == SectionCommitInput && p.Focus {
			cursorPosInView := p.CommitCursor - scrollOffset
			rowStart := row * charsPerRow
			rowEnd := rowStart + charsPerRow
			if cursorPosInView >= rowStart && cursorPosInView < rowEnd {
				cursorX := cursorPosInView - rowStart
				cursorStyle := config.DefStyle.Background(tcell.ColorWhite).Foreground(tcell.ColorBlack)
				cursorChar := ' '
				if p.CommitCursor < len(p.CommitMsg) {
					cursorChar = rune(p.CommitMsg[p.CommitCursor])
				}
				screen.SetContent(p.Region.X+3+cursorX, p.Region.Y+y, cursorChar, nil, cursorStyle)
			}
		}

		y++
	}

	// Bottom border
	p.drawText(screen, 2, y, "└"+strings.Repeat("─", contentWidth)+"┘", boxStyle)
	y++

	// Buttons
	p.drawButtons(screen, y)

	return y + 1
}

// drawButtons draws the commit, push, and pull buttons
func (p *Panel) drawButtons(screen tcell.Screen, y int) {
	// Track buttons Y for click detection
	p.buttonsY = y

	x := 2

	// Commit button - enabled when staged files and commit message
	commitEnabled := len(p.StagedFiles) > 0 && p.CommitMsg != ""
	commitStyle := config.DefStyle.Foreground(tcell.ColorGray)
	if p.Section == SectionCommitBtn && p.Focus {
		// Focused button - hot pink background
		commitStyle = config.DefStyle.Foreground(tcell.ColorBlack).Background(colorBorder)
	} else if commitEnabled {
		commitStyle = config.DefStyle.Foreground(colorAdded).Bold(true)
	}
	commitBtn := "[c]Commit"
	p.drawText(screen, x, y, commitBtn, commitStyle)
	x += len(commitBtn) + 1

	// Push button - enabled when ahead of remote
	pushEnabled := p.AheadCount > 0
	pushStyle := config.DefStyle.Foreground(tcell.ColorGray)
	if p.Section == SectionPushBtn && p.Focus {
		// Focused button - hot pink background
		pushStyle = config.DefStyle.Foreground(tcell.ColorBlack).Background(colorBorder)
	} else if pushEnabled {
		pushStyle = config.DefStyle.Foreground(colorButton).Bold(true)
	}
	pushBtn := "[p]Push"
	if pushEnabled {
		pushBtn = fmt.Sprintf("[p]Push(%d)", p.AheadCount)
	}
	p.drawText(screen, x, y, pushBtn, pushStyle)
	x += len(pushBtn) + 1

	// Pull button - enabled when behind remote
	pullEnabled := p.BehindCount > 0
	pullStyle := config.DefStyle.Foreground(tcell.ColorGray)
	if p.Section == SectionPullBtn && p.Focus {
		// Focused button - hot pink background
		pullStyle = config.DefStyle.Foreground(tcell.ColorBlack).Background(colorBorder)
	} else if pullEnabled {
		pullStyle = config.DefStyle.Foreground(colorModified).Bold(true) // Yellow for pull
	}
	pullBtn := "[l]Pull"
	if pullEnabled {
		pullBtn = fmt.Sprintf("[l]Pull(%d)", p.BehindCount)
	}
	p.drawText(screen, x, y, pullBtn, pullStyle)
}

// drawBorder draws the panel border
func (p *Panel) drawBorder(screen tcell.Screen) {
	var style tcell.Style
	if p.Focus {
		style = config.DefStyle.Foreground(colorBorder) // Hot pink
	} else {
		style = config.DefStyle.Foreground(tcell.ColorGray)
	}

	// Draw vertical lines
	for y := 1; y < p.Region.Height-1; y++ {
		if p.Focus {
			screen.SetContent(p.Region.X, p.Region.Y+y, '║', nil, style)
			screen.SetContent(p.Region.X+p.Region.Width-1, p.Region.Y+y, '║', nil, style)
		} else {
			screen.SetContent(p.Region.X, p.Region.Y+y, '│', nil, style)
			screen.SetContent(p.Region.X+p.Region.Width-1, p.Region.Y+y, '│', nil, style)
		}
	}

	// Draw horizontal lines
	for x := 1; x < p.Region.Width-1; x++ {
		if p.Focus {
			screen.SetContent(p.Region.X+x, p.Region.Y, '═', nil, style)
			screen.SetContent(p.Region.X+x, p.Region.Y+p.Region.Height-1, '═', nil, style)
		} else {
			screen.SetContent(p.Region.X+x, p.Region.Y, '─', nil, style)
			screen.SetContent(p.Region.X+x, p.Region.Y+p.Region.Height-1, '─', nil, style)
		}
	}

	// Draw corners
	if p.Focus {
		screen.SetContent(p.Region.X, p.Region.Y, '╔', nil, style)
		screen.SetContent(p.Region.X+p.Region.Width-1, p.Region.Y, '╗', nil, style)
		screen.SetContent(p.Region.X, p.Region.Y+p.Region.Height-1, '╚', nil, style)
		screen.SetContent(p.Region.X+p.Region.Width-1, p.Region.Y+p.Region.Height-1, '╝', nil, style)
	} else {
		screen.SetContent(p.Region.X, p.Region.Y, '┌', nil, style)
		screen.SetContent(p.Region.X+p.Region.Width-1, p.Region.Y, '┐', nil, style)
		screen.SetContent(p.Region.X, p.Region.Y+p.Region.Height-1, '└', nil, style)
		screen.SetContent(p.Region.X+p.Region.Width-1, p.Region.Y+p.Region.Height-1, '┘', nil, style)
	}
}

// drawText draws text at the given position relative to panel
func (p *Panel) drawText(screen tcell.Screen, x, y int, text string, style tcell.Style) int {
	return p.drawTextAt(screen, x, y, text, style)
}

// drawTextAt draws text at the given position relative to panel
func (p *Panel) drawTextAt(screen tcell.Screen, x, y int, text string, style tcell.Style) int {
	screenX := p.Region.X + x
	screenY := p.Region.Y + y

	count := 0
	for _, r := range text {
		if screenX+count >= p.Region.X+p.Region.Width-1 {
			break
		}
		screen.SetContent(screenX+count, screenY, r, nil, style)
		count++
	}

	return count
}

// drawBranchDialog draws a modal dialog for branch switching
func (p *Panel) drawBranchDialog(screen tcell.Screen) {
	if !p.ShowBranchDialog || len(p.LocalBranches) == 0 {
		return
	}

	// Dialog dimensions
	dialogWidth := 30
	if p.Region.Width-4 < dialogWidth {
		dialogWidth = p.Region.Width - 4
	}
	maxVisible := 8
	if len(p.LocalBranches) < maxVisible {
		maxVisible = len(p.LocalBranches)
	}
	dialogHeight := maxVisible + 4 // Title + border + footer

	// Center dialog in panel
	dialogX := p.Region.X + (p.Region.Width-dialogWidth)/2
	dialogY := p.Region.Y + (p.Region.Height-dialogHeight)/2

	// Clear dialog area
	bgStyle := config.DefStyle
	for dy := 0; dy < dialogHeight; dy++ {
		for dx := 0; dx < dialogWidth; dx++ {
			screen.SetContent(dialogX+dx, dialogY+dy, ' ', nil, bgStyle)
		}
	}

	// Draw border
	borderStyle := config.DefStyle.Foreground(colorBorder)
	// Top border
	screen.SetContent(dialogX, dialogY, '╔', nil, borderStyle)
	for x := 1; x < dialogWidth-1; x++ {
		screen.SetContent(dialogX+x, dialogY, '═', nil, borderStyle)
	}
	screen.SetContent(dialogX+dialogWidth-1, dialogY, '╗', nil, borderStyle)

	// Title
	title := " Switch Branch "
	titleX := dialogX + (dialogWidth-len(title))/2
	titleStyle := config.DefStyle.Foreground(colorHeader).Bold(true)
	for i, r := range title {
		screen.SetContent(titleX+i, dialogY, r, nil, titleStyle)
	}

	// Sides
	for y := 1; y < dialogHeight-1; y++ {
		screen.SetContent(dialogX, dialogY+y, '║', nil, borderStyle)
		screen.SetContent(dialogX+dialogWidth-1, dialogY+y, '║', nil, borderStyle)
	}

	// Separator after title
	screen.SetContent(dialogX, dialogY+1, '╠', nil, borderStyle)
	for x := 1; x < dialogWidth-1; x++ {
		screen.SetContent(dialogX+x, dialogY+1, '─', nil, borderStyle)
	}
	screen.SetContent(dialogX+dialogWidth-1, dialogY+1, '╣', nil, borderStyle)

	// Branch list
	currentBranch := p.GetBranchName()
	listY := dialogY + 2
	visibleCount := 0
	for i := p.BranchTopLine; i < len(p.LocalBranches) && visibleCount < maxVisible; i++ {
		branch := p.LocalBranches[i]
		y := listY + visibleCount

		// Selection highlight
		isSelected := i == p.BranchSelected
		isCurrent := branch == currentBranch

		var style tcell.Style
		if isSelected {
			style = config.DefStyle.Foreground(tcell.ColorBlack).Background(tcell.ColorWhite)
			// Fill line with selection background
			for x := 1; x < dialogWidth-1; x++ {
				screen.SetContent(dialogX+x, y, ' ', nil, style)
			}
		} else {
			style = config.DefStyle.Foreground(tcell.Color252)
		}

		// Branch indicator
		indicator := "  "
		if isCurrent {
			indicator = "* "
			if !isSelected {
				style = config.DefStyle.Foreground(colorAdded) // Green for current
			}
		} else if isSelected {
			indicator = "> "
		}

		// Draw branch name
		branchText := indicator + branch
		if len(branchText) > dialogWidth-3 {
			branchText = branchText[:dialogWidth-6] + "..."
		}
		for i, r := range branchText {
			if dialogX+1+i < dialogX+dialogWidth-1 {
				screen.SetContent(dialogX+1+i, y, r, nil, style)
			}
		}
		visibleCount++
	}

	// Footer separator
	footerY := dialogY + dialogHeight - 2
	screen.SetContent(dialogX, footerY, '╠', nil, borderStyle)
	for x := 1; x < dialogWidth-1; x++ {
		screen.SetContent(dialogX+x, footerY, '─', nil, borderStyle)
	}
	screen.SetContent(dialogX+dialogWidth-1, footerY, '╣', nil, borderStyle)

	// Footer with hints
	footer := " ↑↓:select Enter Esc "
	footerStyle := config.DefStyle.Foreground(tcell.ColorGray)
	footerX := dialogX + (dialogWidth-len(footer))/2
	for i, r := range footer {
		screen.SetContent(footerX+i, footerY+1, r, nil, footerStyle)
	}

	// Bottom border
	screen.SetContent(dialogX, dialogY+dialogHeight-1, '╚', nil, borderStyle)
	for x := 1; x < dialogWidth-1; x++ {
		screen.SetContent(dialogX+x, dialogY+dialogHeight-1, '═', nil, borderStyle)
	}
	screen.SetContent(dialogX+dialogWidth-1, dialogY+dialogHeight-1, '╝', nil, borderStyle)
}
