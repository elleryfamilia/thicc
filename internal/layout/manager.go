package layout

import (
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/ellery/thock/internal/action"
	"github.com/ellery/thock/internal/buffer"
	"github.com/ellery/thock/internal/clipboard"
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

	// Modal dialogs for prompts
	Modal        *Modal
	InputModal   *InputModal
	ConfirmModal *ConfirmModal

	// Tab bar for showing open files
	TabBar *TabBar

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
		TreeWidth:     20,                 // Minimum tree width
		LeftPanelsPct: 60,                 // Tree + Editor = 60% of screen
		TermWidthPct:  40,                 // Terminal = 40% of screen
		Root:          root,
		ActivePanel:   1,                  // Start with editor focused
		Modal:         NewModal(),
		InputModal:    NewInputModal(),
		ConfirmModal:  NewConfirmModal(),
		TabBar:        NewTabBar(),
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
	lm.FileBrowser.OnFileSaved = func(path string) {
		log.Printf("THOCK: File saved callback - refreshing tree and selecting: %s", path)
		lm.FileBrowser.SelectFile(path)
		lm.triggerRedraw()
	}

	// Delete confirmation callback
	lm.FileBrowser.OnDeleteRequest = func(path string, isDir bool, callback func(bool)) {
		itemType := "file"
		if isDir {
			itemType = "folder"
		}
		name := filepath.Base(path)
		lm.ShowConfirmModal(
			" Delete "+itemType,
			"Delete \""+name+"\"?",
			"This action cannot be undone!",
			func(confirmed bool) {
				if confirmed {
					log.Printf("THOCK: Deleting %s: %s", itemType, path)
					err := os.RemoveAll(path)
					if err != nil {
						action.InfoBar.Error("Delete failed: " + err.Error())
					} else {
						lm.ShowTimedMessage("Deleted "+name, 3*time.Second)

						// If the deleted file was open in the editor, clear it
						lm.clearEditorIfPathDeleted(path, isDir)
					}
				}
				callback(confirmed)
				lm.triggerRedraw()
			},
		)
	}

	// Rename input callback
	lm.FileBrowser.OnRenameRequest = func(oldPath string, callback func(string)) {
		currentName := filepath.Base(oldPath)
		lm.ShowInputModal("Rename", "New name:", currentName, func(newName string, canceled bool) {
			if canceled || newName == "" || newName == currentName {
				return
			}

			newPath := filepath.Join(filepath.Dir(oldPath), newName)
			log.Printf("THOCK: Renaming %s -> %s", oldPath, newPath)

			err := os.Rename(oldPath, newPath)
			if err != nil {
				action.InfoBar.Error("Rename failed: " + err.Error())
				return
			}

			lm.ShowTimedMessage("Renamed to "+newName, 3*time.Second)
			lm.FileBrowser.Tree.Refresh()
			lm.FileBrowser.Tree.SelectPath(newPath)
			lm.triggerRedraw()

			callback(newName)
		})
	}

	// New file callback
	lm.FileBrowser.OnNewFileRequest = func(dirPath string, callback func(string)) {
		lm.ShowInputModal("New File", "File name:", "", func(fileName string, canceled bool) {
			if canceled || fileName == "" {
				return
			}

			newPath := filepath.Join(dirPath, fileName)
			log.Printf("THOCK: Creating new file: %s", newPath)

			// Create the file
			file, err := os.Create(newPath)
			if err != nil {
				action.InfoBar.Error("Create file failed: " + err.Error())
				return
			}
			file.Close()

			lm.ShowTimedMessage("Created "+fileName, 3*time.Second)

			// Expand the parent directory and refresh
			lm.FileBrowser.Tree.ExpandedPaths[dirPath] = true
			lm.FileBrowser.Tree.Refresh()
			lm.FileBrowser.Tree.SelectPath(newPath)
			lm.triggerRedraw()

			// Open the new file in editor
			lm.previewFileInEditor(newPath)

			callback(fileName)
		})
	}

	// New folder callback
	lm.FileBrowser.OnNewFolderRequest = func(dirPath string, callback func(string)) {
		lm.ShowInputModal("New Folder", "Folder name:", "", func(folderName string, canceled bool) {
			if canceled || folderName == "" {
				return
			}

			newPath := filepath.Join(dirPath, folderName)
			log.Printf("THOCK: Creating new folder: %s", newPath)

			// Create the folder
			err := os.MkdirAll(newPath, 0755)
			if err != nil {
				action.InfoBar.Error("Create folder failed: " + err.Error())
				return
			}

			lm.ShowTimedMessage("Created "+folderName+"/", 3*time.Second)

			// Expand the parent directory and refresh
			lm.FileBrowser.Tree.ExpandedPaths[dirPath] = true
			lm.FileBrowser.Tree.Refresh()
			lm.FileBrowser.Tree.SelectPath(newPath)
			lm.triggerRedraw()

			callback(folderName)
		})
	}

	// Calculate terminal region
	termX := lm.getLeftPanelsWidth()
	termW := lm.getTermWidth()

	// Create terminal asynchronously (to avoid blocking UI)
	log.Println("THOCK: Creating terminal panel asynchronously")
	go func() {
		// Use default shell (nil means NewPanel will use $SHELL or /bin/bash)
		term, err := terminal.NewPanel(termX, 0, termW, lm.ScreenH, nil)
		if err != nil {
			log.Printf("THOCK: Failed to create terminal: %v", err)
			return
		}

		// Set callback to trigger screen refresh when terminal has new output
		term.OnRedraw = lm.triggerRedraw

		lm.mu.Lock()
		lm.Terminal = term
		lm.mu.Unlock()

		log.Println("THOCK: Terminal panel created successfully")
		lm.triggerRedraw()
	}()

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

	// Draw tab bar below top border
	if lm.TabBar != nil {
		tab := action.MainTab()
		if tab != nil {
			for _, pane := range tab.Panes {
				if bp, ok := pane.(*action.BufPane); ok {
					tabInfo := lm.TabBar.GetCurrentTab(bp)
					lm.TabBar.Region = Region{
						X:      lm.getTreeWidth() + 1,
						Y:      1, // Below top border
						Width:  lm.getEditorWidth() - 2,
						Height: 1,
					}
					lm.TabBar.Focused = (lm.ActivePanel == 1)
					lm.TabBar.Render(screen, tabInfo)
					break
				}
			}
		}
	}

	// Draw modal dialog on top of everything
	if lm.Modal != nil && lm.Modal.Active {
		lm.Modal.Render(screen)
	}

	// Draw input modal on top of everything
	if lm.InputModal != nil && lm.InputModal.Active {
		lm.InputModal.Render(screen)
	}

	// Draw confirm modal on top of everything (dangerous actions)
	if lm.ConfirmModal != nil && lm.ConfirmModal.Active {
		lm.ConfirmModal.Render(screen)
	}
}

// HandleEvent routes events to the active panel
func (lm *LayoutManager) HandleEvent(event tcell.Event) bool {
	// Handle modal dialog first (takes priority over everything)
	if lm.Modal != nil && lm.Modal.Active {
		return lm.Modal.HandleEvent(event)
	}

	// Handle input modal second (also takes priority)
	if lm.InputModal != nil && lm.InputModal.Active {
		return lm.InputModal.HandleEvent(event)
	}

	// Handle confirm modal (dangerous actions)
	if lm.ConfirmModal != nil && lm.ConfirmModal.Active {
		return lm.ConfirmModal.HandleEvent(event)
	}

	// Handle paste for terminal BEFORE other routing
	// This intercepts Cmd-V, Ctrl-V, and EventPaste when terminal has focus
	if lm.ActivePanel == 2 {
		if lm.handleTerminalPaste(event) {
			return true
		}
	}

	// Handle tab bar mouse clicks
	if ev, ok := event.(*tcell.EventMouse); ok {
		if ev.Buttons() == tcell.Button1 {
			x, y := ev.Position()
			if lm.TabBar != nil && lm.TabBar.IsCloseButtonClick(x, y) {
				log.Println("THOCK: Tab close button clicked")
				lm.CloseCurrentTab()
				return true
			}
		}
	}

	// Log all key events for debugging
	if ev, ok := event.(*tcell.EventKey); ok {
		log.Printf("THOCK HandleEvent: Key event, ActivePanel=%d, Key=%v, Rune=%c", lm.ActivePanel, ev.Key(), ev.Rune())

		// Ctrl+Q should always quit, regardless of which panel is focused
		if ev.Key() == tcell.KeyCtrlQ {
			log.Println("THOCK: Ctrl+Q detected, checking for unsaved changes")
			return lm.handleQuit()
		}

		// Ctrl+S for save (with modal for new files) when editor is focused
		if ev.Key() == tcell.KeyCtrlS && lm.ActivePanel == 1 {
			log.Println("THOCK: Ctrl+S detected, saving current buffer")
			return lm.SaveCurrentBuffer()
		}

		// Global file operations (work from any panel when FileBrowser exists)
		if lm.FileBrowser != nil {
			switch ev.Key() {
			case tcell.KeyCtrlN:
				// Ctrl+N = new file, Ctrl+Shift+N = new folder
				if ev.Modifiers()&tcell.ModShift != 0 {
					log.Println("THOCK: Ctrl+Shift+N detected, creating new folder")
					lm.FileBrowser.NewFolderSelected()
				} else {
					log.Println("THOCK: Ctrl+N detected, creating new file")
					lm.FileBrowser.NewFileSelected()
				}
				return true

			case tcell.KeyCtrlD:
				log.Println("THOCK: Ctrl+D detected, deleting selected")
				lm.FileBrowser.DeleteSelected()
				return true

			case tcell.KeyCtrlR:
				log.Println("THOCK: Ctrl+R detected, renaming selected")
				lm.FileBrowser.RenameSelected()
				return true
			}
		}
	}

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
			// For key events, ALWAYS consume them when terminal has focus
			// This prevents micro from intercepting paste/other keybindings
			if _, isKey := event.(*tcell.EventKey); isKey {
				term.HandleEvent(event)
				return true // Always consume key events for terminal
			}
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

// setActivePanel changes the active panel and updates Focus flags immediately
func (lm *LayoutManager) setActivePanel(panel int) {
	lm.ActivePanel = panel

	// Update Focus flags immediately (don't wait for render)
	if lm.FileBrowser != nil {
		lm.FileBrowser.Focus = (panel == 0)
	}

	lm.mu.RLock()
	term := lm.Terminal
	lm.mu.RUnlock()
	if term != nil {
		term.Focus = (panel == 2)
	}
}

// cycleFocus cycles to the next panel
func (lm *LayoutManager) cycleFocus() {
	log.Printf("THOCK cycleFocus: Starting from panel %d", lm.ActivePanel)
	// Cycle through available panels
	for i := 0; i < 3; i++ {
		nextPanel := (lm.ActivePanel + 1) % 3
		log.Printf("THOCK cycleFocus: Trying panel %d (iteration %d)", nextPanel, i)

		// Check if this panel exists
		switch nextPanel {
		case 0:
			log.Printf("THOCK cycleFocus: Checking FileBrowser (nil=%v)", lm.FileBrowser == nil)
			if lm.FileBrowser != nil {
				log.Println("THOCK cycleFocus: FileBrowser exists, setting active")
				lm.setActivePanel(nextPanel)
				return
			}
		case 1:
			log.Println("THOCK cycleFocus: Editor panel (always exists), setting active")
			lm.setActivePanel(nextPanel)
			return // Editor always exists
		case 2:
			log.Println("THOCK cycleFocus: Checking Terminal with lock")
			lm.mu.RLock()
			hasTerminal := lm.Terminal != nil
			lm.mu.RUnlock()
			log.Printf("THOCK cycleFocus: Terminal exists: %v", hasTerminal)

			if hasTerminal {
				log.Println("THOCK cycleFocus: Terminal exists, setting active")
				lm.setActivePanel(nextPanel)
				return
			}
		}
		// Panel doesn't exist, update ActivePanel to continue cycling
		lm.ActivePanel = nextPanel
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

	// Only draw divider before terminal if terminal doesn't exist
	// (terminal draws its own border which serves as the divider)
	lm.mu.RLock()
	hasTerminal := lm.Terminal != nil
	lm.mu.RUnlock()

	if !hasTerminal {
		termX := lm.getLeftPanelsWidth()
		for y := 0; y < lm.ScreenH; y++ {
			screen.SetContent(termX, y, '│', nil, style)
		}
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
		lm.setActivePanel(0)
		log.Println("THOCK: Focus set to file browser")
	}
}

// FocusEditor sets focus to editor
func (lm *LayoutManager) FocusEditor() {
	lm.setActivePanel(1)
	log.Println("THOCK: Focus set to editor")
}

// FocusTerminal sets focus to terminal
func (lm *LayoutManager) FocusTerminal() {
	lm.mu.RLock()
	hasTerminal := lm.Terminal != nil
	lm.mu.RUnlock()

	if hasTerminal {
		lm.setActivePanel(2)
		log.Println("THOCK: Focus set to terminal")
	}
}

// handleQuit handles the quit action with modal for unsaved changes
func (lm *LayoutManager) handleQuit() bool {
	// Check if any buffer has unsaved changes
	tab := action.MainTab()
	if tab == nil {
		return false
	}

	// Find the first BufPane with unsaved changes
	for _, pane := range tab.Panes {
		if bp, ok := pane.(*action.BufPane); ok {
			if bp.Buf.Modified() {
				// Show modal for unsaved changes
				bufName := bp.Buf.GetName()
				if bufName == "" {
					bufName = "Untitled"
				}
				lm.ShowModal("Unsaved Changes",
					"Save changes to "+bufName+"?",
					func(yes, canceled bool) {
						if canceled {
							// User pressed Esc, don't quit
							return
						}
						if yes {
							// Save then quit
							bp.Save()
						}
						// Force quit (whether saved or user said no)
						bp.ForceQuit()
					})
				return true
			}
			// No unsaved changes, just quit
			bp.ForceQuit()
			return true
		}
	}
	return false
}

// ShowModal displays a modal yes/no dialog
func (lm *LayoutManager) ShowModal(title, message string, callback func(yes, canceled bool)) {
	if lm.Modal != nil {
		lm.Modal.Show(title, message, "(y)es  (n)o  (esc)ape", lm.ScreenW, lm.ScreenH, callback)
	}
}

// IsModalActive returns true if a modal dialog is currently shown
func (lm *LayoutManager) IsModalActive() bool {
	return lm.Modal != nil && lm.Modal.Active
}

// ShowTerminalCursor shows the terminal's cursor if terminal is focused and has a visible cursor
func (lm *LayoutManager) ShowTerminalCursor(screen tcell.Screen) {
	lm.mu.RLock()
	term := lm.Terminal
	lm.mu.RUnlock()

	if term != nil && term.Focus {
		term.ShowCursor(screen)
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

// handleTerminalPaste intercepts paste commands when terminal has focus
// Returns true if event was a paste that was handled
func (lm *LayoutManager) handleTerminalPaste(event tcell.Event) bool {
	lm.mu.RLock()
	term := lm.Terminal
	lm.mu.RUnlock()

	if term == nil {
		return false
	}

	// Handle tcell.EventPaste (bracketed paste from outer terminal)
	if ev, ok := event.(*tcell.EventPaste); ok {
		log.Printf("THOCK: EventPaste for terminal, len=%d", len(ev.Text()))
		term.Write([]byte(ev.Text()))
		return true
	}

	// Handle Cmd-V (Mac) or Ctrl-V paste keybinding
	if ev, ok := event.(*tcell.EventKey); ok {
		isPaste := false

		// Check for Ctrl-V
		if ev.Key() == tcell.KeyCtrlV {
			isPaste = true
		}

		// Check for Cmd-V (Mac) or Ctrl-V as rune with modifier
		if ev.Key() == tcell.KeyRune && ev.Rune() == 'v' {
			if ev.Modifiers()&tcell.ModMeta != 0 || ev.Modifiers()&tcell.ModCtrl != 0 {
				isPaste = true
			}
		}

		if isPaste {
			log.Println("THOCK: Paste keybinding for terminal")
			clip, err := clipboard.Read(clipboard.ClipboardReg)
			if err != nil {
				log.Printf("THOCK: Clipboard read failed: %v", err)
				return true // Still consume to prevent editor paste
			}
			log.Printf("THOCK: Pasting %d bytes to terminal", len(clip))
			term.Write([]byte(clip))
			return true
		}
	}

	return false
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
	const tabBarHeight = 2 // Space for tab bar (1) + separator line (1)

	// Adjust all BufPanes in the tab to use middle region
	for _, pane := range tab.Panes {
		if bp, ok := pane.(*action.BufPane); ok {
			// Get the underlying window
			view := bp.GetView()
			if view != nil {
				// Constrain to middle region (always with border space)
				view.X = editorX + borderOffset
				view.Width = editorWidth - (borderOffset * 2)
				view.Y = borderOffset + tabBarHeight // Leave room for tab bar
				view.Height = lm.ScreenH - (borderOffset * 2) - tabBarHeight
			}
		}
	}
}

// ShowInputModal displays a text input modal dialog
func (lm *LayoutManager) ShowInputModal(title, prompt, defaultValue string, callback func(value string, canceled bool)) {
	if lm.InputModal != nil {
		lm.InputModal.Show(title, prompt, defaultValue, lm.ScreenW, lm.ScreenH, callback)
	}
}

// ShowConfirmModal displays a dangerous action confirmation modal
func (lm *LayoutManager) ShowConfirmModal(title, message, warning string, callback func(confirmed bool)) {
	if lm.ConfirmModal != nil {
		lm.ConfirmModal.Show(title, message, warning, lm.ScreenW, lm.ScreenH, callback)
	}
}

// ShowTimedMessage displays a message that auto-clears after the specified duration
func (lm *LayoutManager) ShowTimedMessage(msg string, duration time.Duration) {
	action.InfoBar.Message(msg)
	time.AfterFunc(duration, func() {
		action.InfoBar.Reset()
		lm.triggerRedraw()
	})
}

// clearEditorIfPathDeleted clears the editor if the deleted path was open
func (lm *LayoutManager) clearEditorIfPathDeleted(deletedPath string, isDir bool) {
	tab := action.MainTab()
	if tab == nil {
		return
	}

	for _, pane := range tab.Panes {
		if bp, ok := pane.(*action.BufPane); ok {
			bufPath := bp.Buf.AbsPath
			if bufPath == "" {
				continue
			}

			shouldClear := false
			if isDir {
				// Check if buffer path is inside the deleted directory
				if strings.HasPrefix(bufPath, deletedPath+string(filepath.Separator)) {
					shouldClear = true
				}
			} else {
				// Check if buffer path matches the deleted file
				if bufPath == deletedPath {
					shouldClear = true
				}
			}

			if shouldClear {
				log.Printf("THOCK: Clearing editor - deleted file was open: %s", bufPath)
				// Create a new empty buffer to replace the deleted file's buffer
				newBuf := buffer.NewBufferFromString("", "", buffer.BTDefault)
				bp.OpenBuffer(newBuf)
			}
		}
	}
}

// CloseCurrentTab closes the current tab, deselects the file in tree, and opens an empty buffer
func (lm *LayoutManager) CloseCurrentTab() {
	tab := action.MainTab()
	if tab == nil {
		return
	}

	for _, pane := range tab.Panes {
		if bp, ok := pane.(*action.BufPane); ok {
			// Check for unsaved changes
			if bp.Buf.Modified() {
				bufName := bp.Buf.GetName()
				if bufName == "" {
					bufName = "Untitled"
				}
				lm.ShowModal("Unsaved Changes",
					"Discard changes to "+bufName+"?",
					func(yes, canceled bool) {
						if canceled || !yes {
							return
						}
						lm.closeTabAndReset(bp)
					})
				return
			}

			lm.closeTabAndReset(bp)
			return
		}
	}
}

// closeTabAndReset performs the actual tab close: resets editor and deselects tree
func (lm *LayoutManager) closeTabAndReset(bp *action.BufPane) {
	log.Println("THOCK: Closing tab and resetting to empty buffer")

	// Create a new empty buffer
	newBuf := buffer.NewBufferFromString("", "", buffer.BTDefault)
	bp.OpenBuffer(newBuf)

	// Keep tree selection at 0 (first item) - don't use -1 as it causes slice bounds issues
	if lm.FileBrowser != nil {
		lm.FileBrowser.Selected = 0
		lm.FileBrowser.TopLine = 0
	}

	lm.triggerRedraw()
}

// SaveCurrentBuffer saves the current buffer, showing modal for new files
func (lm *LayoutManager) SaveCurrentBuffer() bool {
	tab := action.MainTab()
	if tab == nil {
		return false
	}

	for _, pane := range tab.Panes {
		if bp, ok := pane.(*action.BufPane); ok {
			if bp.Buf.Path == "" {
				// New file - show input modal for filename
				log.Println("THOCK: New file, showing input modal for filename")
				lm.ShowInputModal("Save File", "Enter filename:", "", func(filename string, canceled bool) {
					if canceled || filename == "" {
						log.Println("THOCK: Save canceled or empty filename")
						return
					}
					log.Printf("THOCK: Saving new file as: %s", filename)

					// Save the file
					err := bp.Buf.SaveAs(filename)
					if err != nil {
						action.InfoBar.Error(err)
						return
					}
					lm.ShowTimedMessage("Saved "+filename, 3*time.Second)

					// Notify tree to refresh and select the file
					if lm.FileBrowser != nil && lm.FileBrowser.OnFileSaved != nil {
						lm.FileBrowser.OnFileSaved(filename)
					}
				})
				return true
			} else {
				// Existing file - just save
				log.Printf("THOCK: Saving existing file: %s", bp.Buf.Path)
				bp.Save()

				// Notify tree (in case it needs to update)
				if lm.FileBrowser != nil && lm.FileBrowser.OnFileSaved != nil {
					lm.FileBrowser.OnFileSaved(bp.Buf.Path)
				}
				return true
			}
		}
	}
	return false
}
