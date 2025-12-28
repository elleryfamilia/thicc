package filebrowser

import (
	"github.com/micro-editor/tcell/v2"
)

// HandleEvent processes keyboard and mouse events
func (p *Panel) HandleEvent(event tcell.Event) bool {
	if !p.Focus {
		return false
	}

	switch ev := event.(type) {
	case *tcell.EventKey:
		return p.handleKey(ev)
	case *tcell.EventMouse:
		return p.handleMouse(ev)
	}

	return false
}

// handleKey processes keyboard events
func (p *Panel) handleKey(ev *tcell.EventKey) bool {
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

	// Handle different mouse button actions
	switch ev.Buttons() {
	case tcell.Button1: // Left click
		// Check if click is on a node (after header)
		if localY >= 2 {
			nodeIndex := p.TopLine + (localY - 2)
			nodes := p.Tree.GetNodes()
			if nodeIndex < len(nodes) {
				p.Selected = nodeIndex
				return true
			}
		}

	case tcell.ButtonNone:
		// This might be a double-click or release
		// For now, we'll handle double-click as open
		if localY >= 2 {
			nodeIndex := p.TopLine + (localY - 2)
			nodes := p.Tree.GetNodes()
			if nodeIndex < len(nodes) && nodeIndex == p.Selected {
				return p.openSelected()
			}
		}
	}

	return false
}

// cursorUp moves selection up and previews file
func (p *Panel) cursorUp() bool {
	if p.Selected > 0 {
		p.Selected--
		p.previewSelected() // Auto-preview on selection change
		return true
	}
	return false
}

// cursorDown moves selection down and previews file
func (p *Panel) cursorDown() bool {
	nodes := p.Tree.GetNodes()
	if p.Selected < len(nodes)-1 {
		p.Selected++
		p.previewSelected() // Auto-preview on selection change
		return true
	}
	return false
}

// previewSelected opens the selected file in the editor (if it's a file, not directory)
func (p *Panel) previewSelected() {
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
		return true
	}

	return false
}

// openSelected toggles directory or moves focus to editor for files
func (p *Panel) openSelected() bool {
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
	contentHeight := p.Region.Height - 2
	if contentHeight < 1 {
		contentHeight = 1
	}

	p.Selected -= contentHeight
	if p.Selected < 0 {
		p.Selected = 0
	}

	return true
}

// pageDown moves down by one page
func (p *Panel) pageDown() bool {
	nodes := p.Tree.GetNodes()
	contentHeight := p.Region.Height - 2
	if contentHeight < 1 {
		contentHeight = 1
	}

	p.Selected += contentHeight
	if p.Selected >= len(nodes) {
		p.Selected = len(nodes) - 1
	}

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
		return true
	}
	return false
}
