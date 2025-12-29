package layout

import (
	"path/filepath"

	"github.com/ellery/thock/internal/action"
	"github.com/micro-editor/tcell/v2"
)

// TabBar renders the tab bar showing open files
type TabBar struct {
	Region       Region
	Focused      bool // Is editor focused
	CloseButtonX int  // X position of close button for click detection
	CloseButtonY int  // Y position of close button
}

// TabInfo contains display info for a single tab
type TabInfo struct {
	Name     string // Display name (filename or "Untitled")
	Path     string // Full path (empty for unsaved)
	Modified bool   // Has unsaved changes
	Active   bool   // Is this the current tab
}

// NewTabBar creates a new tab bar
func NewTabBar() *TabBar {
	return &TabBar{}
}

// IsCloseButtonClick checks if the given coordinates are on the close button
func (t *TabBar) IsCloseButtonClick(x, y int) bool {
	return x == t.CloseButtonX && y == t.CloseButtonY
}

// IsInTabBar checks if the given coordinates are within the tab bar region
func (t *TabBar) IsInTabBar(x, y int) bool {
	return x >= t.Region.X && x < t.Region.X+t.Region.Width &&
		y >= t.Region.Y && y <= t.Region.Y+1 // Include separator line
}

// GetCurrentTab extracts tab info from the current BufPane
func (t *TabBar) GetCurrentTab(bp *action.BufPane) TabInfo {
	name := bp.Buf.GetName()
	if name == "" {
		name = "Untitled"
	} else {
		name = filepath.Base(name)
	}
	return TabInfo{
		Name:     name,
		Path:     bp.Buf.AbsPath,
		Modified: bp.Buf.Modified(),
		Active:   true,
	}
}

// Render draws the tab bar
func (t *TabBar) Render(screen tcell.Screen, tab TabInfo) {
	// Background style (black to match tree and terminal)
	bgStyle := tcell.StyleDefault.Background(tcell.ColorBlack)

	// Fill background
	for x := t.Region.X; x < t.Region.X+t.Region.Width; x++ {
		screen.SetContent(x, t.Region.Y, ' ', nil, bgStyle)
	}

	// Tab styling based on focus
	var tabStyle tcell.Style
	if t.Focused {
		tabStyle = tcell.StyleDefault.
			Background(tcell.Color205). // Hot pink background
			Foreground(tcell.ColorBlack)
	} else {
		tabStyle = tcell.StyleDefault.
			Background(tcell.Color240). // Dimmed gray
			Foreground(tcell.ColorWhite)
	}

	// Build tab text: " ● filename.go X " (modified dot at beginning, X at end)
	tabText := " "
	if tab.Modified {
		tabText += "● "
	}
	tabText += tab.Name + " X "

	// Draw tab (including the X as part of the tab styling)
	// Use rune slice to handle multi-byte characters correctly
	runes := []rune(tabText)
	startX := t.Region.X + 1
	for i, r := range runes {
		screenX := startX + i
		if screenX >= t.Region.X+t.Region.Width {
			break
		}
		screen.SetContent(screenX, t.Region.Y, r, nil, tabStyle)
	}

	// Track close button position (the X character, second to last rune)
	t.CloseButtonX = startX + len(runes) - 2 // Position of 'X' (before the trailing space)
	t.CloseButtonY = t.Region.Y

	// Draw separator line below the tab bar (neutral gray, black background to match)
	separatorY := t.Region.Y + 1
	separatorStyle := tcell.StyleDefault.
		Foreground(tcell.Color243). // Neutral gray
		Background(tcell.ColorBlack)
	for x := t.Region.X; x < t.Region.X+t.Region.Width; x++ {
		screen.SetContent(x, separatorY, '─', nil, separatorStyle)
	}
}
