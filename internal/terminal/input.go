package terminal

import (
	"log"
	"unicode/utf8"

	"github.com/ellery/thicc/internal/clipboard"
	"github.com/micro-editor/tcell/v2"
)

// HandleEvent processes keyboard and mouse events
func (p *Panel) HandleEvent(event tcell.Event) bool {
	log.Printf("THOCK Terminal.HandleEvent: Focus=%v", p.Focus)
	if !p.Focus {
		log.Println("THOCK Terminal: Not focused, returning false")
		return false
	}

	switch ev := event.(type) {
	case *tcell.EventKey:
		log.Printf("THOCK Terminal: Key event, Key=%v, Rune=%c", ev.Key(), ev.Rune())

		// Handle quick command mode first
		if p.QuickCommandMode {
			p.handleQuickCommand(ev)
			return true
		}

		// Ctrl+\ enters quick command mode
		if ev.Key() == tcell.KeyCtrlBackslash {
			p.QuickCommandMode = true
			if p.OnShowMessage != nil {
				p.OnShowMessage("  q: Quit  |  w: Next Pane  |  Esc: Cancel")
			}
			log.Println("THOCK Terminal: Entered quick command mode")
			return true
		}

		// Handle Shift+PageUp/Down for scrolling (before other key handling)
		if ev.Key() == tcell.KeyPgUp && ev.Modifiers()&tcell.ModShift != 0 {
			_, rows := p.VT.Size()
			p.ScrollUp(rows - 1) // Scroll nearly one page
			return true
		}
		if ev.Key() == tcell.KeyPgDn && ev.Modifiers()&tcell.ModShift != 0 {
			_, rows := p.VT.Size()
			p.ScrollDown(rows - 1) // Scroll nearly one page
			return true
		}

		// Handle Ctrl+C: copy selection if exists, otherwise send SIGINT
		if ev.Key() == tcell.KeyCtrlC {
			if p.HasSelection() {
				text := p.GetSelection()
				clipboard.Write(text, clipboard.ClipboardReg)
				log.Printf("THOCK Terminal: Copied %d chars to clipboard", len(text))
				p.ClearSelection()
				// Trigger redraw to clear selection highlight
				if p.OnRedraw != nil {
					p.OnRedraw()
				}
				return true
			}
			// No selection - fall through to send SIGINT to terminal
		}

		// If any key is pressed while scrolled up, snap back to live view
		if p.IsScrolledUp() {
			p.ScrollToBottom()
		}

		result := p.handleKey(ev)
		log.Printf("THOCK Terminal: handleKey returned %v", result)
		return result
	case *tcell.EventPaste:
		// Handle paste events directly (backup if layout manager doesn't catch it)
		log.Printf("THOCK Terminal: Paste event, len=%d", len(ev.Text()))
		_, err := p.Write([]byte(ev.Text()))
		return err == nil
	case *tcell.EventMouse:
		return p.handleMouse(ev)
	}

	return false
}

// handleMouse processes mouse events for text selection and scrolling
func (p *Panel) handleMouse(ev *tcell.EventMouse) bool {
	// Handle scroll wheel (no lock needed, ScrollUp/Down handle it)
	if ev.Buttons() == tcell.WheelUp {
		p.ScrollUp(3) // Scroll 3 lines up (into history)
		return true
	}
	if ev.Buttons() == tcell.WheelDown {
		p.ScrollDown(3) // Scroll 3 lines down (toward live)
		return true
	}

	// Calculate position relative to content area (inside border)
	mouseX, mouseY := ev.Position()
	contentX := p.Region.X + 1
	contentY := p.Region.Y + 1
	x := mouseX - contentX
	y := mouseY - contentY

	// Clamp to content bounds
	contentW := p.Region.Width - 2
	contentH := p.Region.Height - 2
	if x < 0 {
		x = 0
	}
	if x >= contentW {
		x = contentW - 1
	}
	if y < 0 {
		y = 0
	}
	if y >= contentH {
		y = contentH - 1
	}

	if ev.Buttons() == tcell.Button1 {
		if p.mouseReleased {
			// New click - start selection
			p.Selection[0] = Loc{X: x, Y: y}
			p.Selection[1] = Loc{X: x, Y: y}
			log.Printf("THOCK Terminal: Selection start at (%d, %d)", x, y)
		} else {
			// Drag - extend selection
			p.Selection[1] = Loc{X: x, Y: y}
			log.Printf("THOCK Terminal: Selection drag to (%d, %d)", x, y)
		}
		p.mouseReleased = false

		// Trigger redraw to show selection
		if p.OnRedraw != nil {
			p.OnRedraw()
		}
		return true
	} else if ev.Buttons() == tcell.ButtonNone {
		if !p.mouseReleased {
			// Button released - finalize selection
			p.Selection[1] = Loc{X: x, Y: y}
			p.mouseReleased = true
			log.Printf("THOCK Terminal: Selection end at (%d, %d)", x, y)

			// Trigger redraw
			if p.OnRedraw != nil {
				p.OnRedraw()
			}
		}
		return false // Don't consume button-none events
	}

	// Consume other mouse button events to prevent focus stealing
	if ev.Buttons() != tcell.ButtonNone {
		return true
	}
	return false
}

// handleKey processes keyboard events and sends to PTY
func (p *Panel) handleKey(ev *tcell.EventKey) bool {
	// Convert tcell key to bytes
	bytes := keyToBytes(ev)
	if bytes == nil {
		return false
	}

	// Write to PTY
	_, err := p.Write(bytes)
	return err == nil
}

// keyToBytes converts a tcell key event to bytes for PTY
func keyToBytes(ev *tcell.EventKey) []byte {
	// Handle special keys first
	switch ev.Key() {
	case tcell.KeyEnter:
		return []byte{'\r'}

	case tcell.KeyTab:
		return []byte{'\t'}

	case tcell.KeyBackspace, tcell.KeyBackspace2:
		return []byte{0x7f} // DEL character

	case tcell.KeyEscape:
		return []byte{0x1b}

	// Arrow keys
	case tcell.KeyUp:
		return []byte{0x1b, '[', 'A'}
	case tcell.KeyDown:
		return []byte{0x1b, '[', 'B'}
	case tcell.KeyRight:
		return []byte{0x1b, '[', 'C'}
	case tcell.KeyLeft:
		return []byte{0x1b, '[', 'D'}

	// Home/End
	case tcell.KeyHome:
		return []byte{0x1b, '[', 'H'}
	case tcell.KeyEnd:
		return []byte{0x1b, '[', 'F'}

	// Page Up/Down
	case tcell.KeyPgUp:
		return []byte{0x1b, '[', '5', '~'}
	case tcell.KeyPgDn:
		return []byte{0x1b, '[', '6', '~'}

	// Insert/Delete
	case tcell.KeyInsert:
		return []byte{0x1b, '[', '2', '~'}
	case tcell.KeyDelete:
		return []byte{0x1b, '[', '3', '~'}

	// Function keys
	case tcell.KeyF1:
		return []byte{0x1b, 'O', 'P'}
	case tcell.KeyF2:
		return []byte{0x1b, 'O', 'Q'}
	case tcell.KeyF3:
		return []byte{0x1b, 'O', 'R'}
	case tcell.KeyF4:
		return []byte{0x1b, 'O', 'S'}
	case tcell.KeyF5:
		return []byte{0x1b, '[', '1', '5', '~'}
	case tcell.KeyF6:
		return []byte{0x1b, '[', '1', '7', '~'}
	case tcell.KeyF7:
		return []byte{0x1b, '[', '1', '8', '~'}
	case tcell.KeyF8:
		return []byte{0x1b, '[', '1', '9', '~'}
	case tcell.KeyF9:
		return []byte{0x1b, '[', '2', '0', '~'}
	case tcell.KeyF10:
		return []byte{0x1b, '[', '2', '1', '~'}
	case tcell.KeyF11:
		return []byte{0x1b, '[', '2', '3', '~'}
	case tcell.KeyF12:
		return []byte{0x1b, '[', '2', '4', '~'}

	// Ctrl keys
	case tcell.KeyCtrlA:
		return []byte{0x01}
	case tcell.KeyCtrlB:
		return []byte{0x02}
	case tcell.KeyCtrlC:
		return []byte{0x03}
	case tcell.KeyCtrlD:
		return []byte{0x04}
	case tcell.KeyCtrlE:
		return []byte{0x05}
	case tcell.KeyCtrlF:
		return []byte{0x06}
	case tcell.KeyCtrlG:
		return []byte{0x07}
	// KeyCtrlH, KeyCtrlI, KeyCtrlM handled by Backspace, Tab, Enter above
	case tcell.KeyCtrlJ:
		return []byte{'\n'}
	case tcell.KeyCtrlK:
		return []byte{0x0b}
	case tcell.KeyCtrlL:
		return []byte{0x0c}
	case tcell.KeyCtrlN:
		return []byte{0x0e}
	case tcell.KeyCtrlO:
		return []byte{0x0f}
	case tcell.KeyCtrlP:
		return []byte{0x10}
	case tcell.KeyCtrlQ:
		return []byte{0x11}
	case tcell.KeyCtrlR:
		return []byte{0x12}
	case tcell.KeyCtrlS:
		return []byte{0x13}
	case tcell.KeyCtrlT:
		return []byte{0x14}
	case tcell.KeyCtrlU:
		return []byte{0x15}
	case tcell.KeyCtrlV:
		return []byte{0x16}
	case tcell.KeyCtrlW:
		return []byte{0x17}
	case tcell.KeyCtrlX:
		return []byte{0x18}
	case tcell.KeyCtrlY:
		return []byte{0x19}
	case tcell.KeyCtrlZ:
		return []byte{0x1a}
	case tcell.KeyCtrlBackslash:
		return []byte{0x1c}
	case tcell.KeyCtrlRightSq:
		return []byte{0x1d}
	case tcell.KeyCtrlCarat:
		return []byte{0x1e}
	case tcell.KeyCtrlUnderscore:
		return []byte{0x1f}

	// Regular character
	case tcell.KeyRune:
		r := ev.Rune()

		// Handle Ctrl+key combinations that come as runes
		if ev.Modifiers()&tcell.ModCtrl != 0 {
			// Ctrl+letter (a-z) maps to 0x01-0x1a
			if r >= 'a' && r <= 'z' {
				return []byte{byte(r - 'a' + 1)}
			}
			if r >= 'A' && r <= 'Z' {
				return []byte{byte(r - 'A' + 1)}
			}
		}

		// Regular character - encode as UTF-8
		if r < 128 {
			return []byte{byte(r)}
		}

		// Multi-byte UTF-8
		buf := make([]byte, 4)
		n := utf8.EncodeRune(buf, r)
		return buf[:n]
	}

	return nil
}

// handleQuickCommand processes key events in quick command mode
func (p *Panel) handleQuickCommand(ev *tcell.EventKey) {
	// Clear message first
	if p.OnShowMessage != nil {
		p.OnShowMessage("")
	}
	p.QuickCommandMode = false

	switch ev.Key() {
	case tcell.KeyEscape:
		log.Println("THOCK Terminal: Quick command cancelled")
		return
	case tcell.KeyRune:
		switch ev.Rune() {
		case 'q', 'Q':
			log.Println("THOCK Terminal: Quick command - Quit")
			if p.OnQuit != nil {
				p.OnQuit()
			}
			return
		case 'w', 'W':
			log.Println("THOCK Terminal: Quick command - Next Pane")
			if p.OnNextPane != nil {
				p.OnNextPane()
			}
			return
		}
	}
	// Any other key just cancels
	log.Printf("THOCK Terminal: Quick command - unknown key, cancelled")
}
