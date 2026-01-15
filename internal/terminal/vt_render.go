package terminal

import (
	"fmt"
	"reflect"
	"time"

	"github.com/ellery/thicc/internal/config"
	screenPkg "github.com/ellery/thicc/internal/screen"
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

	// Check if we're in alternate screen mode (disable scrollback for fullscreen apps)
	mode := p.VT.Mode()
	useAltScreen := mode&vt10x.ModeAltScreen != 0

	// Show loading indicator if terminal is running but hasn't received output yet
	if p.Running && !p.hasReceivedOutput {
		p.renderLoadingIndicator(screen, contentX, contentY, contentW, contentH)
	} else if p.scrollOffset > 0 && !useAltScreen {
		// If scrolled up and not in alt screen, render scrollback view
		p.renderScrolledView(screen, contentX, contentY, contentW, contentH, cols, rows)
	} else {
		// Render live view (normal rendering)
		p.renderLiveView(screen, contentX, contentY, contentW, contentH, cols, rows, useAltScreen)
	}

	// Draw scroll indicator if scrolled up
	if p.scrollOffset > 0 && !useAltScreen {
		p.drawScrollIndicator(screen)
	}

	// Show cursor if focused and NOT scrolled (offset by content area)
	// Always use fake cursor for terminal to ensure visibility (native cursor may be invisible)
	if p.Focus && p.VT.CursorVisible() && p.scrollOffset == 0 {
		cursor := p.VT.Cursor()
		cx, cy := cursor.X, cursor.Y
		if cx >= 0 && cx < contentW && cy >= 0 && cy < contentH {
			screenPkg.ShowFakeCursor(contentX+cx, contentY+cy)
		}
	}

	// Draw border (always draw, but style changes based on focus)
	p.drawBorder(screen)
}

// Spinner frames for loading animation (braille dots)
var spinnerFrames = []rune{'⠋', '⠙', '⠹', '⠸', '⠼', '⠴', '⠦', '⠧', '⠇', '⠏'}

// renderLoadingIndicator shows a spinner and "Starting..." message while terminal initializes
func (p *Panel) renderLoadingIndicator(screen tcell.Screen, contentX, contentY, contentW, contentH int) {
	// Clear content area with background color
	for y := 0; y < contentH; y++ {
		for x := 0; x < contentW; x++ {
			screen.SetContent(contentX+x, contentY+y, ' ', nil, config.DefStyle)
		}
	}

	// Calculate spinner frame based on time (updates every 80ms)
	frame := int(time.Now().UnixMilli()/80) % len(spinnerFrames)
	spinner := spinnerFrames[frame]

	// Show spinner + loading message centered in content area
	msg := " Starting..."
	fullMsg := string(spinner) + msg
	msgX := contentX + (contentW-len(fullMsg))/2
	msgY := contentY + contentH/2

	// Use a subtle style (dim gray)
	loadingStyle := config.DefStyle.Foreground(tcell.ColorGray)

	// Draw spinner character
	if msgX >= contentX && msgX < contentX+contentW {
		screen.SetContent(msgX, msgY, spinner, nil, loadingStyle)
	}

	// Draw message after spinner
	for i, r := range msg {
		x := msgX + 1 + i
		if x >= contentX && x < contentX+contentW {
			screen.SetContent(x, msgY, r, nil, loadingStyle)
		}
	}
}

// renderLiveView renders the current terminal content (not scrolled)
func (p *Panel) renderLiveView(screen tcell.Screen, contentX, contentY, contentW, contentH, cols, rows int, useAltScreen bool) {
	// Always fill the ENTIRE content area to prevent artifacts from previous renders
	for y := 0; y < contentH; y++ {
		for x := 0; x < contentW; x++ {
			var r rune = ' '
			style := config.DefStyle

			// Only read from VT if within its bounds
			if y < rows && x < cols {
				var glyph vt10x.Glyph

				if useAltScreen {
					glyph = p.getAltCell(x, y)
				} else {
					glyph = p.VT.Cell(x, y)
				}

				r = glyph.Char
				if r == 0 {
					r = ' '
				}

				style = glyphToTcellStyle(glyph)
			}

			if p.isSelected(x, y) {
				style = style.Reverse(true)
			}

			screen.SetContent(contentX+x, contentY+y, r, nil, style)
		}
	}
}

// renderScrolledView renders scrollback history + partial live view
func (p *Panel) renderScrolledView(screen tcell.Screen, contentX, contentY, contentW, contentH, cols, rows int) {
	scrollbackCount := p.Scrollback.Count()

	for y := 0; y < contentH; y++ {
		// Calculate which line to show:
		// lineIndex = scrollbackCount - scrollOffset + y
		// When scrollOffset = scrollbackCount, we show from line 0 (oldest)
		// When scrollOffset = 1, we show scrollbackCount-1 (newest scrollback) at top
		lineIndex := scrollbackCount - p.scrollOffset + y

		if lineIndex < 0 {
			// Above scrollback - render empty line
			for x := 0; x < contentW; x++ {
				style := config.DefStyle
				if p.isSelected(x, y) {
					style = style.Reverse(true)
				}
				screen.SetContent(contentX+x, contentY+y, ' ', nil, style)
			}
		} else if lineIndex < scrollbackCount {
			// In scrollback buffer
			line := p.Scrollback.Get(lineIndex)
			if line != nil {
				for x := 0; x < contentW; x++ {
					var r rune = ' '
					style := config.DefStyle

					if x < len(line.Cells) {
						glyph := line.Cells[x]
						r = glyph.Char
						if r == 0 {
							r = ' '
						}
						style = glyphToTcellStyle(glyph)
					}

					if p.isSelected(x, y) {
						style = style.Reverse(true)
					}

					screen.SetContent(contentX+x, contentY+y, r, nil, style)
				}
			} else {
				// Nil line (shouldn't happen) - render empty
				for x := 0; x < contentW; x++ {
					style := config.DefStyle
					if p.isSelected(x, y) {
						style = style.Reverse(true)
					}
					screen.SetContent(contentX+x, contentY+y, ' ', nil, style)
				}
			}
		} else {
			// In live terminal view
			liveY := lineIndex - scrollbackCount
			if liveY < rows {
				for x := 0; x < contentW && x < cols; x++ {
					glyph := p.VT.Cell(x, liveY)
					r := glyph.Char
					if r == 0 {
						r = ' '
					}
					style := glyphToTcellStyle(glyph)
					if p.isSelected(x, y) {
						style = style.Reverse(true)
					}
					screen.SetContent(contentX+x, contentY+y, r, nil, style)
				}
				// Fill remaining width if cols < contentW
				for x := cols; x < contentW; x++ {
					style := config.DefStyle
					if p.isSelected(x, y) {
						style = style.Reverse(true)
					}
					screen.SetContent(contentX+x, contentY+y, ' ', nil, style)
				}
			} else {
				// Beyond live view - render empty
				for x := 0; x < contentW; x++ {
					style := config.DefStyle
					if p.isSelected(x, y) {
						style = style.Reverse(true)
					}
					screen.SetContent(contentX+x, contentY+y, ' ', nil, style)
				}
			}
		}
	}
}

// drawScrollIndicator draws "[+N]" at top-right corner when scrolled
func (p *Panel) drawScrollIndicator(screen tcell.Screen) {
	indicator := fmt.Sprintf("[+%d]", p.scrollOffset)
	indicatorStyle := config.DefStyle.Foreground(tcell.ColorYellow).Bold(true)

	// Position at top-right of border (inside the top border line)
	x := p.Region.X + p.Region.Width - len(indicator) - 2
	if x < p.Region.X+2 {
		x = p.Region.X + 2 // Ensure it doesn't overlap left border
	}

	for i, r := range indicator {
		screen.SetContent(x+i, p.Region.Y, r, nil, indicatorStyle)
	}
}

// getAltCell retrieves a cell from the alternate screen buffer using reflection.
// Note: vt10x's altLines field is unexported, so we must use reflect.Index()
// directly rather than Interface() which doesn't work for unexported fields.
func (p *Panel) getAltCell(x, y int) vt10x.Glyph {
	// Get the concrete State struct from the Terminal interface
	v := reflect.ValueOf(p.VT)
	if v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface {
		v = v.Elem()
	}

	// Get altLines field (unexported []line where line = []Glyph)
	altLinesField := v.FieldByName("altLines")
	if !altLinesField.IsValid() {
		// Fallback to primary buffer if field doesn't exist
		return p.VT.Cell(x, y)
	}

	// Access the slice directly using Index() - works for unexported fields
	if y >= altLinesField.Len() {
		return vt10x.Glyph{}
	}

	lineVal := altLinesField.Index(y)
	if x >= lineVal.Len() {
		return vt10x.Glyph{}
	}

	// Get the Glyph struct fields directly via reflection
	glyphVal := lineVal.Index(x)

	// Extract Glyph fields: Char (rune), Mode (int16), FG (Color), BG (Color)
	var glyph vt10x.Glyph
	if charField := glyphVal.FieldByName("Char"); charField.IsValid() {
		glyph.Char = rune(charField.Int())
	}
	if modeField := glyphVal.FieldByName("Mode"); modeField.IsValid() {
		glyph.Mode = int16(modeField.Int())
	}
	if fgField := glyphVal.FieldByName("FG"); fgField.IsValid() {
		glyph.FG = vt10x.Color(fgField.Uint())
	}
	if bgField := glyphVal.FieldByName("BG"); bgField.IsValid() {
		glyph.BG = vt10x.Color(bgField.Uint())
	}

	return glyph
}

// drawBorder draws a border around the terminal panel
func (p *Panel) drawBorder(screen tcell.Screen) {
	var style tcell.Style
	if p.PassthroughMode {
		style = config.DefStyle.Foreground(tcell.ColorOrange) // Orange when in passthrough mode
	} else if p.Focus {
		style = GetBorderStyle() // Hot pink when focused
	} else {
		style = config.DefStyle.Foreground(tcell.ColorGray) // Gray when not focused
	}

	// Use double-line border for focused or passthrough mode
	useDoubleLine := p.Focus || p.PassthroughMode

	// Draw vertical lines
	for y := 1; y < p.Region.Height-1; y++ {
		if useDoubleLine {
			screen.SetContent(p.Region.X, p.Region.Y+y, '║', nil, style)
			screen.SetContent(p.Region.X+p.Region.Width-1, p.Region.Y+y, '║', nil, style)
		} else {
			screen.SetContent(p.Region.X, p.Region.Y+y, '│', nil, style)
			screen.SetContent(p.Region.X+p.Region.Width-1, p.Region.Y+y, '│', nil, style)
		}
	}

	// Draw horizontal lines
	for x := 1; x < p.Region.Width-1; x++ {
		if useDoubleLine {
			screen.SetContent(p.Region.X+x, p.Region.Y, '═', nil, style)
			screen.SetContent(p.Region.X+x, p.Region.Y+p.Region.Height-1, '═', nil, style)
		} else {
			screen.SetContent(p.Region.X+x, p.Region.Y, '─', nil, style)
			screen.SetContent(p.Region.X+x, p.Region.Y+p.Region.Height-1, '─', nil, style)
		}
	}

	// Draw corners
	if useDoubleLine {
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
// Always uses fake cursor to ensure visibility (native cursor may be invisible on dark backgrounds)
func (p *Panel) ShowCursor(s tcell.Screen) {
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
		screenPkg.ShowFakeCursor(contentX+cx, contentY+cy)
	}
}

// rgbTo256Color converts RGB values to the closest 256-palette color
func rgbTo256Color(r, g, b int) tcell.Color {
	// Use the 216-color cube (colors 16-231) for approximation
	// Each channel maps to 0-5 range
	ri := (r * 5) / 255
	gi := (g * 5) / 255
	bi := (b * 5) / 255

	// Calculate 216-color index: 16 + 36*r + 6*g + b
	colorIndex := 16 + 36*ri + 6*gi + bi

	return tcell.PaletteColor(colorIndex)
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
			// Extract RGB components and convert to 256-palette for tmux compatibility
			r := int((glyph.FG >> 16) & 0xFF)
			g := int((glyph.FG >> 8) & 0xFF)
			b := int(glyph.FG & 0xFF)
			if config.InTmux {
				style = style.Foreground(rgbTo256Color(r, g, b))
			} else {
				style = style.Foreground(tcell.NewRGBColor(int32(r), int32(g), int32(b)))
			}
		} else {
			// Palette color (0-255)
			style = style.Foreground(tcell.PaletteColor(int(glyph.FG)))
		}
	}

	// Background color - use Thock background for default, otherwise use the glyph's background
	if glyph.BG != vt10x.DefaultBG {
		// Same logic as foreground
		if glyph.BG > 255 {
			// Extract RGB components and convert to 256-palette for tmux compatibility
			r := int((glyph.BG >> 16) & 0xFF)
			g := int((glyph.BG >> 8) & 0xFF)
			b := int(glyph.BG & 0xFF)
			if config.InTmux {
				style = style.Background(rgbTo256Color(r, g, b))
			} else {
				style = style.Background(tcell.NewRGBColor(int32(r), int32(g), int32(b)))
			}
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
