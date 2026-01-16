package filebrowser

import (
	"log"

	"github.com/micro-editor/tcell/v2"
)

// HandleEvent processes keyboard and mouse events
func (p *Panel) HandleEvent(event tcell.Event) bool {
	switch ev := event.(type) {
	case *tcell.EventKey:
		// Keyboard only works when focused
		if !p.Focus {
			return false
		}
		return p.handleKey(ev)
	case *tcell.EventMouse:
		// Mouse always works (clicking focuses the panel)
		return p.handleMouse(ev)
	}

	return false
}

// handleKey processes keyboard events
func (p *Panel) handleKey(ev *tcell.EventKey) bool {
	// Note: Ctrl+N, Ctrl+D, Ctrl+R are handled globally in LayoutManager
	// so they work from any panel

	nodes := p.Tree.GetNodes()
	if len(nodes) == 0 {
		return false
	}

	switch ev.Key() {
	case tcell.KeyUp:
		return p.cursorUp()

	case tcell.KeyDown:
		return p.cursorDown()

	case tcell.KeyLeft:
		return p.collapseSelected()

	case tcell.KeyRight:
		return p.expandSelected()

	case tcell.KeyEnter:
		return p.openSelected()

	case tcell.KeyPgUp:
		return p.pageUp()

	case tcell.KeyPgDn:
		return p.pageDown()

	case tcell.KeyHome:
		return p.goToTop()

	case tcell.KeyEnd:
		return p.goToBottom()

	default:
		// Handle character keys
		switch ev.Rune() {
		case 'k':
			return p.cursorUp()
		case 'j':
			return p.cursorDown()
		case 'h':
			return p.collapseSelected()
		case 'l':
			return p.expandSelected()
		case 'r':
			p.Refresh()
			return true
		case 'g':
			return p.goToTop()
		case 'G':
			return p.goToBottom()
		}
	}

	return false
}

// handleMouse processes mouse events
func (p *Panel) handleMouse(ev *tcell.EventMouse) bool {
	x, y := ev.Position()

	// Check if click is within our region
	if !p.Region.Contains(x, y) {
		return false
	}

	// Convert to local coordinates
	localY := y - p.Region.Y

	// Handle mouse wheel scrolling
	if ev.Buttons() == tcell.WheelUp {
		// Scroll up by 3 lines
		if p.Selected > 0 {
			p.Selected -= 3
			if p.Selected < 0 {
				p.Selected = 0
			}
			p.ensureSelectedVisible()
		}
		return true
	}

	if ev.Buttons() == tcell.WheelDown {
		// Scroll down by 3 lines
		nodes := p.Tree.GetNodes()
		if p.Selected < len(nodes)-1 {
			p.Selected += 3
			if p.Selected >= len(nodes) {
				p.Selected = len(nodes) - 1
			}
			p.ensureSelectedVisible()
		}
		return true
	}

	// Handle left click
	if ev.Buttons() == tcell.Button1 {
		// Check if click is on header (line 1 - line 0 is for border)
		if localY == 1 {
			log.Println("THICC FileBrowser: Header clicked, opening project picker")
			if p.OnProjectPathClick != nil {
				p.OnProjectPathClick()
				return true
			}
		}

		// Check if click is on a node (line 3+, after header at line 1 and separator at line 2)
		if localY >= 3 {
			nodeIndex := p.TopLine + (localY - 3)
			nodes := p.Tree.GetNodes()
			if nodeIndex < len(nodes) {
				p.Selected = nodeIndex
				node := nodes[nodeIndex]

				if node.IsDir {
					// Toggle directory expansion
					p.Tree.Toggle(node)
				} else {
					// Open file (click = actual open, unhides editor)
					if p.OnFileActualOpen != nil {
						p.OnFileActualOpen(node.Path)
					} else if p.OnFileOpen != nil {
						p.OnFileOpen(node.Path)
					}
				}
				return true
			}
		}
	}

	return false
}

// cursorUp moves selection up and previews file
func (p *Panel) cursorUp() bool {
	if p.Selected > 0 {
		p.Selected--
		p.ensureSelectedVisible()
		p.previewSelected() // Auto-preview on selection change
		return true
	}

	// If at first node (Selected == 0), move to header
	if p.Selected == 0 {
		p.Selected = -1 // Header is selected
		p.TopLine = 0   // Scroll to top when header selected
		return true
	}

	return false // Already at header, can't go up further
}

// cursorDown moves selection down and previews file
func (p *Panel) cursorDown() bool {
	// If header is selected, move to first node
	if p.Selected == -1 {
		nodes := p.Tree.GetNodes()
		if len(nodes) > 0 {
			p.Selected = 0
			p.ensureSelectedVisible()
			p.previewSelected() // Preview the first node
			return true
		}
		return false // No nodes to select
	}

	// Normal navigation through nodes
	nodes := p.Tree.GetNodes()
	if p.Selected < len(nodes)-1 {
		p.Selected++
		p.ensureSelectedVisible()
		p.previewSelected() // Auto-preview on selection change
		return true
	}
	return false // Can't go down from bottom
}

// previewSelected opens the selected file in the editor (if it's a file, not directory)
func (p *Panel) previewSelected() {
	// Don't preview when header is selected
	if p.Selected == -1 {
		return
	}

	nodes := p.Tree.GetNodes()
	if p.Selected >= len(nodes) {
		return
	}

	node := nodes[p.Selected]
	if node.IsDir {
		return // Don't preview directories
	}

	// Open file via callback
	if p.OnFileOpen != nil {
		p.OnFileOpen(node.Path)
	}
}

// expandSelected expands the selected directory
func (p *Panel) expandSelected() bool {
	nodes := p.Tree.GetNodes()
	if p.Selected >= len(nodes) {
		return false
	}

	node := nodes[p.Selected]
	if !node.IsDir {
		return false
	}

	// Use ExpandedPaths to check state (node.Expanded may be stale copy)
	if !p.Tree.ExpandedPaths[node.Path] {
		p.Tree.Expand(node)
		return true
	}

	// If already expanded, move down to first child
	return p.cursorDown()
}

// collapseSelected collapses the selected directory
func (p *Panel) collapseSelected() bool {
	nodes := p.Tree.GetNodes()
	if p.Selected >= len(nodes) {
		return false
	}

	node := nodes[p.Selected]
	if !node.IsDir {
		// If not a directory, go to parent
		if node.Owner >= 0 && node.Owner < len(nodes) {
			p.Selected = node.Owner
			p.ensureSelectedVisible()
			return true
		}
		return false
	}

	// Use ExpandedPaths to check state (node.Expanded may be stale copy)
	if p.Tree.ExpandedPaths[node.Path] {
		p.Tree.Collapse(node)
		return true
	}

	// If already collapsed, go to parent
	if node.Owner >= 0 && node.Owner < len(nodes) {
		p.Selected = node.Owner
		p.ensureSelectedVisible()
		return true
	}

	return false
}

// openSelected toggles directory or moves focus to editor for files
func (p *Panel) openSelected() bool {
	// If header is selected, open project picker
	if p.Selected == -1 {
		if p.OnProjectPathClick != nil {
			p.OnProjectPathClick()
			return true
		}
		return false
	}

	// Normal node opening logic
	nodes := p.Tree.GetNodes()
	if p.Selected >= len(nodes) {
		return false
	}

	node := nodes[p.Selected]

	if node.IsDir {
		// Toggle directory expansion
		p.Tree.Toggle(node)
		return true
	}

	// For files, just move focus to editor (file already previewed on selection)
	if p.OnFocusEditor != nil {
		p.OnFocusEditor()
		return true
	}

	return false
}

// pageUp moves up by one page
func (p *Panel) pageUp() bool {
	// Layout: top border (1) + header (1) + separator (1) + content + bottom border (1)
	contentHeight := p.Region.Height - 4
	if contentHeight < 1 {
		contentHeight = 1
	}

	p.Selected -= contentHeight
	if p.Selected < 0 {
		p.Selected = 0
	}
	p.ensureSelectedVisible()

	return true
}

// pageDown moves down by one page
func (p *Panel) pageDown() bool {
	nodes := p.Tree.GetNodes()
	// Layout: top border (1) + header (1) + separator (1) + content + bottom border (1)
	contentHeight := p.Region.Height - 4
	if contentHeight < 1 {
		contentHeight = 1
	}

	p.Selected += contentHeight
	if p.Selected >= len(nodes) {
		p.Selected = len(nodes) - 1
	}
	p.ensureSelectedVisible()

	return true
}

// goToTop moves to the first item
func (p *Panel) goToTop() bool {
	if p.Selected != 0 {
		p.Selected = 0
		p.TopLine = 0
		return true
	}
	return false
}

// goToBottom moves to the last item
func (p *Panel) goToBottom() bool {
	nodes := p.Tree.GetNodes()
	if len(nodes) > 0 && p.Selected != len(nodes)-1 {
		p.Selected = len(nodes) - 1
		p.ensureSelectedVisible()
		return true
	}
	return false
}
