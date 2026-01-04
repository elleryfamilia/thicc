package layout

import (
	"github.com/micro-editor/tcell/v2"
)

// InputModal is a modal dialog that accepts text input
type InputModal struct {
	Active    bool
	Title     string
	Prompt    string
	Value     string // Current input value
	CursorPos int    // Cursor position in value
	Callback  func(value string, canceled bool)
	ScreenW   int
	ScreenH   int
}

// NewInputModal creates a new input modal dialog
func NewInputModal() *InputModal {
	return &InputModal{}
}

// Show displays the input modal with a text prompt
func (m *InputModal) Show(title, prompt, defaultValue string, screenW, screenH int, callback func(value string, canceled bool)) {
	m.Active = true
	m.Title = title
	m.Prompt = prompt
	m.Value = defaultValue
	m.CursorPos = len(defaultValue)
	m.ScreenW = screenW
	m.ScreenH = screenH
	m.Callback = callback
}

// Hide closes the input modal
func (m *InputModal) Hide() {
	m.Active = false
	m.Callback = nil
	m.Value = ""
	m.CursorPos = 0
}

// HandleEvent processes keyboard events for the input modal
// Returns true if the event was consumed
func (m *InputModal) HandleEvent(event tcell.Event) bool {
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
			m.Callback("", true)
		}
		m.Hide()
		return true

	case tcell.KeyEnter:
		if m.Callback != nil {
			m.Callback(m.Value, false)
		}
		m.Hide()
		return true

	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if m.CursorPos > 0 {
			m.Value = m.Value[:m.CursorPos-1] + m.Value[m.CursorPos:]
			m.CursorPos--
		}
		return true

	case tcell.KeyDelete:
		if m.CursorPos < len(m.Value) {
			m.Value = m.Value[:m.CursorPos] + m.Value[m.CursorPos+1:]
		}
		return true

	case tcell.KeyLeft:
		if m.CursorPos > 0 {
			m.CursorPos--
		}
		return true

	case tcell.KeyRight:
		if m.CursorPos < len(m.Value) {
			m.CursorPos++
		}
		return true

	case tcell.KeyHome, tcell.KeyCtrlA:
		m.CursorPos = 0
		return true

	case tcell.KeyEnd, tcell.KeyCtrlE:
		m.CursorPos = len(m.Value)
		return true

	case tcell.KeyRune:
		// Insert character at cursor
		m.Value = m.Value[:m.CursorPos] + string(ev.Rune()) + m.Value[m.CursorPos:]
		m.CursorPos++
		return true
	}

	return true // Consume all events while modal is active
}

// Render draws the input modal dialog centered on screen
func (m *InputModal) Render(screen tcell.Screen) {
	if !m.Active {
		return
	}

	// Calculate dialog dimensions
	titleLen := len(m.Title)
	promptLen := len(m.Prompt)

	// Input field width (at least 30 chars, or prompt length + some padding)
	inputFieldWidth := 30
	if promptLen+10 > inputFieldWidth {
		inputFieldWidth = promptLen + 10
	}

	contentWidth := titleLen
	if promptLen > contentWidth {
		contentWidth = promptLen
	}
	if inputFieldWidth > contentWidth {
		contentWidth = inputFieldWidth
	}

	// Add padding
	boxWidth := contentWidth + 6
	boxHeight := 9 // Title, separator, prompt, input field, blank, options

	// Minimum width
	if boxWidth < 50 {
		boxWidth = 50
	}

	// Center the dialog
	startX := (m.ScreenW - boxWidth) / 2
	startY := (m.ScreenH - boxHeight) / 2

	// Styles - all must have explicit fg AND bg to prevent color changes in light mode
	bgColor := tcell.ColorBlack
	borderStyle := tcell.StyleDefault.Foreground(tcell.Color205).Background(bgColor) // Hot pink
	bgStyle := tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(bgColor)
	titleStyle := tcell.StyleDefault.Foreground(tcell.Color205).Background(bgColor).Bold(true)
	promptStyle := tcell.StyleDefault.Foreground(tcell.ColorWhite).Background(bgColor)
	inputBgStyle := tcell.StyleDefault.Background(tcell.ColorDarkGray).Foreground(tcell.ColorWhite)
	cursorStyle := tcell.StyleDefault.Background(tcell.ColorWhite).Foreground(tcell.ColorBlack)
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

	// Draw prompt (line 3)
	promptX := startX + 3
	for i, r := range m.Prompt {
		screen.SetContent(promptX+i, startY+3, r, nil, promptStyle)
	}

	// Draw input field background (line 5)
	inputX := startX + 3
	inputWidth := boxWidth - 6
	for x := inputX; x < inputX+inputWidth; x++ {
		screen.SetContent(x, startY+5, ' ', nil, inputBgStyle)
	}

	// Draw input value
	valueToShow := m.Value
	if len(valueToShow) > inputWidth-1 {
		// Scroll to keep cursor visible
		start := m.CursorPos - inputWidth + 2
		if start < 0 {
			start = 0
		}
		valueToShow = valueToShow[start:]
		if len(valueToShow) > inputWidth-1 {
			valueToShow = valueToShow[:inputWidth-1]
		}
	}

	for i, r := range valueToShow {
		screen.SetContent(inputX+i, startY+5, r, nil, inputBgStyle)
	}

	// Draw cursor
	cursorX := inputX + m.CursorPos
	if m.CursorPos > len(m.Value) {
		cursorX = inputX + len(m.Value)
	}
	if cursorX < inputX+inputWidth {
		r := ' '
		if m.CursorPos < len(m.Value) {
			r = rune(m.Value[m.CursorPos])
		}
		screen.SetContent(cursorX, startY+5, r, nil, cursorStyle)
	}

	// Draw options (line 7)
	options := "(enter) save  (esc) cancel"
	optX := startX + (boxWidth-len(options))/2
	for i, r := range options {
		screen.SetContent(optX+i, startY+7, r, nil, optStyle)
	}
}
