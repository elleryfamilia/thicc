package layout

import (
	"log"
	"sync"

	"github.com/ellery/thock/internal/action"
	// "github.com/ellery/thock/internal/aiterminal"
	"github.com/ellery/thock/internal/buffer"
	"github.com/ellery/thock/internal/filebrowser"
	"github.com/ellery/thock/internal/terminal"
	"github.com/micro-editor/tcell/v2"
)

// Region defines a rectangular screen region
type Region struct {
	X      int
	Y      int
	Width  int
	Height int
}

// LayoutManager coordinates the 3-panel layout
type LayoutManager struct {
	// Panels
	FileBrowser *filebrowser.Panel
	Terminal    *terminal.Panel
	// Editor uses micro's existing action.Tabs (middle region)

	// Layout configuration
	TreeWidth       int // Left panel width (fixed at 30)
	TermWidthPct    int // Right panel width as percentage (40 = 40%)
	LeftPanelsPct   int // Tree + Editor width as percentage (60 = 60%)
	ScreenW         int // Total screen width
	ScreenH         int // Total screen height

	// Active panel (0=filebrowser, 1=editor, 2=terminal)
	ActivePanel int

	// Root directory for file browser
	Root string

	// Screen reference for triggering redraws
	Screen tcell.Screen

	// Mutex to protect Terminal access during async creation
	mu sync.RWMutex
}

// NewLayoutManager creates a new layout manager
func NewLayoutManager(root string) *LayoutManager {
	return &LayoutManager{
		TreeWidth:     20,               // Minimum tree width
		LeftPanelsPct: 60,               // Tree + Editor = 60% of screen
		TermWidthPct:  40,               // Terminal = 40% of screen
		Root:          root,
		ActivePanel:   1,                // Start with editor focused
	}
}

// getTreeWidth returns the tree width (percentage-based with minimum)
func (lm *LayoutManager) getTreeWidth() int {
	// Use 20% of left panels for tree, with minimum of TreeWidth
	leftWidth := lm.getLeftPanelsWidth()
	treeWidth := leftWidth * 35 / 100 // 35% of left side for tree
	if treeWidth < lm.TreeWidth {
		treeWidth = lm.TreeWidth
	}
	return treeWidth
}

// getTermWidth calculates terminal width based on percentage
func (lm *LayoutManager) getTermWidth() int {
	return lm.ScreenW * lm.TermWidthPct / 100
}

// getLeftPanelsWidth calculates tree + editor width based on percentage
func (lm *LayoutManager) getLeftPanelsWidth() int {
	return lm.ScreenW * lm.LeftPanelsPct / 100
}

// getEditorWidth calculates editor width (left panels minus tree)
func (lm *LayoutManager) getEditorWidth() int {
	return lm.getLeftPanelsWidth() - lm.getTreeWidth()
}

// Initialize creates the panels once screen size is known
func (lm *LayoutManager) Initialize(screen tcell.Screen) error {
	lm.Screen = screen
	lm.ScreenW, lm.ScreenH = screen.Size()

	log.Printf("THOCK: Initializing layout (screen: %dx%d)", lm.ScreenW, lm.ScreenH)

	// Calculate regions
	treeRegion := Region{
		X:      0,
		Y:      0,
		Width:  lm.getTreeWidth(),
		Height: lm.ScreenH,
	}

	// editorW := lm.ScreenW - lm.TreeWidth - lm.TermWidth

	// termRegion := Region{
	// 	X:      lm.TreeWidth + editorW,
	// 	Y:      0,
	// 	Width:  lm.TermWidth,
	// 	Height: lm.ScreenH,
	// }

	// Create file browser
	log.Println("THOCK: Creating file browser panel")
	lm.FileBrowser = filebrowser.NewPanel(
		treeRegion.X, treeRegion.Y,
		treeRegion.Width, treeRegion.Height,
		lm.Root,
	)

	// Set callbacks
	lm.FileBrowser.OnFileOpen = lm.previewFileInEditor // Preview without switching focus
	lm.FileBrowser.OnTreeReady = lm.triggerRedraw
	lm.FileBrowser.OnFocusEditor = lm.FocusEditor

	// Calculate terminal region (commented out while debugging)
	// editorW := lm.ScreenW - lm.TreeWidth - lm.TermWidth
	// termRegion := Region{
	// 	X:      lm.TreeWidth + editorW,
	// 	Y:      0,
	// 	Width:  lm.TermWidth,
	// 	Height: lm.ScreenH,
	// }

	// Terminal creation disabled - TODO: fix hanging issue
	log.Println("THOCK: Terminal creation disabled for debugging")
	// tool := aiterminal.QuickLaunchFirst()
	// if tool != nil {
	// 	log.Printf("THOCK: Will launch %s in terminal (async)", tool.Name)
	// 	go func() {
	// 		log.Printf("THOCK: Starting terminal with %s", tool.Name)
	// 		term, err := terminal.NewPanel(
	// 			termRegion.X, termRegion.Y,
	// 			termRegion.Width, termRegion.Height,
	// 			tool,
	// 		)
	// 		if err != nil {
	// 			log.Printf("THOCK: Failed to create terminal: %v", err)
	// 		} else {
	// 			lm.mu.Lock()
	// 			lm.Terminal = term
	// 			lm.mu.Unlock()
	// 			log.Println("THOCK: Terminal panel created successfully")
	// 		}
	// 	}()
	// } else {
	// 	log.Println("THOCK: No AI tools found, terminal panel not created")
	// }

	log.Println("THOCK: Layout initialization complete")
	return nil
}

// RenderFrame draws all 3 panels (called BEFORE editor renders)
func (lm *LayoutManager) RenderFrame(screen tcell.Screen) {
	// 1. Render file browser (left)
	if lm.FileBrowser != nil {
		lm.FileBrowser.Focus = (lm.ActivePanel == 0)
		lm.FileBrowser.Render(screen)
	}

	// 2. Editor (middle) will be rendered by micro's standard rendering
	//    in DoEvent() after this function returns
	//    Border is drawn in RenderOverlay() AFTER editor renders

	// 3. Render terminal (right)
	lm.mu.RLock()
	term := lm.Terminal
	lm.mu.RUnlock()

	if term != nil {
		term.Focus = (lm.ActivePanel == 2)
		term.Render(screen)
	}

	// 4. Draw dividers between panels
	lm.drawDividers(screen)
}

// RenderOverlay draws elements that should appear ON TOP of the editor (called AFTER editor renders)
func (lm *LayoutManager) RenderOverlay(screen tcell.Screen) {
	// Always draw editor border (bright when focused, dim when not)
	lm.drawEditorBorder(screen, lm.ActivePanel == 1)
}

// HandleEvent routes events to the active panel
func (lm *LayoutManager) HandleEvent(event tcell.Event) bool {
	// Check for focus switching keys
	if lm.handleFocusSwitch(event) {
		return true
	}

	// Route to active panel
	switch lm.ActivePanel {
	case 0: // File browser
		if lm.FileBrowser != nil {
			return lm.FileBrowser.HandleEvent(event)
		}

	case 1: // Editor (handled by micro's action.Tabs)
		return false // Let micro handle it

	case 2: // Terminal
		lm.mu.RLock()
		term := lm.Terminal
		lm.mu.RUnlock()

		if term != nil {
			return term.HandleEvent(event)
		}
	}

	return false
}

// handleFocusSwitch checks for focus switching keys
func (lm *LayoutManager) handleFocusSwitch(event tcell.Event) bool {
	// Handle mouse clicks to change focus
	if ev, ok := event.(*tcell.EventMouse); ok {
		if ev.Buttons() == tcell.Button1 {
			x, _ := ev.Position()
			newPanel := lm.panelAtX(x)
			if newPanel != lm.ActivePanel && newPanel >= 0 {
				log.Printf("THOCK: Mouse click at x=%d, switching focus from panel %d to %d", x, lm.ActivePanel, newPanel)
				lm.setActivePanel(newPanel)
				// Don't consume the event - let the panel handle the click too
			}
		}
		return false // Let the panel also handle mouse events
	}

	// Handle keyboard focus switching
	ev, ok := event.(*tcell.EventKey)
	if !ok {
		return false
	}

	// Ctrl+Space cycles focus: tree → editor → terminal → tree
	if ev.Key() == tcell.KeyCtrlSpace {
		log.Printf("THOCK: Ctrl+Space pressed, current panel: %d", lm.ActivePanel)
		lm.cycleFocus()
		log.Printf("THOCK: Focus switched to panel %d", lm.ActivePanel)
		return true
	}

	return false
}

// panelAtX returns which panel is at the given x coordinate
// Returns: 0=filebrowser, 1=editor, 2=terminal, -1=none
func (lm *LayoutManager) panelAtX(x int) int {
	treeWidth := lm.getTreeWidth()
	leftPanelsWidth := lm.getLeftPanelsWidth()

	if x < treeWidth {
		if lm.FileBrowser != nil {
			return 0
		}
	} else if x < leftPanelsWidth {
		return 1
	} else {
		lm.mu.RLock()
		hasTerminal := lm.Terminal != nil
		lm.mu.RUnlock()
		if hasTerminal {
			return 2
		}
	}
	return -1
}

// setActivePanel changes the active panel (focus states update on next render)
func (lm *LayoutManager) setActivePanel(panel int) {
	lm.ActivePanel = panel
}

// cycleFocus cycles to the next panel
func (lm *LayoutManager) cycleFocus() {
	log.Printf("THOCK cycleFocus: Starting from panel %d", lm.ActivePanel)
	// Cycle through available panels
	for i := 0; i < 3; i++ {
		lm.ActivePanel = (lm.ActivePanel + 1) % 3
		log.Printf("THOCK cycleFocus: Trying panel %d (iteration %d)", lm.ActivePanel, i)

		// Check if this panel exists
		switch lm.ActivePanel {
		case 0:
			log.Printf("THOCK cycleFocus: Checking FileBrowser (nil=%v)", lm.FileBrowser == nil)
			if lm.FileBrowser != nil {
				log.Println("THOCK cycleFocus: FileBrowser exists, returning")
				return
			}
		case 1:
			log.Println("THOCK cycleFocus: Editor panel (always exists), returning")
			return // Editor always exists
		case 2:
			log.Println("THOCK cycleFocus: Checking Terminal with lock")
			lm.mu.RLock()
			hasTerminal := lm.Terminal != nil
			lm.mu.RUnlock()
			log.Printf("THOCK cycleFocus: Terminal exists: %v", hasTerminal)

			if hasTerminal {
				log.Println("THOCK cycleFocus: Terminal exists, returning")
				return
			}
		}
	}
	log.Println("THOCK cycleFocus: Completed loop without finding panel")
}

// previewFileInEditor opens a file in the editor panel WITHOUT switching focus
func (lm *LayoutManager) previewFileInEditor(path string) {
	log.Printf("THOCK: Previewing file: %s", path)

	// Load file into micro's buffer system
	buf, err := buffer.NewBufferFromFile(path, buffer.BTDefault)
	if err != nil {
		log.Printf("THOCK: Error opening file: %v", err)
		action.InfoBar.Error("Error opening file: " + err.Error())
		return
	}

	// Add to current tab
	tab := action.MainTab()
	if tab == nil {
		log.Println("THOCK: No main tab found")
		return
	}

	// Find first BufPane in tab and open buffer there
	for _, pane := range tab.Panes {
		if bp, ok := pane.(*action.BufPane); ok {
			bp.OpenBuffer(buf)
			// Don't switch focus - stay in tree browser
			log.Printf("THOCK: File previewed in editor, focus stays on tree")
			return
		}
	}

	log.Println("THOCK: No BufPane found in tab")
}

// drawDividers draws vertical lines between panels
func (lm *LayoutManager) drawDividers(screen tcell.Screen) {
	style := tcell.StyleDefault.Foreground(tcell.ColorGray)

	// Vertical line after tree (left edge of editor)
	treeW := lm.getTreeWidth()
	for y := 0; y < lm.ScreenH; y++ {
		screen.SetContent(treeW, y, '│', nil, style)
	}

	// Vertical line before terminal (right edge of editor = left panels width)
	termX := lm.getLeftPanelsWidth()
	for y := 0; y < lm.ScreenH; y++ {
		screen.SetContent(termX, y, '│', nil, style)
	}
}

// drawEditorBorder draws a border around the editor region (pink when focused, dim when not)
func (lm *LayoutManager) drawEditorBorder(screen tcell.Screen, focused bool) {
	// Choose style based on focus state (pink for Spider-Verse vibe)
	var style tcell.Style
	if focused {
		style = tcell.StyleDefault.Foreground(tcell.Color205).Background(tcell.ColorBlack) // Hot pink
	} else {
		style = tcell.StyleDefault.Foreground(tcell.ColorGray).Background(tcell.ColorBlack)
	}

	editorX := lm.getTreeWidth()
	editorW := lm.getEditorWidth()
	h := lm.ScreenH

	// Clear the border areas first (in case editor rendered there)
	clearStyle := tcell.StyleDefault.Background(tcell.ColorBlack)
	// Top row
	for x := editorX; x < editorX+editorW; x++ {
		screen.SetContent(x, 0, ' ', nil, clearStyle)
	}
	// Bottom row
	for x := editorX; x < editorX+editorW; x++ {
		screen.SetContent(x, h-1, ' ', nil, clearStyle)
	}
	// Left column
	for y := 0; y < h; y++ {
		screen.SetContent(editorX, y, ' ', nil, clearStyle)
	}
	// Right column
	for y := 0; y < h; y++ {
		screen.SetContent(editorX+editorW-1, y, ' ', nil, clearStyle)
	}

	// Draw vertical lines on left and right (double-line when focused, single when not)
	if focused {
		for y := 1; y < h-1; y++ {
			screen.SetContent(editorX, y, '║', nil, style)
			screen.SetContent(editorX+editorW-1, y, '║', nil, style)
		}
		for x := 1; x < editorW-1; x++ {
			screen.SetContent(editorX+x, 0, '═', nil, style)
			screen.SetContent(editorX+x, h-1, '═', nil, style)
		}
		// Double-line corners
		screen.SetContent(editorX, 0, '╔', nil, style)
		screen.SetContent(editorX+editorW-1, 0, '╗', nil, style)
		screen.SetContent(editorX, h-1, '╚', nil, style)
		screen.SetContent(editorX+editorW-1, h-1, '╝', nil, style)
	} else {
		for y := 1; y < h-1; y++ {
			screen.SetContent(editorX, y, '│', nil, style)
			screen.SetContent(editorX+editorW-1, y, '│', nil, style)
		}
		for x := 1; x < editorW-1; x++ {
			screen.SetContent(editorX+x, 0, '─', nil, style)
			screen.SetContent(editorX+x, h-1, '─', nil, style)
		}
		// Single-line corners
		screen.SetContent(editorX, 0, '┌', nil, style)
		screen.SetContent(editorX+editorW-1, 0, '┐', nil, style)
		screen.SetContent(editorX, h-1, '└', nil, style)
		screen.SetContent(editorX+editorW-1, h-1, '┘', nil, style)
	}
}

// Resize handles screen resize events
func (lm *LayoutManager) Resize(w, h int) {
	lm.ScreenW = w
	lm.ScreenH = h

	// Resize file browser
	if lm.FileBrowser != nil {
		lm.FileBrowser.Region.Width = lm.getTreeWidth()
		lm.FileBrowser.Region.Height = h
	}

	// Resize terminal (starts at left panels width, takes remaining space)
	lm.mu.RLock()
	term := lm.Terminal
	lm.mu.RUnlock()

	if term != nil {
		termW := lm.getTermWidth()
		term.Region.X = lm.getLeftPanelsWidth()
		term.Region.Width = termW
		term.Region.Height = h
		_ = term.Resize(termW, h)
	}

	log.Printf("THOCK: Layout resized to %dx%d (tree=%d, editor=%d, term=%d)",
		w, h, lm.TreeWidth, lm.getEditorWidth(), lm.getTermWidth())
}

// Close cleans up resources
func (lm *LayoutManager) Close() {
	if lm.FileBrowser != nil {
		lm.FileBrowser.Close()
	}

	lm.mu.RLock()
	term := lm.Terminal
	lm.mu.RUnlock()

	if term != nil {
		term.Close()
	}

	log.Println("THOCK: Layout closed")
}

// FocusTree sets focus to file browser
func (lm *LayoutManager) FocusTree() {
	if lm.FileBrowser != nil {
		lm.ActivePanel = 0
		log.Println("THOCK: Focus set to file browser")
	}
}

// FocusEditor sets focus to editor
func (lm *LayoutManager) FocusEditor() {
	lm.ActivePanel = 1
	log.Println("THOCK: Focus set to editor")
}

// FocusTerminal sets focus to terminal
func (lm *LayoutManager) FocusTerminal() {
	lm.mu.RLock()
	hasTerminal := lm.Terminal != nil
	lm.mu.RUnlock()

	if hasTerminal {
		lm.ActivePanel = 2
		log.Println("THOCK: Focus set to terminal")
	}
}

// triggerRedraw posts a resize event to wake up the event loop and trigger a screen refresh
func (lm *LayoutManager) triggerRedraw() {
	if lm.Screen != nil {
		log.Println("THOCK: Posting resize event to trigger screen refresh")
		// Post a resize event with current dimensions to wake up event loop
		w, h := lm.Screen.Size()
		lm.Screen.PostEvent(tcell.NewEventResize(w, h))
	}
}

// ConstrainEditorRegion adjusts the editor panes to only use the middle region
func (lm *LayoutManager) ConstrainEditorRegion() {
	tab := action.MainTab()
	if tab == nil {
		return
	}

	// Calculate editor region bounds (60% left side minus tree width)
	editorX := lm.getTreeWidth()
	editorWidth := lm.getEditorWidth()

	// Always leave room for border on all sides (prevents content "jumping" on focus change)
	const borderOffset = 1

	// Adjust all BufPanes in the tab to use middle region
	for _, pane := range tab.Panes {
		if bp, ok := pane.(*action.BufPane); ok {
			// Get the underlying window
			view := bp.GetView()
			if view != nil {
				// Constrain to middle region (always with border space)
				view.X = editorX + borderOffset
				view.Width = editorWidth - (borderOffset * 2)
				view.Y = borderOffset
				view.Height = lm.ScreenH - (borderOffset * 2)
			}
		}
	}
}
