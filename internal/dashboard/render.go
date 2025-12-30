package dashboard

import (
	"fmt"
	"strings"

	"github.com/micro-editor/tcell/v2"
	"github.com/ellery/thock/internal/util"
)

// Layout constants
const (
	MenuPanelWidth      = 38
	MenuPanelMinHeight  = 16
	RecentMaxVisible    = 5
	FooterHeight        = 2
	ArtLeftMargin       = 4
	ArtTopMargin        = 3
)

// Render draws the entire dashboard
func (d *Dashboard) Render(screen tcell.Screen) {
	// Clear screen with dark background
	d.clearScreen(screen)

	// Calculate layout positions
	d.calculateLayout()

	// Draw components
	d.drawLogo(screen)
	d.drawMenuPanel(screen)
	d.drawKeyboardHints(screen)
	d.drawVersion(screen)

	// Draw project picker overlay if active
	if d.IsProjectPickerActive() {
		d.ProjectPicker.Render(screen)
	}
}

// clearScreen fills the screen with the background color
func (d *Dashboard) clearScreen(screen tcell.Screen) {
	for y := 0; y < d.ScreenH; y++ {
		for x := 0; x < d.ScreenW; x++ {
			screen.SetContent(x, y, ' ', nil, StyleBackground)
		}
	}
}

// calculateLayout determines positions of major elements
func (d *Dashboard) calculateLayout() {
	// Menu panel is centered horizontally, slightly right of center
	menuX := (d.ScreenW - MenuPanelWidth) / 2
	if menuX < MarshmallowArtWidth+ArtLeftMargin+4 {
		menuX = MarshmallowArtWidth + ArtLeftMargin + 4
	}

	// Calculate menu height based on content
	recentCount := len(d.RecentStore.Projects)
	if recentCount > RecentMaxVisible {
		recentCount = RecentMaxVisible
	}
	menuHeight := 4 + len(d.MenuItems) + 1 // header + items + spacing

	if recentCount > 0 {
		menuHeight += 3 + recentCount // section header + items
	}

	menuHeight += 2 // Exit + bottom border

	if menuHeight < MenuPanelMinHeight {
		menuHeight = MenuPanelMinHeight
	}

	menuY := (d.ScreenH - menuHeight) / 2
	if menuY < 2 {
		menuY = 2
	}

	d.menuRegion = Region{
		X:      menuX,
		Y:      menuY,
		Width:  MenuPanelWidth,
		Height: menuHeight,
	}
}

// drawMarshmallow renders the ASCII art mascot above the logo
func (d *Dashboard) drawMarshmallow(screen tcell.Screen) {
	// Calculate logo position first
	logoX := d.menuRegion.X + (d.menuRegion.Width-ThockLogoWidth)/2
	if logoX < 0 {
		logoX = 0
	}
	logoY := d.menuRegion.Y - ThockLogoHeight - 2
	if logoY < 1 {
		logoY = 1
	}

	// Position character above the logo, centered horizontally with it
	startX := logoX + (ThockLogoWidth-MarshmallowArtWidth)/2
	startY := logoY - MarshmallowArtHeight - 1 // 1 line spacing from logo

	// Don't draw if screen is too small or would be off-screen
	if startY < 1 || d.ScreenW < ThockLogoWidth+4 {
		return
	}

	for lineIdx, line := range MarshmallowArt {
		y := startY + lineIdx
		if y >= d.ScreenH-FooterHeight {
			break
		}

		for charIdx, ch := range line {
			x := startX + charIdx
			if x >= d.ScreenW {
				break
			}

			// Determine style for this character
			style := StyleArtPrimary

			// Check for special color regions
			for _, region := range MarshmallowColors {
				if region.Line == lineIdx && charIdx >= region.StartX && charIdx < region.EndX {
					style = region.Style
					break
				}
			}

			// Only draw non-space characters with color
			if ch != ' ' {
				screen.SetContent(x, y, ch, nil, style)
			}
		}
	}
}

// drawLogo renders the THOCK title banner above the menu
func (d *Dashboard) drawLogo(screen tcell.Screen) {
	// Center logo above menu panel
	logoX := d.menuRegion.X + (d.menuRegion.Width-ThockLogoWidth)/2
	if logoX < 0 {
		logoX = 0
	}

	logoY := d.menuRegion.Y - ThockLogoHeight - 2
	if logoY < 1 {
		logoY = 1
	}

	// Check if there's room for the logo
	if logoY < 0 || d.ScreenW < ThockLogoWidth+4 {
		// Draw simple text title instead
		title := "THOCK"
		titleX := d.menuRegion.X + (d.menuRegion.Width-len(title))/2
		d.drawText(screen, titleX, d.menuRegion.Y-2, title, StyleTitle)
		return
	}

	// Draw the logo with two-tone coloring (solid vs shade)
	for lineIdx, line := range ThockLogo {
		y := logoY + lineIdx
		runeIdx := 0
		for _, ch := range line {
			x := logoX + runeIdx
			if x >= 0 && x < d.ScreenW {
				// Get color based on character type (solid vs shade)
				style := GetLogoColorForChar(ch)
				screen.SetContent(x, y, ch, nil, style)
			}
			runeIdx++
		}
	}

	// Draw tagline below logo in cyan
	taglineX := d.menuRegion.X + (d.menuRegion.Width-len(ThockTagline))/2
	taglineY := logoY + ThockLogoHeight + 1
	if taglineY < d.menuRegion.Y {
		taglineStyle := tcell.StyleDefault.Foreground(ColorCyan).Italic(true)
		d.drawText(screen, taglineX, taglineY, ThockTagline, taglineStyle)
	}
}

// drawMenuPanel renders the bordered menu box with items
func (d *Dashboard) drawMenuPanel(screen tcell.Screen) {
	r := d.menuRegion

	// Draw border
	d.drawBorder(screen, r.X, r.Y, r.Width, r.Height)

	// Draw menu items
	y := r.Y + 2
	for i, item := range d.MenuItems {
		// Skip Exit - we'll draw it at the bottom
		if item.ID == MenuExit {
			continue
		}

		isSelected := !d.InRecentPane && d.SelectedIdx == i
		d.drawMenuItem(screen, r.X+2, y, r.Width-4, item, isSelected)
		y++
	}

	// Draw recent projects section if there are any
	if len(d.RecentStore.Projects) > 0 {
		y++ // spacing

		// Section header
		header := "Recent Projects"
		headerX := r.X + (r.Width-len(header))/2
		d.drawText(screen, headerX, y, header, StyleSectionHeader)
		y++

		// Separator line (cyan to match border)
		separatorStyle := tcell.StyleDefault.Foreground(ColorCyan)
		for x := r.X + 2; x < r.X+r.Width-2; x++ {
			screen.SetContent(x, y, '─', nil, separatorStyle)
		}
		y++

		// Recent project items
		visibleCount := len(d.RecentStore.Projects)
		if visibleCount > RecentMaxVisible {
			visibleCount = RecentMaxVisible
		}

		for i := 0; i < visibleCount; i++ {
			proj := d.RecentStore.Projects[i]
			isSelected := d.InRecentPane && d.RecentIdx == i
			d.drawRecentItem(screen, r.X+2, y, r.Width-4, i+1, proj, isSelected)
			y++
		}

		// Show more indicator if needed
		if len(d.RecentStore.Projects) > RecentMaxVisible {
			moreText := fmt.Sprintf("  ... and %d more", len(d.RecentStore.Projects)-RecentMaxVisible)
			d.drawText(screen, r.X+2, y, moreText, StyleVersion)
			y++
		}
	}

	// Draw Exit at the bottom
	y = r.Y + r.Height - 3
	for _, item := range d.MenuItems {
		if item.ID == MenuExit {
			exitIdx := -1
			for idx, mi := range d.MenuItems {
				if mi.ID == MenuExit {
					exitIdx = idx
					break
				}
			}
			isSelected := !d.InRecentPane && d.SelectedIdx == exitIdx
			d.drawMenuItem(screen, r.X+2, y, r.Width-4, item, isSelected)
			break
		}
	}
}

// drawBorder draws a double-line border around a region
func (d *Dashboard) drawBorder(screen tcell.Screen, x, y, w, h int) {
	// Elegant cyan/blue border (comic-book style)
	style := tcell.StyleDefault.Foreground(ColorCyan).Bold(true)

	// Corners
	screen.SetContent(x, y, '╔', nil, style)
	screen.SetContent(x+w-1, y, '╗', nil, style)
	screen.SetContent(x, y+h-1, '╚', nil, style)
	screen.SetContent(x+w-1, y+h-1, '╝', nil, style)

	// Top and bottom edges
	for i := x + 1; i < x+w-1; i++ {
		screen.SetContent(i, y, '═', nil, style)
		screen.SetContent(i, y+h-1, '═', nil, style)
	}

	// Left and right edges
	for i := y + 1; i < y+h-1; i++ {
		screen.SetContent(x, i, '║', nil, style)
		screen.SetContent(x+w-1, i, '║', nil, style)
	}
}

// drawMenuItem renders a single menu item
func (d *Dashboard) drawMenuItem(screen tcell.Screen, x, y, width int, item MenuItem, selected bool) {
	style := StyleMenuItem
	if selected {
		style = StyleMenuSelected
	}

	// Build the line: "  > Label                    shortcut  "
	prefix := "  "
	if selected {
		prefix = "> "
	}

	// Calculate spacing
	labelText := prefix + item.Label
	shortcutText := item.Shortcut
	padding := width - len(labelText) - len(shortcutText)
	if padding < 1 {
		padding = 1
	}

	line := labelText + strings.Repeat(" ", padding) + shortcutText

	// Truncate if needed
	if len(line) > width {
		line = line[:width]
	}

	// Draw the line
	for i, ch := range line {
		if x+i < d.ScreenW {
			// Color the shortcut differently when not selected
			charStyle := style
			if !selected && i >= len(line)-len(shortcutText) {
				charStyle = StyleShortcut
			}
			screen.SetContent(x+i, y, ch, nil, charStyle)
		}
	}

	// Fill remaining width if selected (for highlight bar)
	if selected {
		for i := len(line); i < width; i++ {
			if x+i < d.ScreenW {
				screen.SetContent(x+i, y, ' ', nil, style)
			}
		}
	}
}

// drawRecentItem renders a single recent project item
func (d *Dashboard) drawRecentItem(screen tcell.Screen, x, y, width int, num int, proj RecentProject, selected bool) {
	style := StyleRecentItem
	if selected {
		style = StyleRecentSelected
	} else if proj.IsFolder {
		style = StyleRecentFolder
	}

	// Format: "1. project-name/"
	suffix := ""
	if proj.IsFolder {
		suffix = "/"
	}

	name := proj.Name + suffix
	// Truncate name if too long
	maxNameLen := width - 4 // "N. " prefix
	if len(name) > maxNameLen {
		name = name[:maxNameLen-1] + "…"
	}

	line := fmt.Sprintf("%d. %s", num, name)

	// Pad to width
	if len(line) < width {
		line += strings.Repeat(" ", width-len(line))
	}

	// Draw the line
	for i, ch := range line {
		if x+i < d.ScreenW {
			screen.SetContent(x+i, y, ch, nil, style)
		}
	}
}

// drawKeyboardHints renders the footer with keyboard shortcuts
func (d *Dashboard) drawKeyboardHints(screen tcell.Screen) {
	y := d.ScreenH - 2

	hints := []struct {
		key  string
		desc string
	}{
		{"n", "New"},
		{"o", "Open"},
		{"q", "Quit"},
		{"↑↓", "Navigate"},
	}

	// Add recent hint if there are recent projects
	if len(d.RecentStore.Projects) > 0 {
		hints = append(hints[:3], struct{ key, desc string }{"1-9", "Recent"}, hints[3])
	}

	// Build hint string
	x := 3
	for _, hint := range hints {
		if x >= d.ScreenW-10 {
			break
		}

		// Draw key
		d.drawText(screen, x, y, "[", StyleFooterHint)
		x++
		d.drawText(screen, x, y, hint.key, StyleFooterKey)
		x += len(hint.key)
		d.drawText(screen, x, y, "]", StyleFooterHint)
		x++

		// Draw description
		d.drawText(screen, x, y, " "+hint.desc, StyleFooterHint)
		x += len(hint.desc) + 1

		// Spacing
		x += 2
	}
}

// drawVersion renders the version info in the bottom right
func (d *Dashboard) drawVersion(screen tcell.Screen) {
	version := "v" + util.Version
	x := d.ScreenW - len(version) - 2
	y := d.ScreenH - 2

	if x > 0 {
		d.drawText(screen, x, y, version, StyleVersion)
	}
}

// drawText is a helper to draw a string at a position
func (d *Dashboard) drawText(screen tcell.Screen, x, y int, text string, style tcell.Style) {
	for i, ch := range text {
		if x+i >= 0 && x+i < d.ScreenW && y >= 0 && y < d.ScreenH {
			screen.SetContent(x+i, y, ch, nil, style)
		}
	}
}

// GetMenuItemAtPosition returns the menu item index at the given screen position, or -1
func (d *Dashboard) GetMenuItemAtPosition(x, y int) int {
	r := d.menuRegion

	// Check if within menu bounds
	if x < r.X+2 || x >= r.X+r.Width-2 {
		return -1
	}

	itemY := r.Y + 2

	// Check main menu items (excluding Exit)
	for i, item := range d.MenuItems {
		if item.ID == MenuExit {
			continue
		}
		if y == itemY {
			return i
		}
		itemY++
	}

	// Check Exit at bottom
	exitY := r.Y + r.Height - 3
	for i, item := range d.MenuItems {
		if item.ID == MenuExit && y == exitY {
			return i
		}
	}

	return -1
}

// GetRecentItemAtPosition returns the recent project index at the given screen position, or -1
func (d *Dashboard) GetRecentItemAtPosition(x, y int) int {
	r := d.menuRegion

	// Check if within bounds
	if x < r.X+2 || x >= r.X+r.Width-2 {
		return -1
	}

	// Calculate where recent items start
	recentStartY := r.Y + 2 + len(d.MenuItems) - 1 + 3 // menu items (minus Exit) + spacing + header + separator

	visibleCount := len(d.RecentStore.Projects)
	if visibleCount > RecentMaxVisible {
		visibleCount = RecentMaxVisible
	}

	for i := 0; i < visibleCount; i++ {
		if y == recentStartY+i {
			return i
		}
	}

	return -1
}
