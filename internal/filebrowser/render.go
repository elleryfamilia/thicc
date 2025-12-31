package filebrowser

import (
	"fmt"
	"strings"

	"github.com/ellery/thock/internal/filemanager"
	"github.com/micro-editor/tcell/v2"
)

// Render draws the file browser to the tcell screen
func (p *Panel) Render(screen tcell.Screen) {
	// Clear the region
	p.clearRegion(screen)

	// Draw header
	p.drawHeader(screen)

	// Draw tree nodes
	p.drawNodes(screen)

	// Draw border if needed
	if p.Focus {
		p.drawFocusBorder(screen)
	}
}

// clearRegion clears the panel's screen region
func (p *Panel) clearRegion(screen tcell.Screen) {
	for y := 0; y < p.Region.Height; y++ {
		for x := 0; x < p.Region.Width; x++ {
			screen.SetContent(p.Region.X+x, p.Region.Y+y, ' ', nil, DefaultStyle)
		}
	}
}

// drawHeader draws the header showing current directory
func (p *Panel) drawHeader(screen tcell.Screen) {
	// Line 1: Current directory with folder icon (line 0 reserved for border)
	dir := p.Tree.CurrentDir

	// Add folder icon prefix (📁 U+1F4C1)
	prefix := "📁 "
	displayPath := prefix + dir

	// Truncate if needed (accounting for icon)
	maxWidth := p.Region.Width - 2
	if len(displayPath) > maxWidth {
		// Show "📁 .../end/of/path" format
		suffix := dir[len(dir)-min(len(dir), maxWidth-len(prefix)-3):]
		displayPath = prefix + "..." + suffix
	}

	// Highlight header if selected (Selected == -1)
	style := DirectoryStyle // Default: bright blue
	if p.Selected == -1 && p.Focus {
		// Fill entire line with selection background first
		for i := 0; i < p.Region.Width; i++ {
			screen.SetContent(p.Region.X+i, p.Region.Y+1, ' ', nil, FocusedStyle)
		}
		style = FocusedStyle // When selected: black bg, white text (same as nodes)
	}

	// Draw the path with appropriate style
	p.drawText(screen, 1, 1, displayPath, style)

	// Line 2: Separator
	separator := strings.Repeat("─", p.Region.Width-2)
	p.drawText(screen, 1, 2, separator, DividerStyle)
}

// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// drawNodes draws the file/folder nodes
func (p *Panel) drawNodes(screen tcell.Screen) {
	nodes := p.GetVisibleNodes()
	startY := 3 // After header (line 1) and separator (line 2)

	// Show loading message if tree isn't ready yet
	if nodes == nil {
		p.drawText(screen, 1, startY, "Loading files...", DefaultStyle)
		return
	}

	for i, node := range nodes {
		y := startY + i
		if y >= p.Region.Height {
			break
		}

		// Check if this is the selected node (always show selection, not just when focused)
		globalIndex := p.TopLine + i
		isSelected := (globalIndex == p.Selected)

		// Render the node
		p.renderNode(screen, y, node, isSelected, p.Focus)
	}
}

// SelectedUnfocusedStyle is the style for selected items when panel is not focused
var SelectedUnfocusedStyle = tcell.StyleDefault.
	Foreground(tcell.ColorWhite).
	Background(tcell.Color236) // Dark gray background

// renderNode renders a single tree node
func (p *Panel) renderNode(screen tcell.Screen, y int, node *filemanager.TreeNode, isSelected bool, panelFocused bool) {
	// Determine selection style
	var selStyle tcell.Style
	if isSelected {
		if panelFocused {
			selStyle = FocusedStyle
		} else {
			selStyle = SelectedUnfocusedStyle
		}
		// Fill entire line with selection background first
		for i := 0; i < p.Region.Width; i++ {
			screen.SetContent(p.Region.X+i, p.Region.Y+y, ' ', nil, selStyle)
		}
	}

	x := 1 // Left padding

	// Indentation (1 space per level to save horizontal space)
	indent := strings.Repeat(" ", node.Indent)
	style := DefaultStyle
	if isSelected {
		style = selStyle
	}
	x += p.drawText(screen, x, y, indent, style)

	// Expansion indicator for directories
	if node.IsDir {
		style = DirectoryStyle
		if isSelected {
			style = selStyle
		}
		if node.Expanded {
			x += p.drawText(screen, x, y, "▼ ", style)
		} else {
			x += p.drawText(screen, x, y, "▶ ", style)
		}
	} else {
		style = DefaultStyle
		if isSelected {
			style = selStyle
		}
		x += p.drawText(screen, x, y, "  ", style)
	}

	// Icon with color based on file type
	icon := filemanager.IconForNode(node)
	iconStyle := StyleForPath(node.Path, node.IsDir)
	if isSelected {
		iconStyle = selStyle
	}
	x += p.drawText(screen, x, y, icon+" ", iconStyle)

	// Name
	name := node.Name
	if node.IsDir {
		name += "/"
	}

	// Truncate if needed
	maxWidth := p.Region.Width - x - 2
	if maxWidth > 0 && len(name) > maxWidth {
		name = name[:maxWidth-3] + "..."
	}

	// Choose style for name
	nameStyle := FileStyle
	if node.IsDir {
		nameStyle = DirectoryStyle
	}
	if isSelected {
		nameStyle = selStyle
	}

	p.drawText(screen, x, y, name, nameStyle)
}

// drawText draws text at the given position and returns the number of characters drawn
func (p *Panel) drawText(screen tcell.Screen, x, y int, text string, style tcell.Style) int {
	screenX := p.Region.X + x
	screenY := p.Region.Y + y

	for i, r := range text {
		if screenX+i >= p.Region.X+p.Region.Width {
			break
		}
		screen.SetContent(screenX+i, screenY, r, nil, style)
	}

	return len(text)
}

// BorderStyle is the style for focus borders (pink/magenta for Spider-Verse vibe)
var BorderStyle = tcell.StyleDefault.Foreground(tcell.Color205).Background(tcell.ColorBlack) // Hot pink

// drawFocusBorder draws a border around the panel when focused
// Uses double-line box characters for thicker, more visible appearance
func (p *Panel) drawFocusBorder(screen tcell.Screen) {
	// Draw vertical lines on left and right (double-line)
	for y := 0; y < p.Region.Height; y++ {
		// Left border
		screen.SetContent(p.Region.X, p.Region.Y+y, '║', nil, BorderStyle)
		// Right border
		screen.SetContent(p.Region.X+p.Region.Width-1, p.Region.Y+y, '║', nil, BorderStyle)
	}

	// Top and bottom borders (double-line)
	for x := 0; x < p.Region.Width; x++ {
		// Top border
		screen.SetContent(p.Region.X+x, p.Region.Y, '═', nil, BorderStyle)
		// Bottom border
		screen.SetContent(p.Region.X+x, p.Region.Y+p.Region.Height-1, '═', nil, BorderStyle)
	}

	// Double-line corners
	screen.SetContent(p.Region.X, p.Region.Y, '╔', nil, BorderStyle)
	screen.SetContent(p.Region.X+p.Region.Width-1, p.Region.Y, '╗', nil, BorderStyle)
	screen.SetContent(p.Region.X, p.Region.Y+p.Region.Height-1, '╚', nil, BorderStyle)
	screen.SetContent(p.Region.X+p.Region.Width-1, p.Region.Y+p.Region.Height-1, '╝', nil, BorderStyle)
}

// GetStatusLine returns a status line for the panel
func (p *Panel) GetStatusLine() string {
	nodes := p.Tree.GetNodes()
	if len(nodes) == 0 {
		return "No files"
	}

	node := nodes[p.Selected]
	return fmt.Sprintf("%s [%d/%d]", node.Path, p.Selected+1, len(nodes))
}
