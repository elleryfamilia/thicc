package layout

import (
	"github.com/micro-editor/tcell/v2"
)

// LoadingOverlay provides a simple full-screen overlay with a centered message
type LoadingOverlay struct {
	Active  bool
	Message string
}

// NewLoadingOverlay creates a new loading overlay
func NewLoadingOverlay() *LoadingOverlay {
	return &LoadingOverlay{}
}

// Show displays the overlay with the given message
func (lo *LoadingOverlay) Show(message string) {
	lo.Active = true
	lo.Message = message
}

// Hide hides the overlay
func (lo *LoadingOverlay) Hide() {
	lo.Active = false
}

// Render draws the loading overlay centered on screen
func (lo *LoadingOverlay) Render(s tcell.Screen, screenW, screenH int) {
	if !lo.Active {
		return
	}

	// Semi-transparent dark background
	bgStyle := tcell.StyleDefault.Background(tcell.ColorBlack)
	for y := 0; y < screenH; y++ {
		for x := 0; x < screenW; x++ {
			s.SetContent(x, y, ' ', nil, bgStyle)
		}
	}

	// Center the message
	msgLen := len(lo.Message)
	startX := (screenW - msgLen) / 2
	startY := screenH / 2

	// Draw message with bright text
	msgStyle := tcell.StyleDefault.Foreground(tcell.ColorWhite).Bold(true)
	for i, ch := range lo.Message {
		s.SetContent(startX+i, startY, ch, nil, msgStyle)
	}
}
