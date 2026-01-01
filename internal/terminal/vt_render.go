package terminal

import (
	"reflect"

	"github.com/ellery/thock/internal/config"
	"github.com/hinshun/vt10x"
	"github.com/micro-editor/tcell/v2"
)

// GetBorderStyle returns the style for focus borders (pink for Spider-Verse vibe)
func GetBorderStyle() tcell.Style {
	return config.DefStyle.Foreground(tcell.Color205) // Hot pink
}

// Render draws the terminal to the tcell screen
func (p *Panel) Render(screen tcell.Screen) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Only clear border areas (content will be overwritten by VT cells)
	clearStyle := config.DefStyle

	// Top and bottom borders
	for x := 0; x < p.Region.Width; x++ {
		screen.SetContent(p.Region.X+x, p.Region.Y, ' ', nil, clearStyle)
		screen.SetContent(p.Region.X+x, p.Region.Y+p.Region.Height-1, ' ', nil, clearStyle)
	}

	// Left and right borders (skip corners already done)
	for y := 1; y < p.Region.Height-1; y++ {
		screen.SetContent(p.Region.X, p.Region.Y+y, ' ', nil, clearStyle)
		screen.SetContent(p.Region.X+p.Region.Width-1, p.Region.Y+y, ' ', nil, clearStyle)
	}

	// Content area is inside the border (1 cell margin on each side)
	contentX := p.Region.X + 1
	contentY := p.Region.Y + 1
	contentW := p.Region.Width - 2
	contentH := p.Region.Height - 2

	// Get terminal size from VT emulator
	cols, rows := p.VT.Size()

	// Check if we're in alternate screen mode
	mode := p.VT.Mode()
	useAltScreen := mode&vt10x.ModeAltScreen != 0

	// Render each cell from the active buffer (in content area)
	for y := 0; y < contentH && y < rows; y++ {
		for x := 0; x < contentW && x < cols; x++ {
			// Get cell from active buffer
			var glyph vt10x.Glyph

			if useAltScreen {
				glyph = p.getAltCell(x, y)
			} else {
				glyph = p.VT.Cell(x, y)
			}

			// Convert VT cell to tcell
			r := glyph.Char
			if r == 0 {
				r = ' ' // Empty cells render as space
			}

			style := glyphToTcellStyle(glyph)

			// Apply selection highlight (reverse video)
			if p.isSelected(x, y) {
				style = style.Reverse(true)
			}

			// Draw to screen (in content area)
			screen.SetContent(
				contentX+x,
				contentY+y,
				r,
				nil,
				style,
			)
		}
	}

	// Show cursor if focused (offset by content area)
	if p.Focus && p.VT.CursorVisible() {
		cursor := p.VT.Cursor()
		cx, cy := cursor.X, cursor.Y
		if cx >= 0 && cx < contentW && cy >= 0 && cy < contentH {
			screen.ShowCursor(contentX+cx, contentY+cy)
		}
	}

	// Draw border (always draw, but style changes based on focus)
	p.drawBorder(screen)
}

// getAltCell retrieves a cell from the alternate screen buffer using reflection
func (p *Panel) getAltCell(x, y int) vt10x.Glyph {
	// Try to access altLines via reflection (vt10x doesn't expose it publicly)
	v := reflect.ValueOf(p.VT)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	// Get altLines field
	altLinesField := v.FieldByName("altLines")
	if !altLinesField.IsValid() || !altLinesField.CanInterface() {
		// Fallback to primary buffer if reflection fails
		return p.VT.Cell(x, y)
	}

	// Access altLines as [][]Glyph
	altLines := altLinesField.Interface()
	linesVal := reflect.ValueOf(altLines)

	if y >= linesVal.Len() {
		return vt10x.Glyph{}
	}

	lineVal := linesVal.Index(y)
	if x >= lineVal.Len() {
		return vt10x.Glyph{}
	}

	glyphVal := lineVal.Index(x)
	glyph := glyphVal.Interface().(vt10x.Glyph)

	return glyph
}

// drawBorder draws a border around the terminal panel
func (p *Panel) drawBorder(screen tcell.Screen) {
	var style tcell.Style
	if p.Focus {
		style = GetBorderStyle() // Hot pink when focused
	} else {
		style = config.DefStyle.Foreground(tcell.ColorGray) // Gray when not focused
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

// ShowCursor shows the terminal cursor at its current position
func (p *Panel) ShowCursor(screen tcell.Screen) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.Focus || !p.VT.CursorVisible() {
		return
	}

	// Content area is inside the border
	contentX := p.Region.X + 1
	contentY := p.Region.Y + 1
	contentW := p.Region.Width - 2
	contentH := p.Region.Height - 2

	cursor := p.VT.Cursor()
	cx, cy := cursor.X, cursor.Y
	if cx >= 0 && cx < contentW && cy >= 0 && cy < contentH {
		screen.ShowCursor(contentX+cx, contentY+cy)
	}
}

// glyphToTcellStyle converts VT10x Glyph to tcell style
func glyphToTcellStyle(glyph vt10x.Glyph) tcell.Style {
	// Start with config.DefStyle to get the Thock background
	style := config.DefStyle

	// Foreground color
	if glyph.FG != vt10x.DefaultFG {
		// vt10x stores colors as:
		// - 0-255: palette colors
		// - >255: 24-bit RGB as (r<<16 | g<<8 | b)
		if glyph.FG > 255 {
			// Extract RGB components
			r := int32((glyph.FG >> 16) & 0xFF)
			g := int32((glyph.FG >> 8) & 0xFF)
			b := int32(glyph.FG & 0xFF)
			style = style.Foreground(tcell.NewRGBColor(r, g, b))
		} else {
			// Palette color (0-255)
			style = style.Foreground(tcell.PaletteColor(int(glyph.FG)))
		}
	}

	// Background color - use Thock background for default, otherwise use the glyph's background
	if glyph.BG != vt10x.DefaultBG {
		// Same logic as foreground
		if glyph.BG > 255 {
			// Extract RGB components
			r := int32((glyph.BG >> 16) & 0xFF)
			g := int32((glyph.BG >> 8) & 0xFF)
			b := int32(glyph.BG & 0xFF)
			style = style.Background(tcell.NewRGBColor(r, g, b))
		} else {
			// Palette color (0-255)
			style = style.Background(tcell.PaletteColor(int(glyph.BG)))
		}
	}
	// If glyph.BG == vt10x.DefaultBG, we keep the Thock background from config.DefStyle

	// Text attributes (Mode is int16 with bitflags)
	// Mode flags from vt10x (estimated from typical VT100 implementation)
	const (
		modeBold      = 1 << 0
		modeUnderline = 1 << 1
		modeReverse   = 1 << 2
		modeBlink     = 1 << 3
		modeDim       = 1 << 4
	)

	if glyph.Mode&modeBold != 0 {
		style = style.Bold(true)
	}

	if glyph.Mode&modeUnderline != 0 {
		style = style.Underline(true)
	}

	if glyph.Mode&modeReverse != 0 {
		style = style.Reverse(true)
	}

	if glyph.Mode&modeBlink != 0 {
		style = style.Blink(true)
	}

	if glyph.Mode&modeDim != 0 {
		style = style.Dim(true)
	}

	return style
}
