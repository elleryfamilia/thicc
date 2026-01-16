package sourcecontrol

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

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

	// Commit graph colors
	colorGraphMain   = tcell.Color45  // Cyan - commits on main line
	colorGraphMerge  = tcell.Color205 // Pink/Magenta - merge commits
	colorGraphBranch = tcell.Color214 // Yellow/Orange - commits from merged branches
	colorGraphLine   = tcell.Color243 // Gray - vertical lines
)

// Spinner animation frames (braille dots)
var spinnerFrames = []rune{'⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏'}

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

	// Calculate graph section height (30% of panel)
	graphHeight := p.Region.Height * 30 / 100
	if graphHeight < 5 {
		graphHeight = 5
	}

	// Draw content (in top 60%)
	y := p.drawHeader(screen)
	y = p.drawUnstagedSection(screen, y)
	y = p.drawStagedSection(screen, y)
	y = p.drawCommitSection(screen, y)
	_ = y // silence unused warning

	// Draw commit graph (bottom 40%)
	p.drawCommitGraph(screen, graphHeight)

	// Draw border
	p.drawBorder(screen)

	// Draw spinner overlay if operation in progress
	if p.OperationInProgress != "" {
		p.drawSpinner(screen)
	}

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

	return y + 2 // Extra space after branch name
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
	hintX := len(header) + 2
	hint := "[space] stage"
	if hintX+len(hint) < p.Region.Width-2 {
		p.drawText(screen, hintX, y, hint, hintStyle)
	}
	y++

	// Reset file Y positions
	p.unstagedFileYs = make([]int, 0, len(p.UnstagedFiles))

	// Calculate bottom limit based on whether commit section is shown
	// When staged files exist: need 9 rows for commit section (4 rows + borders + header + buttons)
	// When no staged files: need 2 rows for buttons only
	// Also reserve 40% at bottom for graph
	graphHeight := p.Region.Height * 40 / 100
	if graphHeight < 5 {
		graphHeight = 5
	}
	bottomLimit := p.Region.Height - graphHeight - 2
	if len(p.StagedFiles) > 0 {
		bottomLimit = p.Region.Height - graphHeight - 9
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
	hintX := len(header) + 2
	hint := "[space] unstage"
	if hintX+len(hint) < p.Region.Width-2 {
		p.drawText(screen, hintX, y, hint, hintStyle)
	}
	y++

	// Reset file Y positions
	p.stagedFileYs = make([]int, 0, len(p.StagedFiles))

	// Calculate bottom limit based on whether commit section is shown
	// When staged files exist: need 9 rows for commit section (4 rows + borders + header + buttons)
	// When no staged files: need 2 rows for buttons only
	// Also reserve 40% at bottom for graph
	graphHeight := p.Region.Height * 40 / 100
	if graphHeight < 5 {
		graphHeight = 5
	}
	bottomLimit := p.Region.Height - graphHeight - 2
	if len(p.StagedFiles) > 0 {
		bottomLimit = p.Region.Height - graphHeight - 9
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

	// Filename - show right-to-left so filename is visible, with filename bolded
	maxWidth := p.Region.Width - x - 2
	path := file.Path

	// Split into directory and filename
	dir, filename := filepath.Split(path)

	if len(path) > maxWidth && maxWidth > 3 {
		// Truncate from the left (beginning), keeping the filename visible
		path = "..." + path[len(path)-maxWidth+3:]
		// Recalculate dir/filename for truncated path
		dir, filename = filepath.Split(path)
	}

	if isSelected && p.Focus {
		// Selected: use selection style for both
		if dir != "" {
			p.drawTextAt(screen, x, y, dir, style)
			x += len(dir)
		}
		p.drawTextAt(screen, x, y, filename, style.Bold(true))
	} else {
		// Not selected: dim dir, bold filename
		if dir != "" {
			p.drawTextAt(screen, x, y, dir, config.DefStyle.Foreground(tcell.Color243))
			x += len(dir)
		}
		p.drawTextAt(screen, x, y, filename, config.DefStyle.Foreground(tcell.Color252).Bold(true))
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

	// Calculate graph height to position commit section above it
	graphHeight := p.Region.Height * 30 / 100
	if graphHeight < 5 {
		graphHeight = 5
	}

	// If no staged files, don't draw commit section at all
	if !commitEnabled {
		// Just draw buttons above graph section
		y := p.Region.Height - graphHeight - 2
		p.buttonsY = y
		p.commitSectionY = -1 // Mark as not visible
		p.drawButtons(screen, y)
		return y + 1
	}

	// Position commit section above graph (4 rows + borders + header + buttons = 9)
	y := p.Region.Height - graphHeight - 9

	// Track commit section Y for click detection
	p.commitSectionY = y

	// Section header with (c) hint
	headerStyle := config.DefStyle.Foreground(colorHeader).Bold(true)
	header := fmt.Sprintf(" %s Commit:", IconCommit)
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
	numRows := 4

	// Draw input box border
	boxStyle := config.DefStyle.Foreground(tcell.ColorGray)
	if p.Section == SectionCommitInput && p.Focus {
		boxStyle = config.DefStyle.Foreground(colorBorder) // Hot pink when focused
	}

	// Top border
	p.drawText(screen, 2, y, "┌"+strings.Repeat("─", contentWidth)+"┐", boxStyle)
	y++

	// Draw content rows
	msgStyle := config.DefStyle.Foreground(tcell.Color252)
	placeholderStyle := config.DefStyle.Foreground(tcell.ColorGray)

	// Build visual lines from commit message (split by newlines, then wrap)
	visualLines := p.buildVisualLines(p.CommitMsg, contentWidth)

	// Find cursor's visual position
	cursorRow, cursorCol := p.getCursorVisualPos(p.CommitMsg, p.CommitCursor, contentWidth)

	// Calculate scroll offset to keep cursor visible
	scrollRow := 0
	if cursorRow >= numRows {
		scrollRow = cursorRow - numRows + 1
	}

	for row := 0; row < numRows; row++ {
		// Draw side borders
		p.drawText(screen, 2, y, "│", boxStyle)
		p.drawText(screen, 2+boxWidth-1, y, "│", boxStyle)

		lineIdx := scrollRow + row

		if row == 0 && p.CommitMsg == "" && !(p.Section == SectionCommitInput && p.Focus) {
			// Show placeholder on first row when empty and not focused
			p.drawText(screen, 3, y, "Enter commit message...", placeholderStyle)
		} else if lineIdx < len(visualLines) {
			// Draw this visual line
			p.drawText(screen, 3, y, visualLines[lineIdx], msgStyle)
		}

		// Draw cursor if in this row
		if p.Section == SectionCommitInput && p.Focus {
			if cursorRow == lineIdx {
				cursorStyle := config.DefStyle.Background(tcell.ColorWhite).Foreground(tcell.ColorBlack)
				cursorChar := ' '
				if p.CommitCursor < len(p.CommitMsg) && p.CommitMsg[p.CommitCursor] != '\n' {
					cursorChar = rune(p.CommitMsg[p.CommitCursor])
				}
				screen.SetContent(p.Region.X+3+cursorCol, p.Region.Y+y, cursorChar, nil, cursorStyle)
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
	commitBtn := "[⌥C]Commit"
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
	pushBtn := "[⌥P]Push"
	if pushEnabled {
		pushBtn = fmt.Sprintf("[⌥P]Push(%d)", p.AheadCount)
	}
	p.drawText(screen, x, y, pushBtn, pushStyle)
	x += len(pushBtn) + 1

	// Pull button - always enabled (can pull anytime)
	pullStyle := config.DefStyle.Foreground(colorModified).Bold(true) // Yellow for pull
	if p.Section == SectionPullBtn && p.Focus {
		// Focused button - hot pink background
		pullStyle = config.DefStyle.Foreground(tcell.ColorBlack).Background(colorBorder)
	}
	pullBtn := "[⌥L]Pull"
	if p.BehindCount > 0 {
		pullBtn = fmt.Sprintf("[⌥L]Pull(%d)", p.BehindCount)
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

// buildVisualLines converts a message with newlines into visual lines for display
// Each line is wrapped to fit within lineWidth
func (p *Panel) buildVisualLines(msg string, lineWidth int) []string {
	if msg == "" {
		return []string{""}
	}

	var result []string
	lines := strings.Split(msg, "\n")

	for _, line := range lines {
		if len(line) == 0 {
			result = append(result, "")
			continue
		}
		// Wrap long lines
		for len(line) > lineWidth {
			result = append(result, line[:lineWidth])
			line = line[lineWidth:]
		}
		result = append(result, line)
	}

	return result
}

// getCursorVisualPos returns the visual row and column for a cursor position
func (p *Panel) getCursorVisualPos(msg string, cursor int, lineWidth int) (row, col int) {
	if cursor <= 0 || msg == "" {
		return 0, 0
	}

	// Count through the message up to cursor position
	row = 0
	col = 0

	for i := 0; i < cursor && i < len(msg); i++ {
		if msg[i] == '\n' {
			row++
			col = 0
		} else {
			col++
			if col >= lineWidth {
				row++
				col = 0
			}
		}
	}

	return row, col
}

// drawSpinner draws a spinner overlay during git operations
func (p *Panel) drawSpinner(screen tcell.Screen) {
	if p.OperationInProgress == "" {
		return
	}

	// Calculate spinner frame based on time
	frame := int(time.Now().UnixMilli()/80) % len(spinnerFrames)
	spinner := spinnerFrames[frame]

	// Build message
	msg := fmt.Sprintf(" %c %s... ", spinner, p.OperationInProgress)

	// Center in panel
	msgLen := len(msg) + 1 // +1 for spinner rune width
	x := p.Region.X + (p.Region.Width-msgLen)/2
	y := p.Region.Y + p.Region.Height/2

	// Draw background box
	boxStyle := config.DefStyle.Background(tcell.Color236) // Dark gray background
	for dx := -1; dx <= msgLen; dx++ {
		screen.SetContent(x+dx, y-1, ' ', nil, boxStyle)
		screen.SetContent(x+dx, y, ' ', nil, boxStyle)
		screen.SetContent(x+dx, y+1, ' ', nil, boxStyle)
	}

	// Draw spinner and message
	spinnerStyle := config.DefStyle.Foreground(colorBorder).Background(tcell.Color236)
	screen.SetContent(x, y, spinner, nil, spinnerStyle)

	textStyle := config.DefStyle.Foreground(tcell.ColorWhite).Background(tcell.Color236)
	for i, r := range p.OperationInProgress + "..." {
		screen.SetContent(x+2+i, y, r, nil, textStyle)
	}
}

// drawCommitGraph draws the commit history graph at the bottom of the panel
func (p *Panel) drawCommitGraph(screen tcell.Screen, graphHeight int) {
	// Calculate starting Y position (bottom of panel minus graph height)
	startY := p.Region.Height - graphHeight

	// Track graph section Y for click detection
	p.graphSectionY = startY
	p.graphRowYs = make([]int, 0)
	p.graphYToRow = make(map[int]int)

	y := startY

	// Draw "History" label left-aligned with divider line
	dividerStyle := config.DefStyle.Foreground(tcell.ColorGray)
	labelStyle := config.DefStyle.Foreground(colorHeader).Bold(true)

	// Draw "History" label
	p.drawText(screen, 1, y, " History ", labelStyle)

	// Draw divider line after label
	labelWidth := 9 // len(" History ")
	dividerWidth := p.Region.Width - labelWidth - 3
	if dividerWidth > 0 {
		divider := strings.Repeat("─", dividerWidth)
		p.drawText(screen, 1+labelWidth, y, divider, dividerStyle)
	}
	y++

	// Empty state
	if len(p.CommitGraph) == 0 {
		emptyStyle := config.DefStyle.Foreground(tcell.ColorGray)
		p.drawText(screen, 3, y, "No commits yet", emptyStyle)
		return
	}

	// Build flattened list of rows (commits + expanded files)
	// For expanded commits, message can take up to 3 lines
	type graphRow struct {
		isCommit    bool
		commitIdx   int
		fileIdx     int // Only valid if !isCommit
		commitEntry *CommitEntry
		file        *FileStatus
	}
	var rows []graphRow
	for i := range p.CommitGraph {
		commit := &p.CommitGraph[i]
		rows = append(rows, graphRow{isCommit: true, commitIdx: i, commitEntry: commit})
		if commit.Expanded {
			for j := range commit.Files {
				rows = append(rows, graphRow{isCommit: false, commitIdx: i, fileIdx: j, commitEntry: commit, file: &commit.Files[j]})
			}
		}
	}

	// Calculate how many screen lines each row takes
	getRowHeight := func(row graphRow) int {
		if row.isCommit && row.commitEntry.Expanded {
			// Calculate wrapped lines for expanded commit
			prefixWidth := 5
			textWidth := p.Region.Width - prefixWidth - 2
			if textWidth < 10 {
				textWidth = 10
			}
			subject := row.commitEntry.Subject
			lines := 1
			for len(subject) > textWidth && lines < 3 {
				lines++
				subject = subject[textWidth:]
			}
			return lines
		}
		return 1
	}

	// Calculate visible area
	visibleScreenLines := graphHeight - 2 // Minus divider and bottom border

	// Clamp selection
	if p.GraphSelected >= len(rows) {
		p.GraphSelected = len(rows) - 1
	}
	if p.GraphSelected < 0 {
		p.GraphSelected = 0
	}
	// Adjust scroll to keep selected visible
	// Calculate screen line of selected row from top
	selectedScreenLine := 0
	for i := p.GraphTopLine; i < p.GraphSelected && i < len(rows); i++ {
		selectedScreenLine += getRowHeight(rows[i])
	}
	selectedHeight := getRowHeight(rows[p.GraphSelected])

	// Scroll up if selected is above view
	for p.GraphSelected < p.GraphTopLine {
		p.GraphTopLine--
	}

	// Scroll down if selected is below view
	for selectedScreenLine+selectedHeight > visibleScreenLines && p.GraphTopLine < p.GraphSelected {
		p.GraphTopLine++
		selectedScreenLine -= getRowHeight(rows[p.GraphTopLine-1])
	}

	// Draw visible rows
	for i := p.GraphTopLine; i < len(rows) && y < p.Region.Height-1; i++ {
		row := rows[i]
		isSelected := i == p.GraphSelected

		// Track Y position for click detection (first line of this row)
		p.graphRowYs = append(p.graphRowYs, y)

		if row.isCommit {
			linesUsed := p.drawGraphCommitRow(screen, y, row.commitEntry, isSelected)
			// Map all Y positions used by this commit to this row index
			for line := 0; line < linesUsed; line++ {
				p.graphYToRow[y+line] = i
			}
			y += linesUsed
		} else {
			p.graphYToRow[y] = i
			p.drawGraphFileRow(screen, y, row.file, isSelected)
			y++
		}
	}
}

// drawGraphCommitRow draws a commit row in the graph
// Returns the number of lines used (1 when collapsed, up to 3 when expanded)
func (p *Panel) drawGraphCommitRow(screen tcell.Screen, y int, commit *CommitEntry, isSelected bool) int {
	// Determine how many lines we'll use
	maxLines := 1
	if commit.Expanded {
		maxLines = 3
	}

	// Calculate text area width (after graph prefix)
	prefixWidth := 4 // "● ▾ " = 4 chars
	textWidth := p.Region.Width - prefixWidth - 2

	// Wrap the subject to multiple lines if expanded
	var subjectLines []string
	if commit.Expanded && len(commit.Subject) > textWidth {
		// Wrap to multiple lines
		remaining := commit.Subject
		for len(remaining) > 0 && len(subjectLines) < maxLines {
			if len(remaining) <= textWidth {
				subjectLines = append(subjectLines, remaining)
				break
			}
			// Find a good break point (prefer space)
			breakAt := textWidth
			for i := textWidth - 1; i > textWidth/2; i-- {
				if remaining[i] == ' ' {
					breakAt = i
					break
				}
			}
			subjectLines = append(subjectLines, remaining[:breakAt])
			remaining = strings.TrimLeft(remaining[breakAt:], " ")
		}
		// If there's still more text, add ellipsis to last line
		if len(remaining) > 0 && len(subjectLines) == maxLines {
			lastIdx := len(subjectLines) - 1
			if len(subjectLines[lastIdx]) > textWidth-3 {
				subjectLines[lastIdx] = subjectLines[lastIdx][:textWidth-3] + "..."
			} else {
				subjectLines[lastIdx] = subjectLines[lastIdx] + "..."
			}
		}
	} else {
		// Single line, truncate if needed
		subject := commit.Subject
		if len(subject) > textWidth && textWidth > 3 {
			subject = subject[:textWidth-3] + "..."
		}
		subjectLines = []string{subject}
	}

	linesUsed := len(subjectLines)

	// Draw each line
	for lineNum, lineText := range subjectLines {
		currentY := y + lineNum

		// Selection background for this line
		style := config.DefStyle
		if isSelected {
			if p.Focus && p.Section == SectionCommitGraph {
				style = config.DefStyle.Foreground(tcell.ColorBlack).Background(tcell.ColorWhite)
			} else {
				style = config.DefStyle.Background(tcell.Color236) // Dark gray
			}
			// Fill line with selection background
			for x := 1; x < p.Region.Width-1; x++ {
				screen.SetContent(p.Region.X+x, p.Region.Y+currentY, ' ', nil, style)
			}
		}

		x := 1

		// Commit dot (only on first line)
		if lineNum == 0 {
			// Commit dot with color based on type
			var dotStyle tcell.Style
			var dot string
			if commit.IsMerge {
				dotStyle = config.DefStyle.Foreground(colorGraphMerge)
				dot = "◆"
			} else if commit.FromBranch {
				dotStyle = config.DefStyle.Foreground(colorGraphBranch)
				dot = "●"
			} else {
				dotStyle = config.DefStyle.Foreground(colorGraphMain)
				dot = "●"
			}
			if isSelected && p.Focus && p.Section == SectionCommitGraph {
				dotStyle = style
			}
			p.drawTextAt(screen, x, currentY, dot, dotStyle)
			x += 2 // dot + space

			// Expansion indicator if has files
			if len(commit.Files) > 0 {
				indicator := "▸"
				if commit.Expanded {
					indicator = "▾"
				}
				indicatorStyle := config.DefStyle.Foreground(tcell.ColorGray)
				if isSelected && p.Focus && p.Section == SectionCommitGraph {
					indicatorStyle = style
				}
				p.drawTextAt(screen, x, currentY, indicator, indicatorStyle)
				x += 2
			}
		} else {
			// Continuation lines: just indent to align with first line's text
			x = prefixWidth
		}

		// Draw the text for this line
		subjectStyle := config.DefStyle.Foreground(tcell.Color252) // Light gray
		if isSelected && p.Focus && p.Section == SectionCommitGraph {
			subjectStyle = style
		}
		p.drawTextAt(screen, x, currentY, lineText, subjectStyle)
	}

	return linesUsed
}

// drawGraphFileRow draws a single file row under an expanded commit
func (p *Panel) drawGraphFileRow(screen tcell.Screen, y int, file *FileStatus, isSelected bool) {
	// Selection background
	style := config.DefStyle
	if isSelected {
		if p.Focus && p.Section == SectionCommitGraph {
			style = config.DefStyle.Foreground(tcell.ColorBlack).Background(tcell.ColorWhite)
		} else {
			style = config.DefStyle.Background(tcell.Color236) // Dark gray
		}
		// Fill line with selection background
		for x := 1; x < p.Region.Width-1; x++ {
			screen.SetContent(p.Region.X+x, p.Region.Y+y, ' ', nil, style)
		}
	}

	x := 1

	// Tree branch character
	lineStyle := config.DefStyle.Foreground(colorGraphLine)
	if isSelected && p.Focus && p.Section == SectionCommitGraph {
		lineStyle = style
	}
	p.drawTextAt(screen, x, y, "└", lineStyle)
	x += 2

	// Status tag
	statusTag, tagStyle := p.getStatusTag(file.Status)
	if isSelected && p.Focus && p.Section == SectionCommitGraph {
		tagStyle = style
	}
	p.drawTextAt(screen, x, y, statusTag, tagStyle)
	x += len(statusTag) + 1

	// File path - show right-to-left so filename is visible, with filename bolded
	maxPath := p.Region.Width - x - 2
	path := file.Path

	// Split into directory and filename
	dir, filename := filepath.Split(path)

	if len(path) > maxPath && maxPath > 3 {
		// Truncate from the left (beginning), keeping the filename visible
		path = "..." + path[len(path)-maxPath+3:]
		// Recalculate dir/filename for truncated path
		dir, filename = filepath.Split(path)
	}

	// Draw directory part (not bold)
	dirStyle := config.DefStyle.Foreground(tcell.Color243) // Dimmer gray for dir
	if isSelected && p.Focus && p.Section == SectionCommitGraph {
		dirStyle = style
	}
	if dir != "" {
		p.drawTextAt(screen, x, y, dir, dirStyle)
		x += len(dir)
	}

	// Draw filename part (bold)
	filenameStyle := config.DefStyle.Foreground(tcell.Color252).Bold(true)
	if isSelected && p.Focus && p.Section == SectionCommitGraph {
		filenameStyle = style.Bold(true)
	}
	p.drawTextAt(screen, x, y, filename, filenameStyle)
}
