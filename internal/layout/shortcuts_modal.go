package layout

import (
	"github.com/micro-editor/tcell/v2"
)

// ShortcutsModal displays a keyboard shortcuts help dialog
type ShortcutsModal struct {
	Active  bool
	ScreenW int
	ScreenH int
}

// NewShortcutsModal creates a new shortcuts modal
func NewShortcutsModal() *ShortcutsModal {
	return &ShortcutsModal{}
}

// Show displays the shortcuts modal
func (m *ShortcutsModal) Show(screenW, screenH int) {
	m.Active = true
	m.ScreenW = screenW
	m.ScreenH = screenH
}

// Hide closes the modal
func (m *ShortcutsModal) Hide() {
	m.Active = false
}

// HandleEvent processes keyboard events for the modal
// Returns true if the event was consumed
func (m *ShortcutsModal) HandleEvent(event tcell.Event) bool {
	if !m.Active {
		return false
	}

	ev, ok := event.(*tcell.EventKey)
	if !ok {
		return true // Consume non-key events while modal is active
	}

	// Close on Escape or Ctrl+/
	switch ev.Key() {
	case tcell.KeyEscape:
		m.Hide()
		return true
	case tcell.KeyRune:
		if ev.Rune() == '/' && ev.Modifiers()&tcell.ModCtrl != 0 {
			m.Hide()
			return true
		}
	}

	// Also check for Ctrl+/ which may come as a different key code
	if ev.Key() == tcell.KeyCtrlUnderscore { // Ctrl+/ often sends this
		m.Hide()
		return true
	}

	return true // Consume all events while modal is active
}

// shortcutEntry represents a single shortcut in the help display
type shortcutEntry struct {
	key  string
	desc string
}

// shortcutSection represents a section of shortcuts
type shortcutSection struct {
	title    string
	shortcuts []shortcutEntry
}

// Render draws the shortcuts modal centered on screen
func (m *ShortcutsModal) Render(screen tcell.Screen) {
	if !m.Active {
		return
	}

	// Define all shortcuts organized by section
	sections := []shortcutSection{
		{
			title: "Pane Visibility",
			shortcuts: []shortcutEntry{
				{"Alt+1", "Toggle tree"},
				{"Alt+2", "Toggle editor"},
				{"Alt+3", "Toggle terminal"},
				{"Alt+4", "Toggle terminal 2"},
				{"Alt+5", "Toggle terminal 3"},
			},
		},
		{
			title: "Navigation",
			shortcuts: []shortcutEntry{
				{"Ctrl+Space", "Cycle focus"},
				{"Tab", "Tree to editor"},
				{"Shift+Tab", "Editor to tree"},
			},
		},
		{
			title: "Editor Tabs",
			shortcuts: []shortcutEntry{
				{"Ctrl+]", "Next tab"},
				{"Ctrl+[", "Previous tab"},
				{"Ctrl+W", "Close tab"},
			},
		},
		{
			title: "Files",
			shortcuts: []shortcutEntry{
				{"Ctrl+N", "New file"},
				{"Ctrl+Shift+N", "New folder"},
				{"Ctrl+S", "Save"},
				{"Ctrl+D", "Delete"},
				{"Ctrl+R", "Rename"},
			},
		},
		{
			title: "Application",
			shortcuts: []shortcutEntry{
				{"Ctrl+Q", "Quit"},
				{"Ctrl+/", "This help"},
			},
		},
	}

	// Calculate dialog dimensions
	// Find max shortcut key width and description width
	maxKeyWidth := 0
	maxDescWidth := 0
	totalRows := 0
	for _, section := range sections {
		totalRows++ // Section title
		for _, s := range section.shortcuts {
			if len(s.key) > maxKeyWidth {
				maxKeyWidth = len(s.key)
			}
			if len(s.desc) > maxDescWidth {
				maxDescWidth = len(s.desc)
			}
			totalRows++
		}
		totalRows++ // Empty line after section
	}

	// Add padding
	contentWidth := maxKeyWidth + 3 + maxDescWidth // key + " : " + desc
	boxWidth := contentWidth + 6
	boxHeight := totalRows + 4 // +2 for top/bottom border, +2 for title and separator

	// Minimum/maximum bounds
	if boxWidth < 45 {
		boxWidth = 45
	}
	if boxWidth > m.ScreenW-4 {
		boxWidth = m.ScreenW - 4
	}
	if boxHeight > m.ScreenH-4 {
		boxHeight = m.ScreenH - 4
	}

	// Center the dialog
	startX := (m.ScreenW - boxWidth) / 2
	startY := (m.ScreenH - boxHeight) / 2

	// Styles - all must have explicit fg AND bg to prevent color changes in light mode
	bgColor := tcell.ColorBlack
	borderStyle := tcell.StyleDefault.Foreground(tcell.Color205).Background(bgColor) // Hot pink
	bgStyle := tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(bgColor)
	titleStyle := tcell.StyleDefault.Foreground(tcell.Color205).Background(bgColor).Bold(true)
	sectionStyle := tcell.StyleDefault.Foreground(tcell.Color51).Background(bgColor).Bold(true) // Cyan
	keyStyle := tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(bgColor).Bold(true)
	descStyle := tcell.StyleDefault.Foreground(tcell.ColorGray).Background(bgColor)
	hintStyle := tcell.StyleDefault.Foreground(tcell.Color243).Background(bgColor) // Dim gray

	// Draw background
	for y := startY; y < startY+boxHeight; y++ {
		for x := startX; x < startX+boxWidth; x++ {
			screen.SetContent(x, y, ' ', nil, bgStyle)
		}
	}

	// Draw border (double-line style)
	// Top border
	screen.SetContent(startX, startY, '╔', nil, borderStyle)
	screen.SetContent(startX+boxWidth-1, startY, '╗', nil, borderStyle)
	for x := startX + 1; x < startX+boxWidth-1; x++ {
		screen.SetContent(x, startY, '═', nil, borderStyle)
	}

	// Bottom border
	screen.SetContent(startX, startY+boxHeight-1, '╚', nil, borderStyle)
	screen.SetContent(startX+boxWidth-1, startY+boxHeight-1, '╝', nil, borderStyle)
	for x := startX + 1; x < startX+boxWidth-1; x++ {
		screen.SetContent(x, startY+boxHeight-1, '═', nil, borderStyle)
	}

	// Side borders
	for y := startY + 1; y < startY+boxHeight-1; y++ {
		screen.SetContent(startX, y, '║', nil, borderStyle)
		screen.SetContent(startX+boxWidth-1, y, '║', nil, borderStyle)
	}

	// Draw title (centered, line 1)
	title := "Keyboard Shortcuts"
	titleX := startX + (boxWidth-len(title))/2
	for i, r := range title {
		screen.SetContent(titleX+i, startY+1, r, nil, titleStyle)
	}

	// Draw separator line
	for x := startX + 1; x < startX+boxWidth-1; x++ {
		screen.SetContent(x, startY+2, '─', nil, borderStyle)
	}

	// Draw shortcuts
	currentY := startY + 3
	contentX := startX + 3

	for _, section := range sections {
		// Check if we have room for this section
		if currentY >= startY+boxHeight-2 {
			break
		}

		// Draw section title
		for i, r := range section.title {
			screen.SetContent(contentX+i, currentY, r, nil, sectionStyle)
		}
		currentY++

		// Draw shortcuts in this section
		for _, shortcut := range section.shortcuts {
			if currentY >= startY+boxHeight-2 {
				break
			}

			// Draw key
			for i, r := range shortcut.key {
				screen.SetContent(contentX+i, currentY, r, nil, keyStyle)
			}

			// Draw separator
			sepX := contentX + maxKeyWidth + 1
			screen.SetContent(sepX, currentY, ':', nil, descStyle)

			// Draw description
			descX := sepX + 2
			for i, r := range shortcut.desc {
				screen.SetContent(descX+i, currentY, r, nil, descStyle)
			}

			currentY++
		}

		currentY++ // Empty line after section
	}

	// Draw hint at bottom
	hint := "Press Esc to close"
	hintX := startX + (boxWidth-len(hint))/2
	for i, r := range hint {
		screen.SetContent(hintX+i, startY+boxHeight-2, r, nil, hintStyle)
	}
}
