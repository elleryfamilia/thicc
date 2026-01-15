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

// paneClickRegion tracks the clickable area for a pane
type paneClickRegion struct {
	StartX int
	EndX   int
	Number int // The pane number (1-5)
}

// PaneNavBar renders the pane navigation bar at the top of the screen
type PaneNavBar struct {
	Region       Region
	Manager      *LayoutManager
	clickRegions []paneClickRegion // Populated during Render
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

	// Reset click regions
	n.clickRegions = nil

	// Draw each pane entry
	panes := n.getPanes()
	for _, pane := range panes {
		startX := x // Track start of this pane's clickable region

		var style tcell.Style
		if pane.IsVisible {
			style = tcell.StyleDefault.
				Background(tcell.ColorBlack).
				Foreground(tcell.Color226) // Spider-Verse yellow
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

		// Record clickable region (before spacing)
		n.clickRegions = append(n.clickRegions, paneClickRegion{
			StartX: startX,
			EndX:   x,
			Number: pane.Number,
		})

		// Add more spacing between panes
		for i := 0; i < 4; i++ {
			screen.SetContent(x, n.Region.Y, ' ', nil, bgStyle)
			x++
		}
	}
}

// IsInNavBar returns true if the coordinates are within the nav bar
func (n *PaneNavBar) IsInNavBar(x, y int) bool {
	return y == n.Region.Y && x >= n.Region.X && x < n.Region.X+n.Region.Width
}

// GetClickedPane returns the pane number (1-5) if a pane was clicked, or 0 if not
func (n *PaneNavBar) GetClickedPane(x, y int) int {
	if y != n.Region.Y {
		return 0
	}
	for _, region := range n.clickRegions {
		if x >= region.StartX && x < region.EndX {
			return region.Number
		}
	}
	return 0
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
