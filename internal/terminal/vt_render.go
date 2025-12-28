package terminal

import (
	"github.com/hinshun/vt10x"
	"github.com/micro-editor/tcell/v2"
)

// Render draws the terminal to the tcell screen
func (p *Panel) Render(screen tcell.Screen) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Get terminal size
	cols, rows := p.VT.Size()

	// Render each cell from VT to tcell
	for y := 0; y < p.Region.Height && y < rows; y++ {
		for x := 0; x < p.Region.Width && x < cols; x++ {
			// Get cell from VT
			glyph := p.VT.Cell(x, y)

			// Convert VT cell to tcell
			r := glyph.Char
			if r == 0 {
				r = ' ' // Empty cells render as space
			}

			style := glyphToTcellStyle(glyph)

			// Draw to screen
			screen.SetContent(
				p.Region.X+x,
				p.Region.Y+y,
				r,
				nil,
				style,
			)
		}
	}

	// Show cursor if focused
	if p.Focus && p.VT.CursorVisible() {
		cursor := p.VT.Cursor()
		cx, cy := cursor.X, cursor.Y
		if cx >= 0 && cx < p.Region.Width && cy >= 0 && cy < p.Region.Height {
			screen.ShowCursor(p.Region.X+cx, p.Region.Y+cy)
		}
	}
}

// glyphToTcellStyle converts VT10x Glyph to tcell style
func glyphToTcellStyle(glyph vt10x.Glyph) tcell.Style {
	style := tcell.StyleDefault

	// Foreground color
	if glyph.FG != vt10x.DefaultFG {
		style = style.Foreground(tcell.Color(glyph.FG))
	}

	// Background color
	if glyph.BG != vt10x.DefaultBG {
		style = style.Background(tcell.Color(glyph.BG))
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
