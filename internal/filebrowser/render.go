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

// drawHeader draws the header showing current directory with powerline style
func (p *Panel) drawHeader(screen tcell.Screen) {
	// Powerline constants
	const powerlineRound = '\ue0b4' // Rounded cap for badge/pill shape
	const folderIcon = "\uF07C"
	colorNavy := tcell.Color24    // Navy background
	colorWhite := tcell.Color231  // White text

	// Line 1: Directory name with powerline style (matching terminal prompt)
	dirName := filepath.Base(p.Tree.CurrentDir)
	if dirName == "" || dirName == "." {
		dirName = p.Tree.CurrentDir // Fallback for root paths
	}

	// Build the folder segment content
	folderContent := fmt.Sprintf(" %s %s ", folderIcon, dirName)

	// Calculate width needed
	contentWidth := len(folderContent)
	maxWidth := p.Region.Width - 4 // Leave room for border and cap

	// Truncate if needed
	if contentWidth > maxWidth {
		truncLen := maxWidth - 4 // Room for icon, spaces, and ellipsis
		if truncLen > 0 {
			folderContent = fmt.Sprintf(" %s %s... ", folderIcon, dirName[:truncLen])
			contentWidth = len(folderContent)
		}
	}

	// Determine styles based on selection state
	var bgColor tcell.Color
	var fgColor tcell.Color
	if p.Selected == -1 && p.Focus {
		// Selected: white background, black text
		bgColor = tcell.ColorWhite
		fgColor = tcell.ColorBlack
	} else {
		// Normal: navy background, white text (matching terminal)
		bgColor = colorNavy
		fgColor = colorWhite
	}

	// Draw the folder segment with background
	segmentStyle := config.DefStyle.Foreground(fgColor).Background(bgColor).Bold(true)
	x := 1
	// Fill background first
	for i := 0; i < contentWidth && x+i < p.Region.Width-2; i++ {
		screen.SetContent(p.Region.X+x+i, p.Region.Y+1, ' ', nil, segmentStyle)
	}
	// Draw text over background (handles multi-byte runes correctly)
	p.drawText(screen, x, 1, folderContent, segmentStyle)
	x += contentWidth

	// Draw rounded powerline cap (foreground = segment bg, background = default)
	capStyle := config.DefStyle.Foreground(bgColor)
	screen.SetContent(p.Region.X+x, p.Region.Y+1, powerlineRound, nil, capStyle)

	// Line 2: Separator
	separator := strings.Repeat("─", p.Region.Width-4)
	p.drawText(screen, 3, 2, separator, GetDividerStyle())
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
		p.drawText(screen, 3, startY, "Loading files...", GetDefaultStyle())
		return
	}

	for i, node := range nodes {
		y := startY + i
		if y >= p.Region.Height-1 { // Stop before bottom border
			break
		}

		// Check if this is the selected node (always show selection, not just when focused)
		globalIndex := p.TopLine + i
		isSelected := (globalIndex == p.Selected)

		// Render the node
		p.renderNode(screen, y, node, isSelected, p.Focus)
	}
}

// GetSelectedUnfocusedStyle returns the style for selected items when panel is not focused
// Uses config.DefStyle to ensure correct background with dark theme
func GetSelectedUnfocusedStyle() tcell.Style {
	return config.DefStyle.
		Foreground(tcell.ColorWhite).
		Background(tcell.Color236) // Dark gray background
}

// renderNode renders a single tree node
func (p *Panel) renderNode(screen tcell.Screen, y int, node *filemanager.TreeNode, isSelected bool, panelFocused bool) {
	// Determine selection style
	var selStyle tcell.Style
	if isSelected {
		if panelFocused {
			selStyle = GetFocusedStyle()
		} else {
			selStyle = GetSelectedUnfocusedStyle()
		}
		// Fill entire line with selection background first
		for i := 0; i < p.Region.Width; i++ {
			screen.SetContent(p.Region.X+i, p.Region.Y+y, ' ', nil, selStyle)
		}
	}

	x := 3 // Left padding (3 chars from border)

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

	count := 0
	for _, r := range text {
		if screenX+count >= p.Region.X+p.Region.Width {
			break
		}
		screen.SetContent(screenX+count, screenY, r, nil, style)
		count++
	}

	return count
}

// GetBorderStyle returns the style for focus borders (pink/magenta for Spider-Verse vibe)
func GetBorderStyle() tcell.Style {
	return config.DefStyle.Foreground(tcell.Color205) // Hot pink
}

// drawBorder draws a border around the panel
// Focused: double-line pink border
// Unfocused: single-line violet border
func (p *Panel) drawBorder(screen tcell.Screen) {
	pinkStyle := GetBorderStyle()                                              // Hot pink
	violetStyle := config.DefStyle.Foreground(tcell.NewRGBColor(100, 40, 140)) // Darker violet

	if p.Focus {
		// Double-line border in pink
		for y := 1; y < p.Region.Height-1; y++ {
			screen.SetContent(p.Region.X, p.Region.Y+y, '║', nil, pinkStyle)
			screen.SetContent(p.Region.X+p.Region.Width-1, p.Region.Y+y, '║', nil, pinkStyle)
		}
		for x := 1; x < p.Region.Width-1; x++ {
			screen.SetContent(p.Region.X+x, p.Region.Y, '═', nil, pinkStyle)
			screen.SetContent(p.Region.X+x, p.Region.Y+p.Region.Height-1, '═', nil, pinkStyle)
		}
		screen.SetContent(p.Region.X, p.Region.Y, '╔', nil, pinkStyle)
		screen.SetContent(p.Region.X+p.Region.Width-1, p.Region.Y, '╗', nil, pinkStyle)
		screen.SetContent(p.Region.X, p.Region.Y+p.Region.Height-1, '╚', nil, pinkStyle)
		screen.SetContent(p.Region.X+p.Region.Width-1, p.Region.Y+p.Region.Height-1, '╝', nil, pinkStyle)
	} else {
		// Single-line border in violet
		for y := 1; y < p.Region.Height-1; y++ {
			screen.SetContent(p.Region.X, p.Region.Y+y, '│', nil, violetStyle)
			screen.SetContent(p.Region.X+p.Region.Width-1, p.Region.Y+y, '│', nil, violetStyle)
		}
		for x := 1; x < p.Region.Width-1; x++ {
			screen.SetContent(p.Region.X+x, p.Region.Y, '─', nil, violetStyle)
			screen.SetContent(p.Region.X+x, p.Region.Y+p.Region.Height-1, '─', nil, violetStyle)
		}
		screen.SetContent(p.Region.X, p.Region.Y, '┌', nil, violetStyle)
		screen.SetContent(p.Region.X+p.Region.Width-1, p.Region.Y, '┐', nil, violetStyle)
		screen.SetContent(p.Region.X, p.Region.Y+p.Region.Height-1, '└', nil, violetStyle)
		screen.SetContent(p.Region.X+p.Region.Width-1, p.Region.Y+p.Region.Height-1, '┘', nil, violetStyle)
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
