package dashboard

import (
	"fmt"

	"github.com/micro-editor/tcell/v2"
)

// guidePage represents a single page of the onboarding guide
type guidePage struct {
	title       string
	shortcuts   [][2]string // Key-value pairs like {"Alt+1", "file tree"}
	description string
}

// guidePages contains all pages of the onboarding guide
var guidePages = []guidePage{
	{
		title: "Toggle Panes",
		shortcuts: [][2]string{
			{"Alt+1", "file tree"},
			{"Alt+2", "editor"},
			{"Alt+3", "terminal 1"},
			{"Alt+4", "terminal 2"},
			{"Alt+5", "terminal 3"},
		},
		description: "Get as much or as little UI as you need.",
	},
	{
		title: "Switch Projects",
		shortcuts: [][2]string{},
		description: "Click the project name at the top of\nthe file tree to switch projects.",
	},
	{
		title: "Terminal & AI",
		shortcuts: [][2]string{},
		description: "Start your session with your favorite\nAI agent or a beautiful shell.",
	},
	{
		title: "File Management",
		shortcuts: [][2]string{
			{"Ctrl+N", "new file"},
			{"Ctrl+R", "rename"},
			{"Ctrl+D", "delete"},
		},
		description: "Create and manage files right\nin the file tree.",
	},
}

// OnboardingGuide displays a multi-page first-run guide
type OnboardingGuide struct {
	Active      bool
	CurrentPage int
	ScreenW     int
	ScreenH     int
	OnClose     func()
}

// NewOnboardingGuide creates a new onboarding guide
func NewOnboardingGuide() *OnboardingGuide {
	return &OnboardingGuide{
		CurrentPage: 0,
	}
}

// Show displays the onboarding guide
func (g *OnboardingGuide) Show(screenW, screenH int) {
	g.Active = true
	g.ScreenW = screenW
	g.ScreenH = screenH
	g.CurrentPage = 0
}

// Hide closes the guide
func (g *OnboardingGuide) Hide() {
	g.Active = false
	if g.OnClose != nil {
		g.OnClose()
	}
}

// TotalPages returns the number of pages
func (g *OnboardingGuide) TotalPages() int {
	return len(guidePages)
}

// NextPage advances to the next page or closes on last page
func (g *OnboardingGuide) NextPage() {
	if g.CurrentPage < g.TotalPages()-1 {
		g.CurrentPage++
	} else {
		g.Hide()
	}
}

// PrevPage goes to the previous page
func (g *OnboardingGuide) PrevPage() {
	if g.CurrentPage > 0 {
		g.CurrentPage--
	}
}

// HandleEvent processes keyboard events for the guide
func (g *OnboardingGuide) HandleEvent(event tcell.Event) bool {
	if !g.Active {
		return false
	}

	ev, ok := event.(*tcell.EventKey)
	if !ok {
		return true // Consume non-key events
	}

	// Navigation
	switch ev.Key() {
	case tcell.KeyCtrlQ:
		// Let Ctrl+Q pass through to exit the app
		return false
	case tcell.KeyEscape:
		g.Hide()
		return true
	case tcell.KeyEnter:
		g.NextPage()
		return true
	case tcell.KeyRight:
		g.NextPage()
		return true
	case tcell.KeyLeft:
		g.PrevPage()
		return true
	}

	// Character keys
	switch ev.Rune() {
	case ' ':
		g.NextPage()
		return true
	case 'l':
		g.NextPage()
		return true
	case 'h':
		g.PrevPage()
		return true
	case 'q':
		g.Hide()
		return true
	}

	return true // Consume all events while guide is active
}

// Render draws the guide modal centered on screen
func (g *OnboardingGuide) Render(screen tcell.Screen) {
	if !g.Active {
		return
	}

	// Fill entire screen with dark background to hide dashboard
	// Must set both fg and bg explicitly for light mode compatibility
	fullBgStyle := tcell.StyleDefault.Foreground(ColorTextBright).Background(ColorBgDark)
	for y := 0; y < g.ScreenH; y++ {
		for x := 0; x < g.ScreenW; x++ {
			screen.SetContent(x, y, ' ', nil, fullBgStyle)
		}
	}

	page := guidePages[g.CurrentPage]

	// Calculate dialog dimensions
	boxWidth := 54
	boxHeight := 20

	// Ensure it fits on screen
	if boxWidth > g.ScreenW-4 {
		boxWidth = g.ScreenW - 4
	}
	if boxHeight > g.ScreenH-4 {
		boxHeight = g.ScreenH - 4
	}

	// Center the dialog
	startX := (g.ScreenW - boxWidth) / 2
	startY := (g.ScreenH - boxHeight) / 2

	// Styles - all must have explicit foreground AND background to prevent
	// color changes in light mode terminals
	borderStyle := tcell.StyleDefault.Foreground(ColorMagenta).Background(ColorBgDark).Bold(true)
	bgStyle := tcell.StyleDefault.Foreground(ColorTextBright).Background(ColorBgDark)
	titleStyle := tcell.StyleDefault.Foreground(ColorMagenta).Background(ColorBgDark).Bold(true)
	pageTitleStyle := tcell.StyleDefault.Foreground(ColorCyan).Background(ColorBgDark).Bold(true)
	keyStyle := tcell.StyleDefault.Foreground(ColorYellow).Background(ColorBgDark).Bold(true)
	valueStyle := tcell.StyleDefault.Foreground(ColorTextDim).Background(ColorBgDark)
	descStyle := tcell.StyleDefault.Foreground(ColorTextMuted).Background(ColorBgDark)
	hintStyle := tcell.StyleDefault.Foreground(ColorTextMuted).Background(ColorBgDark)
	dotActiveStyle := tcell.StyleDefault.Foreground(ColorMagenta).Background(ColorBgDark)
	dotInactiveStyle := tcell.StyleDefault.Foreground(ColorTextMuted).Background(ColorBgDark)

	// Draw background
	for y := startY; y < startY+boxHeight; y++ {
		for x := startX; x < startX+boxWidth; x++ {
			screen.SetContent(x, y, ' ', nil, bgStyle)
		}
	}

	// Draw border (double-line style)
	// Top border
	screen.SetContent(startX, startY, '\u2554', nil, borderStyle) // ╔
	screen.SetContent(startX+boxWidth-1, startY, '\u2557', nil, borderStyle) // ╗
	for x := startX + 1; x < startX+boxWidth-1; x++ {
		screen.SetContent(x, startY, '\u2550', nil, borderStyle) // ═
	}

	// Bottom border
	screen.SetContent(startX, startY+boxHeight-1, '\u255a', nil, borderStyle) // ╚
	screen.SetContent(startX+boxWidth-1, startY+boxHeight-1, '\u255d', nil, borderStyle) // ╝
	for x := startX + 1; x < startX+boxWidth-1; x++ {
		screen.SetContent(x, startY+boxHeight-1, '\u2550', nil, borderStyle) // ═
	}

	// Side borders
	for y := startY + 1; y < startY+boxHeight-1; y++ {
		screen.SetContent(startX, y, '\u2551', nil, borderStyle) // ║
		screen.SetContent(startX+boxWidth-1, y, '\u2551', nil, borderStyle) // ║
	}

	// Draw header title with page number
	headerTitle := fmt.Sprintf("Getting Started (%d/%d)", g.CurrentPage+1, g.TotalPages())
	headerX := startX + (boxWidth-len(headerTitle))/2
	g.drawText(screen, headerX, startY+1, headerTitle, titleStyle)

	// Draw separator line
	for x := startX + 1; x < startX+boxWidth-1; x++ {
		screen.SetContent(x, startY+2, '\u2500', nil, borderStyle) // ─
	}

	// Content area starts at line 4 (after header, separator, and blank line)
	contentY := startY + 4

	// Draw page title (centered)
	pageTitleX := startX + (boxWidth-len(page.title))/2
	g.drawText(screen, pageTitleX, contentY, page.title, pageTitleStyle)
	contentY += 2

	// Draw shortcuts
	contentX := startX + 4
	for _, shortcut := range page.shortcuts {
		key := shortcut[0]
		value := shortcut[1]

		// Draw key (right-aligned in a fixed column)
		keyWidth := 14
		keyX := contentX + keyWidth - len(key)
		g.drawText(screen, keyX, contentY, key, keyStyle)

		// Draw value
		g.drawText(screen, contentX+keyWidth+2, contentY, value, valueStyle)
		contentY++
	}

	contentY++ // Blank line

	// Draw description (may be multi-line)
	descLines := splitLines(page.description)
	for _, line := range descLines {
		g.drawText(screen, contentX, contentY, line, descStyle)
		contentY++
	}

	// Draw page dots (centered, near bottom)
	dotsY := startY + boxHeight - 4
	// Draw dots with appropriate styles
	dotX := startX + (boxWidth-(g.TotalPages()*2-1))/2
	for i := 0; i < g.TotalPages(); i++ {
		style := dotInactiveStyle
		if i == g.CurrentPage {
			style = dotActiveStyle
		}
		if i == g.CurrentPage {
			screen.SetContent(dotX, dotsY, '\u25cf', nil, style) // ●
		} else {
			screen.SetContent(dotX, dotsY, '\u25cb', nil, style) // ○
		}
		dotX += 2
	}

	// Draw footer hints
	hintY := startY + boxHeight - 2
	var hint string
	if g.CurrentPage < g.TotalPages()-1 {
		hint = "[Enter] Next  [Esc] Close"
	} else {
		hint = "[Enter] Done  [Esc] Close"
	}
	hintX := startX + (boxWidth-len(hint))/2
	g.drawText(screen, hintX, hintY, hint, hintStyle)
}

// drawText draws a string at the given position
func (g *OnboardingGuide) drawText(screen tcell.Screen, x, y int, text string, style tcell.Style) {
	for i, ch := range text {
		if x+i >= 0 && x+i < g.ScreenW && y >= 0 && y < g.ScreenH {
			screen.SetContent(x+i, y, ch, nil, style)
		}
	}
}

// splitLines splits a string by newlines
func splitLines(s string) []string {
	var lines []string
	current := ""
	for _, ch := range s {
		if ch == '\n' {
			lines = append(lines, current)
			current = ""
		} else {
			current += string(ch)
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}
