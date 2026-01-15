package layout

import (
	"github.com/micro-editor/tcell/v2"
)

// Pane icons (larger Nerd Font glyphs)
const (
	PaneIconFolder   = '\uf07b' // nf-fa-folder
	PaneIconCode     = '\uf121' // nf-fa-code
	PaneIconTerminal = '\uf489' // nf-oct-terminal
)

// PaneInfo describes a single pane for the nav bar
type PaneInfo struct {
	Number    int    // Display number (1-5)
	Name      string // Display name
	Icon      rune   // Nerd Font icon
	IsVisible bool   // Current visibility state
}

// PaneNavBar renders the pane navigation bar at the top of the screen
type PaneNavBar struct {
	Region  Region
	Manager *LayoutManager
}

// NewPaneNavBar creates a new pane navigation bar
func NewPaneNavBar() *PaneNavBar {
	return &PaneNavBar{}
}

// Render draws the pane navigation bar
func (n *PaneNavBar) Render(screen tcell.Screen) {
	if n.Manager == nil {
		return
	}

	// Fill background with black
	bgStyle := tcell.StyleDefault.Background(tcell.ColorBlack)
	for x := 0; x < n.Region.Width; x++ {
		screen.SetContent(n.Region.X+x, n.Region.Y, ' ', nil, bgStyle)
	}

	x := n.Region.X

	// Draw " ALT+ " with neutral dark background
	altIndicator := " ALT+ "
	altStyle := tcell.StyleDefault.
		Background(tcell.Color236). // Dark grey
		Foreground(tcell.Color250)  // Light grey text

	for _, r := range altIndicator {
		screen.SetContent(x, n.Region.Y, r, nil, altStyle)
		x++
	}

	// Add spacing after ALT+
	x++

	// Draw each pane entry
	panes := n.getPanes()
	for _, pane := range panes {
		var style tcell.Style
		if pane.IsVisible {
			style = tcell.StyleDefault.
				Background(tcell.ColorBlack).
				Foreground(tcell.ColorWhite)
		} else {
			style = tcell.StyleDefault.
				Background(tcell.ColorBlack).
				Foreground(tcell.Color240) // Dark grey
		}

		// Draw number
		screen.SetContent(x, n.Region.Y, rune('0'+pane.Number), nil, style)
		x++

		// Space
		screen.SetContent(x, n.Region.Y, ' ', nil, bgStyle)
		x++

		// Draw icon
		screen.SetContent(x, n.Region.Y, pane.Icon, nil, style)
		x++

		// Space
		screen.SetContent(x, n.Region.Y, ' ', nil, bgStyle)
		x++

		// Draw name
		for _, r := range pane.Name {
			screen.SetContent(x, n.Region.Y, r, nil, style)
			x++
		}

		// Add more spacing between panes
		for i := 0; i < 4; i++ {
			screen.SetContent(x, n.Region.Y, ' ', nil, bgStyle)
			x++
		}
	}
}

// getPanes returns the current state of all panes
func (n *PaneNavBar) getPanes() []PaneInfo {
	return []PaneInfo{
		{1, "Files", PaneIconFolder, n.Manager.TreeVisible},
		{2, "Editor", PaneIconCode, n.Manager.EditorVisible},
		{3, "Term", PaneIconTerminal, n.Manager.TerminalVisible},
		{4, "Term", PaneIconTerminal, n.Manager.Terminal2Visible},
		{5, "Term", PaneIconTerminal, n.Manager.Terminal3Visible},
	}
}
