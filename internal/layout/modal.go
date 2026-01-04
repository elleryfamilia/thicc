package layout

import (
	"github.com/micro-editor/tcell/v2"
)

// Modal represents a centered dialog box for important prompts
type Modal struct {
	Active   bool
	Title    string
	Message  string
	Options  string // e.g., "(y,n,esc)"
	Callback func(yes, canceled bool)
	ScreenW  int
	ScreenH  int
}

// NewModal creates a new modal dialog
func NewModal() *Modal {
	return &Modal{}
}

// Show displays the modal with a yes/no prompt
func (m *Modal) Show(title, message, options string, screenW, screenH int, callback func(yes, canceled bool)) {
	m.Active = true
	m.Title = title
	m.Message = message
	m.Options = options
	m.ScreenW = screenW
	m.ScreenH = screenH
	m.Callback = callback
}

// Hide closes the modal
func (m *Modal) Hide() {
	m.Active = false
	m.Callback = nil
}

// HandleEvent processes keyboard events for the modal
// Returns true if the event was consumed
func (m *Modal) HandleEvent(event tcell.Event) bool {
	if !m.Active {
		return false
	}

	ev, ok := event.(*tcell.EventKey)
	if !ok {
		return true // Consume non-key events while modal is active
	}

	switch ev.Key() {
	case tcell.KeyEscape:
		if m.Callback != nil {
			m.Callback(false, true) // canceled
		}
		m.Hide()
		return true
	case tcell.KeyRune:
		switch ev.Rune() {
		case 'y', 'Y':
			if m.Callback != nil {
				m.Callback(true, false) // yes
			}
			m.Hide()
			return true
		case 'n', 'N':
			if m.Callback != nil {
				m.Callback(false, false) // no
			}
			m.Hide()
			return true
		}
	}

	return true // Consume all events while modal is active
}

// Render draws the modal dialog centered on screen
func (m *Modal) Render(screen tcell.Screen) {
	if !m.Active {
		return
	}

	// Calculate dialog dimensions
	titleLen := len(m.Title)
	msgLen := len(m.Message)
	optLen := len(m.Options)

	contentWidth := titleLen
	if msgLen > contentWidth {
		contentWidth = msgLen
	}
	if optLen > contentWidth {
		contentWidth = optLen
	}

	// Add padding
	boxWidth := contentWidth + 6
	boxHeight := 7

	// Minimum width
	if boxWidth < 40 {
		boxWidth = 40
	}

	// Center the dialog
	startX := (m.ScreenW - boxWidth) / 2
	startY := (m.ScreenH - boxHeight) / 2

	// Styles - all must have explicit fg AND bg to prevent color changes in light mode
	bgColor := tcell.ColorBlack
	borderStyle := tcell.StyleDefault.Foreground(tcell.Color205).Background(bgColor) // Hot pink
	bgStyle := tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(bgColor)
	titleStyle := tcell.StyleDefault.Foreground(tcell.Color205).Background(bgColor).Bold(true)
	textStyle := tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(bgColor)
	optStyle := tcell.StyleDefault.Foreground(tcell.Color51).Background(bgColor) // Cyan

	// Draw background
	for y := startY; y < startY+boxHeight; y++ {
		for x := startX; x < startX+boxWidth; x++ {
			screen.SetContent(x, y, ' ', nil, bgStyle)
		}
	}

	// Draw border (double-line style)
	// Top border
	screen.SetContent(startX, startY, '╔', nil, borderStyle)
	screen.SetContent(startX+boxWidth-1, startY, '╗', nil, borderStyle)
	for x := startX + 1; x < startX+boxWidth-1; x++ {
		screen.SetContent(x, startY, '═', nil, borderStyle)
	}

	// Bottom border
	screen.SetContent(startX, startY+boxHeight-1, '╚', nil, borderStyle)
	screen.SetContent(startX+boxWidth-1, startY+boxHeight-1, '╝', nil, borderStyle)
	for x := startX + 1; x < startX+boxWidth-1; x++ {
		screen.SetContent(x, startY+boxHeight-1, '═', nil, borderStyle)
	}

	// Side borders
	for y := startY + 1; y < startY+boxHeight-1; y++ {
		screen.SetContent(startX, y, '║', nil, borderStyle)
		screen.SetContent(startX+boxWidth-1, y, '║', nil, borderStyle)
	}

	// Draw title (centered, line 1)
	titleX := startX + (boxWidth-titleLen)/2
	for i, r := range m.Title {
		screen.SetContent(titleX+i, startY+1, r, nil, titleStyle)
	}

	// Draw separator line
	for x := startX + 1; x < startX+boxWidth-1; x++ {
		screen.SetContent(x, startY+2, '─', nil, borderStyle)
	}

	// Draw message (centered, line 3)
	msgX := startX + (boxWidth-msgLen)/2
	for i, r := range m.Message {
		screen.SetContent(msgX+i, startY+3, r, nil, textStyle)
	}

	// Draw options (centered, line 5)
	optX := startX + (boxWidth-optLen)/2
	for i, r := range m.Options {
		screen.SetContent(optX+i, startY+5, r, nil, optStyle)
	}
}
