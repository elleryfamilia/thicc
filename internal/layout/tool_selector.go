package layout

import (
	"log"

	"github.com/ellery/thicc/internal/aiterminal"
	"github.com/ellery/thicc/internal/config"
	"github.com/micro-editor/tcell/v2"
)

// ToolSelector is a modal for selecting shell or AI tool for a new terminal
type ToolSelector struct {
	AITools     []aiterminal.AITool
	SelectedIdx int
	Active      bool
	OnSelect    func(cmdArgs []string) // callback with selected command
	OnCancel    func()                 // callback when cancelled
}

// NewToolSelector creates a new tool selector with available tools
func NewToolSelector() *ToolSelector {
	// Get available tools - shell will be last in the list
	tools := aiterminal.GetAvailableToolsOnly()

	// Move Shell to the front (default selection)
	shellIdx := -1
	for i, t := range tools {
		if t.Name == "Shell (default)" {
			shellIdx = i
			break
		}
	}
	if shellIdx > 0 {
		shell := tools[shellIdx]
		tools = append([]aiterminal.AITool{shell}, append(tools[:shellIdx], tools[shellIdx+1:]...)...)
	}

	return &ToolSelector{
		AITools:     tools,
		SelectedIdx: 0, // Default to first item (Shell)
		Active:      false,
	}
}

// Show activates the tool selector
func (ts *ToolSelector) Show(onSelect func(cmdArgs []string), onCancel func()) {
	ts.Active = true
	ts.SelectedIdx = 0 // Reset to shell
	ts.OnSelect = onSelect
	ts.OnCancel = onCancel
	log.Println("THICC: Tool selector shown")
}

// Hide deactivates the tool selector
func (ts *ToolSelector) Hide() {
	ts.Active = false
	log.Println("THICC: Tool selector hidden")
}

// IsActive returns whether the tool selector is currently showing
func (ts *ToolSelector) IsActive() bool {
	return ts.Active
}

// HandleEvent processes keyboard input for the tool selector
func (ts *ToolSelector) HandleEvent(event tcell.Event) bool {
	if !ts.Active {
		return false
	}

	switch ev := event.(type) {
	case *tcell.EventKey:
		switch ev.Key() {
		case tcell.KeyUp, tcell.KeyBacktab:
			if ts.SelectedIdx > 0 {
				ts.SelectedIdx--
			}
			return true
		case tcell.KeyDown, tcell.KeyTab:
			if ts.SelectedIdx < len(ts.AITools)-1 {
				ts.SelectedIdx++
			}
			return true
		case tcell.KeyEnter:
			ts.selectCurrent()
			return true
		case tcell.KeyEscape:
			ts.cancel()
			return true
		case tcell.KeyRune:
			// Vim-style navigation
			switch ev.Rune() {
			case 'j':
				if ts.SelectedIdx < len(ts.AITools)-1 {
					ts.SelectedIdx++
				}
				return true
			case 'k':
				if ts.SelectedIdx > 0 {
					ts.SelectedIdx--
				}
				return true
			}
		}
	}

	return false
}

// selectCurrent selects the current tool and triggers the callback
func (ts *ToolSelector) selectCurrent() {
	if ts.SelectedIdx >= 0 && ts.SelectedIdx < len(ts.AITools) {
		tool := ts.AITools[ts.SelectedIdx]
		log.Printf("THICC: Tool selected: %s", tool.Name)

		var cmdArgs []string
		if tool.Name == "Shell (default)" {
			cmdArgs = nil // nil means use default shell
		} else {
			cmdArgs = tool.GetCommandLine()
		}

		ts.Hide()
		if ts.OnSelect != nil {
			ts.OnSelect(cmdArgs)
		}
	}
}

// cancel cancels the selection (defaults to shell)
func (ts *ToolSelector) cancel() {
	log.Println("THICC: Tool selector cancelled, defaulting to shell")
	ts.Hide()
	if ts.OnCancel != nil {
		ts.OnCancel()
	}
}

// Render draws the tool selector modal centered over the terminal region
func (ts *ToolSelector) Render(screen tcell.Screen, termRegionX, termRegionW, screenH int) {
	if !ts.Active {
		return
	}

	// Calculate modal size
	modalWidth := 40
	if modalWidth > termRegionW-4 {
		modalWidth = termRegionW - 4 // Ensure modal fits in terminal region
	}
	modalHeight := len(ts.AITools) + 6 // tools + title + borders + instructions

	// Center the modal within the terminal region
	startX := termRegionX + (termRegionW-modalWidth)/2
	startY := (screenH - modalHeight) / 2

	// Styles
	borderStyle := config.DefStyle.Foreground(tcell.ColorTeal)
	titleStyle := config.DefStyle.Foreground(tcell.ColorTeal).Bold(true)
	normalStyle := config.DefStyle
	selectedStyle := config.DefStyle.Foreground(tcell.ColorBlack).Background(tcell.ColorTeal)
	hintStyle := config.DefStyle.Foreground(tcell.ColorGray)

	// Draw background
	for y := startY; y < startY+modalHeight; y++ {
		for x := startX; x < startX+modalWidth; x++ {
			screen.SetContent(x, y, ' ', nil, normalStyle)
		}
	}

	// Draw border
	drawBox(screen, startX, startY, modalWidth, modalHeight, borderStyle)

	// Draw title
	title := "Select Terminal Mode"
	titleX := startX + (modalWidth-len(title))/2
	drawString(screen, titleX, startY+1, title, titleStyle)

	// Draw separator
	for x := startX + 1; x < startX+modalWidth-1; x++ {
		screen.SetContent(x, startY+2, '─', nil, borderStyle)
	}

	// Draw tool options
	for i, tool := range ts.AITools {
		y := startY + 3 + i
		style := normalStyle
		if i == ts.SelectedIdx {
			style = selectedStyle
		}

		// Radio button
		radio := "( )"
		if i == ts.SelectedIdx {
			radio = "(*)"
		}

		line := radio + " " + tool.Name
		// Pad to fill width
		for len(line) < modalWidth-4 {
			line += " "
		}

		drawString(screen, startX+2, y, line, style)
	}

	// Draw instructions
	hint := "↑/↓:Navigate  Enter:Select  Esc:Shell"
	hintX := startX + (modalWidth-len(hint))/2
	drawString(screen, hintX, startY+modalHeight-2, hint, hintStyle)
}

// drawBox draws a box at the specified position
func drawBox(screen tcell.Screen, x, y, w, h int, style tcell.Style) {
	// Corners
	screen.SetContent(x, y, '┌', nil, style)
	screen.SetContent(x+w-1, y, '┐', nil, style)
	screen.SetContent(x, y+h-1, '└', nil, style)
	screen.SetContent(x+w-1, y+h-1, '┘', nil, style)

	// Horizontal lines
	for i := x + 1; i < x+w-1; i++ {
		screen.SetContent(i, y, '─', nil, style)
		screen.SetContent(i, y+h-1, '─', nil, style)
	}

	// Vertical lines
	for i := y + 1; i < y+h-1; i++ {
		screen.SetContent(x, i, '│', nil, style)
		screen.SetContent(x+w-1, i, '│', nil, style)
	}
}

// drawString draws a string at the specified position
func drawString(screen tcell.Screen, x, y int, s string, style tcell.Style) {
	for i, r := range s {
		screen.SetContent(x+i, y, r, nil, style)
	}
}
