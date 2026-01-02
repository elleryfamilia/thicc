package filebrowser

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/ellery/thicc/internal/config"
	"github.com/ellery/thicc/internal/filemanager"
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

	// Draw border (always, with style based on focus)
	p.drawBorder(screen)
}

// clearRegion clears the panel's screen region
func (p *Panel) clearRegion(screen tcell.Screen) {
	style := GetDefaultStyle()
	for y := 0; y < p.Region.Height; y++ {
		for x := 0; x < p.Region.Width; x++ {
			screen.SetContent(p.Region.X+x, p.Region.Y+y, ' ', nil, style)
		}
	}
}

// drawHeader draws the header showing current directory
func (p *Panel) drawHeader(screen tcell.Screen) {
	// Line 1: Directory name with open folder icon (clickable to change directory)
	dirName := filepath.Base(p.Tree.CurrentDir)
	if dirName == "" || dirName == "." {
		dirName = p.Tree.CurrentDir // Fallback for root paths
	}

	// Add open folder icon prefix (Nerd Font U+F07C)
	displayName := " \uF07C " + dirName

	// Truncate if needed
	maxWidth := p.Region.Width - 2
	if len(displayName) > maxWidth {
		displayName = displayName[:maxWidth-3] + "..."
	}

	// Highlight header if selected (Selected == -1)
	style := GetDirectoryStyle() // Default: bright blue
	if p.Selected == -1 && p.Focus {
		// Fill entire line with selection background first
		for i := 0; i < p.Region.Width; i++ {
			screen.SetContent(p.Region.X+i, p.Region.Y+1, ' ', nil, FocusedStyle)
		}
		style = FocusedStyle // When selected: black bg, white text (same as nodes)
	}

	// Draw the directory name with appropriate style
	p.drawText(screen, 1, 1, displayName, style)

	// Line 2: Separator
	separator := strings.Repeat("─", p.Region.Width-2)
	p.drawText(screen, 1, 2, separator, GetDividerStyle())
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
		p.drawText(screen, 1, startY, "Loading files...", GetDefaultStyle())
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
	style := GetDefaultStyle()
	if isSelected {
		style = selStyle
	}
	x += p.drawText(screen, x, y, indent, style)

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
	nameStyle := GetFileStyle()
	if node.IsDir {
		nameStyle = GetDirectoryStyle()
	}
	if isSelected {
		nameStyle = selStyle
	}

	x += p.drawText(screen, x, y, name, nameStyle)
	_ = x // Silence unused variable warning
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

// GetBorderStyle returns the style for focus borders (pink/magenta for Spider-Verse vibe)
func GetBorderStyle() tcell.Style {
	return config.DefStyle.Foreground(tcell.Color205) // Hot pink
}

// drawBorder draws a border around the panel
// Uses double-line (focused/pink) or single-line (unfocused/gray) to match other panels
func (p *Panel) drawBorder(screen tcell.Screen) {
	var style tcell.Style
	if p.Focus {
		style = GetBorderStyle() // Hot pink
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

// GetStatusLine returns a status line for the panel
func (p *Panel) GetStatusLine() string {
	nodes := p.Tree.GetNodes()
	if len(nodes) == 0 {
		return "No files"
	}

	node := nodes[p.Selected]
	return fmt.Sprintf("%s [%d/%d]", node.Path, p.Selected+1, len(nodes))
}
