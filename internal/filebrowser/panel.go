package filebrowser

import (
	"log"
	"sync/atomic"

	"github.com/ellery/thock/internal/filemanager"
)

// Panel is a standalone file browser with direct tcell rendering
type Panel struct {
	Tree     *filemanager.Tree
	Region   Region
	Selected int   // Currently selected line (in visible nodes)
	TopLine  int   // First visible line (for scrolling)
	Focus    bool  // Is this panel focused?
	ready    int32 // Atomic flag: 1 = tree loaded and ready to render

	// Callbacks
	OnFileOpen    func(path string) // Called when user previews/opens a file
	OnTreeReady   func()            // Called when tree finishes loading
	OnFocusEditor func()            // Called when user wants to focus the editor (Enter on file)
}

// NewPanel creates a new file browser panel
func NewPanel(x, y, w, h int, root string) *Panel {
	p := &Panel{
		Tree:     filemanager.NewTree(root),
		Region:   Region{X: x, Y: y, Width: w, Height: h},
		Selected: 0,
		TopLine:  0,
		Focus:    false,
	}

	// Initial scan in background with safeguards (depth limit, skip list)
	go func() {
		log.Println("THOCK FileBrowser: Starting tree refresh with safeguards")
		err := p.Tree.Refresh()
		if err != nil {
			log.Printf("THOCK FileBrowser: Refresh failed: %v", err)
		} else {
			atomic.StoreInt32(&p.ready, 1)
			log.Printf("THOCK FileBrowser: Tree refresh complete, loaded %d nodes", len(p.Tree.GetNodes()))

			// Trigger screen refresh if callback is set
			if p.OnTreeReady != nil {
				log.Println("THOCK FileBrowser: Calling OnTreeReady callback")
				p.OnTreeReady()
			}
		}
	}()

	return p
}

// GetVisibleNodes returns the nodes that should be visible given the current scroll position
func (p *Panel) GetVisibleNodes() []*filemanager.TreeNode {
	// Don't try to get nodes if tree isn't ready (avoids blocking on lock)
	if atomic.LoadInt32(&p.ready) == 0 {
		return nil
	}

	nodes := p.Tree.GetNodes()
	if len(nodes) == 0 {
		return nodes
	}

	// Calculate how many lines we have for content (minus header)
	contentHeight := p.Region.Height - 2 // 2 lines for header
	if contentHeight < 1 {
		contentHeight = 1
	}

	// Adjust topLine if needed
	if p.TopLine > len(nodes)-1 {
		p.TopLine = len(nodes) - 1
	}
	if p.TopLine < 0 {
		p.TopLine = 0
	}

	// Ensure selected is visible
	if p.Selected < p.TopLine {
		p.TopLine = p.Selected
	}
	if p.Selected >= p.TopLine+contentHeight {
		p.TopLine = p.Selected - contentHeight + 1
	}

	// Extract visible slice
	endLine := p.TopLine + contentHeight
	if endLine > len(nodes) {
		endLine = len(nodes)
	}

	return nodes[p.TopLine:endLine]
}

// Refresh rescans the file tree
func (p *Panel) Refresh() {
	p.Tree.Refresh()
}

// Close cleans up resources
func (p *Panel) Close() {
	// No resources to clean up currently
}
