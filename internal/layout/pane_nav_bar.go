package layout

import (
	"log"
	"sync"
	"time"

	"github.com/ellery/thicc/internal/sourcecontrol"
	"github.com/micro-editor/tcell/v2"
)

// Pane icons (larger Nerd Font glyphs)
const (
	PaneIconFolder        = '\uef81' // nf-md-folder
	PaneIconSourceControl = '\U000F02A2' // nf-md-git
	PaneIconCode          = '\U000F0169' // nf-md-application_brackets_outline
	PaneIconTerminal      = '\uf489' // nf-oct-terminal
)

// PaneInfo describes a single pane for the nav bar
type PaneInfo struct {
	Key       string // Display key (1-5 or 'a')
	Name      string // Display name
	Icon      rune   // Nerd Font icon
	IsVisible bool   // Current visibility state
}

// paneClickRegion tracks the clickable area for a pane
type paneClickRegion struct {
	StartX int
	EndX   int
	Key    string // The pane key (1-5 or 'a')
}

// PaneNavBar renders the pane navigation bar at the top of the screen
type PaneNavBar struct {
	Region       Region
	Manager      *LayoutManager
	clickRegions []paneClickRegion // Populated during Render

	// PR Meter state (independent of SourceControl panel)
	prMeterMu     sync.RWMutex
	prMeter       *sourcecontrol.PRMeterState // Current meter state
	prMeterRoot   string                      // Git repo root for PR meter
	prPollStop    chan struct{}               // Stop channel for PR meter polling
	prPolling     bool                        // Whether PR meter polling is active

	// Animation state for PR meter Pac-Man
	animMu          sync.Mutex
	animTick        int           // Current animation frame (for bounce)
	animDirection   int           // 1 = right, -1 = left (for bounce)
	animating       bool          // Whether bounce animation is running
	animStop        chan struct{} // Stop channel for animation goroutine

	// Eating animation state (when PR grows or on startup)
	eatingAnimating  bool  // Whether eating animation is running
	eatingCurrent    int   // Current "eaten" position during animation
	eatingTarget     int   // Target eaten pellets to reach
	eatingStop       chan struct{}
	lastKnownEaten   int   // Last known eaten count (to detect increases)
	initialized      bool  // Whether we've received first meter data
}

// NewPaneNavBar creates a new pane navigation bar
func NewPaneNavBar() *PaneNavBar {
	return &PaneNavBar{
		animDirection:  1,
		animStop:       make(chan struct{}),
		eatingStop:     make(chan struct{}),
		prPollStop:     make(chan struct{}),
		eatingCurrent:  0,
		lastKnownEaten: 0,
		initialized:    false,
	}
}

// InitPRMeter initializes the PR meter with the given git repo root
func (n *PaneNavBar) InitPRMeter(repoRoot string) {
	n.prMeterMu.Lock()
	n.prMeterRoot = repoRoot
	n.prMeterMu.Unlock()

	// Do initial refresh
	n.RefreshPRMeter()

	// Start polling
	n.StartPRMeterPolling()
}

// RefreshPRMeter updates the PR meter state from git
func (n *PaneNavBar) RefreshPRMeter() {
	n.prMeterMu.RLock()
	root := n.prMeterRoot
	n.prMeterMu.RUnlock()

	if root == "" {
		return
	}

	stats, err := sourcecontrol.GetPRDiffStats(root)
	if err != nil {
		log.Printf("THICC PRMeter: GetPRDiffStats error: %v", err)
		return
	}

	meter := sourcecontrol.CalculateMeterState(stats)

	n.prMeterMu.Lock()
	n.prMeter = meter
	n.prMeterMu.Unlock()

	log.Printf("THICC PRMeter: Refreshed - rawLines=%d, weightedLines=%d, patience=%.2f, eatenPellets=%d",
		meter.RawLines, meter.WeightedLines, meter.Patience, meter.EatenPellets)

	// Trigger redraw
	if n.Manager != nil {
		n.Manager.triggerRedraw()
	}
}

// GetPRMeter returns the current PR meter state
func (n *PaneNavBar) GetPRMeter() *sourcecontrol.PRMeterState {
	n.prMeterMu.RLock()
	defer n.prMeterMu.RUnlock()
	return n.prMeter
}

// StartPRMeterPolling starts periodic PR meter refresh
func (n *PaneNavBar) StartPRMeterPolling() {
	n.prMeterMu.Lock()
	if n.prPolling {
		n.prMeterMu.Unlock()
		return
	}
	n.prPolling = true
	n.prPollStop = make(chan struct{})
	n.prMeterMu.Unlock()

	go func() {
		ticker := time.NewTicker(5 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-n.prPollStop:
				return
			case <-ticker.C:
				n.RefreshPRMeter()
			}
		}
	}()
}

// StopPRMeterPolling stops the PR meter polling
func (n *PaneNavBar) StopPRMeterPolling() {
	n.prMeterMu.Lock()
	defer n.prMeterMu.Unlock()

	if !n.prPolling {
		return
	}
	n.prPolling = false
	close(n.prPollStop)
}

// StartAnimation starts the Pac-Man animation when AI is working
func (n *PaneNavBar) StartAnimation() {
	n.animMu.Lock()
	if n.animating {
		n.animMu.Unlock()
		return
	}
	n.animating = true
	n.animStop = make(chan struct{})
	n.animMu.Unlock()

	go n.animationLoop()
}

// StopAnimation stops the Pac-Man animation
func (n *PaneNavBar) StopAnimation() {
	n.animMu.Lock()
	defer n.animMu.Unlock()

	if !n.animating {
		return
	}
	n.animating = false
	close(n.animStop)
}

// animationLoop runs the animation ticker
func (n *PaneNavBar) animationLoop() {
	ticker := time.NewTicker(120 * time.Millisecond) // ~8 fps for smooth bouncing
	defer ticker.Stop()

	for {
		select {
		case <-n.animStop:
			return
		case <-ticker.C:
			n.animMu.Lock()
			// Update animation tick (bounce within eaten area)
			n.animTick += n.animDirection

			// Get current meter state to know bounds
			var maxTick int
			if meter := n.GetPRMeter(); meter != nil && meter.EatenPellets > 0 {
				maxTick = meter.EatenPellets - 1
			}

			// Bounce at edges
			if n.animTick >= maxTick {
				n.animTick = maxTick
				n.animDirection = -1
			} else if n.animTick <= 0 {
				n.animTick = 0
				n.animDirection = 1
			}
			n.animMu.Unlock()

			// Trigger redraw
			if n.Manager != nil {
				n.Manager.triggerRedraw()
			}
		}
	}
}

// UpdateAnimationState checks if AI is active and starts/stops animation accordingly
func (n *PaneNavBar) UpdateAnimationState() {
	if n.Manager == nil {
		return
	}

	aiActive := n.Manager.HasActiveAISession()

	n.animMu.Lock()
	isAnimating := n.animating
	n.animMu.Unlock()

	if aiActive && !isAnimating {
		n.StartAnimation()
	} else if !aiActive && isAnimating {
		n.StopAnimation()
	}
}

// StartEatingAnimation starts the eating animation to consume pellets
func (n *PaneNavBar) StartEatingAnimation(target int) {
	n.animMu.Lock()
	if n.eatingAnimating {
		// Already eating, just update target if it's higher
		if target > n.eatingTarget {
			n.eatingTarget = target
		}
		n.animMu.Unlock()
		return
	}
	n.eatingAnimating = true
	n.eatingTarget = target
	n.eatingStop = make(chan struct{})
	n.animMu.Unlock()

	go n.eatingLoop()
}

// StopEatingAnimation stops the eating animation
func (n *PaneNavBar) StopEatingAnimation() {
	n.animMu.Lock()
	defer n.animMu.Unlock()

	if !n.eatingAnimating {
		return
	}
	n.eatingAnimating = false
	close(n.eatingStop)
}

// eatingLoop animates Pac-Man eating pellets one by one
func (n *PaneNavBar) eatingLoop() {
	log.Printf("THICC PRMeter: eatingLoop started, target=%d", n.eatingTarget)
	ticker := time.NewTicker(160 * time.Millisecond) // Satisfying chomp pace
	defer ticker.Stop()

	for {
		select {
		case <-n.eatingStop:
			log.Println("THICC PRMeter: eatingLoop stopped via channel")
			return
		case <-ticker.C:
			n.animMu.Lock()
			if n.eatingCurrent < n.eatingTarget {
				n.eatingCurrent++
				log.Printf("THICC PRMeter: Eating tick - current=%d, target=%d", n.eatingCurrent, n.eatingTarget)
			}

			// Check if we've reached the target
			done := n.eatingCurrent >= n.eatingTarget
			n.animMu.Unlock()

			// Trigger redraw
			if n.Manager != nil {
				n.Manager.triggerRedraw()
			}

			if done {
				log.Printf("THICC PRMeter: Eating animation complete at %d", n.eatingCurrent)
				n.animMu.Lock()
				n.eatingAnimating = false
				n.animMu.Unlock()
				return
			}
		}
	}
}

// CheckAndStartEatingAnimation checks if we need to eat more pellets and starts animation
func (n *PaneNavBar) CheckAndStartEatingAnimation(newEatenCount int) {
	n.animMu.Lock()

	if !n.initialized {
		// First time receiving data - animate from 0 to current
		n.initialized = true
		n.lastKnownEaten = 0
		log.Printf("THICC PRMeter: First initialization, will animate from 0 to %d", newEatenCount)
	}

	needsEating := newEatenCount > n.eatingCurrent
	currentEating := n.eatingCurrent
	n.animMu.Unlock()

	log.Printf("THICC PRMeter: CheckAndStartEating - new=%d, current=%d, needsEating=%v",
		newEatenCount, currentEating, needsEating)

	if needsEating && newEatenCount > currentEating {
		log.Printf("THICC PRMeter: Starting eating animation from %d to %d", currentEating, newEatenCount)
		n.StartEatingAnimation(newEatenCount)
	}

	n.animMu.Lock()
	n.lastKnownEaten = newEatenCount
	n.animMu.Unlock()
}

// GetDisplayedEatenCount returns the eaten count to display (animated or actual)
func (n *PaneNavBar) GetDisplayedEatenCount(actualEaten int) int {
	n.animMu.Lock()
	defer n.animMu.Unlock()

	if n.eatingAnimating || n.eatingCurrent < actualEaten {
		return n.eatingCurrent
	}
	return actualEaten
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

		// Draw key (number or letter)
		for _, r := range pane.Key {
			screen.SetContent(x, n.Region.Y, r, nil, style)
			x++
		}

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
			Key:    pane.Key,
		})

		// Add more spacing between panes
		for i := 0; i < 4; i++ {
			screen.SetContent(x, n.Region.Y, ' ', nil, bgStyle)
			x++
		}
	}

	// Render PR Size Meter on the right side
	n.renderPRMeter(screen)
}

// IsInNavBar returns true if the coordinates are within the nav bar
func (n *PaneNavBar) IsInNavBar(x, y int) bool {
	return y == n.Region.Y && x >= n.Region.X && x < n.Region.X+n.Region.Width
}

// GetClickedPane returns the pane number (1-5) if a pane was clicked, or 0 if not
// Returns 6 for source control (key "a")
func (n *PaneNavBar) GetClickedPane(x, y int) int {
	if y != n.Region.Y {
		return 0
	}
	for _, region := range n.clickRegions {
		if x >= region.StartX && x < region.EndX {
			switch region.Key {
			case "1":
				return 1
			case "2":
				return 2
			case "3":
				return 3
			case "4":
				return 4
			case "5":
				return 5
			case "a":
				return 6 // Source Control
			}
		}
	}
	return 0
}

// getPanes returns the current state of all panes
func (n *PaneNavBar) getPanes() []PaneInfo {
	return []PaneInfo{
		{"1", "Files", PaneIconFolder, n.Manager.TreeVisible},
		{"a", "Git", PaneIconSourceControl, n.Manager.SourceControlVisible},
		{"2", "Editor", PaneIconCode, n.Manager.EditorVisible},
		{"3", "Term", PaneIconTerminal, n.Manager.TerminalVisible},
		{"4", "Term", PaneIconTerminal, n.Manager.Terminal2Visible},
		{"5", "Term", PaneIconTerminal, n.Manager.Terminal3Visible},
	}
}

// PR Meter constants
const (
	PacmanChar     = 'ᗧ'  // Pac-Man character (U+15E7)
	FilledPellet   = '●'  // Filled pellet (not eaten)
	EatenPellet    = '·'  // Eaten pellet
	TotalPellets   = 16   // Fixed number of pellets
)

// getLabelColor returns the color for the "PR" label based on patience percentage
// Gradual transition: green (healthy) → yellow → orange → red (danger)
func getLabelColor(patience float64) tcell.Color {
	if patience >= 0.75 {
		return tcell.Color46 // Bright green - very healthy
	} else if patience >= 0.60 {
		return tcell.Color118 // Yellow-green - healthy
	} else if patience >= 0.45 {
		return tcell.Color226 // Yellow - getting there
	} else if patience >= 0.30 {
		return tcell.Color214 // Orange - caution
	} else if patience >= 0.15 {
		return tcell.Color208 // Dark orange - warning
	}
	return tcell.Color196 // Red - danger
}

// renderPRMeter draws the Pac-Man PR size meter on the right side of the nav bar
func (n *PaneNavBar) renderPRMeter(screen tcell.Screen) {
	// Get the PR meter state (from nav bar's own state)
	meter := n.GetPRMeter()
	if meter == nil {
		// No meter data yet, show full patience (all pellets)
		meter = &sourcecontrol.PRMeterState{
			Patience:     1.0,
			TotalPellets: TotalPellets,
			EatenPellets: 0,
		}
	}

	// Check if we need to start eating animation (new pellets to consume)
	n.CheckAndStartEatingAnimation(meter.EatenPellets)

	// Check and update bounce animation state (for when AI is working)
	n.UpdateAnimationState()

	// Calculate meter width: "PR " label + eaten + pacman + remaining + spacing
	// Each pellet takes 2 cells (character + space)
	// Format: "PR · · · · ᗧ ● ● ● ● ●"
	labelWidth := 3 // "PR "
	meterWidth := labelWidth + (TotalPellets * 2) + 2 // label + pellets with spaces + pacman + final space

	// Position meter on the right side with some padding
	rightPadding := 2
	startX := n.Region.X + n.Region.Width - meterWidth - rightPadding

	if startX < n.Region.X {
		return // Not enough room
	}

	// Styles
	bgStyle := tcell.StyleDefault.Background(tcell.ColorBlack)
	labelColor := getLabelColor(meter.Patience)
	labelStyle := tcell.StyleDefault.
		Background(tcell.ColorBlack).
		Foreground(labelColor).
		Bold(true) // Bold label for visibility
	pacmanStyle := tcell.StyleDefault.
		Background(tcell.ColorBlack).
		Foreground(tcell.Color205) // Hot pink
	eatenStyle := tcell.StyleDefault.
		Background(tcell.ColorBlack).
		Foreground(tcell.Color240) // Dark grey
	filledStyle := tcell.StyleDefault.
		Background(tcell.ColorBlack).
		Foreground(tcell.Color226) // Always yellow pellets

	x := startX
	y := n.Region.Y

	// Draw "PR " label
	for _, r := range "PR " {
		screen.SetContent(x, y, r, nil, labelStyle)
		x++
	}

	// Get the displayed eaten count (may be mid-animation)
	displayedEaten := n.GetDisplayedEatenCount(meter.EatenPellets)

	// Determine Pac-Man position for animation
	// When AI is animating (bounce), Pac-Man bounces in the eaten area
	// Otherwise, Pac-Man sits at the edge (after eaten pellets, before filled ones)
	n.animMu.Lock()
	isBouncing := n.animating
	bouncePosition := n.animTick
	isEating := n.eatingAnimating
	n.animMu.Unlock()

	// pacmanAfterEaten: how many eaten pellets to draw before Pac-Man
	// Default: all displayed eaten pellets (Pac-Man at edge, eating forward)
	// When bouncing: Pac-Man bounces within eaten area
	pacmanAfterEaten := displayedEaten

	// Only bounce if not currently eating and there's room to bounce
	if isBouncing && !isEating && displayedEaten > 1 {
		// Pac-Man can bounce between position 0 and displayedEaten-1
		pacmanAfterEaten = bouncePosition
		if pacmanAfterEaten > displayedEaten-1 {
			pacmanAfterEaten = displayedEaten - 1
		}
		if pacmanAfterEaten < 0 {
			pacmanAfterEaten = 0
		}
	}

	// Draw eaten pellets before Pac-Man
	for i := 0; i < pacmanAfterEaten; i++ {
		screen.SetContent(x, y, EatenPellet, nil, eatenStyle)
		x++
		screen.SetContent(x, y, ' ', nil, bgStyle)
		x++
	}

	// Draw Pac-Man
	screen.SetContent(x, y, PacmanChar, nil, pacmanStyle)
	x++
	screen.SetContent(x, y, ' ', nil, bgStyle)
	x++

	// Draw remaining eaten pellets after Pac-Man (when bouncing)
	for i := pacmanAfterEaten; i < displayedEaten; i++ {
		screen.SetContent(x, y, EatenPellet, nil, eatenStyle)
		x++
		screen.SetContent(x, y, ' ', nil, bgStyle)
		x++
	}

	// Draw filled pellets (based on displayed eaten, not actual)
	remaining := TotalPellets - displayedEaten
	for i := 0; i < remaining; i++ {
		screen.SetContent(x, y, FilledPellet, nil, filledStyle)
		x++
		screen.SetContent(x, y, ' ', nil, bgStyle)
		x++
	}
}
