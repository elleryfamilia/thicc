package terminal

import (
	"log"

	"github.com/hinshun/vt10x"
	"github.com/micro-editor/tcell/v2"
)

var debugOnce = true

// BorderStyle is the style for focus borders (pink for Spider-Verse vibe)
var BorderStyle = tcell.StyleDefault.Foreground(tcell.Color205) // Hot pink

// Render draws the terminal to the tcell screen
func (p *Panel) Render(screen tcell.Screen) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Clear the entire region first
	clearStyle := tcell.StyleDefault.Background(tcell.ColorBlack)
	for y := 0; y < p.Region.Height; y++ {
		for x := 0; x < p.Region.Width; x++ {
			screen.SetContent(p.Region.X+x, p.Region.Y+y, ' ', nil, clearStyle)
		}
	}

	// Content area is inside the border (1 cell margin on each side)
	contentX := p.Region.X + 1
	contentY := p.Region.Y + 1
	contentW := p.Region.Width - 2
	contentH := p.Region.Height - 2

	// Get terminal size from VT emulator
	cols, rows := p.VT.Size()

	// Debug: log first line characters once
	if debugOnce {
		debugOnce = false
		var chars []rune
		for x := 0; x < cols && x < 40; x++ {
			g := p.VT.Cell(x, 0)
			chars = append(chars, g.Char)
		}
		log.Printf("THOCK TERM DEBUG: First line chars: %v", chars)
		log.Printf("THOCK TERM DEBUG: First line string: %s", string(chars))
	}

	// Render each cell from VT to tcell (in content area)
	for y := 0; y < contentH && y < rows; y++ {
		for x := 0; x < contentW && x < cols; x++ {
			// Get cell from VT
			glyph := p.VT.Cell(x, y)

			// Convert VT cell to tcell
			r := glyph.Char
			if r == 0 {
				r = ' ' // Empty cells render as space
			}

			style := glyphToTcellStyle(glyph)

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

// drawBorder draws a border around the terminal panel
func (p *Panel) drawBorder(screen tcell.Screen) {
	var style tcell.Style
	if p.Focus {
		style = BorderStyle // Hot pink when focused
	} else {
		style = tcell.StyleDefault.Foreground(tcell.ColorGray) // Gray when not focused
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
	style := tcell.StyleDefault

	// Foreground color - use PaletteColor for 256-color support
	if glyph.FG != vt10x.DefaultFG {
		style = style.Foreground(tcell.PaletteColor(int(glyph.FG)))
	}

	// Background color - use PaletteColor for 256-color support
	if glyph.BG != vt10x.DefaultBG {
		style = style.Background(tcell.PaletteColor(int(glyph.BG)))
	}

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
