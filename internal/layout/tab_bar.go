package layout

import (
	"path/filepath"

	"github.com/ellery/thock/internal/action"
	"github.com/ellery/thock/internal/buffer"
	"github.com/ellery/thock/internal/filemanager"
	"github.com/micro-editor/tcell/v2"
)

// OpenTab represents an open tab with its buffer
type OpenTab struct {
	Buffer *buffer.Buffer
	Name   string // Cached display name (filename or "Untitled")
	Path   string // Full path for dedup check
	Loaded bool   // true if Buffer is loaded, false for stub/lazy tabs
}

// tabPosition tracks where a tab is rendered for click detection
type tabPosition struct {
	StartX       int
	EndX         int
	CloseButtonX int
}

// TabBar renders the tab bar showing open files
type TabBar struct {
	Region       Region
	Focused      bool // Is editor focused
	CloseButtonX int  // X position of close button for click detection (active tab only)
	CloseButtonY int  // Y position of close button

	// Multi-tab support
	Tabs         []OpenTab     // All open tabs
	ActiveIndex  int           // Currently active tab
	ScrollOffset int           // First visible tab (for overflow)
	tabPositions []tabPosition // Positions of rendered tabs for click detection

	// Overflow indicator positions for click detection
	leftOverflowX  int  // X position of left overflow indicator (‹), -1 if not shown
	rightOverflowX int  // X position of right overflow indicator (›), -1 if not shown
	hasLeftOverflow  bool
	hasRightOverflow bool

	// Track visible tab count for scroll calculation
	lastVisibleTabCount int // How many tabs were visible in last render
}

// TabInfo contains display info for a single tab (used for rendering)
type TabInfo struct {
	Name     string // Display name (filename or "Untitled")
	Path     string // Full path (empty for unsaved)
	Modified bool   // Has unsaved changes
	Active   bool   // Is this the current tab
}

// MaxTabNameLen is the maximum characters to show for a tab name
const MaxTabNameLen = 20

// NewTabBar creates a new tab bar
func NewTabBar() *TabBar {
	return &TabBar{
		Tabs:        []OpenTab{},
		ActiveIndex: 0,
	}
}

// truncateName shortens a name to MaxTabNameLen with ellipsis if needed
func truncateName(name string) string {
	runes := []rune(name)
	if len(runes) <= MaxTabNameLen {
		return name
	}
	return string(runes[:MaxTabNameLen-1]) + "…"
}

// AddTab adds a loaded buffer to the tab list and returns its index
func (t *TabBar) AddTab(buf *buffer.Buffer) int {
	name := buf.GetName()
	if name == "" {
		name = "Untitled"
	} else {
		name = truncateName(filepath.Base(name))
	}

	tab := OpenTab{
		Buffer: buf,
		Name:   name,
		Path:   buf.AbsPath,
		Loaded: true,
	}
	t.Tabs = append(t.Tabs, tab)
	t.ActiveIndex = len(t.Tabs) - 1
	// Note: ensureActiveVisible is called during Render, not here
	// This avoids issues with stale/missing region info
	return t.ActiveIndex
}

// AddTabStub creates a stub tab without loading the buffer (for lazy loading)
// The buffer will be loaded when the tab becomes active
func (t *TabBar) AddTabStub(path string) int {
	name := filepath.Base(path)
	if name == "" || name == "." {
		name = "Untitled"
	} else {
		name = truncateName(name)
	}

	tab := OpenTab{
		Buffer: nil,
		Name:   name,
		Path:   path,
		Loaded: false,
	}
	t.Tabs = append(t.Tabs, tab)
	t.ActiveIndex = len(t.Tabs) - 1
	// Note: ensureActiveVisible is called during Render, not here
	// This avoids issues with stale/missing region info
	return t.ActiveIndex
}

// FindTabByPath finds an existing tab by path, returns -1 if not found
func (t *TabBar) FindTabByPath(path string) int {
	if path == "" {
		return -1 // Don't match empty paths (new/untitled buffers)
	}
	for i, tab := range t.Tabs {
		if tab.Path == path {
			return i
		}
	}
	return -1
}

// CloseTab removes a tab from the list
func (t *TabBar) CloseTab(index int) {
	if index < 0 || index >= len(t.Tabs) {
		return
	}
	t.Tabs = append(t.Tabs[:index], t.Tabs[index+1:]...)

	// Adjust active index if needed
	if len(t.Tabs) == 0 {
		t.ActiveIndex = 0
	} else if t.ActiveIndex >= len(t.Tabs) {
		t.ActiveIndex = len(t.Tabs) - 1
	} else if index < t.ActiveIndex {
		t.ActiveIndex--
	}
}

// GetActiveTab returns the currently active tab, or nil if no tabs
func (t *TabBar) GetActiveTab() *OpenTab {
	if len(t.Tabs) == 0 || t.ActiveIndex < 0 || t.ActiveIndex >= len(t.Tabs) {
		return nil
	}
	return &t.Tabs[t.ActiveIndex]
}

// UpdateTabName updates the cached name for a tab (call after save)
func (t *TabBar) UpdateTabName(index int, buf *buffer.Buffer) {
	if index < 0 || index >= len(t.Tabs) {
		return
	}
	name := buf.GetName()
	if name == "" {
		name = "Untitled"
	} else {
		name = truncateName(filepath.Base(name))
	}
	t.Tabs[index].Name = name
	t.Tabs[index].Path = buf.AbsPath
}

// MarkTabLoaded updates a stub tab with its loaded buffer
func (t *TabBar) MarkTabLoaded(index int, buf *buffer.Buffer) {
	if index < 0 || index >= len(t.Tabs) {
		return
	}
	t.Tabs[index].Buffer = buf
	t.Tabs[index].Loaded = true
	// Update name from buffer (might have better info)
	name := buf.GetName()
	if name != "" {
		t.Tabs[index].Name = truncateName(filepath.Base(name))
	}
	t.Tabs[index].Path = buf.AbsPath
}

// IsCloseButtonClick checks if the given coordinates are on a close button
// Returns the tab index if on a close button, -1 otherwise
func (t *TabBar) IsCloseButtonClick(x, y int) int {
	if y != t.CloseButtonY {
		return -1
	}
	for i, pos := range t.tabPositions {
		if x == pos.CloseButtonX {
			// Convert visible index to actual tab index
			return t.ScrollOffset + i
		}
	}
	return -1
}

// GetClickedTab returns the tab index at the given position, or -1
func (t *TabBar) GetClickedTab(x, y int) int {
	if y != t.Region.Y {
		return -1
	}
	for i, pos := range t.tabPositions {
		if x >= pos.StartX && x < pos.EndX {
			return t.ScrollOffset + i
		}
	}
	return -1
}

// IsInTabBar checks if the given coordinates are within the tab bar region
func (t *TabBar) IsInTabBar(x, y int) bool {
	return x >= t.Region.X && x < t.Region.X+t.Region.Width &&
		y >= t.Region.Y && y <= t.Region.Y+1 // Include separator line
}

// IsLeftOverflowClick checks if click is on the left overflow indicator (‹)
func (t *TabBar) IsLeftOverflowClick(x, y int) bool {
	return t.hasLeftOverflow && y == t.Region.Y && x == t.leftOverflowX
}

// IsRightOverflowClick checks if click is on the right overflow indicator (›)
func (t *TabBar) IsRightOverflowClick(x, y int) bool {
	return t.hasRightOverflow && y == t.Region.Y && x == t.rightOverflowX
}

// ScrollLeft scrolls the tab bar one tab to the left (shows earlier tabs)
func (t *TabBar) ScrollLeft() {
	if t.ScrollOffset > 0 {
		t.ScrollOffset--
	}
}

// ScrollRight scrolls the tab bar one tab to the right (shows later tabs)
func (t *TabBar) ScrollRight() {
	if t.ScrollOffset < len(t.Tabs)-1 {
		t.ScrollOffset++
	}
}

// GetCurrentTab extracts tab info from the current BufPane (legacy, for compatibility)
func (t *TabBar) GetCurrentTab(bp *action.BufPane) TabInfo {
	name := bp.Buf.GetName()
	if name == "" {
		name = "Untitled"
	} else {
		name = truncateName(filepath.Base(name))
	}
	return TabInfo{
		Name:     name,
		Path:     bp.Buf.AbsPath,
		Modified: bp.Buf.Modified(),
		Active:   true,
	}
}

// Render draws the tab bar with all open tabs
func (t *TabBar) Render(screen tcell.Screen) {
	// Safety check: Region must have reasonable width
	if t.Region.Width < 10 {
		return
	}

	// Background style (black to match tree and terminal)
	bgStyle := tcell.StyleDefault.Background(tcell.ColorBlack)

	// Fill background
	for x := t.Region.X; x < t.Region.X+t.Region.Width; x++ {
		screen.SetContent(x, t.Region.Y, ' ', nil, bgStyle)
	}

	// Styles
	activeStyle := tcell.StyleDefault.
		Background(tcell.Color205). // Hot pink background
		Foreground(tcell.ColorBlack)
	inactiveStyle := tcell.StyleDefault.
		Background(tcell.Color240). // Dimmed gray
		Foreground(tcell.ColorWhite)
	overflowStyle := tcell.StyleDefault.
		Background(tcell.ColorBlack).
		Foreground(tcell.Color243) // Neutral gray arrows

	// If no tabs, we're done
	if len(t.Tabs) == 0 {
		t.tabPositions = nil
		t.lastVisibleTabCount = 0
		t.drawSeparator(screen)
		return
	}

	// Ensure active tab is FULLY visible BEFORE rendering (uses actual width calculations)
	t.ensureActiveVisible()

	// Clear tab positions and overflow state
	t.tabPositions = nil
	t.CloseButtonY = t.Region.Y
	t.leftOverflowX = -1
	t.rightOverflowX = -1
	t.hasLeftOverflow = false
	t.hasRightOverflow = false

	// Calculate starting position
	startX := t.Region.X + 1

	// Calculate edges
	leftEdge := t.Region.X + 1  // 1 char margin
	rightEdge := t.Region.X + t.Region.Width - 2 // Leave space for potential overflow indicator

	// Check if we need left overflow indicator
	t.hasLeftOverflow = t.ScrollOffset > 0

	// Calculate starting position for the first fully visible tab
	x := startX

	// If there's left overflow, try to show a partial tab on the left
	if t.hasLeftOverflow && t.ScrollOffset > 0 {
		// Draw overflow indicator
		t.leftOverflowX = startX
		screen.SetContent(startX, t.Region.Y, '‹', nil, overflowStyle)

		// Calculate where the partial tab would start (before the visible area)
		partialTabIdx := t.ScrollOffset - 1
		partialTab := t.Tabs[partialTabIdx]
		partialTabWidth := t.calcTabWidth(partialTab, partialTabIdx == t.ActiveIndex)

		// The partial tab starts such that it ends right after the overflow indicator
		// We want to show the rightmost portion of this tab
		partialStartX := startX + 2 - partialTabWidth // This will be negative or small

		// Only render partial tab if some of it would be visible
		if partialStartX+partialTabWidth > startX+2 {
			partialStyle := inactiveStyle
			if partialTabIdx == t.ActiveIndex {
				partialStyle = activeStyle
			}
			endX := t.renderSingleTab(screen, partialStartX, partialTab, partialTabIdx == t.ActiveIndex, partialStyle)
			x = endX + 1
		} else {
			x = startX + 2
		}
	}

	// Calculate which tabs to render
	visibleTabs := 0

	for i := t.ScrollOffset; i < len(t.Tabs); i++ {
		tab := t.Tabs[i]
		tabWidth := t.calcTabWidth(tab, i == t.ActiveIndex)

		// If tab starts beyond the edge, we're done
		if x >= rightEdge {
			t.hasRightOverflow = true
			break
		}

		// Get tab style
		style := inactiveStyle
		if !t.Focused {
			style = inactiveStyle
		} else if i == t.ActiveIndex {
			style = activeStyle
		}

		// Check if this tab would be clipped
		wouldBeClipped := x+tabWidth > rightEdge

		// ACTIVE TAB MUST NEVER BE CLIPPED
		// If active tab would be clipped, don't render it (shouldn't happen with proper ensureActiveVisible)
		if i == t.ActiveIndex && wouldBeClipped {
			// This is a safety check - ensureActiveVisible should prevent this
			// If we get here, there's a bug, but we shouldn't render a clipped active tab
			t.hasRightOverflow = true
			break
		}

		// Draw the tab (may be partially clipped at edge, which is OK for non-active tabs only)
		endX := t.renderSingleTab(screen, x, tab, i == t.ActiveIndex, style)

		// Track position for click detection
		// With format "[x] ", the 'x' is at endX - 3 (positions: [, x, ], space)
		// Set closeX to -1 if close button is hidden or clipped
		closeX := -1
		if t.shouldShowCloseButton(tab) && endX-3 < rightEdge && endX-3 >= leftEdge {
			closeX = endX - 3
		}
		t.tabPositions = append(t.tabPositions, tabPosition{
			StartX:       x,
			EndX:         endX,
			CloseButtonX: closeX,
		})

		// For non-active tabs: if clipped, mark overflow and stop
		if wouldBeClipped {
			if i < len(t.Tabs)-1 {
				t.hasRightOverflow = true
			}
			break // Don't try to render more tabs after a clipped one
		}

		x = endX + 1 // 1 char spacing between tabs
		visibleTabs++
	}

	// Draw right overflow indicator if needed
	if t.hasRightOverflow {
		t.rightOverflowX = t.Region.X + t.Region.Width - 2
		screen.SetContent(t.rightOverflowX, t.Region.Y, '›', nil, overflowStyle)
	}

	// Store visible tab count for next ensureActiveVisible() call
	t.lastVisibleTabCount = visibleTabs

	// Draw separator line
	t.drawSeparator(screen)
}

// shouldShowCloseButton returns true if a tab should show [x] close button
// Hide [x] only for a single Untitled tab (can't close it)
func (t *TabBar) shouldShowCloseButton(tab OpenTab) bool {
	if len(t.Tabs) == 1 && tab.Name == "Untitled" {
		return false
	}
	return true
}

// calcTabWidth calculates the width of a tab
func (t *TabBar) calcTabWidth(tab OpenTab, isActive bool) int {
	// Format: " icon ● name [x] " or " icon name [x] " (no [x] for single Untitled tab)
	width := 1 // leading space
	width += 2 // icon + space (icon is 1 cell wide in Nerd Fonts)
	if tab.Buffer != nil && tab.Buffer.Modified() {
		width += 2 // "● "
	}
	width += len([]rune(tab.Name)) // name (use rune count for unicode)
	if t.shouldShowCloseButton(tab) {
		width += 5 // " [x] "
	} else {
		width += 1 // just trailing space
	}
	return width
}

// renderSingleTab draws a single tab and returns the ending X position
// leftEdge is the minimum X position where drawing is allowed (for left clipping)
func (t *TabBar) renderSingleTab(screen tcell.Screen, startX int, tab OpenTab, isActive bool, style tcell.Style) int {
	leftEdge := t.Region.X
	rightEdge := t.Region.X + t.Region.Width

	// Get file icon based on path/name
	icon := filemanager.IconForPath(tab.Path, false)
	if tab.Name == "Untitled" {
		icon = filemanager.DefaultFileIcon
	}

	// Build tab text without close button: " icon ● name " or " icon name "
	text := " " + icon + " "
	if tab.Buffer != nil && tab.Buffer.Modified() {
		text += "● "
	}
	text += tab.Name + " "

	// Draw the main text with tab style
	runes := []rune(text)
	x := startX
	for _, r := range runes {
		if x >= rightEdge {
			break
		}
		// Only draw if we're past the left edge
		if x >= leftEdge {
			screen.SetContent(x, t.Region.Y, r, nil, style)
		}
		x++
	}

	// Show close button unless it's a single Untitled tab
	if t.shouldShowCloseButton(tab) {
		// Draw [x] close button with white foreground on same background
		closeStyle := style.Foreground(tcell.ColorWhite)
		closeText := "[x] "
		for _, r := range closeText {
			if x >= rightEdge {
				break
			}
			// Only draw if we're past the left edge
			if x >= leftEdge {
				screen.SetContent(x, t.Region.Y, r, nil, closeStyle)
			}
			x++
		}
	}

	return x
}

// drawSeparator draws the separator line below the tab bar
func (t *TabBar) drawSeparator(screen tcell.Screen) {
	separatorY := t.Region.Y + 1
	separatorStyle := tcell.StyleDefault.
		Foreground(tcell.Color243). // Neutral gray
		Background(tcell.ColorBlack)
	for x := t.Region.X; x < t.Region.X+t.Region.Width; x++ {
		screen.SetContent(x, separatorY, '─', nil, separatorStyle)
	}
}

// ensureActiveVisible adjusts scroll offset so active tab is FULLY visible
// Uses actual tab width calculations instead of estimates
func (t *TabBar) ensureActiveVisible() {
	if len(t.Tabs) == 0 {
		t.ScrollOffset = 0
		return
	}

	// Bounds check ActiveIndex
	if t.ActiveIndex < 0 {
		t.ActiveIndex = 0
	}
	if t.ActiveIndex >= len(t.Tabs) {
		t.ActiveIndex = len(t.Tabs) - 1
	}

	// If active tab is before scroll offset, scroll left to show it
	if t.ActiveIndex < t.ScrollOffset {
		t.ScrollOffset = t.ActiveIndex
		return
	}

	// Calculate available width for tabs
	// Account for: left margin (1), right margin (2 for overflow indicator)
	// If we'll have left overflow, also reserve 2 chars for left indicator
	availableWidth := t.Region.Width - 3 // 1 left margin + 2 right margin

	// Start with the active tab - it MUST fit
	activeTabWidth := t.calcTabWidth(t.Tabs[t.ActiveIndex], true)
	usedWidth := activeTabWidth

	// Work backwards from active tab to find optimal ScrollOffset
	// Try to show as many tabs to the left of active as possible
	targetScrollOffset := t.ActiveIndex

	for i := t.ActiveIndex - 1; i >= 0; i-- {
		tabWidth := t.calcTabWidth(t.Tabs[i], false)
		spacing := 1 // space between tabs

		// If this would be the first visible tab (i == targetScrollOffset - 1 after adding)
		// and there are more tabs before it, we need left overflow indicator
		needsLeftOverflow := i > 0
		leftOverflowSpace := 0
		if needsLeftOverflow && targetScrollOffset == t.ActiveIndex {
			// First time we're adding a tab to the left, check if we need overflow space
			leftOverflowSpace = 2
		}

		if usedWidth+tabWidth+spacing+leftOverflowSpace <= availableWidth {
			usedWidth += tabWidth + spacing
			targetScrollOffset = i
		} else {
			break
		}
	}

	// Update scroll offset - only scroll right if needed (never left unnecessarily)
	if targetScrollOffset > t.ScrollOffset {
		t.ScrollOffset = targetScrollOffset
	}
}
