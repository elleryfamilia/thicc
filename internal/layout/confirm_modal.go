package layout

import (
	"github.com/micro-editor/tcell/v2"
)

// ConfirmModal represents a dangerous action confirmation dialog with red styling
type ConfirmModal struct {
	Active   bool
	Title    string
	Message  string
	Warning  string // Warning text shown in red (e.g., "This cannot be undone!")
	Callback func(confirmed bool)
	ScreenW  int
	ScreenH  int
}

// NewConfirmModal creates a new confirmation modal dialog
func NewConfirmModal() *ConfirmModal {
	return &ConfirmModal{}
}

// Show displays the confirmation modal
func (m *ConfirmModal) Show(title, message, warning string, screenW, screenH int, callback func(confirmed bool)) {
	m.Active = true
	m.Title = title
	m.Message = message
	m.Warning = warning
	m.ScreenW = screenW
	m.ScreenH = screenH
	m.Callback = callback
}

// Hide closes the modal
func (m *ConfirmModal) Hide() {
	m.Active = false
	m.Callback = nil
}

// HandleEvent processes keyboard events for the modal
// Returns true if the event was consumed
func (m *ConfirmModal) HandleEvent(event tcell.Event) bool {
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
			m.Callback(false) // not confirmed
		}
		m.Hide()
		return true
	case tcell.KeyRune:
		switch ev.Rune() {
		case 'y', 'Y':
			if m.Callback != nil {
				m.Callback(true) // confirmed
			}
			m.Hide()
			return true
		case 'n', 'N':
			if m.Callback != nil {
				m.Callback(false) // not confirmed
			}
			m.Hide()
			return true
		}
	}

	return true // Consume all events while modal is active
}

// Render draws the confirmation modal dialog centered on screen with danger styling
func (m *ConfirmModal) Render(screen tcell.Screen) {
	if !m.Active {
		return
	}

	// Calculate dialog dimensions
	titleLen := len(m.Title)
	msgLen := len(m.Message)
	warnLen := len(m.Warning)
	optLen := len("(y)es  (n)o  (esc)ape")

	contentWidth := titleLen
	if msgLen > contentWidth {
		contentWidth = msgLen
	}
	if warnLen > contentWidth {
		contentWidth = warnLen
	}
	if optLen > contentWidth {
		contentWidth = optLen
	}

	// Add padding
	boxWidth := contentWidth + 6
	boxHeight := 9 // Extra line for warning

	// Minimum width
	if boxWidth < 45 {
		boxWidth = 45
	}

	// Center the dialog
	startX := (m.ScreenW - boxWidth) / 2
	startY := (m.ScreenH - boxHeight) / 2

	// Danger styles - red theme
	borderStyle := tcell.StyleDefault.Foreground(tcell.Color196) // Bright red
	bgStyle := tcell.StyleDefault.Background(tcell.ColorBlack)
	titleStyle := tcell.StyleDefault.Foreground(tcell.Color196).Bold(true)
	textStyle := tcell.StyleDefault.Foreground(tcell.ColorWhite)
	warningStyle := tcell.StyleDefault.Foreground(tcell.Color196) // Red warning text
	optStyle := tcell.StyleDefault.Foreground(tcell.Color51)      // Cyan

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

	// Draw title (centered, line 1) - with warning glyph
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
		screen.SetContent(msgX+i, startY+4, r, nil, textStyle)
	}

	// Draw warning (centered, line 5) - in red
	if m.Warning != "" {
		warnX := startX + (boxWidth-warnLen)/2
		for i, r := range m.Warning {
			screen.SetContent(warnX+i, startY+5, r, nil, warningStyle)
		}
	}

	// Draw options (centered, line 7)
	options := "(y)es  (n)o  (esc)ape"
	optX := startX + (boxWidth-len(options))/2
	for i, r := range options {
		screen.SetContent(optX+i, startY+7, r, nil, optStyle)
	}
}
