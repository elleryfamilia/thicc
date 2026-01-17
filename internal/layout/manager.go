package layout

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/ellery/thicc/internal/action"
	"github.com/ellery/thicc/internal/buffer"
	"github.com/ellery/thicc/internal/screen"
	"github.com/ellery/thicc/internal/clipboard"
	"github.com/ellery/thicc/internal/config"
	"github.com/ellery/thicc/internal/dashboard"
	"github.com/ellery/thicc/internal/filebrowser"
	"github.com/ellery/thicc/internal/filemanager"
	"github.com/ellery/thicc/internal/sourcecontrol"
	"github.com/ellery/thicc/internal/terminal"
	"github.com/ellery/thicc/internal/thicc"
	"github.com/micro-editor/tcell/v2"
)

// Region defines a rectangular screen region
type Region struct {
	X      int
	Y      int
	Width  int
	Height int
}

// WorkInProgress holds information about unsaved work
type WorkInProgress struct {
	UnsavedBuffers   []string // Names of modified buffers
	ActiveAISessions []string // Names of active AI tool terminals
}

// HasWork returns true if there is any unsaved work
func (w *WorkInProgress) HasWork() bool {
	return len(w.UnsavedBuffers) > 0 || len(w.ActiveAISessions) > 0
}

// Message returns a human-readable description of the work in progress
func (w *WorkInProgress) Message() string {
	var parts []string
	if n := len(w.UnsavedBuffers); n == 1 {
		parts = append(parts, "1 file with unsaved changes")
	} else if n > 1 {
		parts = append(parts, fmt.Sprintf("%d files with unsaved changes", n))
	}
	if n := len(w.ActiveAISessions); n == 1 {
		parts = append(parts, "1 active AI session ("+w.ActiveAISessions[0]+")")
	} else if n > 1 {
		parts = append(parts, fmt.Sprintf("%d active AI sessions", n))
	}
	return strings.Join(parts, ", ")
}

// LayoutManager coordinates the 3-panel layout
type LayoutManager struct {
	// Panels
	FileBrowser   *filebrowser.Panel
	SourceControl *sourcecontrol.Panel
	Terminal      *terminal.Panel
	Terminal2     *terminal.Panel
	Terminal3     *terminal.Panel
	// Editor uses micro's existing action.Tabs (middle region)

	// Modal dialogs for prompts
	Modal          *Modal
	InputModal     *InputModal
	ConfirmModal   *ConfirmModal
	ShortcutsModal *ShortcutsModal
	LoadingOverlay *LoadingOverlay
	ProjectPicker  *dashboard.ProjectPicker
	QuickFindPicker *QuickFindPicker

	// File index for quick find
	FileIndex *filemanager.FileIndex

	// Tab bar for showing open files
	TabBar *TabBar

	// Pane navigation bar at top of screen
	PaneNavBar *PaneNavBar

	// Layout configuration
	TreeWidth         int // Left panel width (fixed at 30)
	TreeWidthExpanded int // Expanded tree width when focused with single pane
	TermWidthPct      int // Right panel width as percentage (40 = 40%)
	LeftPanelsPct   int // Tree + Editor width as percentage (60 = 60%)
	ScreenW         int // Total screen width
	ScreenH         int // Total screen height

	// Pane visibility state
	TreeVisible          bool // Whether tree pane is visible (default: true)
	SourceControlVisible bool // Whether source control pane is visible (default: false)
	EditorVisible        bool // Whether editor pane is visible (default: true)
	TerminalVisible      bool // Whether terminal pane is visible (default: true)
	Terminal2Visible     bool // Whether terminal2 pane is visible (default: false)
	Terminal3Visible     bool // Whether terminal3 pane is visible (default: false)

	// Track if terminals have been initialized (tool selector shown)
	TerminalInitialized  bool // Terminal 1 (main terminal)
	Terminal2Initialized bool
	Terminal3Initialized bool

	// Tool selector modal for choosing shell/AI tool
	ToolSelector        *ToolSelector
	ToolSelectorTarget  int  // Which terminal (2=term1, 3=term2, 4=term3) is being configured
	ShowingToolSelector bool

	// Active panel (0=filebrowser, 1=editor, 2=terminal, 3=terminal2, 4=terminal3)
	ActivePanel int

	// Root directory for file browser
	Root string

	// Screen reference for triggering redraws
	Screen tcell.Screen

	// AI Tool command to auto-launch in terminal (nil = default shell)
	AIToolCommand []string

	// Track what command the preloaded terminal was created with
	// Used to detect if selection changed and we need to recreate terminal
	preloadedWithCommand []string

	// Mutex to protect Terminal access during async creation
	mu sync.RWMutex

	// Quick command mode state
	QuickCommandMode bool

	// Multiplexer detection - true if running in tmux/zellij/screen
	InMultiplexer bool

	// Idle detection for suspending watchers
	lastActivity      time.Time     // Last user input timestamp
	watchersSuspended bool          // Whether watchers are currently suspended
	idleCheckStop     chan struct{} // Stop channel for idle checker

	// aiToolEverSpawned tracks if ANY AI tool was spawned this session
	// Enables dynamic detection on quit (vs fast-path when only shells used)
	aiToolEverSpawned bool
}

// NewLayoutManager creates a new layout manager
func NewLayoutManager(root string) *LayoutManager {
	// Detect if running inside a terminal multiplexer
	inMux := os.Getenv("TMUX") != "" ||
		os.Getenv("ZELLIJ") != "" ||
		os.Getenv("STY") != ""

	lm := &LayoutManager{
		TreeWidth:         30,              // Fixed tree width
		TreeWidthExpanded: 40,              // Wider when focused or single pane
		LeftPanelsPct:     55,              // Tree + Editor = 55% of screen
		TermWidthPct:    45,                // Terminal = 45% of screen
		Root:            root,
		ActivePanel:     1,                 // Start with editor focused
		TreeVisible:     true,              // All panes visible by default
		EditorVisible:   true,
		TerminalVisible: true,
		Modal:           NewModal(),
		InputModal:      NewInputModal(),
		ConfirmModal:    NewConfirmModal(),
		ShortcutsModal:  NewShortcutsModal(),
		LoadingOverlay:  NewLoadingOverlay(),
		ProjectPicker:   nil, // Initialized when screen is available
		TabBar:          NewTabBar(),
		PaneNavBar:      NewPaneNavBar(),
		InMultiplexer:   inMux,
		lastActivity:    time.Now(),
		idleCheckStop:   make(chan struct{}),
	}

	// Start idle checker goroutine
	go lm.idleChecker()

	return lm
}

// idleTimeout is how long to wait without user input before suspending watchers
const idleTimeout = 1 * time.Minute

// idleChecker periodically checks for inactivity and suspends watchers
func (lm *LayoutManager) idleChecker() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-lm.idleCheckStop:
			return
		case <-ticker.C:
			if time.Since(lm.lastActivity) > idleTimeout && !lm.watchersSuspended {
				lm.suspendWatchers()
			}
		}
	}
}

// suspendWatchers stops file watching and git polling to reduce CPU usage
func (lm *LayoutManager) suspendWatchers() {
	lm.watchersSuspended = true
	if lm.FileBrowser != nil && lm.FileBrowser.Tree != nil {
		lm.FileBrowser.Tree.DisableWatching()
	}
	if lm.SourceControl != nil {
		lm.SourceControl.StopPolling()
	}
	log.Println("THICC: Watchers suspended due to inactivity")
}

// resumeWatchers restarts file watching and git polling after user activity
func (lm *LayoutManager) resumeWatchers() {
	if !lm.watchersSuspended {
		return
	}
	lm.watchersSuspended = false
	if lm.TreeVisible && lm.FileBrowser != nil && lm.FileBrowser.Tree != nil {
		lm.FileBrowser.Tree.Refresh()
		lm.FileBrowser.Tree.EnableWatching()
	}
	if lm.SourceControlVisible && lm.SourceControl != nil {
		lm.SourceControl.RefreshStatus()
		lm.SourceControl.StartPolling()
	}
	log.Println("THICC: Watchers resumed")
}

// SetAIToolCommand sets the AI tool command to auto-launch in the terminal
// Must be called before Initialize() for the command to take effect
func (lm *LayoutManager) SetAIToolCommand(cmd []string) {
	lm.AIToolCommand = cmd
}

// isShellCommand returns true if the command is a shell (bash, zsh, sh, fish, etc.)
func isShellCommand(cmdArgs []string) bool {
	if len(cmdArgs) == 0 {
		return false
	}
	base := filepath.Base(cmdArgs[0])
	switch base {
	case "bash", "zsh", "sh", "fish", "ksh", "csh", "tcsh", "dash":
		return true
	}
	return false
}

// MarkAIToolSpawned records that an AI tool was spawned this session.
// This enables dynamic PTY detection on quit (vs fast-path when only shells used).
func (lm *LayoutManager) MarkAIToolSpawned() {
	lm.aiToolEverSpawned = true
}

// PreloadTerminal creates the terminal in the background while dashboard is showing
// This allows the shell prompt to initialize before the user sees it
func (lm *LayoutManager) PreloadTerminal(screenW, screenH int) {
	lm.mu.Lock()
	if lm.Terminal != nil {
		lm.mu.Unlock()
		return // Already preloaded
	}
	lm.mu.Unlock()

	// Store screen size for later
	lm.ScreenW = screenW
	lm.ScreenH = screenH

	// Temporarily set active panel to terminal to calculate correct dimensions
	// This ensures shouldExpandTree() returns the correct value for the final layout
	// (when terminal is active, tree collapses, giving terminal more space)
	savedActivePanel := lm.ActivePanel
	lm.ActivePanel = 2 // Terminal panel
	termX := lm.getTermX()
	termW := lm.getTermWidth()
	lm.ActivePanel = savedActivePanel // Restore

	// Record what command we're preloading with (for later comparison)
	lm.preloadedWithCommand = lm.AIToolCommand

	// Mark if we're spawning an AI tool (not a shell)
	if lm.AIToolCommand != nil && len(lm.AIToolCommand) > 0 && !isShellCommand(lm.AIToolCommand) {
		lm.MarkAIToolSpawned()
	}

	log.Println("THICC: Preloading terminal panel in background")
	go func() {
		// Use configured AI tool command, or default shell if nil
		cmdArgs := lm.AIToolCommand
		if cmdArgs != nil && len(cmdArgs) > 0 {
			log.Printf("THICC: Auto-launching AI tool: %v", cmdArgs)
		}
		term, err := terminal.NewPanel(termX, 1, termW, lm.ScreenH-1, cmdArgs)
		if err != nil {
			log.Printf("THICC: Failed to preload terminal: %v", err)
			return
		}

		// Set up terminal callbacks
		lm.setupTerminalCallbacks(term)

		// Safely set terminal (prevent race with Initialize)
		lm.mu.Lock()
		if lm.Terminal == nil {
			lm.Terminal = term
			log.Println("THICC: Terminal panel preloaded successfully")
		} else {
			// Another goroutine already created a terminal, close this one
			log.Println("THICC: Terminal already exists, closing preloaded one")
			term.Close()
		}
		lm.mu.Unlock()
	}()
}

// slicesEqual compares two string slices for equality
func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// setupTerminalCallbacks configures callbacks for a terminal panel
func (lm *LayoutManager) setupTerminalCallbacks(term *terminal.Panel) {
	term.OnRedraw = lm.triggerRedraw
	term.OnShowMessage = func(msg string) {
		if msg == "" {
			action.InfoBar.Reset()
		} else {
			action.InfoBar.Message(msg)
		}
		lm.triggerRedraw()
	}
	term.OnQuit = func() {
		lm.handleQuit()
	}
	term.OnNextPane = func() {
		lm.cycleFocus()
		lm.triggerRedraw()
	}
	term.OnSessionEnd = func() {
		// Called when terminal process exits - hide pane and reset to show tool selector next time
		lm.mu.Lock()

		// Determine which terminal this is and hide it
		if lm.Terminal == term {
			log.Println("THICC: Terminal 1 session ended, hiding pane")
			lm.TerminalVisible = false
			lm.TerminalInitialized = false
			lm.Terminal = nil
			if lm.ActivePanel == 2 {
				lm.mu.Unlock()
				lm.focusNextVisiblePane()
				lm.mu.Lock()
			}
		} else if lm.Terminal2 == term {
			log.Println("THICC: Terminal 2 session ended, hiding pane")
			lm.Terminal2Visible = false
			lm.Terminal2Initialized = false
			lm.Terminal2 = nil
			if lm.ActivePanel == 3 {
				lm.mu.Unlock()
				lm.focusNextVisiblePane()
				lm.mu.Lock()
			}
		} else if lm.Terminal3 == term {
			log.Println("THICC: Terminal 3 session ended, hiding pane")
			lm.Terminal3Visible = false
			lm.Terminal3Initialized = false
			lm.Terminal3 = nil
			if lm.ActivePanel == 4 {
				lm.mu.Unlock()
				lm.focusNextVisiblePane()
				lm.mu.Lock()
			}
		}

		lm.mu.Unlock()
		lm.updatePanelRegions()
		lm.triggerRedraw()
	}
}

// getActiveTerminal returns the terminal panel for the currently active panel (or nil)
func (lm *LayoutManager) getActiveTerminal() *terminal.Panel {
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	switch lm.ActivePanel {
	case 2:
		return lm.Terminal
	case 3:
		return lm.Terminal2
	case 4:
		return lm.Terminal3
	}
	return nil
}

// enterQuickCommandMode activates quick command mode
func (lm *LayoutManager) enterQuickCommandMode() {
	lm.QuickCommandMode = true
	lm.triggerRedraw()
}

// handleQuickCommand processes a key in quick command mode
func (lm *LayoutManager) handleQuickCommand(ev *tcell.EventKey) bool {
	lm.QuickCommandMode = false

	switch ev.Key() {
	case tcell.KeyEscape:
		log.Println("THICC: Quick command cancelled")
		lm.triggerRedraw()
		return true
	case tcell.KeyRune:
		switch ev.Rune() {
		case 'q', 'Q':
			log.Println("THICC: Quick command - Quit")
			lm.handleQuit()
			return true
		case 'w', 'W':
			// Close tab - only works in editor
			if lm.ActivePanel == 1 {
				log.Println("THICC: Quick command - Close Tab")
				lm.CloseTabAtIndex(lm.TabBar.ActiveIndex)
				lm.triggerRedraw()
				return true
			}
		case ' ':
			log.Println("THICC: Quick command - Next Pane")
			lm.cycleFocus()
			lm.triggerRedraw()
			return true
		case 's', 'S':
			// Save - only works in editor
			if lm.ActivePanel == 1 {
				log.Println("THICC: Quick command - Save")
				lm.SaveCurrentBuffer()
				lm.triggerRedraw()
				return true
			}
		case 'n', 'N':
			// New file - only works in tree
			if lm.ActivePanel == 0 && lm.FileBrowser != nil {
				log.Println("THICC: Quick command - New File")
				lm.FileBrowser.NewFileSelected()
				lm.triggerRedraw()
				return true
			}
		case 'f', 'F':
			// New folder - only works in tree
			if lm.ActivePanel == 0 && lm.FileBrowser != nil {
				log.Println("THICC: Quick command - New Folder")
				lm.FileBrowser.NewFolderSelected()
				lm.triggerRedraw()
				return true
			}
		case 'd', 'D':
			// Delete - only works in tree
			if lm.ActivePanel == 0 && lm.FileBrowser != nil {
				log.Println("THICC: Quick command - Delete")
				lm.FileBrowser.DeleteSelected()
				lm.triggerRedraw()
				return true
			}
		case 'r', 'R':
			// Rename - only works in tree
			if lm.ActivePanel == 0 && lm.FileBrowser != nil {
				log.Println("THICC: Quick command - Rename")
				lm.FileBrowser.RenameSelected()
				lm.triggerRedraw()
				return true
			}
		case 'p', 'P':
			// Passthrough mode - only works in terminal
			if lm.ActivePanel >= 2 && lm.ActivePanel <= 4 {
				term := lm.getActiveTerminal()
				if term != nil {
					log.Printf("THICC: Entered passthrough mode for terminal %d", lm.ActivePanel)
					term.PassthroughMode = true
					lm.triggerRedraw()
					return true
				}
			}
		}
	}
	// Unknown key - just cancel
	log.Printf("THICC: Quick command - unknown key, cancelled")
	lm.triggerRedraw()
	return true
}

// RenderQuickCommandHints draws the nano-style hint bar at the bottom of the screen
// Shows context-sensitive shortcuts based on which panel has focus
func (lm *LayoutManager) RenderQuickCommandHints(s tcell.Screen) {
	w, h := s.Size()
	y := h - 1 // Bottom row

	// Style: reverse video for visibility
	style := tcell.StyleDefault.Reverse(true)

	// Clear the bottom row
	for x := 0; x < w; x++ {
		s.SetContent(x, y, ' ', nil, style)
	}

	// Context-sensitive hint text based on focused panel
	var hints string
	switch lm.ActivePanel {
	case 0: // Tree
		hints = "  N File   F Folder   D Delete   R Rename   Q Quit   [Space] Next   ESC Cancel"
	case 1: // Editor
		hints = "  S Save   W Close   Q Quit   [Space] Next   ESC Cancel"
	default: // Terminal (2, 3, 4)
		hints = "  P Passthrough   Q Quit   [Space] Next   ESC Cancel"
	}

	x := 0
	for _, r := range hints {
		if x >= w {
			break
		}
		s.SetContent(x, y, r, nil, style)
		x++
	}
}

// RenderMultiplexerHint draws a subtle hint bar when running in a multiplexer
func (lm *LayoutManager) RenderMultiplexerHint(s tcell.Screen) {
	w, h := s.Size()
	y := h - 1 // Bottom row

	// Style: dim for subtle appearance
	style := tcell.StyleDefault.Dim(true)

	// Clear the bottom row
	for x := 0; x < w; x++ {
		s.SetContent(x, y, ' ', nil, style)
	}

	hint := "Press Control-\\ for commands"

	x := 2 // Left-aligned with small indent
	for _, r := range hint {
		if x >= w {
			break
		}
		s.SetContent(x, y, r, nil, style)
		x++
	}
}

// RenderPassthroughHint draws a prominent hint bar when in passthrough mode
func (lm *LayoutManager) RenderPassthroughHint(s tcell.Screen) {
	w, h := s.Size()
	y := h - 1 // Bottom row

	// Style: orange background for visibility
	style := tcell.StyleDefault.Foreground(tcell.ColorBlack).Background(tcell.ColorOrange)

	// Clear the bottom row
	for x := 0; x < w; x++ {
		s.SetContent(x, y, ' ', nil, style)
	}

	hint := "  PASSTHROUGH MODE - Press Ctrl+\\ Ctrl+\\ to exit"

	x := 0
	for _, r := range hint {
		if x >= w {
			break
		}
		s.SetContent(x, y, r, nil, style)
		x++
	}
}

// IsTerminalInPassthroughMode returns true if the active terminal is in passthrough mode
func (lm *LayoutManager) IsTerminalInPassthroughMode() bool {
	if lm.ActivePanel >= 2 && lm.ActivePanel <= 4 {
		term := lm.getActiveTerminal()
		if term != nil {
			return term.PassthroughMode
		}
	}
	return false
}

// PlaceholderWidth is the width of placeholder boxes when both editor and terminal are hidden
const PlaceholderWidth = 20

// getTreeWidth returns the tree width (fixed at 30 when visible, 0 when hidden)
func (lm *LayoutManager) getTreeWidth() int {
	// Return width if either file browser OR source control is visible
	// (they share the same left pane space)
	if !lm.TreeVisible && !lm.SourceControlVisible {
		return 0
	}
	if lm.shouldExpandTree() {
		return lm.TreeWidthExpanded
	}
	return lm.TreeWidth
}

// getVisibleTerminalCount returns how many terminal panes are currently visible
func (lm *LayoutManager) getVisibleTerminalCount() int {
	count := 0
	if lm.TerminalVisible {
		count++
	}
	if lm.Terminal2Visible {
		count++
	}
	if lm.Terminal3Visible {
		count++
	}
	return count
}

// anyTerminalVisible returns true if any terminal pane is visible
func (lm *LayoutManager) anyTerminalVisible() bool {
	return lm.TerminalVisible || lm.Terminal2Visible || lm.Terminal3Visible
}

// shouldExpandTree returns true if tree should use expanded width
// Expanded when: file browser is focused OR only one other pane is visible
func (lm *LayoutManager) shouldExpandTree() bool {
	// Always expand when file browser has focus
	if lm.ActivePanel == 0 {
		return true
	}

	// Stay expanded if only one other pane is visible
	termCount := lm.getVisibleTerminalCount()

	// File browser + editor only (no terminals)
	if lm.EditorVisible && termCount == 0 {
		return true
	}

	// File browser + single terminal only (no editor)
	if !lm.EditorVisible && termCount == 1 {
		return true
	}

	return false
}

// getTotalTerminalSpace returns total space available for all terminal panes
func (lm *LayoutManager) getTotalTerminalSpace() int {
	if !lm.anyTerminalVisible() {
		return 0
	}

	// If editor is also visible, terminals get their percentage of TOTAL screen width
	if lm.EditorVisible {
		space := lm.ScreenW * lm.TermWidthPct / 100
		// When tree is expanded, reduce terminal space (not editor space)
		if lm.shouldExpandTree() {
			space -= (lm.TreeWidthExpanded - lm.TreeWidth)
		}
		if space < 0 {
			space = 0
		}
		return space
	}

	// Terminals take all space after tree
	return lm.ScreenW - lm.getTreeWidth()
}

// getSingleTerminalWidth returns width for each visible terminal pane
func (lm *LayoutManager) getSingleTerminalWidth() int {
	count := lm.getVisibleTerminalCount()
	if count == 0 {
		return 0
	}
	return lm.getTotalTerminalSpace() / count
}

// getTermWidth calculates terminal width based on visibility
func (lm *LayoutManager) getTermWidth() int {
	if !lm.TerminalVisible {
		return 0
	}
	return lm.getSingleTerminalWidth()
}

// getTerm2Width calculates terminal2 width based on visibility
func (lm *LayoutManager) getTerm2Width() int {
	if !lm.Terminal2Visible {
		return 0
	}
	return lm.getSingleTerminalWidth()
}

// getTerm3Width calculates terminal3 width based on visibility
func (lm *LayoutManager) getTerm3Width() int {
	if !lm.Terminal3Visible {
		return 0
	}
	return lm.getSingleTerminalWidth()
}

// getEditorX returns the X position of the editor
func (lm *LayoutManager) getEditorX() int {
	return lm.getTreeWidth()
}

// getTermX returns the X position of the first terminal
func (lm *LayoutManager) getTermX() int {
	return lm.getTreeWidth() + lm.getEditorWidth()
}

// getTerm2X returns the X position of terminal2
func (lm *LayoutManager) getTerm2X() int {
	x := lm.getTermX()
	if lm.TerminalVisible {
		x += lm.getTermWidth()
	}
	return x
}

// getTerm3X returns the X position of terminal3
func (lm *LayoutManager) getTerm3X() int {
	x := lm.getTerm2X()
	if lm.Terminal2Visible {
		x += lm.getTerm2Width()
	}
	return x
}

// getEditorWidth calculates editor width based on visibility
func (lm *LayoutManager) getEditorWidth() int {
	if !lm.EditorVisible {
		return 0
	}

	// If any terminal is visible, editor gets remaining space after tree and all terminals
	if lm.anyTerminalVisible() {
		return lm.ScreenW - lm.getTreeWidth() - lm.getTotalTerminalSpace()
	}

	// Editor takes all space after tree
	return lm.ScreenW - lm.getTreeWidth()
}

// needsPlaceholders returns true if we need to show placeholders
// (when both editor AND all terminals are hidden)
func (lm *LayoutManager) needsPlaceholders() bool {
	return !lm.EditorVisible && !lm.anyTerminalVisible()
}

// Initialize creates the panels once screen size is known
func (lm *LayoutManager) Initialize(screen tcell.Screen) error {
	lm.Screen = screen
	lm.ScreenW, lm.ScreenH = screen.Size()

	log.Printf("THICC: Initializing layout (screen: %dx%d)", lm.ScreenW, lm.ScreenH)

	// Calculate regions (Y=1 to leave room for pane nav bar at Y=0)
	treeRegion := Region{
		X:      0,
		Y:      1,
		Width:  lm.getTreeWidth(),
		Height: lm.ScreenH - 1,
	}

	// editorW := lm.ScreenW - lm.TreeWidth - lm.TermWidth

	// termRegion := Region{
	// 	X:      lm.TreeWidth + editorW,
	// 	Y:      0,
	// 	Width:  lm.TermWidth,
	// 	Height: lm.ScreenH,
	// }

	// Create file browser
	log.Println("THICC: Creating file browser panel")
	lm.FileBrowser = filebrowser.NewPanel(
		treeRegion.X, treeRegion.Y,
		treeRegion.Width, treeRegion.Height,
		lm.Root,
	)

	// Set callbacks
	lm.FileBrowser.OnFileOpen = lm.previewFileInEditor // Preview without switching focus (navigation)
	lm.FileBrowser.OnFileActualOpen = func(path string) { // Actual open (click) - unhides editor
		if !lm.EditorVisible {
			lm.EditorVisible = true
			lm.updateLayout()
			lm.triggerRedraw()
		}
		lm.previewFileInEditor(path)
	}
	lm.FileBrowser.OnTreeReady = lm.triggerRedraw
	lm.FileBrowser.OnFocusEditor = lm.FocusEditor
	lm.FileBrowser.OnProjectPathClick = lm.ShowProjectPicker
	lm.FileBrowser.OnFileSaved = func(path string) {
		log.Printf("THICC: File saved callback - refreshing tree and selecting: %s", path)
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
					log.Printf("THICC: Deleting %s: %s", itemType, path)
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
			log.Printf("THICC: Renaming %s -> %s", oldPath, newPath)

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
			log.Printf("THICC: Creating new file: %s", newPath)

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

			// Open the new file in editor and pin it (new files should be permanent)
			lm.previewFileInEditor(newPath)
			if lm.TabBar != nil {
				lm.TabBar.PinTab(lm.TabBar.ActiveIndex)
			}

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
			log.Printf("THICC: Creating new folder: %s", newPath)

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

	// Calculate terminal region with terminal as active panel
	// This ensures shouldExpandTree() returns the correct value for the final layout
	savedActivePanel := lm.ActivePanel
	lm.ActivePanel = 2 // Terminal panel
	termX := lm.getTermX()
	termW := lm.getTermWidth()
	lm.ActivePanel = savedActivePanel // Restore

	// Check if terminal was already preloaded
	lm.mu.RLock()
	terminalExists := lm.Terminal != nil
	lm.mu.RUnlock()

	// Check if the AI tool selection changed since preload
	selectionChanged := !slicesEqual(lm.AIToolCommand, lm.preloadedWithCommand)
	if selectionChanged && terminalExists {
		log.Printf("THICC: AI tool selection changed (was %v, now %v), recreating terminal",
			lm.preloadedWithCommand, lm.AIToolCommand)
		// Close the preloaded terminal
		lm.mu.Lock()
		if lm.Terminal != nil {
			lm.Terminal.Close()
			lm.Terminal = nil
		}
		lm.mu.Unlock()
		terminalExists = false
	}

	if terminalExists {
		// Terminal was preloaded with correct command - just update its region
		log.Println("THICC: Using preloaded terminal panel")
		lm.mu.Lock()
		lm.Terminal.Region.X = termX
		// Call Resize with new values - it will skip if size unchanged and update Region
		_ = lm.Terminal.Resize(termW, lm.ScreenH)
		lm.mu.Unlock()
	} else {
		// Create terminal asynchronously (to avoid blocking UI)
		log.Println("THICC: Creating terminal panel asynchronously")
		go func() {
			// Use configured AI tool command, or default shell if nil
			cmdArgs := lm.AIToolCommand
			if cmdArgs != nil && len(cmdArgs) > 0 {
				log.Printf("THICC: Auto-launching AI tool: %v", cmdArgs)
			}
			term, err := terminal.NewPanel(termX, 1, termW, lm.ScreenH-1, cmdArgs)
			if err != nil {
				log.Printf("THICC: Failed to create terminal: %v", err)
				return
			}

			// Set up terminal callbacks
			lm.setupTerminalCallbacks(term)

			// Safely set terminal (prevent race with PreloadTerminal)
			lm.mu.Lock()
			if lm.Terminal == nil {
				lm.Terminal = term
				log.Println("THICC: Terminal panel created successfully")
			} else {
				// PreloadTerminal already created one, close this
				log.Println("THICC: Preloaded terminal exists, closing duplicate")
				term.Close()
			}
			lm.mu.Unlock()

			lm.triggerRedraw()
		}()
	}

	// Initialize project picker
	lm.ProjectPicker = dashboard.NewProjectPicker(screen,
		func(path string) {
			lm.navigateToProject(path)
		},
		func() {
			lm.ProjectPicker.Hide()
			lm.triggerRedraw()
		},
	)
	lm.ProjectPicker.Hide() // Start hidden
	log.Println("THICC: Project picker initialized")

	// Initialize file index for quick find (build in background)
	lm.FileIndex = filemanager.NewFileIndex(lm.Root)
	go func() {
		lm.FileIndex.Build()
		// Enable file system watching for index after initial build
		if err := lm.FileIndex.EnableWatching(); err != nil {
			log.Printf("THICC: Failed to enable file index watching: %v", err)
		}
	}()
	log.Println("THICC: File index build started in background")

	// Initialize quick find picker
	lm.QuickFindPicker = NewQuickFindPicker(screen, lm.FileIndex,
		func(path string) {
			lm.QuickFindPicker.Hide()
			// Make editor visible if it's hidden
			if !lm.EditorVisible {
				lm.EditorVisible = true
				lm.updateLayout()
			}
			lm.previewFileInEditor(path)
			// Also select the file in the file browser
			if lm.FileBrowser != nil {
				lm.FileBrowser.SelectFile(path)
			}
			lm.FocusEditor() // Switch focus to editor after opening
			lm.triggerRedraw()
		},
		func() {
			lm.QuickFindPicker.Hide()
			lm.triggerRedraw()
		},
	)
	log.Println("THICC: Quick find picker initialized")

	log.Println("THICC: Layout initialization complete")
	return nil
}

// RenderFrame draws all 3 panels (called BEFORE editor renders)
func (lm *LayoutManager) RenderFrame(screen tcell.Screen) {
	// 0. Render pane navigation bar at the very top
	if lm.PaneNavBar != nil {
		lm.PaneNavBar.Region = Region{
			X:      0,
			Y:      0,
			Width:  lm.ScreenW,
			Height: 1,
		}
		lm.PaneNavBar.Manager = lm
		lm.PaneNavBar.Render(screen)
	}

	// 1. Render file browser or source control (left) - mutually exclusive
	if lm.SourceControlVisible && lm.SourceControl != nil {
		lm.SourceControl.Focus = (lm.ActivePanel == 0)
		lm.SourceControl.Render(screen)
	} else if lm.FileBrowser != nil && lm.TreeVisible {
		lm.FileBrowser.Focus = (lm.ActivePanel == 0)
		lm.FileBrowser.Render(screen)
	}

	// 2. Editor (middle) will be rendered by micro's standard rendering
	//    in DoEvent() after this function returns
	//    Border is drawn in RenderOverlay() AFTER editor renders

	// 3. Render terminals (right side) - only if visible
	lm.mu.RLock()
	term := lm.Terminal
	term2 := lm.Terminal2
	term3 := lm.Terminal3
	lm.mu.RUnlock()

	if lm.TerminalVisible && term != nil {
		term.Focus = (lm.ActivePanel == 2)
		term.Render(screen)
	}

	if lm.Terminal2Visible && term2 != nil {
		term2.Focus = (lm.ActivePanel == 3)
		term2.Render(screen)
	}

	if lm.Terminal3Visible && term3 != nil {
		term3.Focus = (lm.ActivePanel == 4)
		term3.Render(screen)
	}

	// 4. Draw placeholders if both editor and all terminals are hidden
	if lm.needsPlaceholders() {
		lm.drawPlaceholders(screen)
	}

	// 5. Draw dividers between visible panels
	lm.drawDividers(screen)
}

// RenderOverlay draws elements that should appear ON TOP of the editor (called AFTER editor renders)
func (lm *LayoutManager) RenderOverlay(screen tcell.Screen) {
	// Sync initial buffer to tab bar (ensures Untitled tab shows on startup)
	lm.SyncInitialTab()

	// Check if preview tab needs to be pinned due to modification
	lm.checkAndPinOnModification()

	// Only draw editor border and tab bar if editor is visible
	if lm.EditorVisible {
		// Draw editor border (bright when focused, dim when not)
		lm.drawEditorBorder(screen, lm.ActivePanel == 1)

		// Draw tab bar below top border (Y=2 since pane nav bar is at Y=0, editor border starts at Y=1)
		if lm.TabBar != nil {
			lm.TabBar.Region = Region{
				X:      lm.getTreeWidth() + 1,
				Y:      2, // Below top border (which is now at Y=1)
				Width:  lm.getEditorWidth() - 2,
				Height: 1,
			}
			lm.TabBar.Focused = (lm.ActivePanel == 1)
			lm.TabBar.Render(screen)
		}
	}

	// Draw loading overlay on top of everything (full screen)
	if lm.LoadingOverlay != nil && lm.LoadingOverlay.Active {
		lm.LoadingOverlay.Render(screen, lm.ScreenW, lm.ScreenH)
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

	// Draw project picker on top of everything
	if lm.ProjectPicker != nil && lm.ProjectPicker.Active {
		lm.ProjectPicker.Render(screen)
	}

	// Draw quick find picker on top of everything
	if lm.QuickFindPicker != nil && lm.QuickFindPicker.Active {
		lm.QuickFindPicker.Render(screen)
	}

	// Draw tool selector modal centered over the entire terminal region
	if lm.ShowingToolSelector && lm.ToolSelector != nil && lm.ToolSelector.IsActive() {
		termX := lm.getTermX()
		termW := lm.getTotalTerminalSpace()
		lm.ToolSelector.Render(screen, termX, termW, lm.ScreenH)
	}

	// Draw shortcuts modal on top of everything
	if lm.ShortcutsModal != nil && lm.ShortcutsModal.Active {
		lm.ShortcutsModal.Render(screen)
	}
}


// HandleEvent routes events to the active panel
func (lm *LayoutManager) HandleEvent(event tcell.Event) bool {
	// Track activity and resume watchers if suspended
	lm.lastActivity = time.Now()
	if lm.watchersSuspended {
		lm.resumeWatchers()
	}

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

	// Handle project picker
	if lm.ProjectPicker != nil && lm.ProjectPicker.Active {
		return lm.ProjectPicker.HandleEvent(event)
	}

	// Handle quick find picker
	if lm.QuickFindPicker != nil && lm.QuickFindPicker.Active {
		return lm.QuickFindPicker.HandleEvent(event)
	}

	// Handle tool selector modal
	if lm.ShowingToolSelector && lm.ToolSelector != nil && lm.ToolSelector.IsActive() {
		return lm.ToolSelector.HandleEvent(event)
	}

	// Handle shortcuts modal
	if lm.ShortcutsModal != nil && lm.ShortcutsModal.Active {
		return lm.ShortcutsModal.HandleEvent(event)
	}

	// Handle quick command mode
	if lm.QuickCommandMode {
		if ev, ok := event.(*tcell.EventKey); ok {
			return lm.handleQuickCommand(ev)
		}
	}

	// Handle raw escape sequences for Alt+1-5 (universal terminal support)
	// Many terminals (Kitty, iTerm2, Alacritty) send Alt+key as ESC-prefixed
	// sequences rather than setting the ModAlt flag
	if rawEv, ok := event.(*tcell.EventRaw); ok {
		seq := rawEv.EscSeq()
		switch seq {
		case "\x1b1":
			log.Println("THICC: ESC+1 raw sequence detected, toggling tree")
			lm.ToggleTree()
			return true
		case "\x1b2":
			log.Println("THICC: ESC+2 raw sequence detected, toggling editor")
			lm.ToggleEditor()
			return true
		case "\x1b3":
			log.Println("THICC: ESC+3 raw sequence detected, toggling terminal")
			lm.ToggleTerminal()
			return true
		case "\x1b4":
			log.Println("THICC: ESC+4 raw sequence detected, toggling terminal2")
			lm.ToggleTerminal2()
			return true
		case "\x1b5":
			log.Println("THICC: ESC+5 raw sequence detected, toggling terminal3")
			lm.ToggleTerminal3()
			return true
		case "\x1ba":
			log.Println("THICC: ESC+a raw sequence detected, toggling source control")
			lm.ToggleSourceControl()
			return true
		}
	}

	// Handle macOS Option+1-5 which generates special Unicode characters
	// instead of Alt modifier events (universal macOS support)
	if ev, ok := event.(*tcell.EventKey); ok && ev.Key() == tcell.KeyRune {
		switch ev.Rune() {
		case '¡': // Option+1 on macOS
			log.Println("THICC: macOS Option+1 detected, toggling tree")
			lm.ToggleTree()
			return true
		case '™': // Option+2 on macOS
			log.Println("THICC: macOS Option+2 detected, toggling editor")
			lm.ToggleEditor()
			return true
		case '£': // Option+3 on macOS
			log.Println("THICC: macOS Option+3 detected, toggling terminal")
			lm.ToggleTerminal()
			return true
		case '¢': // Option+4 on macOS
			log.Println("THICC: macOS Option+4 detected, toggling terminal2")
			lm.ToggleTerminal2()
			return true
		case '∞': // Option+5 on macOS
			log.Println("THICC: macOS Option+5 detected, toggling terminal3")
			lm.ToggleTerminal3()
			return true
		case 'å': // Option+a on macOS
			log.Println("THICC: macOS Option+a detected, toggling source control")
			lm.ToggleSourceControl()
			return true
		}
	}

	// CRITICAL: Terminals get ALL keyboard events when focused (except global shortcuts)
	// This must happen BEFORE global key handlers to prevent editor from intercepting keys like Esc
	if lm.ActivePanel >= 2 && lm.ActivePanel <= 4 {
		// Get the active terminal
		lm.mu.RLock()
		var term *terminal.Panel
		switch lm.ActivePanel {
		case 2:
			term = lm.Terminal
		case 3:
			term = lm.Terminal2
		case 4:
			term = lm.Terminal3
		}
		lm.mu.RUnlock()

		// PASSTHROUGH MODE: Send ALL keys to terminal except double-tap Ctrl+\ for exit
		if term != nil && term.PassthroughMode {
			if ev, ok := event.(*tcell.EventKey); ok {
				if ev.Key() == tcell.KeyCtrlBackslash {
					now := time.Now()
					// Double-tap within 500ms exits passthrough mode
					if now.Sub(term.LastCtrlBackslash) < 500*time.Millisecond {
						term.PassthroughMode = false
						term.LastCtrlBackslash = time.Time{} // Reset
						log.Printf("THICC: Exited passthrough mode for terminal %d", lm.ActivePanel)
						lm.triggerRedraw()
						return true
					}
					term.LastCtrlBackslash = now
					// First Ctrl+\ - send to terminal (could be SIGQUIT)
					term.HandleEvent(event)
					return true
				}
				// All other keys go directly to terminal
				term.HandleEvent(event)
				return true
			}
			// Handle paste in passthrough mode too
			if lm.handleTerminalPaste(event) {
				return true
			}
		}

		if ev, ok := event.(*tcell.EventKey); ok {
			// Allow global shortcuts to pass through: Ctrl+Q, Ctrl+T, Ctrl+W, Ctrl+Space, Ctrl+\, Ctrl+[, Ctrl+], Alt+Arrow
			// Also allow pane toggle shortcuts: Alt+1 through Alt+5, Ctrl+/
			isGlobalShortcut := ev.Key() == tcell.KeyCtrlQ ||
				ev.Key() == tcell.KeyCtrlT ||
				ev.Key() == tcell.KeyCtrlW ||
				ev.Key() == tcell.KeyCtrlSpace ||
				ev.Key() == tcell.KeyCtrlBackslash ||
				ev.Key() == tcell.KeyCtrlP ||
				(ev.Key() == tcell.KeyRight && ev.Modifiers()&tcell.ModAlt != 0) ||
				(ev.Key() == tcell.KeyLeft && ev.Modifiers()&tcell.ModAlt != 0) ||
				(ev.Rune() == ']' && ev.Modifiers()&tcell.ModCtrl != 0) ||
				(ev.Rune() == '[' && ev.Modifiers()&tcell.ModCtrl != 0) ||
				(ev.Rune() == '1' && ev.Modifiers()&tcell.ModAlt != 0) ||
				(ev.Rune() == '2' && ev.Modifiers()&tcell.ModAlt != 0) ||
				(ev.Rune() == '3' && ev.Modifiers()&tcell.ModAlt != 0) ||
				(ev.Rune() == '4' && ev.Modifiers()&tcell.ModAlt != 0) ||
				(ev.Rune() == '5' && ev.Modifiers()&tcell.ModAlt != 0) ||
				(ev.Rune() == 'a' && ev.Modifiers()&tcell.ModAlt != 0) ||
				(ev.Rune() == '/' && ev.Modifiers()&tcell.ModCtrl != 0) ||
				ev.Key() == tcell.KeyCtrlUnderscore // Ctrl+/ often sends this

			if !isGlobalShortcut && term != nil {
				term.HandleEvent(event)
				return true // Always consume keyboard events for terminal
			}
		}

		// Handle paste for terminal
		if lm.handleTerminalPaste(event) {
			return true
		}
	}

	// Handle pane nav bar mouse clicks
	if ev, ok := event.(*tcell.EventMouse); ok {
		if ev.Buttons() == tcell.Button1 {
			x, y := ev.Position()
			if lm.PaneNavBar != nil && lm.PaneNavBar.IsInNavBar(x, y) {
				paneNum := lm.PaneNavBar.GetClickedPane(x, y)
				if paneNum > 0 {
					log.Printf("THICC: Pane %d clicked in nav bar", paneNum)
					switch paneNum {
					case 1:
						lm.ToggleTree()
					case 2:
						lm.ToggleEditor()
					case 3:
						lm.ToggleTerminal()
					case 4:
						lm.ToggleTerminal2()
					case 5:
						lm.ToggleTerminal3()
					case 6:
						lm.ToggleSourceControl()
					}
					lm.triggerRedraw()
					return true
				}
			}
		}
	}

	// Handle tab bar mouse clicks
	if ev, ok := event.(*tcell.EventMouse); ok {
		if ev.Buttons() == tcell.Button1 {
			x, y := ev.Position()
			if lm.TabBar != nil && lm.TabBar.IsInTabBar(x, y) {
				// Check for overflow indicator clicks first (for scrolling)
				if lm.TabBar.IsLeftOverflowClick(x, y) {
					log.Println("THICC: Left overflow clicked, scrolling left")
					lm.TabBar.ScrollLeft()
					lm.triggerRedraw()
					return true
				}
				if lm.TabBar.IsRightOverflowClick(x, y) {
					log.Println("THICC: Right overflow clicked, scrolling right")
					lm.TabBar.ScrollRight()
					lm.triggerRedraw()
					return true
				}

				// Check for close button click
				closeIdx := lm.TabBar.IsCloseButtonClick(x, y)
				if closeIdx >= 0 {
					log.Printf("THICC: Tab %d close button clicked", closeIdx)
					lm.CloseTabAtIndex(closeIdx)
					return true
				}

				// Check for tab click (to switch tabs)
				tabIdx := lm.TabBar.GetClickedTab(x, y)
				if tabIdx >= 0 && tabIdx != lm.TabBar.ActiveIndex {
					log.Printf("THICC: Tab %d clicked, switching", tabIdx)
					lm.switchToTab(tabIdx)
					return true
				}
			}
		}
	}

	// Log all key events for debugging
	if ev, ok := event.(*tcell.EventKey); ok {
		log.Printf("THICC HandleEvent: Key=%v, Rune=%q (0x%x), Mod=%v, ActivePanel=%d",
			ev.Key(), ev.Rune(), ev.Rune(), ev.Modifiers(), lm.ActivePanel)

		// Ctrl+Q should always quit, regardless of which panel is focused
		if ev.Key() == tcell.KeyCtrlQ {
			log.Println("THICC: Ctrl+Q detected, checking for unsaved changes")
			return lm.handleQuit()
		}

		// Control-\ enters quick command mode (works globally)
		if ev.Key() == tcell.KeyCtrlBackslash {
			log.Println("THICC: Control-\\ detected, entering quick command mode")
			lm.enterQuickCommandMode()
			return true
		}

		// Ctrl+S for save (with modal for new files) when editor is focused
		if ev.Key() == tcell.KeyCtrlS && lm.ActivePanel == 1 {
			log.Println("THICC: Ctrl+S detected, saving current buffer")
			return lm.SaveCurrentBuffer()
		}

		// Ctrl+P for quick find (works globally)
		if ev.Key() == tcell.KeyCtrlP {
			log.Println("THICC: Ctrl+P detected, showing quick find")
			lm.ShowQuickFind()
			return true
		}


		// Global file operations (work from any panel when FileBrowser exists)
		if lm.FileBrowser != nil {
			switch ev.Key() {
			case tcell.KeyCtrlN:
				// Ctrl+N = new file, Ctrl+Shift+N = new folder
				if ev.Modifiers()&tcell.ModShift != 0 {
					log.Println("THICC: Ctrl+Shift+N detected, creating new folder")
					lm.FileBrowser.NewFolderSelected()
				} else {
					log.Println("THICC: Ctrl+N detected, creating new file")
					lm.FileBrowser.NewFileSelected()
				}
				return true

			case tcell.KeyCtrlD:
				log.Println("THICC: Ctrl+D detected, deleting selected")
				lm.FileBrowser.DeleteSelected()
				return true

			case tcell.KeyCtrlR:
				log.Println("THICC: Ctrl+R detected, renaming selected")
				lm.FileBrowser.RenameSelected()
				return true
			}
		}

		// Tab switching shortcuts:
		// Ctrl+] or Option+Right = next tab
		// Ctrl+[ or Option+Left = previous tab
		// Note: Ctrl+] sends ASCII 29 (KeyCtrlRightSq), Ctrl+[ sends ESC (KeyCtrlLeftSq)
		if ev.Key() == tcell.KeyCtrlRightSq {
			log.Println("THICC: Ctrl+] detected, next tab")
			lm.NextTab()
			return true
		}
		if ev.Key() == tcell.KeyCtrlLeftSq && ev.Modifiers() == 0 {
			// Only handle bare Ctrl+[ (ESC), not Escape sequences with modifiers
			log.Println("THICC: Ctrl+[ detected, previous tab")
			lm.PreviousTab()
			return true
		}
		if ev.Key() == tcell.KeyRight && ev.Modifiers()&tcell.ModAlt != 0 {
			log.Println("THICC: Option+Right detected, next tab")
			lm.NextTab()
			return true
		}
		if ev.Key() == tcell.KeyLeft && ev.Modifiers()&tcell.ModAlt != 0 {
			log.Println("THICC: Option+Left detected, previous tab")
			lm.PreviousTab()
			return true
		}

		// Ctrl+W closes the active tab
		if ev.Key() == tcell.KeyCtrlW {
			log.Println("THICC: Ctrl+W detected, closing active tab")
			lm.CloseActiveTab()
			return true
		}

		// Pane visibility toggle shortcuts (Alt+1 through Alt+5)
		if ev.Key() == tcell.KeyRune && ev.Modifiers()&tcell.ModAlt != 0 {
			switch ev.Rune() {
			case '1':
				log.Println("THICC: Alt+1 detected, toggling tree")
				lm.ToggleTree()
				return true
			case '2':
				log.Println("THICC: Alt+2 detected, toggling editor")
				lm.ToggleEditor()
				return true
			case '3':
				log.Println("THICC: Alt+3 detected, toggling terminal")
				lm.ToggleTerminal()
				return true
			case '4':
				log.Println("THICC: Alt+4 detected, toggling terminal2")
				lm.ToggleTerminal2()
				return true
			case '5':
				log.Println("THICC: Alt+5 detected, toggling terminal3")
				lm.ToggleTerminal3()
				return true
			case 'a':
				log.Println("THICC: Alt+a detected, toggling source control")
				lm.ToggleSourceControl()
				return true
			case 's':
				log.Println("THICC: Alt+s detected, opening settings")
				lm.OpenSettings()
				return true
			}
		}

		// Ctrl+/ for shortcuts help (may come as KeyRune '/' with Ctrl or as KeyCtrlUnderscore)
		if (ev.Key() == tcell.KeyRune && ev.Rune() == '/' && ev.Modifiers()&tcell.ModCtrl != 0) ||
			ev.Key() == tcell.KeyCtrlUnderscore {
			log.Println("THICC: Ctrl+/ detected, showing shortcuts")
			lm.ShowShortcutsModal()
			return true
		}

		// Also support '?' for shortcuts help (Shift+/ without Ctrl)
		if ev.Key() == tcell.KeyRune && ev.Rune() == '?' && ev.Modifiers() == 0 {
			// Only trigger if not in a text input context (editor focused)
			if lm.ActivePanel != 1 {
				log.Println("THICC: ? detected, showing shortcuts")
				lm.ShowShortcutsModal()
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
	case 0: // File browser or Source Control (left pane)
		if lm.SourceControlVisible && lm.SourceControl != nil {
			return lm.SourceControl.HandleEvent(event)
		} else if lm.FileBrowser != nil {
			return lm.FileBrowser.HandleEvent(event)
		}

	case 1: // Editor (handled by micro's action.Tabs)
		return false // Let micro handle it

	case 2, 3, 4: // Terminal, Terminal2, Terminal3
		term := lm.getActiveTerminal()

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
				log.Printf("THICC: Mouse click at x=%d, switching focus from panel %d to %d", x, lm.ActivePanel, newPanel)
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

	// Tab key: tree → editor (only when no modifiers)
	if ev.Key() == tcell.KeyTab && ev.Modifiers() == 0 {
		if lm.ActivePanel == 0 { // Tree has focus
			log.Println("THICC: Tab pressed in tree, switching to editor")
			lm.FocusEditor()
			return true // Consume the event to prevent insertion
		}
	}

	// Shift+Tab: editor → tree
	if ev.Key() == tcell.KeyBacktab ||
		(ev.Key() == tcell.KeyTab && ev.Modifiers()&tcell.ModShift != 0) {
		if lm.ActivePanel == 1 { // Editor has focus
			log.Println("THICC: Shift+Tab pressed in editor, switching to tree")
			lm.FocusTree()
			return true // Consume the event
		}
	}

	// Ctrl+Space cycles focus: tree → editor → terminal → tree
	if ev.Key() == tcell.KeyCtrlSpace {
		log.Printf("THICC: Ctrl+Space pressed, current panel: %d", lm.ActivePanel)
		lm.cycleFocus()
		log.Printf("THICC: Focus switched to panel %d", lm.ActivePanel)
		return true
	}

	return false
}

// panelAtX returns which panel is at the given x coordinate
// Returns: 0=filebrowser, 1=editor, 2=terminal, 3=terminal2, 4=terminal3, -1=none
func (lm *LayoutManager) panelAtX(x int) int {
	treeWidth := lm.getTreeWidth()
	editorEndX := lm.getTermX()
	term2X := lm.getTerm2X()
	term3X := lm.getTerm3X()

	// Left panel region (file browser or source control)
	if x < treeWidth {
		if lm.SourceControlVisible && lm.SourceControl != nil {
			return 0
		}
		if lm.TreeVisible && lm.FileBrowser != nil {
			return 0
		}
	}

	// Editor region
	if x >= treeWidth && x < editorEndX && lm.EditorVisible {
		return 1
	}

	// Terminal region (first terminal)
	if x >= editorEndX && x < term2X && lm.TerminalVisible {
		lm.mu.RLock()
		hasTerminal := lm.Terminal != nil
		lm.mu.RUnlock()
		if hasTerminal {
			return 2
		}
	}

	// Terminal2 region
	if x >= term2X && x < term3X && lm.Terminal2Visible {
		lm.mu.RLock()
		hasTerminal2 := lm.Terminal2 != nil
		lm.mu.RUnlock()
		if hasTerminal2 {
			return 3
		}
	}

	// Terminal3 region
	if x >= term3X && lm.Terminal3Visible {
		lm.mu.RLock()
		hasTerminal3 := lm.Terminal3 != nil
		lm.mu.RUnlock()
		if hasTerminal3 {
			return 4
		}
	}

	return -1
}

// setActivePanel changes the active panel and updates Focus flags immediately
func (lm *LayoutManager) setActivePanel(panel int) {
	log.Printf("THICC: setActivePanel: %d -> %d", lm.ActivePanel, panel)
	lm.ActivePanel = panel

	// Update Focus flags immediately (don't wait for render)
	if lm.FileBrowser != nil {
		lm.FileBrowser.Focus = (panel == 0)
	}

	// Refresh source control on focus
	if panel == 0 && lm.SourceControlVisible && lm.SourceControl != nil {
		lm.SourceControl.RefreshStatus()
	}

	lm.mu.RLock()
	term := lm.Terminal
	term2 := lm.Terminal2
	term3 := lm.Terminal3
	lm.mu.RUnlock()

	if term != nil {
		term.Focus = (panel == 2)
	}
	if term2 != nil {
		term2.Focus = (panel == 3)
	}
	if term3 != nil {
		term3.Focus = (panel == 4)
	}

	// Update editor active state (controls cursor visibility)
	editorActive := (panel == 1)
	if tab := action.MainTab(); tab != nil {
		for _, pane := range tab.Panes {
			if bp, ok := pane.(*action.BufPane); ok {
				bp.SetActive(editorActive)
			}
		}
	}

	// Pin preview tab when switching to editor (user is committing to this file)
	if panel == 1 && lm.TabBar != nil {
		activeTab := lm.TabBar.GetActiveTab()
		if activeTab != nil && activeTab.IsPreview {
			log.Printf("THICC: Pinning preview tab on panel switch: %s", activeTab.Name)
			lm.TabBar.PinTab(lm.TabBar.ActiveIndex)
		}
	}

	// Update layout when focus changes (tree width may change based on focus)
	lm.updateLayout()
}

// cycleFocus cycles to the next panel
func (lm *LayoutManager) cycleFocus() {
	log.Printf("THICC cycleFocus: Starting from panel %d", lm.ActivePanel)
	// Cycle through available panels (0-4: tree, editor, term, term2, term3)
	for i := 0; i < 5; i++ {
		nextPanel := (lm.ActivePanel + 1) % 5
		log.Printf("THICC cycleFocus: Trying panel %d (iteration %d)", nextPanel, i)

		// Check if this panel exists and is visible
		switch nextPanel {
		case 0:
			// Panel 0 is either file browser OR source control (mutually exclusive)
			if lm.SourceControlVisible && lm.SourceControl != nil {
				log.Println("THICC cycleFocus: SourceControl visible, setting active")
				lm.setActivePanel(nextPanel)
				return
			}
			if lm.TreeVisible && lm.FileBrowser != nil {
				log.Println("THICC cycleFocus: FileBrowser exists, setting active")
				lm.setActivePanel(nextPanel)
				return
			}
		case 1:
			if lm.EditorVisible {
				log.Println("THICC cycleFocus: Editor visible, setting active")
				lm.setActivePanel(nextPanel)
				return
			}
		case 2:
			if lm.TerminalVisible {
				lm.mu.RLock()
				hasTerminal := lm.Terminal != nil
				lm.mu.RUnlock()
				if hasTerminal {
					log.Println("THICC cycleFocus: Terminal exists, setting active")
					lm.setActivePanel(nextPanel)
					return
				}
			}
		case 3:
			if lm.Terminal2Visible {
				lm.mu.RLock()
				hasTerminal2 := lm.Terminal2 != nil
				lm.mu.RUnlock()
				if hasTerminal2 {
					log.Println("THICC cycleFocus: Terminal2 exists, setting active")
					lm.setActivePanel(nextPanel)
					return
				}
			}
		case 4:
			if lm.Terminal3Visible {
				lm.mu.RLock()
				hasTerminal3 := lm.Terminal3 != nil
				lm.mu.RUnlock()
				if hasTerminal3 {
					log.Println("THICC cycleFocus: Terminal3 exists, setting active")
					lm.setActivePanel(nextPanel)
					return
				}
			}
		}
		// Panel doesn't exist or isn't visible, update ActivePanel to continue cycling
		lm.ActivePanel = nextPanel
	}
	log.Println("THICC cycleFocus: Completed loop without finding panel")
}

// previewFileInEditor opens a file in the editor panel WITHOUT switching focus
// Uses lazy loading - creates a preview tab first, then loads the buffer
// Preview tabs are italicized and get replaced when navigating to another file
// Note: Does NOT unhide editor - use OnFileActualOpen callback for that
func (lm *LayoutManager) previewFileInEditor(path string) {
	log.Printf("THICC: Previewing file: %s", path)

	// Make path absolute for dedup check
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	// Check if file is already open in a tab
	if lm.TabBar != nil {
		existingIdx := lm.TabBar.FindTabByPath(absPath)
		if existingIdx >= 0 {
			log.Printf("THICC: File already open in tab %d, switching to it", existingIdx)
			lm.switchToTab(existingIdx)
			return
		}
	}

	// Create preview tab (replaces existing preview if any)
	if lm.TabBar != nil {
		lm.TabBar.AddPreviewTabStub(absPath)
		log.Printf("THICC: Created preview tab, now have %d tabs, active=%d", len(lm.TabBar.Tabs), lm.TabBar.ActiveIndex)
	}

	// Load and display the active tab
	lm.loadAndDisplayActiveTab()

	log.Printf("THICC: File previewed in editor, focus stays on tree")
}

// loadAndDisplayActiveTab loads the buffer for the active tab (if not loaded) and displays it
func (lm *LayoutManager) loadAndDisplayActiveTab() {
	if lm.TabBar == nil {
		return
	}

	tab := lm.TabBar.GetActiveTab()
	if tab == nil {
		log.Println("THICC: No active tab to load")
		return
	}

	// If already loaded, just display the buffer
	if tab.Loaded && tab.Buffer != nil {
		log.Printf("THICC: Tab already loaded, displaying buffer")
		lm.displayBufferInEditor(tab.Buffer)
		return
	}

	// Load the buffer
	log.Printf("THICC: Loading buffer for %s (currently %d buffers open)", tab.Path, len(buffer.OpenBuffers))
	buf, err := buffer.NewBufferFromFile(tab.Path, buffer.BTDefault)
	if err != nil {
		log.Printf("THICC: Error loading file: %v", err)
		action.InfoBar.Error(err.Error())
		// Close the stub tab since loading failed
		lm.CloseTabAtIndex(lm.TabBar.ActiveIndex)
		return
	}
	log.Printf("THICC: Buffer loaded successfully, now %d buffers open", len(buffer.OpenBuffers))

	// Mark tab as loaded
	lm.TabBar.MarkTabLoaded(lm.TabBar.ActiveIndex, buf)

	// Display in editor
	lm.displayBufferInEditor(buf)
}

// displayBufferInEditor switches the BufPane to show the given buffer
func (lm *LayoutManager) displayBufferInEditor(buf *buffer.Buffer) {
	if buf == nil {
		return
	}

	tab := action.MainTab()
	if tab == nil {
		log.Println("THICC: No main tab found")
		return
	}

	for _, pane := range tab.Panes {
		if bp, ok := pane.(*action.BufPane); ok {
			bp.SwitchBuffer(buf)
			return
		}
	}

	log.Println("THICC: No BufPane found in tab")
}

// switchToTab switches to the tab at the given index
// Uses lazy loading - loads the buffer if not already loaded
func (lm *LayoutManager) switchToTab(index int) {
	if lm.TabBar == nil || index < 0 || index >= len(lm.TabBar.Tabs) {
		return
	}

	lm.TabBar.ActiveIndex = index
	log.Printf("THICC: Switching to tab %d (%s)", index, lm.TabBar.Tabs[index].Name)

	// Load and display the buffer (lazy loading handles unloaded tabs)
	lm.loadAndDisplayActiveTab()

	// Sync file browser selection
	openTab := lm.TabBar.GetActiveTab()
	if lm.FileBrowser != nil && openTab != nil && openTab.Path != "" {
		lm.FileBrowser.SelectFile(openTab.Path)
	}

	lm.triggerRedraw()
}

// NextTab switches to the next tab (cycles around)
func (lm *LayoutManager) NextTab() {
	if lm.TabBar == nil || len(lm.TabBar.Tabs) <= 1 {
		return
	}
	newIdx := (lm.TabBar.ActiveIndex + 1) % len(lm.TabBar.Tabs)
	lm.switchToTab(newIdx)
}

// PreviousTab switches to the previous tab (cycles around)
func (lm *LayoutManager) PreviousTab() {
	if lm.TabBar == nil || len(lm.TabBar.Tabs) <= 1 {
		return
	}
	newIdx := lm.TabBar.ActiveIndex - 1
	if newIdx < 0 {
		newIdx = len(lm.TabBar.Tabs) - 1
	}
	lm.switchToTab(newIdx)
}

// CloseActiveTab closes the currently active tab
// If it's the last tab and it's Untitled, don't close
// If closing the last real file, create a new Untitled buffer
func (lm *LayoutManager) CloseActiveTab() {
	if lm.TabBar == nil || len(lm.TabBar.Tabs) == 0 {
		return
	}

	activeTab := lm.TabBar.GetActiveTab()
	if activeTab == nil {
		return
	}

	// Don't close if it's the only tab AND it's Untitled
	if len(lm.TabBar.Tabs) == 1 && activeTab.Name == "Untitled" {
		log.Println("THICC: Can't close the only Untitled tab")
		return
	}

	// Remember the index we're closing
	closingIdx := lm.TabBar.ActiveIndex

	// Close the buffer if it's loaded
	if activeTab.Loaded && activeTab.Buffer != nil {
		activeTab.Buffer.Close()
	}

	// Remove from tab bar
	lm.TabBar.CloseTab(closingIdx)
	log.Printf("THICC: Closed tab %d, now have %d tabs", closingIdx, len(lm.TabBar.Tabs))

	// If no tabs left, create a new Untitled buffer
	if len(lm.TabBar.Tabs) == 0 {
		log.Println("THICC: No tabs left, creating new Untitled buffer")
		newBuf := buffer.NewBufferFromString("", "", buffer.BTDefault)
		lm.TabBar.AddTab(newBuf)
		lm.displayBufferInEditor(newBuf)
	} else {
		// Switch to the new active tab (CloseTab already adjusted ActiveIndex)
		lm.loadAndDisplayActiveTab()
	}

	lm.triggerRedraw()
}

// Powerline separator glyphs
const (
	PowerlineArrowRight = '\uE0B0' //
	PowerlineArrowLeft  = '\uE0B2' //
)

// drawDividers draws Powerline-style separators between visible panels
func (lm *LayoutManager) drawDividers(screen tcell.Screen) {
	// Get the background color from DefStyle
	_, bgColor, _ := config.DefStyle.Decompose()

	// Powerline arrow: foreground = left panel color, background = right panel color
	// This creates the seamless "flowing" effect
	powerlineStyle := tcell.StyleDefault.
		Foreground(bgColor).
		Background(bgColor)

	// Only draw divider after tree if tree is visible and there's an editor or placeholder
	// (don't draw if terminal is directly adjacent - it draws its own left border)
	// Start at Y=1 to avoid overwriting pane nav bar at Y=0
	if lm.TreeVisible && (lm.EditorVisible || lm.needsPlaceholders()) {
		treeW := lm.getTreeWidth()
		for y := 1; y < lm.ScreenH; y++ {
			screen.SetContent(treeW, y, PowerlineArrowRight, nil, powerlineStyle)
		}
	}

	// Only draw divider before terminal if terminal doesn't exist but is supposed to be visible
	// (terminal draws its own border which serves as the divider when visible)
	lm.mu.RLock()
	hasTerminal := lm.Terminal != nil
	hasTerminal2 := lm.Terminal2 != nil
	hasTerminal3 := lm.Terminal3 != nil
	lm.mu.RUnlock()

	if !hasTerminal && lm.TerminalVisible {
		termX := lm.getTermX()
		for y := 1; y < lm.ScreenH; y++ {
			screen.SetContent(termX, y, PowerlineArrowRight, nil, powerlineStyle)
		}
	}

	// Draw divider between Terminal and Terminal2 if both are visible but Terminal2 not yet created
	if lm.TerminalVisible && lm.Terminal2Visible && hasTerminal && !hasTerminal2 {
		term2X := lm.getTerm2X()
		for y := 1; y < lm.ScreenH; y++ {
			screen.SetContent(term2X, y, PowerlineArrowRight, nil, powerlineStyle)
		}
	}

	// Draw divider between Terminal2 and Terminal3 if both are visible but Terminal3 not yet created
	if lm.Terminal2Visible && lm.Terminal3Visible && hasTerminal2 && !hasTerminal3 {
		term3X := lm.getTerm3X()
		for y := 1; y < lm.ScreenH; y++ {
			screen.SetContent(term3X, y, PowerlineArrowRight, nil, powerlineStyle)
		}
	}
}

// drawPlaceholders draws placeholder boxes when both editor and terminal are hidden
func (lm *LayoutManager) drawPlaceholders(screen tcell.Screen) {
	// Calculate positions (content starts at Y=1 due to pane nav bar)
	startX := lm.getTreeWidth()
	availableWidth := lm.ScreenW - startX
	centerY := 1 + (lm.ScreenH-1)/2 // Center in content area below nav bar

	// Each placeholder is a small box
	boxWidth := PlaceholderWidth
	boxHeight := 5
	gap := 2

	// Center both placeholders horizontally
	totalWidth := boxWidth*2 + gap
	editorX := startX + (availableWidth-totalWidth)/2
	terminalX := editorX + boxWidth + gap

	// Draw editor placeholder
	lm.drawPlaceholderBox(screen, editorX, centerY-boxHeight/2, boxWidth, boxHeight, "Editor", "Alt+2")

	// Draw terminal placeholder
	lm.drawPlaceholderBox(screen, terminalX, centerY-boxHeight/2, boxWidth, boxHeight, "Terminal", "Alt+3")
}

// drawPlaceholderBox draws a single placeholder box with title and shortcut hint
func (lm *LayoutManager) drawPlaceholderBox(screen tcell.Screen, x, y, width, height int, title, shortcut string) {
	borderStyle := tcell.StyleDefault.Foreground(tcell.NewRGBColor(100, 40, 140)) // Darker violet
	titleStyle := tcell.StyleDefault.Foreground(tcell.Color205) // Hot pink
	hintStyle := tcell.StyleDefault.Foreground(tcell.Color51)   // Cyan

	// Draw border
	// Top border
	screen.SetContent(x, y, '┌', nil, borderStyle)
	screen.SetContent(x+width-1, y, '┐', nil, borderStyle)
	for i := 1; i < width-1; i++ {
		screen.SetContent(x+i, y, '─', nil, borderStyle)
	}

	// Bottom border
	screen.SetContent(x, y+height-1, '└', nil, borderStyle)
	screen.SetContent(x+width-1, y+height-1, '┘', nil, borderStyle)
	for i := 1; i < width-1; i++ {
		screen.SetContent(x+i, y+height-1, '─', nil, borderStyle)
	}

	// Side borders
	for i := 1; i < height-1; i++ {
		screen.SetContent(x, y+i, '│', nil, borderStyle)
		screen.SetContent(x+width-1, y+i, '│', nil, borderStyle)
	}

	// Clear inside
	bgStyle := tcell.StyleDefault.Background(tcell.ColorBlack)
	for row := 1; row < height-1; row++ {
		for col := 1; col < width-1; col++ {
			screen.SetContent(x+col, y+row, ' ', nil, bgStyle)
		}
	}

	// Draw title (centered, row 1)
	titleX := x + (width-len(title))/2
	for i, r := range title {
		screen.SetContent(titleX+i, y+1, r, nil, titleStyle)
	}

	// Draw shortcut hint (centered, row 2)
	hintText := shortcut + " to show"
	hintX := x + (width-len(hintText))/2
	for i, r := range hintText {
		screen.SetContent(hintX+i, y+2, r, nil, hintStyle)
	}
}

// drawEditorBorder draws a border around the editor region (pink when focused, dim when not)
func (lm *LayoutManager) drawEditorBorder(screen tcell.Screen, focused bool) {
	// Choose style based on focus state (pink for Spider-Verse vibe)
	var style tcell.Style
	if focused {
		style = config.DefStyle.Foreground(tcell.Color205) // Hot pink
	} else {
		style = config.DefStyle.Foreground(tcell.NewRGBColor(100, 40, 140)) // Darker violet
	}

	editorX := lm.getTreeWidth()
	editorW := lm.getEditorWidth()
	// Editor starts at Y=1 (below pane nav bar) and goes to bottom
	startY := 1
	h := lm.ScreenH

	// Clear the border areas first (in case editor rendered there)
	clearStyle := config.DefStyle
	// Top row
	for x := editorX; x < editorX+editorW; x++ {
		screen.SetContent(x, startY, ' ', nil, clearStyle)
	}
	// Bottom row
	for x := editorX; x < editorX+editorW; x++ {
		screen.SetContent(x, h-1, ' ', nil, clearStyle)
	}
	// Left column
	for y := startY; y < h; y++ {
		screen.SetContent(editorX, y, ' ', nil, clearStyle)
	}
	// Right column
	for y := startY; y < h; y++ {
		screen.SetContent(editorX+editorW-1, y, ' ', nil, clearStyle)
	}

	// Draw vertical lines on left and right (double-line when focused, single when not)
	if focused {
		for y := startY + 1; y < h-1; y++ {
			screen.SetContent(editorX, y, '║', nil, style)
			screen.SetContent(editorX+editorW-1, y, '║', nil, style)
		}
		for x := 1; x < editorW-1; x++ {
			screen.SetContent(editorX+x, startY, '═', nil, style)
			screen.SetContent(editorX+x, h-1, '═', nil, style)
		}
		// Double-line corners
		screen.SetContent(editorX, startY, '╔', nil, style)
		screen.SetContent(editorX+editorW-1, startY, '╗', nil, style)
		screen.SetContent(editorX, h-1, '╚', nil, style)
		screen.SetContent(editorX+editorW-1, h-1, '╝', nil, style)
	} else {
		for y := startY + 1; y < h-1; y++ {
			screen.SetContent(editorX, y, '│', nil, style)
			screen.SetContent(editorX+editorW-1, y, '│', nil, style)
		}
		for x := 1; x < editorW-1; x++ {
			screen.SetContent(editorX+x, startY, '─', nil, style)
			screen.SetContent(editorX+x, h-1, '─', nil, style)
		}
		// Single-line corners
		screen.SetContent(editorX, startY, '┌', nil, style)
		screen.SetContent(editorX+editorW-1, startY, '┐', nil, style)
		screen.SetContent(editorX, h-1, '└', nil, style)
		screen.SetContent(editorX+editorW-1, h-1, '┘', nil, style)
	}
}

// updateLayout recalculates panel regions without changing screen size
// Called when focus changes (which may affect tree width)
func (lm *LayoutManager) updateLayout() {
	if lm.ScreenW == 0 || lm.ScreenH == 0 {
		return // Not initialized yet
	}

	// Update file browser region
	if lm.FileBrowser != nil {
		lm.FileBrowser.Region.Width = lm.getTreeWidth()
	}

	// Update source control region (uses same space as file browser)
	if lm.SourceControl != nil {
		lm.SourceControl.Region.Width = lm.getTreeWidth()
	}

	// Update terminal regions
	lm.mu.RLock()
	term := lm.Terminal
	term2 := lm.Terminal2
	term3 := lm.Terminal3
	lm.mu.RUnlock()

	if term != nil {
		term.Region.X = lm.getTermX()
		// Call Resize with new values - it will skip if size unchanged
		_ = term.Resize(lm.getTermWidth(), term.Region.Height)
	}

	if term2 != nil {
		term2.Region.X = lm.getTerm2X()
		// Call Resize with new values - it will skip if size unchanged
		_ = term2.Resize(lm.getTerm2Width(), term2.Region.Height)
	}

	if term3 != nil {
		term3.Region.X = lm.getTerm3X()
		// Call Resize with new values - it will skip if size unchanged
		_ = term3.Resize(lm.getTerm3Width(), term3.Region.Height)
	}
}

// Resize handles screen resize events
func (lm *LayoutManager) Resize(w, h int) {
	lm.ScreenW = w
	lm.ScreenH = h

	// Content height (screen height minus pane nav bar)
	contentH := h - 1

	// Resize file browser
	if lm.FileBrowser != nil {
		lm.FileBrowser.Region.Y = 1
		lm.FileBrowser.Region.Width = lm.getTreeWidth()
		lm.FileBrowser.Region.Height = contentH
	}

	// Resize source control (uses same space as file browser)
	if lm.SourceControl != nil {
		lm.SourceControl.Region.Y = 1
		lm.SourceControl.Region.Width = lm.getTreeWidth()
		lm.SourceControl.Region.Height = contentH
	}

	// Resize terminals
	lm.mu.RLock()
	term := lm.Terminal
	term2 := lm.Terminal2
	term3 := lm.Terminal3
	lm.mu.RUnlock()

	if term != nil {
		term.Region.X = lm.getTermX()
		term.Region.Y = 1
		// Call Resize with new values - it will skip if size unchanged and update Region
		_ = term.Resize(lm.getTermWidth(), contentH)
	}

	if term2 != nil {
		term2.Region.X = lm.getTerm2X()
		term2.Region.Y = 1
		// Call Resize with new values - it will skip if size unchanged and update Region
		_ = term2.Resize(lm.getTerm2Width(), contentH)
	}

	if term3 != nil {
		term3.Region.X = lm.getTerm3X()
		term3.Region.Y = 1
		// Call Resize with new values - it will skip if size unchanged and update Region
		_ = term3.Resize(lm.getTerm3Width(), contentH)
	}

	log.Printf("THICC: Layout resized to %dx%d (tree=%d, editor=%d, term=%d, term2=%d, term3=%d)",
		w, h, lm.TreeWidth, lm.getEditorWidth(), lm.getTermWidth(), lm.getTerm2Width(), lm.getTerm3Width())
}

// Close cleans up resources
func (lm *LayoutManager) Close() {
	// Stop idle checker goroutine
	if lm.idleCheckStop != nil {
		close(lm.idleCheckStop)
	}

	if lm.FileBrowser != nil {
		lm.FileBrowser.Close()
	}

	if lm.FileIndex != nil {
		lm.FileIndex.Close()
	}

	lm.mu.RLock()
	term := lm.Terminal
	term2 := lm.Terminal2
	term3 := lm.Terminal3
	lm.mu.RUnlock()

	if term != nil {
		term.Close()
	}
	if term2 != nil {
		term2.Close()
	}
	if term3 != nil {
		term3.Close()
	}

	log.Println("THICC: Layout closed")
}

// FocusTree sets focus to file browser
func (lm *LayoutManager) FocusTree() {
	if lm.FileBrowser != nil {
		lm.setActivePanel(0)
		log.Println("THICC: Focus set to file browser")
	}
}

// FocusEditor sets focus to editor and pins the active preview tab
func (lm *LayoutManager) FocusEditor() {
	lm.setActivePanel(1)

	// Pin the active tab if it's a preview (user is now focusing on it)
	if lm.TabBar != nil {
		activeTab := lm.TabBar.GetActiveTab()
		if activeTab != nil && activeTab.IsPreview {
			log.Printf("THICC: Pinning preview tab on editor focus: %s", activeTab.Name)
			lm.TabBar.PinTab(lm.TabBar.ActiveIndex)
		}
	}

	log.Println("THICC: Focus set to editor")
}

// FocusTerminal sets focus to terminal
func (lm *LayoutManager) FocusTerminal() {
	lm.mu.RLock()
	hasTerminal := lm.Terminal != nil
	lm.mu.RUnlock()

	if hasTerminal {
		lm.setActivePanel(2)
		log.Println("THICC: Focus set to terminal")
	}
}

// ToggleTree toggles the visibility of the tree pane
func (lm *LayoutManager) ToggleTree() {
	if !lm.TreeVisible {
		// Show tree, hide source control (mutually exclusive)
		lm.TreeVisible = true
		lm.SourceControlVisible = false
		log.Printf("THICC: Tree visibility toggled to %v (hiding Source Control)", lm.TreeVisible)
		// Re-enable watching if not suspended
		if !lm.watchersSuspended && lm.FileBrowser != nil && lm.FileBrowser.Tree != nil {
			lm.FileBrowser.Tree.EnableWatching()
		}
		lm.setActivePanel(0)
	} else {
		// Hide tree - disable watching to save resources
		lm.TreeVisible = false
		if lm.FileBrowser != nil && lm.FileBrowser.Tree != nil {
			lm.FileBrowser.Tree.DisableWatching()
		}
		log.Printf("THICC: Tree visibility toggled to %v", lm.TreeVisible)

		// If we just hid the focused pane, move focus to next visible pane
		if lm.ActivePanel == 0 {
			lm.focusNextVisiblePane()
		}
	}

	// Update panel regions
	lm.updatePanelRegions()
	lm.triggerRedraw()
}

// ToggleEditor toggles the visibility of the editor pane
func (lm *LayoutManager) ToggleEditor() {
	lm.EditorVisible = !lm.EditorVisible
	log.Printf("THICC: Editor visibility toggled to %v", lm.EditorVisible)

	// If we just hid the focused pane, move focus to next visible pane
	if !lm.EditorVisible && lm.ActivePanel == 1 {
		lm.focusNextVisiblePane()
	}

	// Update panel regions
	lm.updatePanelRegions()
	lm.triggerRedraw()
}

// ToggleTerminal toggles the visibility of the terminal pane
func (lm *LayoutManager) ToggleTerminal() {
	if !lm.TerminalVisible {
		// Showing Terminal
		lm.TerminalVisible = true
		log.Printf("THICC: Terminal visibility toggled to %v", lm.TerminalVisible)

		// If not initialized, show tool selector
		if !lm.TerminalInitialized {
			lm.showToolSelectorFor(2) // 2 = terminal1 panel
			return
		}

		// Already initialized, just show it and focus
		lm.setActivePanel(2)
		lm.updatePanelRegions()
		lm.triggerRedraw()
	} else {
		// Hiding Terminal
		lm.TerminalVisible = false
		log.Printf("THICC: Terminal visibility toggled to %v", lm.TerminalVisible)

		// If we just hid the focused pane, move focus to next visible pane
		if lm.ActivePanel == 2 {
			lm.focusNextVisiblePane()
		}

		// Update panel regions
		lm.updatePanelRegions()
		lm.triggerRedraw()
	}
}

// ToggleTerminal2 toggles the visibility of the second terminal pane
func (lm *LayoutManager) ToggleTerminal2() {
	if !lm.Terminal2Visible {
		// Showing Terminal2
		lm.Terminal2Visible = true
		log.Printf("THICC: Terminal2 visibility toggled to %v", lm.Terminal2Visible)

		// If not initialized, show tool selector
		if !lm.Terminal2Initialized {
			lm.showToolSelectorFor(3) // 3 = terminal2 panel
			return
		}

		// Already initialized, just show it and focus
		lm.setActivePanel(3)
		lm.updatePanelRegions()
		lm.triggerRedraw()
	} else {
		// Hiding Terminal2
		lm.Terminal2Visible = false
		log.Printf("THICC: Terminal2 visibility toggled to %v", lm.Terminal2Visible)

		// If we just hid the focused pane, move focus to next visible pane
		if lm.ActivePanel == 3 {
			lm.focusNextVisiblePane()
		}

		// Update panel regions
		lm.updatePanelRegions()
		lm.triggerRedraw()
	}
}

// ToggleTerminal3 toggles the visibility of the third terminal pane
func (lm *LayoutManager) ToggleTerminal3() {
	if !lm.Terminal3Visible {
		// Showing Terminal3
		lm.Terminal3Visible = true
		log.Printf("THICC: Terminal3 visibility toggled to %v", lm.Terminal3Visible)

		// If not initialized, show tool selector
		if !lm.Terminal3Initialized {
			lm.showToolSelectorFor(4) // 4 = terminal3 panel
			return
		}

		// Already initialized, just show it and focus
		lm.setActivePanel(4)
		lm.updatePanelRegions()
		lm.triggerRedraw()
	} else {
		// Hiding Terminal3
		lm.Terminal3Visible = false
		log.Printf("THICC: Terminal3 visibility toggled to %v", lm.Terminal3Visible)

		// If we just hid the focused pane, move focus to next visible pane
		if lm.ActivePanel == 4 {
			lm.focusNextVisiblePane()
		}

		// Update panel regions
		lm.updatePanelRegions()
		lm.triggerRedraw()
	}
}

// ToggleSourceControl toggles the visibility of the source control pane
// Source Control and File Browser are mutually exclusive - showing one hides the other
func (lm *LayoutManager) ToggleSourceControl() {
	if !lm.SourceControlVisible {
		// Show Source Control, hide File Browser
		lm.SourceControlVisible = true
		lm.TreeVisible = false
		log.Printf("THICC: Source Control visibility toggled to %v (hiding File Browser)", lm.SourceControlVisible)

		// Initialize source control if needed
		if lm.SourceControl == nil {
			lm.initSourceControl()
		} else {
			// Refresh git status
			lm.SourceControl.RefreshStatus()
		}

		// Focus source control and start polling
		lm.setActivePanel(0)
		lm.SourceControl.StartPolling()
		lm.updatePanelRegions()
		lm.triggerRedraw()
	} else {
		// Hide Source Control and stop polling
		lm.SourceControlVisible = false
		if lm.SourceControl != nil {
			lm.SourceControl.StopPolling()
		}
		log.Printf("THICC: Source Control visibility toggled to %v", lm.SourceControlVisible)

		// If we just hid the focused pane, move focus to next visible pane
		if lm.ActivePanel == 0 {
			lm.focusNextVisiblePane()
		}

		// Update panel regions
		lm.updatePanelRegions()
		lm.triggerRedraw()
	}
}

// initSourceControl initializes the source control panel
func (lm *LayoutManager) initSourceControl() {
	contentH := lm.ScreenH - 1
	treeW := lm.getTreeWidth()
	lm.SourceControl = sourcecontrol.NewPanel(
		0, 1,
		treeW, contentH,
		lm.Root,
	)

	// Set up callbacks
	lm.SourceControl.OnFileSelect = func(path string, isStaged bool) {
		log.Printf("THICC: Source Control file selected: %s (staged=%v)", path, isStaged)
		// Open the file in the editor for diff view
		lm.openFileForDiff(path)
	}

	lm.SourceControl.OnCommitSelect = func(commitHash string, path string) {
		log.Printf("THICC: Source Control commit file selected: %s in commit %s", path, commitHash)
		// Open the commit diff in the editor
		lm.openCommitDiff(commitHash, path)
	}

	lm.SourceControl.OnRefresh = func() {
		lm.triggerRedraw()
	}
}

// openFileForDiff opens a file in the editor and shows unified diff
func (lm *LayoutManager) openFileForDiff(path string) {
	log.Printf("THICC: openFileForDiff called with path: %s", path)

	// Make path absolute
	absPath := path
	if !filepath.IsAbs(path) {
		absPath = filepath.Join(lm.Root, path)
	}
	log.Printf("THICC: Absolute path for diff: %s", absPath)

	// Hide terminal for cleaner diff view (only SC + editor visible)
	lm.TerminalVisible = false

	// Show editor if hidden
	lm.EditorVisible = true
	lm.updatePanelRegions()

	// Show unified diff in editor
	diffBuf, success := action.ShowUnifiedDiff(absPath)
	log.Printf("THICC: ShowUnifiedDiff returned: %v", success)

	// Update the tab bar with the diff buffer
	if success && diffBuf != nil && lm.TabBar != nil {
		// Update the current tab's buffer reference and name
		if lm.TabBar.ActiveIndex >= 0 && lm.TabBar.ActiveIndex < len(lm.TabBar.Tabs) {
			lm.TabBar.Tabs[lm.TabBar.ActiveIndex].Buffer = diffBuf
			lm.TabBar.Tabs[lm.TabBar.ActiveIndex].Name = truncateName(diffBuf.GetName())
			lm.TabBar.Tabs[lm.TabBar.ActiveIndex].Loaded = true
		}
	}

	// Keep focus on SC so user can navigate through files
	lm.triggerRedraw()
}

// openCommitDiff opens the diff for a file in a specific commit
func (lm *LayoutManager) openCommitDiff(commitHash string, path string) {
	log.Printf("THICC: openCommitDiff called with commit: %s, path: %s", commitHash, path)

	// Hide terminal for cleaner diff view (only SC + editor visible)
	lm.TerminalVisible = false

	// Show editor if hidden
	lm.EditorVisible = true
	lm.updatePanelRegions()

	// Show commit diff in editor
	diffBuf, success := action.ShowCommitDiff(commitHash, path, lm.Root)
	log.Printf("THICC: ShowCommitDiff returned: %v", success)

	// Update the tab bar with the diff buffer
	if success && diffBuf != nil && lm.TabBar != nil {
		// Update the current tab's buffer reference and name
		if lm.TabBar.ActiveIndex >= 0 && lm.TabBar.ActiveIndex < len(lm.TabBar.Tabs) {
			lm.TabBar.Tabs[lm.TabBar.ActiveIndex].Buffer = diffBuf
			lm.TabBar.Tabs[lm.TabBar.ActiveIndex].Name = truncateName(diffBuf.GetName())
			lm.TabBar.Tabs[lm.TabBar.ActiveIndex].Loaded = true
		}
	}

	// Keep focus on SC so user can navigate through commits
	lm.triggerRedraw()
}

// showToolSelectorFor shows the tool selector for the specified terminal panel
func (lm *LayoutManager) showToolSelectorFor(panel int) {
	if lm.ToolSelector == nil {
		lm.ToolSelector = NewToolSelector()
	}

	lm.ToolSelectorTarget = panel
	lm.ShowingToolSelector = true

	lm.ToolSelector.Show(
		func(cmdArgs []string) {
			// Tool selected - create terminal with this command
			lm.createTerminalForPanel(panel, cmdArgs)
		},
		func(installCmd string) {
			// Install selected - create shell terminal and run install command
			lm.createTerminalWithInstallCommand(panel, installCmd)
		},
		func() {
			// Cancelled - default to shell
			lm.createTerminalForPanel(panel, nil)
		},
	)

	lm.triggerRedraw()
}

// createTerminalForPanel creates a terminal for the specified panel with the given command
func (lm *LayoutManager) createTerminalForPanel(panel int, cmdArgs []string) {
	// Hide tool selector FIRST
	lm.ShowingToolSelector = false

	// Set active panel BEFORE calculating dimensions
	// This ensures shouldExpandTree() returns the correct value for the final layout
	// (tree collapses when terminal is active, giving terminal more space)
	lm.setActivePanel(panel)

	var termX, termW int

	switch panel {
	case 2: // Terminal1 (main terminal)
		termX = lm.getTermX()
		termW = lm.getTermWidth()
	case 3: // Terminal2
		termX = lm.getTerm2X()
		termW = lm.getTerm2Width()
	case 4: // Terminal3
		termX = lm.getTerm3X()
		termW = lm.getTerm3Width()
	default:
		log.Printf("THICC: Invalid panel for terminal creation: %d", panel)
		return
	}

	log.Printf("THICC: createTerminalForPanel: panel=%d, x=%d, w=%d, ActivePanel=%d, shouldExpandTree=%v, TerminalVisible=%v, EditorVisible=%v",
		panel, termX, termW, lm.ActivePanel, lm.shouldExpandTree(), lm.TerminalVisible, lm.EditorVisible)

	// Mark if we're spawning an AI tool (not a shell)
	if cmdArgs != nil && len(cmdArgs) > 0 && !isShellCommand(cmdArgs) {
		lm.MarkAIToolSpawned()
	}

	go func() {
		term, err := terminal.NewPanel(termX, 1, termW, lm.ScreenH-1, cmdArgs)
		if err != nil {
			log.Printf("THICC: Failed to create terminal for panel %d: %v", panel, err)
			return
		}

		lm.setupTerminalCallbacks(term)

		lm.mu.Lock()
		switch panel {
		case 2:
			lm.Terminal = term
			lm.TerminalInitialized = true
		case 3:
			lm.Terminal2 = term
			lm.Terminal2Initialized = true
		case 4:
			lm.Terminal3 = term
			lm.Terminal3Initialized = true
		}
		lm.mu.Unlock()

		log.Printf("THICC: Terminal for panel %d created successfully", panel)

		// Update regions (active panel already set before calculating dimensions)
		lm.updatePanelRegions()
		lm.triggerRedraw()
	}()
}

// createTerminalWithInstallCommand creates a shell terminal and types the install command
func (lm *LayoutManager) createTerminalWithInstallCommand(panel int, installCmd string) {
	// Hide tool selector FIRST
	lm.ShowingToolSelector = false

	// Set active panel BEFORE calculating dimensions
	// This ensures shouldExpandTree() returns the correct value for the final layout
	lm.setActivePanel(panel)

	var termX, termW int

	switch panel {
	case 2: // Terminal1 (main terminal)
		termX = lm.getTermX()
		termW = lm.getTermWidth()
	case 3: // Terminal2
		termX = lm.getTerm2X()
		termW = lm.getTerm2Width()
	case 4: // Terminal3
		termX = lm.getTerm3X()
		termW = lm.getTerm3Width()
	default:
		log.Printf("THICC: Invalid panel for terminal creation: %d", panel)
		return
	}

	log.Printf("THICC: Creating terminal for panel %d with install command: %s", panel, installCmd)

	go func() {
		// Create a shell terminal (nil cmdArgs = default shell)
		term, err := terminal.NewPanel(termX, 1, termW, lm.ScreenH-1, nil)
		if err != nil {
			log.Printf("THICC: Failed to create terminal for panel %d: %v", panel, err)
			return
		}

		lm.setupTerminalCallbacks(term)

		lm.mu.Lock()
		switch panel {
		case 2:
			lm.Terminal = term
			lm.TerminalInitialized = true
		case 3:
			lm.Terminal2 = term
			lm.Terminal2Initialized = true
		case 4:
			lm.Terminal3 = term
			lm.Terminal3Initialized = true
		}
		lm.mu.Unlock()

		log.Printf("THICC: Terminal for panel %d created successfully", panel)

		// Wait a bit for shell to initialize, then type the install command
		go func() {
			// Give the shell time to start and show prompt
			time.Sleep(500 * time.Millisecond)

			// Type the install command (user needs to press Enter to execute)
			term.Write([]byte(installCmd))
			log.Printf("THICC: Typed install command: %s", installCmd)
		}()

		// Update regions (active panel already set before calculating dimensions)
		lm.updatePanelRegions()
		lm.triggerRedraw()
	}()
}

// SpawnTerminalWithInstallCommand spawns the main terminal with an install command typed
// This is used when user selects an installable tool from the dashboard
func (lm *LayoutManager) SpawnTerminalWithInstallCommand(installCmd string) {
	log.Printf("THICC: Spawning terminal with install command: %s", installCmd)

	// Wait for terminal to be ready, then type the install command
	// Terminal is created asynchronously, so we need to poll for it
	go func() {
		// Wait for terminal to be created (poll with timeout)
		maxWait := 10 * time.Second
		pollInterval := 100 * time.Millisecond
		waited := time.Duration(0)

		for waited < maxWait {
			lm.mu.RLock()
			term := lm.Terminal
			lm.mu.RUnlock()

			if term != nil {
				// Terminal is ready - wait for shell prompt injection to complete
				// injectSexyPrompt waits 1000ms before writing, so we wait 1500ms total
				log.Printf("THICC: Terminal ready after %v, waiting for prompt injection to complete", waited)
				time.Sleep(1500 * time.Millisecond)

				// Type the install command (user needs to press Enter to execute)
				term.Write([]byte(installCmd))
				log.Printf("THICC: Typed install command: %s", installCmd)

				// Focus the terminal
				lm.setActivePanel(2)
				lm.triggerRedraw()
				return
			}

			time.Sleep(pollInterval)
			waited += pollInterval
		}

		log.Printf("THICC: Timed out waiting for terminal after %v", maxWait)
	}()
}

// focusNextVisiblePane moves focus to the next visible pane
// Priority: Editor > Terminal > Tree
func (lm *LayoutManager) focusNextVisiblePane() {
	// Try editor first
	if lm.EditorVisible {
		lm.setActivePanel(1)
		log.Println("THICC: Focus moved to editor")
		return
	}

	// Try terminal
	if lm.TerminalVisible {
		lm.mu.RLock()
		hasTerminal := lm.Terminal != nil
		lm.mu.RUnlock()
		if hasTerminal {
			lm.setActivePanel(2)
			log.Println("THICC: Focus moved to terminal")
			return
		}
	}

	// Try terminal2
	if lm.Terminal2Visible {
		lm.mu.RLock()
		hasTerminal2 := lm.Terminal2 != nil
		lm.mu.RUnlock()
		if hasTerminal2 {
			lm.setActivePanel(3)
			log.Println("THICC: Focus moved to terminal2")
			return
		}
	}

	// Try terminal3
	if lm.Terminal3Visible {
		lm.mu.RLock()
		hasTerminal3 := lm.Terminal3 != nil
		lm.mu.RUnlock()
		if hasTerminal3 {
			lm.setActivePanel(4)
			log.Println("THICC: Focus moved to terminal3")
			return
		}
	}

	// Try tree
	if lm.TreeVisible && lm.FileBrowser != nil {
		lm.setActivePanel(0)
		log.Println("THICC: Focus moved to tree")
		return
	}

	// No visible pane with focus capability
	log.Println("THICC: No visible pane to focus")
}

// updatePanelRegions recalculates and updates all panel regions based on visibility
func (lm *LayoutManager) updatePanelRegions() {
	// Content height (screen height minus pane nav bar)
	contentH := lm.ScreenH - 1

	// Update file browser region
	if lm.FileBrowser != nil {
		if lm.TreeVisible && !lm.SourceControlVisible {
			lm.FileBrowser.Region.X = 0
			lm.FileBrowser.Region.Y = 1
			lm.FileBrowser.Region.Width = lm.getTreeWidth()
			lm.FileBrowser.Region.Height = contentH
		} else {
			lm.FileBrowser.Region.Width = 0
		}
	}

	// Update source control region (uses same space as file browser)
	if lm.SourceControl != nil {
		if lm.SourceControlVisible {
			lm.SourceControl.Region.X = 0
			lm.SourceControl.Region.Y = 1
			lm.SourceControl.Region.Width = lm.getTreeWidth()
			lm.SourceControl.Region.Height = contentH
		} else {
			lm.SourceControl.Region.Width = 0
		}
	}

	// Update terminal regions
	lm.mu.RLock()
	term := lm.Terminal
	term2 := lm.Terminal2
	term3 := lm.Terminal3
	lm.mu.RUnlock()

	if term != nil {
		if lm.TerminalVisible {
			newX := lm.getTermX()
			newW := lm.getTermWidth()
			log.Printf("THICC: updatePanelRegions term1: newX=%d, newW=%d, ActivePanel=%d, shouldExpandTree=%v",
				newX, newW, lm.ActivePanel, lm.shouldExpandTree())
			term.Region.X = newX
			term.Region.Y = 1
			// Call Resize with new values - it will skip if size unchanged
			_ = term.Resize(newW, contentH)
		} else {
			term.Region.Width = 0
		}
	}

	if term2 != nil {
		if lm.Terminal2Visible {
			term2.Region.X = lm.getTerm2X()
			term2.Region.Y = 1
			// Call Resize with new values - it will skip if size unchanged
			_ = term2.Resize(lm.getTerm2Width(), contentH)
		} else {
			term2.Region.Width = 0
		}
	}

	if term3 != nil {
		if lm.Terminal3Visible {
			term3.Region.X = lm.getTerm3X()
			term3.Region.Y = 1
			// Call Resize with new values - it will skip if size unchanged
			_ = term3.Resize(lm.getTerm3Width(), contentH)
		} else {
			term3.Region.Width = 0
		}
	}

	log.Printf("THICC: Panel regions updated (tree=%d, editor=%d, term=%d, term2=%d, term3=%d)",
		lm.getTreeWidth(), lm.getEditorWidth(), lm.getTermWidth(), lm.getTerm2Width(), lm.getTerm3Width())
}

// ShowShortcutsModal displays the keyboard shortcuts help modal
func (lm *LayoutManager) ShowShortcutsModal() {
	if lm.ShortcutsModal != nil {
		lm.ShortcutsModal.Show(lm.ScreenW, lm.ScreenH)
		lm.triggerRedraw()
	}
}

// SyncInitialTab ensures the TabBar has the initial buffer as a tab
// Called on each render to ensure Untitled tab shows on startup
func (lm *LayoutManager) SyncInitialTab() {
	if lm.TabBar == nil {
		return
	}
	// If no tabs, add the current buffer
	if len(lm.TabBar.Tabs) == 0 {
		tab := action.MainTab()
		if tab != nil {
			for _, pane := range tab.Panes {
				if bp, ok := pane.(*action.BufPane); ok {
					lm.TabBar.AddTab(bp.Buf)
					log.Println("THICC: Synced initial buffer to tab bar")
					break
				}
			}
		}
	}
}

// checkAndPinOnModification checks if the active preview tab was modified and pins it
// Called during render to auto-pin when user starts editing
func (lm *LayoutManager) checkAndPinOnModification() {
	if lm.TabBar == nil {
		return
	}

	activeTab := lm.TabBar.GetActiveTab()
	if activeTab == nil || !activeTab.IsPreview {
		return
	}

	// If the buffer has been modified, pin the tab
	if activeTab.Buffer != nil && activeTab.Buffer.Modified() {
		log.Printf("THICC: Pinning preview tab due to modification: %s", activeTab.Name)
		lm.TabBar.PinTab(lm.TabBar.ActiveIndex)
	}
}

// GetWorkInProgress checks all buffers and terminals for unsaved work
func (lm *LayoutManager) GetWorkInProgress() *WorkInProgress {
	wip := &WorkInProgress{}

	// Check ALL open buffers
	for _, b := range buffer.OpenBuffers {
		if b.Modified() {
			name := b.GetName()
			if name == "" {
				name = "Untitled"
			}
			wip.UnsavedBuffers = append(wip.UnsavedBuffers, name)
		}
	}

	// Always check terminal panels for AI tools (removed early return)
	// This catches manually launched AI tools like typing "claude" in shell
	lm.mu.RLock()
	defer lm.mu.RUnlock()

	log.Printf("THICC: GetWorkInProgress checking terminals - T1=%v T2=%v T3=%v",
		lm.Terminal != nil, lm.Terminal2 != nil, lm.Terminal3 != nil)

	if lm.Terminal != nil {
		if tool := lm.Terminal.GetForegroundAITool(); tool != "" {
			log.Printf("THICC: Terminal1 has AI tool: %s", tool)
			wip.ActiveAISessions = append(wip.ActiveAISessions, tool)
		}
	}
	if lm.Terminal2 != nil {
		if tool := lm.Terminal2.GetForegroundAITool(); tool != "" {
			log.Printf("THICC: Terminal2 has AI tool: %s", tool)
			wip.ActiveAISessions = append(wip.ActiveAISessions, tool)
		}
	}
	if lm.Terminal3 != nil {
		if tool := lm.Terminal3.GetForegroundAITool(); tool != "" {
			log.Printf("THICC: Terminal3 has AI tool: %s", tool)
			wip.ActiveAISessions = append(wip.ActiveAISessions, tool)
		}
	}

	return wip
}

// handleQuit handles the quit action with warning for unsaved changes or active AI sessions
func (lm *LayoutManager) handleQuit() bool {
	log.Println("THICC: handleQuit called")

	// Always do full detection - removed fast path to catch manually launched AI tools
	lm.LoadingOverlay.Show("Quitting...")
	lm.triggerRedraw()

	// Do the dynamic PTY detection (this may involve syscalls)
	go func() {
		wip := lm.GetWorkInProgress()

		// Hide loading overlay
		lm.LoadingOverlay.Hide()

		if !wip.HasWork() {
			log.Println("THICC: No work in progress, quitting")
			lm.forceQuitAll()
			return
		}

		log.Printf("THICC: Work in progress detected: unsaved=%v, aiSessions=%v",
			wip.UnsavedBuffers, wip.ActiveAISessions)

		// Show danger modal with work-in-progress warning
		lm.ConfirmModal.Show(
			"Exit THICC?",
			wip.Message(),
			"All changes will be lost!",
			lm.ScreenW, lm.ScreenH,
			func(confirmed bool) {
				if confirmed {
					lm.forceQuitAll()
				}
			},
		)
		lm.triggerRedraw()
	}()

	return true
}

// forceQuitAll closes all buffers and exits the application
func (lm *LayoutManager) forceQuitAll() {
	// Close all buffers and exit
	buffer.CloseOpenBuffers()
	screen.Screen.Fini()
	action.InfoBar.Close()
	runtime.Goexit()
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
	term2 := lm.Terminal2
	term3 := lm.Terminal3
	lm.mu.RUnlock()

	if term != nil && term.Focus {
		term.ShowCursor(screen)
	}
	if term2 != nil && term2.Focus {
		term2.ShowCursor(screen)
	}
	if term3 != nil && term3.Focus {
		term3.ShowCursor(screen)
	}
}

// triggerRedraw posts a resize event to wake up the event loop and trigger a screen refresh
func (lm *LayoutManager) triggerRedraw() {
	if lm.Screen != nil {
		log.Println("THICC: Posting resize event to trigger screen refresh")
		// Post a resize event with current dimensions to wake up event loop
		w, h := lm.Screen.Size()
		lm.Screen.PostEvent(tcell.NewEventResize(w, h))
	}
}

// handleTerminalPaste intercepts paste commands when terminal has focus
// Returns true if event was a paste that was handled
func (lm *LayoutManager) handleTerminalPaste(event tcell.Event) bool {
	lm.mu.RLock()
	var term *terminal.Panel
	switch lm.ActivePanel {
	case 2:
		term = lm.Terminal
	case 3:
		term = lm.Terminal2
	case 4:
		term = lm.Terminal3
	}
	lm.mu.RUnlock()

	if term == nil {
		return false
	}

	// Handle tcell.EventPaste (bracketed paste from outer terminal)
	if ev, ok := event.(*tcell.EventPaste); ok {
		log.Printf("THICC: EventPaste for terminal, len=%d", len(ev.Text()))
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
			log.Println("THICC: Paste keybinding for terminal")
			clip, err := clipboard.Read(clipboard.ClipboardReg)
			if err != nil {
				log.Printf("THICC: Clipboard read failed: %v", err)
				return true // Still consume to prevent editor paste
			}
			log.Printf("THICC: Pasting %d bytes to terminal", len(clip))
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
	const paneNavBarHeight = 1 // Pane nav bar at Y=0
	const borderOffset = 1     // Top border at Y=1
	const tabBarHeight = 2     // Tab bar (1) + separator line (1) at Y=2-3

	// Adjust all BufPanes in the tab to use middle region
	for _, pane := range tab.Panes {
		if bp, ok := pane.(*action.BufPane); ok {
			// Get the underlying window
			view := bp.GetView()
			if view != nil {
				// Constrain to middle region (always with border space)
				view.X = editorX + borderOffset
				view.Width = editorWidth - (borderOffset * 2)
				view.Y = paneNavBarHeight + borderOffset + tabBarHeight // Leave room for pane nav bar, border, tab bar
				view.Height = lm.ScreenH - view.Y - borderOffset        // Subtract top offset and bottom border
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

// clearEditorIfPathDeleted closes any tabs that have the deleted path open
func (lm *LayoutManager) clearEditorIfPathDeleted(deletedPath string, isDir bool) {
	if lm.TabBar == nil {
		return
	}

	// Find and close any tabs with the deleted file (iterate backwards to handle index shifts)
	for i := len(lm.TabBar.Tabs) - 1; i >= 0; i-- {
		tabPath := lm.TabBar.Tabs[i].Path
		if tabPath == "" {
			continue
		}

		shouldClose := false
		if isDir {
			// Check if tab path is inside the deleted directory
			if strings.HasPrefix(tabPath, deletedPath+string(filepath.Separator)) {
				shouldClose = true
			}
		} else {
			// Check if tab path matches the deleted file
			if tabPath == deletedPath {
				shouldClose = true
			}
		}

		if shouldClose {
			log.Printf("THICC: Closing tab - deleted file was open: %s", tabPath)
			lm.doCloseTab(i)
		}
	}
}

// CloseCurrentTab closes the current tab
func (lm *LayoutManager) CloseCurrentTab() {
	lm.CloseTabAtIndex(lm.TabBar.ActiveIndex)
}

// CloseTabAtIndex closes the tab at the given index
func (lm *LayoutManager) CloseTabAtIndex(index int) {
	if lm.TabBar == nil || index < 0 || index >= len(lm.TabBar.Tabs) {
		return
	}

	openTab := lm.TabBar.Tabs[index]

	// Don't close if it's the only tab AND it's Untitled
	if len(lm.TabBar.Tabs) == 1 && openTab.Name == "Untitled" {
		log.Println("THICC: Can't close the only Untitled tab")
		return
	}

	// Check for unsaved changes
	if openTab.Buffer != nil && openTab.Buffer.Modified() {
		bufName := openTab.Name
		lm.ShowModal("Unsaved Changes",
			"Discard changes to "+bufName+"?",
			func(yes, canceled bool) {
				if canceled || !yes {
					return
				}
				lm.doCloseTab(index)
			})
		return
	}

	lm.doCloseTab(index)
}

// doCloseTab performs the actual tab close
func (lm *LayoutManager) doCloseTab(index int) {
	if lm.TabBar == nil || index < 0 || index >= len(lm.TabBar.Tabs) {
		return
	}

	log.Printf("THICC: Closing tab %d", index)

	// Close the buffer
	openTab := lm.TabBar.Tabs[index]
	if openTab.Buffer != nil {
		openTab.Buffer.Close()
	}

	// Remove from tab list
	lm.TabBar.CloseTab(index)

	// Switch to appropriate tab or reset to empty
	if len(lm.TabBar.Tabs) == 0 {
		// No tabs left - create empty buffer
		log.Println("THICC: No tabs left, creating empty buffer")
		lm.resetToEmptyBuffer()
	} else {
		// Switch to the new active tab
		lm.switchToTab(lm.TabBar.ActiveIndex)
	}

	lm.triggerRedraw()
}

// resetToEmptyBuffer creates a new empty buffer when all tabs are closed
func (lm *LayoutManager) resetToEmptyBuffer() {
	newBuf := buffer.NewBufferFromString("", "", buffer.BTDefault)

	// Add to tab bar
	if lm.TabBar != nil {
		lm.TabBar.AddTab(newBuf)
	}

	tab := action.MainTab()
	if tab == nil {
		return
	}

	for _, pane := range tab.Panes {
		if bp, ok := pane.(*action.BufPane); ok {
			bp.OpenBuffer(newBuf) // Use OpenBuffer to close any remaining buffer
			break
		}
	}

	// Keep tree selection at 0
	if lm.FileBrowser != nil {
		lm.FileBrowser.Selected = 0
		lm.FileBrowser.TopLine = 0
	}
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
				log.Println("THICC: New file, showing input modal for filename")
				lm.ShowInputModal("Save File", "Enter filename:", "", func(filename string, canceled bool) {
					if canceled || filename == "" {
						log.Println("THICC: Save canceled or empty filename")
						return
					}
					log.Printf("THICC: Saving new file as: %s", filename)

					// Save the file
					err := bp.Buf.SaveAs(filename)
					if err != nil {
						action.InfoBar.Error(err)
						return
					}
					lm.ShowTimedMessage("Saved "+filename, 3*time.Second)

					// Update tab name and path
					if lm.TabBar != nil {
						lm.TabBar.UpdateTabName(lm.TabBar.ActiveIndex, bp.Buf)
					}

					// Notify tree to refresh and select the file
					if lm.FileBrowser != nil && lm.FileBrowser.OnFileSaved != nil {
						lm.FileBrowser.OnFileSaved(filename)
					}

					// Check if this is the settings file and reload
					lm.handleSettingsFileSaved(filename)
				})
				return true
			} else {
				// Existing file - check if it's settings file and validate first
				if thicc.IsSettingsFile(bp.Buf.Path) {
					// Validate settings before saving
					content := bp.Buf.Bytes()
					_, errors := thicc.ValidateSettingsJSON(content)
					if len(errors) > 0 {
						var errMsgs []string
						for _, err := range errors {
							errMsgs = append(errMsgs, err.Error())
						}
						action.InfoBar.Error("Cannot save - invalid settings: " + strings.Join(errMsgs, "; "))
						return false
					}
				}

				// Save the file
				log.Printf("THICC: Saving existing file: %s", bp.Buf.Path)
				bp.Save()

				// Update tab name (in case it changed)
				if lm.TabBar != nil {
					lm.TabBar.UpdateTabName(lm.TabBar.ActiveIndex, bp.Buf)
				}

				// Notify tree (in case it needs to update)
				if lm.FileBrowser != nil && lm.FileBrowser.OnFileSaved != nil {
					lm.FileBrowser.OnFileSaved(bp.Buf.Path)
				}

				// Check if this is the settings file and reload
				lm.handleSettingsFileSaved(bp.Buf.Path)
				return true
			}
		}
	}
	return false
}

// handleSettingsFileSaved handles validation and reload when settings file is saved
func (lm *LayoutManager) handleSettingsFileSaved(path string) {
	if !thicc.IsSettingsFile(path) {
		return
	}

	log.Println("THICC: Settings file saved, validating and reloading...")

	errors := thicc.ReloadSettings()
	if len(errors) > 0 {
		// Show validation errors
		var errMsgs []string
		for _, err := range errors {
			errMsgs = append(errMsgs, err.Error())
		}
		action.InfoBar.Error("Settings validation failed: " + strings.Join(errMsgs, "; "))
		return
	}

	// Apply settings that can be hot-reloaded
	config.ReloadThiccBackground()
	config.InitDoubleClickThreshold()

	lm.ShowTimedMessage("Settings reloaded", 2*time.Second)
	lm.triggerRedraw()
}

// ShowProjectPicker shows the project picker with current directory pre-filled
func (lm *LayoutManager) ShowProjectPicker() {
	if lm.ProjectPicker != nil && lm.FileBrowser != nil {
		currentDir := lm.FileBrowser.Tree.CurrentDir
		// Set input path and let Show() handle tilde collapsing and directory loading
		lm.ProjectPicker.InputPath = currentDir
		lm.ProjectPicker.CursorPos = len(currentDir)
		lm.ProjectPicker.Show()
		lm.triggerRedraw()
	}
}

// ShowQuickFind shows the quick find file picker
func (lm *LayoutManager) ShowQuickFind() {
	if lm.QuickFindPicker != nil {
		lm.QuickFindPicker.Show()
		lm.triggerRedraw()
	}
}

// OpenSettings opens the THICC settings file in the editor
func (lm *LayoutManager) OpenSettings() {
	// Ensure settings file exists with defaults
	if err := thicc.EnsureSettingsFile(); err != nil {
		action.InfoBar.Error("Failed to create settings file: " + err.Error())
		return
	}

	settingsPath := thicc.GetSettingsFilePath()

	// Make editor visible if hidden
	if !lm.EditorVisible {
		lm.EditorVisible = true
		lm.updateLayout()
	}

	// Open the settings file in the editor
	lm.previewFileInEditor(settingsPath)

	// Pin the tab since this was intentionally opened
	if lm.TabBar != nil {
		lm.TabBar.PinTab(lm.TabBar.ActiveIndex)
	}

	// Focus the editor
	lm.FocusEditor()
	lm.triggerRedraw()
}

// navigateToProject changes the root directory and reinitializes the file browser
func (lm *LayoutManager) navigateToProject(newRoot string) {
	log.Printf("THICC: Navigating to new project: %s", newRoot)

	// Hide the picker
	if lm.ProjectPicker != nil {
		lm.ProjectPicker.Hide()
	}

	// Update root
	lm.Root = newRoot

	// Recreate file browser with new root (Y=1 for pane nav bar at Y=0)
	treeRegion := Region{
		X:      0,
		Y:      1,
		Width:  lm.getTreeWidth(),
		Height: lm.ScreenH - 1,
	}

	lm.FileBrowser = filebrowser.NewPanel(
		treeRegion.X, treeRegion.Y,
		treeRegion.Width, treeRegion.Height,
		newRoot,
	)

	// Re-register all callbacks (pattern from Initialize method)
	lm.FileBrowser.OnFileOpen = lm.previewFileInEditor
	lm.FileBrowser.OnFileActualOpen = func(path string) {
		if !lm.EditorVisible {
			lm.EditorVisible = true
			lm.updateLayout()
			lm.triggerRedraw()
		}
		lm.previewFileInEditor(path)
	}
	lm.FileBrowser.OnTreeReady = lm.triggerRedraw
	lm.FileBrowser.OnFocusEditor = lm.FocusEditor
	lm.FileBrowser.OnProjectPathClick = lm.ShowProjectPicker

	lm.FileBrowser.OnFileSaved = func(path string) {
		log.Printf("THICC: File saved callback - refreshing tree and selecting: %s", path)
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
					log.Printf("THICC: Deleting %s: %s", itemType, path)
					err := os.RemoveAll(path)
					if err != nil {
						action.InfoBar.Error("Delete failed: " + err.Error())
					} else {
						lm.ShowTimedMessage("Deleted "+name, 3*time.Second)
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
			log.Printf("THICC: Renaming %s -> %s", oldPath, newPath)

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
			log.Printf("THICC: Creating new file: %s", newPath)

			file, err := os.Create(newPath)
			if err != nil {
				action.InfoBar.Error("Create file failed: " + err.Error())
				return
			}
			file.Close()

			lm.ShowTimedMessage("Created "+fileName, 3*time.Second)

			lm.FileBrowser.Tree.ExpandedPaths[dirPath] = true
			lm.FileBrowser.Tree.Refresh()
			lm.FileBrowser.Tree.SelectPath(newPath)
			lm.triggerRedraw()

			// Open the new file in editor and pin it (new files should be permanent)
			lm.previewFileInEditor(newPath)
			if lm.TabBar != nil {
				lm.TabBar.PinTab(lm.TabBar.ActiveIndex)
			}

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
			log.Printf("THICC: Creating new folder: %s", newPath)

			err := os.MkdirAll(newPath, 0755)
			if err != nil {
				action.InfoBar.Error("Create folder failed: " + err.Error())
				return
			}

			lm.ShowTimedMessage("Created "+folderName+"/", 3*time.Second)

			lm.FileBrowser.Tree.ExpandedPaths[dirPath] = true
			lm.FileBrowser.Tree.Refresh()
			lm.FileBrowser.Tree.SelectPath(newPath)
			lm.triggerRedraw()

			callback(folderName)
		})
	}

	// Close all tabs and reset to empty buffer
	if lm.TabBar != nil {
		for i := len(lm.TabBar.Tabs) - 1; i >= 0; i-- {
			if lm.TabBar.Tabs[i].Buffer != nil {
				lm.TabBar.Tabs[i].Buffer.Close()
			}
		}
		lm.TabBar.Tabs = nil
		lm.TabBar.ActiveIndex = 0
	}

	lm.resetToEmptyBuffer()
	lm.triggerRedraw()
}
