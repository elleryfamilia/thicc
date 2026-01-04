package dashboard

import (
	"log"

	"github.com/ellery/thicc/internal/aiterminal"
	"github.com/micro-editor/tcell/v2"
)

// MenuItemID identifies menu items for action handling
type MenuItemID int

const (
	MenuNewFile MenuItemID = iota
	MenuOpenProject
	MenuExit
)

// MenuItem represents a selectable menu option
type MenuItem struct {
	ID       MenuItemID
	Icon     string // Unicode icon
	Label    string // Display text
	Shortcut string // Keyboard shortcut hint (e.g., "n")
}

// Dashboard is the welcome screen shown when thock starts without arguments
type Dashboard struct {
	// Screen reference
	Screen  tcell.Screen
	ScreenW int
	ScreenH int

	// Menu state
	MenuItems   []MenuItem
	SelectedIdx int

	// Recent projects
	RecentStore        *RecentStore
	RecentIdx          int  // Selected index in recent list (-1 if not in recent pane)
	InRecentPane       bool // True if focus is on recent projects list
	RecentScrollOffset int  // Scroll offset for recent projects

	// AI Tools section
	PrefsStore         *PreferencesStore      // Persistent preferences
	AITools            []aiterminal.AITool    // Available AI tools (cached)
	InstallTools       []aiterminal.AITool    // Installable but not installed tools
	AIToolsIdx         int                    // Selected index in AI tools list (includes both available and installable)
	InAIToolsPane      bool                   // True if focus is on AI tools section
	SelectedInstallCmd string                 // Install command if an installable tool is selected (non-persisted)

	// Layout regions (calculated during render)
	menuRegion    Region
	aiToolsRegion Region
	recentRegion  Region

	// Project Picker modal
	ProjectPicker *ProjectPicker

	// Onboarding Guide modal
	OnboardingGuide *OnboardingGuide

	// Callbacks - set by the caller to handle actions
	OnNewFile     func()             // Create new empty file
	OnOpenProject func(path string)  // Open a project folder
	OnOpenFile    func(path string)  // Open specific file
	OnOpenFolder  func(path string)  // Open specific folder
	OnInstallTool func(cmd string)   // Install a tool (opens shell with command)
	OnExit        func()             // Exit application
}

// Region defines a rectangular screen area
type Region struct {
	X      int
	Y      int
	Width  int
	Height int
}

// Contains checks if a point is within this region
func (r Region) Contains(x, y int) bool {
	return x >= r.X && x < r.X+r.Width &&
		y >= r.Y && y < r.Y+r.Height
}

// NewDashboard creates a new dashboard instance
func NewDashboard(screen tcell.Screen) *Dashboard {
	w, h := screen.Size()

	d := &Dashboard{
		Screen:  screen,
		ScreenW: w,
		ScreenH: h,

		MenuItems: []MenuItem{
			{ID: MenuNewFile, Icon: "+", Label: "New File", Shortcut: "n"},
			{ID: MenuOpenProject, Icon: ">", Label: "Open Project", Shortcut: "o"},
			{ID: MenuExit, Icon: "x", Label: "Exit", Shortcut: "q"},
		},
		SelectedIdx: 0,

		RecentStore:  NewRecentStore(),
		RecentIdx:    -1,
		InRecentPane: false,

		PrefsStore:    NewPreferencesStore(),
		AITools:       aiterminal.GetAvailableToolsOnly(),
		InstallTools:  aiterminal.GetInstallableTools(),
		AIToolsIdx:    0,
		InAIToolsPane: false,
	}

	// Load recent projects from disk
	d.RecentStore.Load()

	// Load preferences from disk
	d.PrefsStore.Load()

	// Initialize AIToolsIdx based on saved preference
	d.initAIToolsIdx()

	// Initialize onboarding guide
	d.OnboardingGuide = NewOnboardingGuide()

	return d
}

// initAIToolsIdx sets AIToolsIdx based on the saved preference
func (d *Dashboard) initAIToolsIdx() {
	savedTool := d.PrefsStore.GetSelectedAITool()
	if savedTool == "" {
		// No tool selected, default to first (which should be Shell)
		d.AIToolsIdx = 0
		return
	}

	// Find the index of the saved tool (match by Name since it's unique)
	for i, tool := range d.AITools {
		if tool.Name == savedTool {
			d.AIToolsIdx = i
			return
		}
	}

	// Saved tool not found (maybe uninstalled), reset to first
	d.AIToolsIdx = 0
}

// totalAIToolItems returns total count of available + installable tools
func (d *Dashboard) totalAIToolItems() int {
	return len(d.AITools) + len(d.InstallTools)
}

// isInstallToolIdx returns true if the index points to an installable tool
func (d *Dashboard) isInstallToolIdx(idx int) bool {
	return idx >= len(d.AITools)
}

// getInstallToolAt returns the install tool at the given index (adjusted for AITools offset)
func (d *Dashboard) getInstallToolAt(idx int) *aiterminal.AITool {
	installIdx := idx - len(d.AITools)
	if installIdx >= 0 && installIdx < len(d.InstallTools) {
		return &d.InstallTools[installIdx]
	}
	return nil
}

// Resize updates the dashboard dimensions
func (d *Dashboard) Resize(w, h int) {
	d.ScreenW = w
	d.ScreenH = h
}

// GetSelectedMenuItem returns the currently selected menu item
func (d *Dashboard) GetSelectedMenuItem() *MenuItem {
	if d.SelectedIdx >= 0 && d.SelectedIdx < len(d.MenuItems) {
		return &d.MenuItems[d.SelectedIdx]
	}
	return nil
}

// GetSelectedRecentProject returns the currently selected recent project
func (d *Dashboard) GetSelectedRecentProject() *RecentProject {
	if d.InRecentPane && d.RecentIdx >= 0 && d.RecentIdx < len(d.RecentStore.Projects) {
		return &d.RecentStore.Projects[d.RecentIdx]
	}
	return nil
}

// ActivateSelection activates the current selection (menu item or recent project)
func (d *Dashboard) ActivateSelection() {
	// Helper to trigger install if an installable tool is selected
	triggerInstallIfNeeded := func() {
		if d.SelectedInstallCmd != "" && d.OnInstallTool != nil {
			log.Printf("THICC Dashboard: Triggering install command: %s", d.SelectedInstallCmd)
			d.OnInstallTool(d.SelectedInstallCmd)
		}
	}

	if d.InRecentPane {
		if proj := d.GetSelectedRecentProject(); proj != nil {
			triggerInstallIfNeeded()
			if proj.IsFolder {
				if d.OnOpenFolder != nil {
					d.OnOpenFolder(proj.Path)
				}
			} else {
				if d.OnOpenFile != nil {
					d.OnOpenFile(proj.Path)
				}
			}
		}
		return
	}

	item := d.GetSelectedMenuItem()
	if item == nil {
		return
	}

	switch item.ID {
	case MenuNewFile:
		triggerInstallIfNeeded()
		if d.OnNewFile != nil {
			d.OnNewFile()
		}
	case MenuOpenProject:
		d.ShowProjectPicker()
	case MenuExit:
		if d.OnExit != nil {
			d.OnExit()
		}
	}
}

// MoveNext moves selection to the next item (seamlessly across all sections)
func (d *Dashboard) MoveNext() {
	if d.InRecentPane {
		// In recent pane - move down or wrap to menu
		if len(d.RecentStore.Projects) > 0 {
			d.RecentIdx++
			if d.RecentIdx >= len(d.RecentStore.Projects) {
				// Wrap to menu
				d.SwitchToMenuPane()
				d.SelectedIdx = 0
			}
		}
	} else if d.InAIToolsPane {
		// In AI tools pane - move down or go to recent/menu
		totalTools := d.totalAIToolItems()
		if totalTools > 0 {
			d.AIToolsIdx++
			if d.AIToolsIdx >= totalTools {
				// Move to recent projects if available, else wrap to menu
				if len(d.RecentStore.Projects) > 0 {
					d.SwitchToRecentPane()
					d.RecentIdx = 0
				} else {
					d.SwitchToMenuPane()
					d.SelectedIdx = 0
				}
			}
		}
	} else {
		// In menu pane - move down or go to AI tools/recent
		d.SelectedIdx++
		if d.SelectedIdx >= len(d.MenuItems) {
			// Move to AI tools if available
			if d.totalAIToolItems() > 0 {
				d.SwitchToAIToolsPane()
				d.AIToolsIdx = 0
			} else if len(d.RecentStore.Projects) > 0 {
				d.SwitchToRecentPane()
				d.RecentIdx = 0
			} else {
				d.SelectedIdx = 0 // Wrap within menu
			}
		}
	}
}

// MovePrevious moves selection to the previous item (seamlessly across all sections)
func (d *Dashboard) MovePrevious() {
	totalTools := d.totalAIToolItems()

	if d.InRecentPane {
		// In recent pane - move up or go to AI tools/menu
		if len(d.RecentStore.Projects) > 0 {
			d.RecentIdx--
			if d.RecentIdx < 0 {
				// Move to AI tools if available, else menu
				if totalTools > 0 {
					d.SwitchToAIToolsPane()
					d.AIToolsIdx = totalTools - 1
				} else {
					d.SwitchToMenuPane()
					d.SelectedIdx = len(d.MenuItems) - 1
				}
			}
		}
	} else if d.InAIToolsPane {
		// In AI tools pane - move up or go to menu
		if totalTools > 0 {
			d.AIToolsIdx--
			if d.AIToolsIdx < 0 {
				// Move to menu
				d.SwitchToMenuPane()
				d.SelectedIdx = len(d.MenuItems) - 1
			}
		}
	} else {
		// In menu pane - move up or wrap to recent/AI tools
		d.SelectedIdx--
		if d.SelectedIdx < 0 {
			// Wrap to bottom of list (recent > AI tools > menu)
			if len(d.RecentStore.Projects) > 0 {
				d.SwitchToRecentPane()
				d.RecentIdx = len(d.RecentStore.Projects) - 1
			} else if totalTools > 0 {
				d.SwitchToAIToolsPane()
				d.AIToolsIdx = totalTools - 1
			} else {
				d.SelectedIdx = len(d.MenuItems) - 1 // Wrap within menu
			}
		}
	}
}

// SwitchToRecentPane switches focus to the recent projects pane
func (d *Dashboard) SwitchToRecentPane() {
	if len(d.RecentStore.Projects) > 0 {
		d.InRecentPane = true
		d.InAIToolsPane = false
		if d.RecentIdx < 0 {
			d.RecentIdx = 0
		}
	}
}

// SwitchToMenuPane switches focus to the menu pane
func (d *Dashboard) SwitchToMenuPane() {
	d.InRecentPane = false
	d.InAIToolsPane = false
	d.RecentIdx = -1
}

// SwitchToAIToolsPane switches focus to the AI tools pane
func (d *Dashboard) SwitchToAIToolsPane() {
	totalTools := d.totalAIToolItems()
	if totalTools > 0 {
		d.InAIToolsPane = true
		d.InRecentPane = false
		d.RecentIdx = -1
		// Ensure AIToolsIdx is valid
		if d.AIToolsIdx < 0 || d.AIToolsIdx >= totalTools {
			d.AIToolsIdx = 0
		}
	}
}

// ToggleAIToolSelection toggles the selection of the current AI tool
// This implements radio-button behavior - selecting one deselects others
// For installable tools, it selects them (install happens when user opens a file/project)
func (d *Dashboard) ToggleAIToolSelection() {
	log.Printf("THICC Dashboard: ToggleAIToolSelection called, AIToolsIdx=%d, InAIToolsPane=%v, totalItems=%d, availableCount=%d",
		d.AIToolsIdx, d.InAIToolsPane, d.totalAIToolItems(), len(d.AITools))

	if !d.InAIToolsPane || d.totalAIToolItems() == 0 {
		log.Println("THICC Dashboard: ToggleAIToolSelection early return - not in pane or no tools")
		return
	}

	// Check if this is an installable tool
	if d.isInstallToolIdx(d.AIToolsIdx) {
		tool := d.getInstallToolAt(d.AIToolsIdx)
		log.Printf("THICC Dashboard: Install tool detected at idx %d, tool=%v", d.AIToolsIdx, tool)
		if tool != nil {
			if d.SelectedInstallCmd == tool.InstallCommand {
				// Deselect
				log.Printf("THICC Dashboard: Deselecting install tool")
				d.SelectedInstallCmd = ""
			} else {
				// Select this installable tool (clears regular tool selection)
				log.Printf("THICC Dashboard: Selecting install tool: %s", tool.InstallCommand)
				d.SelectedInstallCmd = tool.InstallCommand
				d.PrefsStore.ClearSelectedAITool() // Clear regular selection
			}
		}
		return
	}

	// Regular available tool - clear any install selection
	d.SelectedInstallCmd = ""
	selectedTool := &d.AITools[d.AIToolsIdx]

	// Check if this tool is already selected (use Name since it's unique)
	currentSelected := d.PrefsStore.GetSelectedAITool()
	if currentSelected == selectedTool.Name {
		// Deselect (go back to shell/none)
		d.PrefsStore.ClearSelectedAITool()
	} else {
		// Select this tool (store by Name, not Command)
		d.PrefsStore.SetSelectedAITool(selectedTool.Name)
	}
}

// GetSelectedAITool returns the currently selected AI tool, or nil if none selected
func (d *Dashboard) GetSelectedAITool() *aiterminal.AITool {
	selectedName := d.PrefsStore.GetSelectedAITool()
	if selectedName == "" {
		return nil
	}

	// Match by Name since it's unique (Command is not unique, e.g., Claude vs Claude YOLO)
	for i := range d.AITools {
		if d.AITools[i].Name == selectedName {
			return &d.AITools[i]
		}
	}
	return nil
}

// GetSelectedAIToolCommand returns the command line for the selected AI tool
// Returns nil if no tool is selected OR if the default shell is selected
// (we return nil for shell so that the terminal uses its built-in shell handling
// which includes injecting the pretty prompt)
func (d *Dashboard) GetSelectedAIToolCommand() []string {
	tool := d.GetSelectedAITool()
	if tool == nil {
		return nil
	}
	// If "Shell (default)" is selected, return nil to use built-in shell handling
	// which includes the pretty prompt injection
	if tool.Name == "Shell (default)" {
		return nil
	}
	return tool.GetCommandLine()
}

// IsAIToolSelected returns true if the given tool name is currently selected
func (d *Dashboard) IsAIToolSelected(name string) bool {
	return d.PrefsStore.GetSelectedAITool() == name
}

// IsInstallToolSelected returns true if an installable tool with the given install command is selected
func (d *Dashboard) IsInstallToolSelected(installCmd string) bool {
	return d.SelectedInstallCmd == installCmd
}

// HasInstallToolSelected returns true if any installable tool is selected
func (d *Dashboard) HasInstallToolSelected() bool {
	return d.SelectedInstallCmd != ""
}

// OpenRecentByNumber opens a recent project by its displayed number (1-9)
func (d *Dashboard) OpenRecentByNumber(num int) {
	idx := num - 1 // Convert 1-based to 0-based
	if idx >= 0 && idx < len(d.RecentStore.Projects) {
		// Trigger install if an installable tool is selected
		if d.SelectedInstallCmd != "" && d.OnInstallTool != nil {
			log.Printf("THICC Dashboard: Triggering install command: %s", d.SelectedInstallCmd)
			d.OnInstallTool(d.SelectedInstallCmd)
		}

		proj := &d.RecentStore.Projects[idx]
		if proj.IsFolder {
			if d.OnOpenFolder != nil {
				d.OnOpenFolder(proj.Path)
			}
		} else {
			if d.OnOpenFile != nil {
				d.OnOpenFile(proj.Path)
			}
		}
	}
}

// RemoveSelectedRecent removes the currently selected recent project from the list
func (d *Dashboard) RemoveSelectedRecent() {
	if d.InRecentPane && d.RecentIdx >= 0 && d.RecentIdx < len(d.RecentStore.Projects) {
		proj := d.RecentStore.Projects[d.RecentIdx]
		d.RecentStore.RemoveProject(proj.Path)

		// Adjust selection
		if d.RecentIdx >= len(d.RecentStore.Projects) {
			d.RecentIdx = len(d.RecentStore.Projects) - 1
		}
		if len(d.RecentStore.Projects) == 0 {
			d.SwitchToMenuPane()
		}
	}
}

// ShowProjectPicker displays the project picker modal
func (d *Dashboard) ShowProjectPicker() {
	if d.ProjectPicker == nil {
		d.ProjectPicker = NewProjectPicker(d.Screen, func(path string) {
			// Project selected - call the callback
			d.ProjectPicker.Hide()

			// Trigger install if an installable tool is selected
			if d.SelectedInstallCmd != "" && d.OnInstallTool != nil {
				log.Printf("THICC Dashboard: Triggering install command: %s", d.SelectedInstallCmd)
				d.OnInstallTool(d.SelectedInstallCmd)
			}

			if d.OnOpenProject != nil {
				d.OnOpenProject(path)
			}
		}, func() {
			// Cancelled - hide picker
			d.ProjectPicker.Hide()
		})
	}
	d.ProjectPicker.Show()
}

// IsProjectPickerActive returns true if the project picker is currently shown
func (d *Dashboard) IsProjectPickerActive() bool {
	return d.ProjectPicker != nil && d.ProjectPicker.Active
}

// ShowOnboardingGuide displays the onboarding guide
func (d *Dashboard) ShowOnboardingGuide() {
	if d.OnboardingGuide != nil {
		d.OnboardingGuide.Show(d.ScreenW, d.ScreenH)
	}
}

// IsOnboardingGuideActive returns true if the onboarding guide is currently shown
func (d *Dashboard) IsOnboardingGuideActive() bool {
	return d.OnboardingGuide != nil && d.OnboardingGuide.Active
}
