package dashboard

import (
	"github.com/micro-editor/tcell/v2"
)

// HandleEvent processes all events for the dashboard
// Returns true if the event was consumed
func (d *Dashboard) HandleEvent(event tcell.Event) bool {
	// If project picker is active, route events to it
	if d.IsProjectPickerActive() {
		return d.ProjectPicker.HandleEvent(event)
	}

	switch ev := event.(type) {
	case *tcell.EventKey:
		return d.handleKey(ev)
	case *tcell.EventMouse:
		return d.handleMouse(ev)
	case *tcell.EventResize:
		d.ScreenW, d.ScreenH = ev.Size()
		return true
	}
	return false
}

// handleKey processes keyboard events
func (d *Dashboard) handleKey(ev *tcell.EventKey) bool {
	// Quick shortcuts (work anywhere)
	switch ev.Key() {
	case tcell.KeyCtrlQ:
		if d.OnExit != nil {
			d.OnExit()
		}
		return true

	case tcell.KeyEsc:
		// If in recent pane, go back to menu
		if d.InRecentPane {
			d.SwitchToMenuPane()
			return true
		}
		// Otherwise exit
		if d.OnExit != nil {
			d.OnExit()
		}
		return true

	case tcell.KeyCtrlC:
		if d.OnExit != nil {
			d.OnExit()
		}
		return true
	}

	// Character shortcuts
	switch ev.Rune() {
	case 'q', 'Q':
		if d.OnExit != nil {
			d.OnExit()
		}
		return true

	case 'n':
		if d.OnNewFile != nil {
			d.OnNewFile()
		}
		return true

	case 'o', 'O':
		d.ShowProjectPicker()
		return true

	// Number shortcuts for recent projects
	case '1', '2', '3', '4', '5', '6', '7', '8', '9':
		num := int(ev.Rune() - '0')
		d.OpenRecentByNumber(num)
		return true
	}

	// Navigation keys
	switch ev.Key() {
	case tcell.KeyUp:
		d.MovePrevious()
		return true

	case tcell.KeyDown:
		d.MoveNext()
		return true

	case tcell.KeyLeft:
		if d.InRecentPane {
			d.SwitchToMenuPane()
		}
		return true

	case tcell.KeyRight:
		if !d.InRecentPane && len(d.RecentStore.Projects) > 0 {
			d.SwitchToRecentPane()
		}
		return true

	case tcell.KeyTab:
		// Tab cycles: menu -> recent -> menu
		if d.InRecentPane {
			d.SwitchToMenuPane()
		} else if len(d.RecentStore.Projects) > 0 {
			d.SwitchToRecentPane()
		}
		return true

	case tcell.KeyBacktab:
		// Shift+Tab cycles in reverse
		if d.InRecentPane {
			d.SwitchToMenuPane()
		} else if len(d.RecentStore.Projects) > 0 {
			d.SwitchToRecentPane()
		}
		return true

	case tcell.KeyEnter:
		d.ActivateSelection()
		return true

	case tcell.KeyDelete, tcell.KeyBackspace, tcell.KeyBackspace2:
		// Delete removes selected recent project
		if d.InRecentPane {
			d.RemoveSelectedRecent()
		}
		return true

	case tcell.KeyHome:
		// Go to first item
		if d.InRecentPane {
			d.RecentIdx = 0
		} else {
			d.SelectedIdx = 0
		}
		return true

	case tcell.KeyEnd:
		// Go to last item
		if d.InRecentPane {
			if len(d.RecentStore.Projects) > 0 {
				d.RecentIdx = len(d.RecentStore.Projects) - 1
			}
		} else {
			d.SelectedIdx = len(d.MenuItems) - 1
		}
		return true
	}

	// Vim-style navigation
	switch ev.Rune() {
	case 'j':
		d.MoveNext()
		return true
	case 'k':
		d.MovePrevious()
		return true
	case 'h':
		if d.InRecentPane {
			d.SwitchToMenuPane()
		}
		return true
	case 'l':
		if !d.InRecentPane && len(d.RecentStore.Projects) > 0 {
			d.SwitchToRecentPane()
		}
		return true
	case 'g':
		// gg - go to first item (simplified: just g)
		if d.InRecentPane {
			d.RecentIdx = 0
		} else {
			d.SelectedIdx = 0
		}
		return true
	case 'G':
		// G - go to last item
		if d.InRecentPane {
			if len(d.RecentStore.Projects) > 0 {
				d.RecentIdx = len(d.RecentStore.Projects) - 1
			}
		} else {
			d.SelectedIdx = len(d.MenuItems) - 1
		}
		return true
	}

	return true // Dashboard consumes all events
}

// handleMouse processes mouse events
func (d *Dashboard) handleMouse(ev *tcell.EventMouse) bool {
	x, y := ev.Position()
	buttons := ev.Buttons()

	switch buttons {
	case tcell.Button1:
		// Left click - select and activate
		return d.handleLeftClick(x, y)

	case tcell.WheelUp:
		d.MovePrevious()
		return true

	case tcell.WheelDown:
		d.MoveNext()
		return true
	}

	return true
}

// handleLeftClick processes left mouse button clicks
func (d *Dashboard) handleLeftClick(x, y int) bool {
	// Check if click is on a menu item
	if itemIdx := d.GetMenuItemAtPosition(x, y); itemIdx >= 0 {
		d.SelectedIdx = itemIdx
		d.InRecentPane = false
		d.ActivateSelection()
		return true
	}

	// Check if click is on a recent project
	if recentIdx := d.GetRecentItemAtPosition(x, y); recentIdx >= 0 {
		d.RecentIdx = recentIdx
		d.InRecentPane = true
		d.ActivateSelection()
		return true
	}

	return true
}
