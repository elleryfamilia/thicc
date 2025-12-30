package dashboard

import (
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
	RecentStore    *RecentStore
	RecentIdx      int  // Selected index in recent list (-1 if not in recent pane)
	InRecentPane   bool // True if focus is on recent projects list
	RecentScrollOffset int // Scroll offset for recent projects

	// Layout regions (calculated during render)
	menuRegion   Region
	recentRegion Region

	// Project Picker modal
	ProjectPicker *ProjectPicker

	// Callbacks - set by the caller to handle actions
	OnNewFile    func()             // Create new empty file
	OnOpenProject func(path string) // Open a project folder
	OnOpenFile   func(path string)  // Open specific file
	OnOpenFolder func(path string)  // Open specific folder
	OnExit       func()             // Exit application
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
	}

	// Load recent projects from disk
	d.RecentStore.Load()

	return d
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
	if d.InRecentPane {
		if proj := d.GetSelectedRecentProject(); proj != nil {
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

// MoveNext moves selection to the next item
func (d *Dashboard) MoveNext() {
	if d.InRecentPane {
		if len(d.RecentStore.Projects) > 0 {
			d.RecentIdx++
			if d.RecentIdx >= len(d.RecentStore.Projects) {
				d.RecentIdx = 0
			}
		}
	} else {
		d.SelectedIdx++
		if d.SelectedIdx >= len(d.MenuItems) {
			d.SelectedIdx = 0
		}
	}
}

// MovePrevious moves selection to the previous item
func (d *Dashboard) MovePrevious() {
	if d.InRecentPane {
		if len(d.RecentStore.Projects) > 0 {
			d.RecentIdx--
			if d.RecentIdx < 0 {
				d.RecentIdx = len(d.RecentStore.Projects) - 1
			}
		}
	} else {
		d.SelectedIdx--
		if d.SelectedIdx < 0 {
			d.SelectedIdx = len(d.MenuItems) - 1
		}
	}
}

// SwitchToRecentPane switches focus to the recent projects pane
func (d *Dashboard) SwitchToRecentPane() {
	if len(d.RecentStore.Projects) > 0 {
		d.InRecentPane = true
		if d.RecentIdx < 0 {
			d.RecentIdx = 0
		}
	}
}

// SwitchToMenuPane switches focus to the menu pane
func (d *Dashboard) SwitchToMenuPane() {
	d.InRecentPane = false
	d.RecentIdx = -1
}

// OpenRecentByNumber opens a recent project by its displayed number (1-9)
func (d *Dashboard) OpenRecentByNumber(num int) {
	idx := num - 1 // Convert 1-based to 0-based
	if idx >= 0 && idx < len(d.RecentStore.Projects) {
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
