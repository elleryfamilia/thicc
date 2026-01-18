package dashboard

import (
	"fmt"
	"strings"

	"github.com/ellery/thicc/internal/aiterminal"
	"github.com/ellery/thicc/internal/util"
	"github.com/micro-editor/tcell/v2"
)

// Layout constants
const (
	LeftColumnWidth     = 34
	RightColumnWidth    = 32
	TotalPanelWidth     = 67 // LeftColumnWidth + 1 (divider) + RightColumnWidth
	MenuPanelWidth      = 38 // Legacy, kept for compatibility
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

	// Draw file picker overlay if active
	if d.IsFilePickerActive() {
		d.FilePicker.Render(screen)
	}

	// Draw folder creator overlay if active
	if d.IsFolderCreatorActive() {
		d.FolderCreator.Render(screen)
	}

	// Draw onboarding guide overlay if active
	if d.IsOnboardingGuideActive() {
		d.OnboardingGuide.Render(screen)
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
	// Calculate left column height (menu items + recent projects)
	recentCount := len(d.RecentStore.Projects)
	if recentCount > RecentMaxVisible {
		recentCount = RecentMaxVisible
	}
	leftColumnHeight := 2 + len(d.MenuItems) + 1 // top border + items + 1 for Exit spacing
	if recentCount > 0 {
		leftColumnHeight += 3 + recentCount // section header + separator + items
	}
	leftColumnHeight += 2 // bottom padding + border

	// Calculate right column height (AI tools)
	totalTools := d.totalAIToolItems()
	rightColumnHeight := 4 // header + separator + border padding
	if totalTools > 0 {
		extraRows := 0
		if len(d.InstallTools) > 0 {
			extraRows = 1 // "Not Installed" separator
		}
		rightColumnHeight += totalTools + extraRows
	} else {
		rightColumnHeight += 1 // "No AI tools" message
	}

	// Use the taller column height for both
	panelHeight := leftColumnHeight
	if rightColumnHeight > panelHeight {
		panelHeight = rightColumnHeight
	}
	if panelHeight < MenuPanelMinHeight {
		panelHeight = MenuPanelMinHeight
	}

	// Total width includes left column + 1 char divider + right column
	// The divider replaces the gap between columns
	totalWidth := LeftColumnWidth + 1 + RightColumnWidth
	startX := (d.ScreenW - totalWidth) / 2
	if startX < 2 {
		startX = 2
	}

	panelY := (d.ScreenH - panelHeight) / 2
	if panelY < 2 {
		panelY = 2
	}

	// Set left column region (inside the outer border)
	d.leftColumnRegion = Region{
		X:      startX,
		Y:      panelY,
		Width:  LeftColumnWidth,
		Height: panelHeight,
	}

	// Set right column region (after the divider)
	d.rightColumnRegion = Region{
		X:      startX + LeftColumnWidth + 1, // +1 for the divider
		Y:      panelY,
		Width:  RightColumnWidth,
		Height: panelHeight,
	}

	// menuRegion covers the entire panel (used by logo positioning)
	d.menuRegion = Region{
		X:      startX,
		Y:      panelY,
		Width:  totalWidth,
		Height: panelHeight,
	}
}

// drawMarshmallow renders the ASCII art mascot above the logo
func (d *Dashboard) drawMarshmallow(screen tcell.Screen) {
	// Calculate logo position first
	logoX := d.menuRegion.X + (d.menuRegion.Width-ThiccLogoWidth)/2
	if logoX < 0 {
		logoX = 0
	}
	logoY := d.menuRegion.Y - ThiccLogoHeight - 2
	if logoY < 1 {
		logoY = 1
	}

	// Position character above the logo, centered horizontally with it
	startX := logoX + (ThiccLogoWidth-MarshmallowArtWidth)/2
	startY := logoY - MarshmallowArtHeight - 1 // 1 line spacing from logo

	// Don't draw if screen is too small or would be off-screen
	if startY < 1 || d.ScreenW < ThiccLogoWidth+4 {
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

// drawLogo renders the THICC title banner above the menu
func (d *Dashboard) drawLogo(screen tcell.Screen) {
	// Center logo above menu panel
	logoX := d.menuRegion.X + (d.menuRegion.Width-ThiccLogoWidth)/2
	if logoX < 0 {
		logoX = 0
	}

	logoY := d.menuRegion.Y - ThiccLogoHeight - 2
	if logoY < 1 {
		logoY = 1
	}

	// Check if there's room for the logo (need extra width for ASCII art parentheses)
	totalLogoWidth := ThiccLogoWidth + LeftParenWidth + RightParenWidth + 2 // +2 for spacing
	if logoY < 0 || d.ScreenW < totalLogoWidth+4 {
		// Draw simple text title instead
		title := "(THICC)"
		titleX := d.menuRegion.X + (d.menuRegion.Width-len(title))/2
		d.drawText(screen, titleX, d.menuRegion.Y-2, title, StyleTitle)
		return
	}

	// Draw the logo with two-tone coloring (solid vs shade)
	for lineIdx, line := range ThiccLogo {
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

	// Draw ASCII art parentheses around the logo
	leftParenX := logoX - LeftParenWidth - 1
	rightParenX := logoX + ThiccLogoWidth + 1

	// Draw left parenthesis
	if leftParenX >= 0 {
		for lineIdx, line := range LeftParen {
			y := logoY + lineIdx
			runeIdx := 0
			for _, ch := range line {
				x := leftParenX + runeIdx
				if x >= 0 && x < d.ScreenW {
					style := GetLogoColorForChar(ch)
					screen.SetContent(x, y, ch, nil, style)
				}
				runeIdx++
			}
		}
	}

	// Draw right parenthesis
	if rightParenX+RightParenWidth <= d.ScreenW {
		for lineIdx, line := range RightParen {
			y := logoY + lineIdx
			runeIdx := 0
			for _, ch := range line {
				x := rightParenX + runeIdx
				if x >= 0 && x < d.ScreenW {
					style := GetLogoColorForChar(ch)
					screen.SetContent(x, y, ch, nil, style)
				}
				runeIdx++
			}
		}
	}

	// Draw tagline below logo in cyan
	taglineX := d.menuRegion.X + (d.menuRegion.Width-len(ThiccTagline))/2
	taglineY := logoY + ThiccLogoHeight + 1
	if taglineY < d.menuRegion.Y {
		taglineStyle := tcell.StyleDefault.Foreground(ColorCyan).Background(ColorBgDark).Italic(true)
		d.drawText(screen, taglineX, taglineY, ThiccTagline, taglineStyle)
	}
}

// drawMenuPanel renders the two-column layout as a single box with center divider
func (d *Dashboard) drawMenuPanel(screen tcell.Screen) {
	hasOverlay := d.IsProjectPickerActive() || d.IsFilePickerActive() || d.IsFolderCreatorActive() || d.IsOnboardingGuideActive()
	r := d.menuRegion

	// Draw single outer border
	d.drawBorder(screen, r.X, r.Y, r.Width, r.Height, !hasOverlay)

	// Draw center divider
	dividerX := d.leftColumnRegion.X + d.leftColumnRegion.Width
	d.drawCenterDivider(screen, dividerX, r.Y, r.Height, !hasOverlay)

	// Draw left column content (menu + recent projects)
	d.drawLeftColumnContent(screen)

	// Draw right column content (AI tools)
	d.drawRightColumnContent(screen)
}

// drawCenterDivider draws a vertical divider with T-connectors at top and bottom
func (d *Dashboard) drawCenterDivider(screen tcell.Screen, x, y, height int, focused bool) {
	style := tcell.StyleDefault.Foreground(ColorCyan).Background(ColorBgDark).Bold(true)
	if !focused {
		style = StyleBorderDim
	}

	// Top T-connector
	screen.SetContent(x, y, '╦', nil, style)

	// Vertical line
	for i := y + 1; i < y+height-1; i++ {
		screen.SetContent(x, i, '║', nil, style)
	}

	// Bottom T-connector
	screen.SetContent(x, y+height-1, '╩', nil, style)
}

// drawLeftColumnContent renders the left column content (menu items and recent projects)
func (d *Dashboard) drawLeftColumnContent(screen tcell.Screen) {
	r := d.leftColumnRegion

	// Draw all menu items
	y := r.Y + 2
	for i, item := range d.MenuItems {
		// Add spacing before Exit (destructive action)
		if item.ID == MenuExit {
			y++
		}
		isSelected := d.LeftColumnFocus && !d.InRecentPane && d.SelectedIdx == i
		d.drawMenuItem(screen, r.X+2, y, r.Width-3, item, isSelected)
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
		recentSepStyle := tcell.StyleDefault.Foreground(ColorCyan).Background(ColorBgDark)
		for x := r.X + 2; x < r.X+r.Width-1; x++ {
			screen.SetContent(x, y, '─', nil, recentSepStyle)
		}
		y++

		// Store recent region for click detection
		visibleCount := len(d.RecentStore.Projects)
		if visibleCount > RecentMaxVisible {
			visibleCount = RecentMaxVisible
		}
		d.recentRegion = Region{X: r.X + 2, Y: y, Width: r.Width - 3, Height: visibleCount}

		// Recent project items
		for i := 0; i < visibleCount; i++ {
			proj := d.RecentStore.Projects[i]
			isSelected := d.LeftColumnFocus && d.InRecentPane && d.RecentIdx == i
			d.drawRecentItem(screen, r.X+2, y, r.Width-3, i+1, proj, isSelected)
			y++
		}

		// Show more indicator if needed
		if len(d.RecentStore.Projects) > RecentMaxVisible {
			moreText := fmt.Sprintf("  ... +%d more", len(d.RecentStore.Projects)-RecentMaxVisible)
			d.drawText(screen, r.X+2, y, moreText, StyleVersion)
		}
	}
}

// drawRightColumnContent renders the right column content (AI tools)
func (d *Dashboard) drawRightColumnContent(screen tcell.Screen) {
	r := d.rightColumnRegion

	y := r.Y + 2

	// Section header
	header := "AI Tools"
	headerX := r.X + (r.Width-len(header))/2
	d.drawText(screen, headerX, y, header, StyleSectionHeader)
	y++

	// Separator line (cyan to match border)
	separatorStyle := tcell.StyleDefault.Foreground(ColorCyan).Background(ColorBgDark)
	for x := r.X + 1; x < r.X+r.Width-2; x++ {
		screen.SetContent(x, y, '─', nil, separatorStyle)
	}
	y++

	totalTools := d.totalAIToolItems()
	if totalTools > 0 {
		// Store the start of AI tools region for click detection
		extraRows := 0
		if len(d.InstallTools) > 0 {
			extraRows = 1 // separator line
		}
		d.aiToolsRegion = Region{X: r.X + 1, Y: y, Width: r.Width - 3, Height: totalTools + extraRows}

		// Available AI tool items with radio buttons
		inRightColumn := !d.LeftColumnFocus
		for i, tool := range d.AITools {
			isFocused := inRightColumn && d.AIToolsIdx == i
			isToolSelected := d.IsAIToolSelected(tool.Name)
			d.drawAIToolItem(screen, r.X+1, y, r.Width-3, tool, isFocused, isToolSelected)
			y++
		}

		// Installable tools section
		if len(d.InstallTools) > 0 {
			// Draw "Not Installed" separator
			hintStyle := tcell.StyleDefault.Foreground(ColorViolet).Background(ColorBgDark)
			sepText := "─ Not Installed ─"
			sepX := r.X + (r.Width-len(sepText))/2
			d.drawText(screen, sepX, y, sepText, hintStyle)
			y++

			// Installable tool items
			for i, tool := range d.InstallTools {
				idx := len(d.AITools) + i
				isFocused := inRightColumn && d.AIToolsIdx == idx
				d.drawInstallToolItem(screen, r.X+1, y, r.Width-3, tool, isFocused)
				y++
			}
		}
	} else {
		// No AI tools message
		noToolsMsg := "No AI tools"
		msgX := r.X + (r.Width-len(noToolsMsg))/2
		d.drawText(screen, msgX, y, noToolsMsg, StyleVersion)
	}
}

// drawBorder draws a double-line border around a region
func (d *Dashboard) drawBorder(screen tcell.Screen, x, y, w, h int, focused bool) {
	// Elegant cyan/blue border (comic-book style)
	// All styles must have explicit fg AND bg to prevent color changes in light mode
	style := tcell.StyleDefault.Foreground(ColorCyan).Background(ColorBgDark).Bold(true)
	if !focused {
		style = StyleBorderDim
	}

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

	// Format: "  project-name/                    1" (shortcut on right like menu items)
	suffix := ""
	if proj.IsFolder {
		suffix = "/"
	}

	// Build the line like menu items: prefix + label + padding + shortcut
	prefix := "  "
	if selected {
		prefix = "> "
	}

	name := proj.Name + suffix
	shortcut := fmt.Sprintf("%d", num)

	// Calculate spacing
	labelText := prefix + name
	padding := width - len(labelText) - len(shortcut)
	if padding < 1 {
		padding = 1
	}

	// Truncate name if needed
	if padding < 1 {
		maxNameLen := width - len(prefix) - len(shortcut) - 1
		if maxNameLen > 0 && len(name) > maxNameLen {
			name = name[:maxNameLen-1] + "…"
			labelText = prefix + name
			padding = width - len(labelText) - len(shortcut)
		}
	}

	line := labelText + strings.Repeat(" ", padding) + shortcut

	// Truncate if needed
	if len(line) > width {
		line = line[:width]
	}

	// Draw the line
	for i, ch := range line {
		if x+i < d.ScreenW {
			// Color the shortcut differently when not selected
			charStyle := style
			if !selected && i >= len(line)-len(shortcut) {
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

// drawAIToolItem renders a single AI tool item with radio button
func (d *Dashboard) drawAIToolItem(screen tcell.Screen, x, y, width int, tool aiterminal.AITool, focused bool, selected bool) {
	// Installed tools are colorful (cyan)
	style := tcell.StyleDefault.Foreground(ColorCyan).Background(ColorBgDark)
	if focused {
		style = StyleMenuSelected
	}

	// Radio button character
	radioChar := "( )"
	if selected {
		radioChar = "(*)"
	}

	// Format: "(*) Tool Name"
	name := tool.Name
	// Truncate name if too long
	maxNameLen := width - 5 // "(*) " prefix + space
	if len(name) > maxNameLen {
		name = name[:maxNameLen-1] + "…"
	}

	line := radioChar + " " + name

	// Pad to width
	if len(line) < width {
		line += strings.Repeat(" ", width-len(line))
	}

	// Draw the line
	for i, ch := range line {
		if x+i < d.ScreenW {
			charStyle := style
			// Highlight the radio button when selected
			if selected && i < 3 {
				charStyle = tcell.StyleDefault.Foreground(ColorMagenta).Background(ColorBgDark).Bold(true)
				if focused {
					charStyle = style // Keep the focused highlight
				}
			}
			screen.SetContent(x+i, y, ch, nil, charStyle)
		}
	}
}

// drawInstallToolItem renders a single installable tool item with [Install] suffix
func (d *Dashboard) drawInstallToolItem(screen tcell.Screen, x, y, width int, tool aiterminal.AITool, focused bool) {
	// Check if this install tool is selected
	isSelected := d.SelectedInstallCmd == tool.InstallCommand

	// Not-installed tools are greyed out, but [Install] stays yellow
	greyStyle := tcell.StyleDefault.Foreground(ColorTextDim).Background(ColorBgDark)
	yellowStyle := tcell.StyleDefault.Foreground(tcell.ColorYellow).Background(ColorBgDark)
	focusedStyle := tcell.StyleDefault.Foreground(tcell.ColorBlack).Background(tcell.ColorYellow)

	// Radio button character - filled if selected
	radioChar := "( )"
	if isSelected {
		radioChar = "(*)"
	}

	// Format: "( ) Tool Name (install)"
	suffix := " (install)"
	name := tool.Name
	// Truncate name if too long
	maxNameLen := width - 5 - len(suffix) // "( ) " prefix + suffix
	if len(name) > maxNameLen && maxNameLen > 3 {
		name = name[:maxNameLen-1] + "…"
	}

	nameWithRadio := radioChar + " " + name
	line := nameWithRadio + suffix

	// Pad to width
	if len(line) < width {
		line += strings.Repeat(" ", width-len(line))
	}

	// Draw the line - grey for name, yellow for [Install]
	suffixStart := len(nameWithRadio)
	suffixEnd := suffixStart + len(suffix)

	for i, ch := range line {
		if x+i < d.ScreenW {
			var style tcell.Style
			if focused {
				style = focusedStyle
			} else if i >= suffixStart && i < suffixEnd {
				style = yellowStyle // [Install] in yellow
			} else {
				style = greyStyle // Tool name in grey
			}
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
		{"n", "New File"},
		{"f", "Open File"},
		{"o", "Open Project"},
		{"d", "New Folder"},
		{"q", "Quit"},
		{"?", "Help"},
	}

	// Add recent hint if there are recent projects
	if len(d.RecentStore.Projects) > 0 {
		hints = append(hints[:4], append([]struct{ key, desc string }{{"1-9", "Recent"}}, hints[4:]...)...)
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
	r := d.leftColumnRegion

	// Check if within left column content area (inside border, before divider)
	if x < r.X+2 || x >= r.X+r.Width {
		return -1
	}

	itemY := r.Y + 2

	// Check all menu items (accounting for Exit spacing)
	for i, item := range d.MenuItems {
		// Exit has extra spacing before it
		if item.ID == MenuExit {
			itemY++
		}
		if y == itemY {
			return i
		}
		itemY++
	}

	return -1
}

// GetRecentItemAtPosition returns the recent project index at the given screen position, or -1
func (d *Dashboard) GetRecentItemAtPosition(x, y int) int {
	r := d.leftColumnRegion

	// Check if within left column content area
	if x < r.X+2 || x >= r.X+r.Width {
		return -1
	}

	// Calculate where recent items start (after menu items in left column)
	// +1 for Exit spacing
	recentStartY := r.Y + 2 + len(d.MenuItems) + 1 // menu items + Exit spacing
	recentStartY += 3                              // spacing + header + separator for recent

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

// GetAIToolItemAtPosition returns the AI tool index at the given screen position, or -1
// The returned index covers both available tools (0 to len(AITools)-1) and
// installable tools (len(AITools) to totalAIToolItems()-1)
func (d *Dashboard) GetAIToolItemAtPosition(x, y int) int {
	r := d.rightColumnRegion

	// Check if within right column content area (after divider, inside border)
	if x < r.X+1 || x >= r.X+r.Width-2 {
		return -1
	}

	totalTools := d.totalAIToolItems()
	if totalTools == 0 {
		return -1
	}

	// Calculate where AI tools items start (after header + separator)
	aiToolsStartY := r.Y + 2 + 2 // border offset + header + separator

	// Check available tools
	for i := 0; i < len(d.AITools); i++ {
		if y == aiToolsStartY+i {
			return i
		}
	}

	// Check installable tools (after separator line)
	if len(d.InstallTools) > 0 {
		installStartY := aiToolsStartY + len(d.AITools) + 1 // +1 for "Not Installed" separator
		for i := 0; i < len(d.InstallTools); i++ {
			if y == installStartY+i {
				return len(d.AITools) + i
			}
		}
	}

	return -1
}
