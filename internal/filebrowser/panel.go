package filebrowser

import (
	"log"
	"path/filepath"
	"sync/atomic"

	"github.com/ellery/thicc/internal/filemanager"
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
	OnFileOpen       func(path string) // Called when user previews/opens a file (navigation)
	OnFileActualOpen func(path string) // Called when user clicks/enters on a file (unhides editor)
	OnTreeReady      func()            // Called when tree finishes loading
	OnFocusEditor   func()                                                 // Called when user wants to focus the editor (Enter on file)
	OnFileSaved        func(path string)                                      // Called when a file is saved (for tree refresh)
	OnProjectPathClick func()                                                 // Called when user clicks the project path header
	OnDeleteRequest    func(path string, isDir bool, callback func(confirmed bool)) // Called when user wants to delete a file/folder
	OnRenameRequest    func(oldPath string, callback func(newName string))          // Called when user wants to rename a file/folder
	OnNewFileRequest   func(dirPath string, callback func(fileName string))         // Called when user wants to create a new file
	OnNewFolderRequest func(dirPath string, callback func(folderName string))       // Called when user wants to create a new folder
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

	// Ensure selected is visible (skip if header is selected)
	if p.Selected >= 0 {
		if p.Selected < p.TopLine {
			p.TopLine = p.Selected
		}
		if p.Selected >= p.TopLine+contentHeight {
			p.TopLine = p.Selected - contentHeight + 1
		}
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

// SelectFile refreshes the tree and selects the given file
func (p *Panel) SelectFile(path string) {
	log.Printf("THOCK FileBrowser: SelectFile called for: %s", path)

	// Make path absolute if it isn't already
	absPath, err := filepath.Abs(path)
	if err != nil {
		log.Printf("THOCK FileBrowser: Error getting absolute path: %v", err)
		absPath = path
	}

	// Expand parent directory to make sure file is visible
	dir := filepath.Dir(absPath)
	if dir != p.Tree.Root {
		log.Printf("THOCK FileBrowser: Expanding parent dir: %s", dir)
		p.Tree.ExpandedPaths[dir] = true
	}

	// Refresh to pick up new file
	log.Println("THOCK FileBrowser: Refreshing tree")
	p.Tree.Refresh()

	// Select the file
	if p.Tree.SelectPath(absPath) {
		log.Printf("THOCK FileBrowser: Selected file at index %d", p.Tree.SelectedIdx)
		// Update panel's Selected to match tree's SelectedIdx
		p.Selected = p.Tree.SelectedIdx
		// Ensure visible
		p.ensureSelectedVisible()
	} else {
		log.Printf("THOCK FileBrowser: Could not select file: %s", absPath)
	}
}

// ensureSelectedVisible adjusts scrolling to make the selected item visible
func (p *Panel) ensureSelectedVisible() {
	contentHeight := p.Region.Height - 2 // 2 lines for header
	if contentHeight < 1 {
		contentHeight = 1
	}

	// Adjust topLine if selected is above visible area
	if p.Selected < p.TopLine {
		p.TopLine = p.Selected
	}

	// Adjust topLine if selected is below visible area
	if p.Selected >= p.TopLine+contentHeight {
		p.TopLine = p.Selected - contentHeight + 1
	}
}

// GetSelectedNode returns the currently selected tree node, or nil if none
func (p *Panel) GetSelectedNode() *filemanager.TreeNode {
	nodes := p.Tree.GetNodes()
	if p.Selected < 0 || p.Selected >= len(nodes) {
		return nil
	}
	return nodes[p.Selected]
}

// DeleteSelected initiates deletion of the currently selected item
func (p *Panel) DeleteSelected() {
	node := p.GetSelectedNode()
	if node == nil {
		log.Println("THOCK FileBrowser: DeleteSelected - no node selected")
		return
	}

	// Don't allow deleting the root
	if node.Path == p.Tree.Root {
		log.Println("THOCK FileBrowser: DeleteSelected - cannot delete root")
		return
	}

	log.Printf("THOCK FileBrowser: DeleteSelected - requesting delete for: %s (isDir=%v)", node.Path, node.IsDir)

	if p.OnDeleteRequest != nil {
		p.OnDeleteRequest(node.Path, node.IsDir, func(confirmed bool) {
			if confirmed {
				log.Printf("THOCK FileBrowser: Delete confirmed for: %s", node.Path)
				// Adjust selection before refresh (move up if we're at the last item)
				nodes := p.Tree.GetNodes()
				if p.Selected >= len(nodes)-1 && p.Selected > 0 {
					p.Selected--
				}
				p.Tree.Refresh()
			} else {
				log.Println("THOCK FileBrowser: Delete canceled")
			}
		})
	}
}

// RenameSelected initiates renaming of the currently selected item
func (p *Panel) RenameSelected() {
	node := p.GetSelectedNode()
	if node == nil {
		log.Println("THOCK FileBrowser: RenameSelected - no node selected")
		return
	}

	// Don't allow renaming the root
	if node.Path == p.Tree.Root {
		log.Println("THOCK FileBrowser: RenameSelected - cannot rename root")
		return
	}

	log.Printf("THOCK FileBrowser: RenameSelected - requesting rename for: %s", node.Path)

	if p.OnRenameRequest != nil {
		oldPath := node.Path
		p.OnRenameRequest(oldPath, func(newName string) {
			if newName == "" {
				log.Println("THOCK FileBrowser: Rename canceled (empty name)")
				return
			}

			currentName := filepath.Base(oldPath)
			if newName == currentName {
				log.Println("THOCK FileBrowser: Rename canceled (same name)")
				return
			}

			log.Printf("THOCK FileBrowser: Rename confirmed: %s -> %s", currentName, newName)
			// The actual rename and tree refresh will be handled by the callback setter
		})
	}
}

// getTargetDir returns the directory where new files/folders should be created
// If selected item is a directory, use it; otherwise use its parent
func (p *Panel) getTargetDir() string {
	node := p.GetSelectedNode()
	if node == nil {
		return p.Tree.Root
	}
	if node.IsDir {
		return node.Path
	}
	return filepath.Dir(node.Path)
}

// NewFileSelected initiates creation of a new file
func (p *Panel) NewFileSelected() {
	targetDir := p.getTargetDir()
	log.Printf("THOCK FileBrowser: NewFileSelected - requesting new file in: %s", targetDir)

	if p.OnNewFileRequest != nil {
		p.OnNewFileRequest(targetDir, func(fileName string) {
			if fileName == "" {
				log.Println("THOCK FileBrowser: New file canceled (empty name)")
				return
			}
			log.Printf("THOCK FileBrowser: New file confirmed: %s", fileName)
			// The actual file creation will be handled by the callback setter
		})
	}
}

// NewFolderSelected initiates creation of a new folder
func (p *Panel) NewFolderSelected() {
	targetDir := p.getTargetDir()
	log.Printf("THOCK FileBrowser: NewFolderSelected - requesting new folder in: %s", targetDir)

	if p.OnNewFolderRequest != nil {
		p.OnNewFolderRequest(targetDir, func(folderName string) {
			if folderName == "" {
				log.Println("THOCK FileBrowser: New folder canceled (empty name)")
				return
			}
			log.Printf("THOCK FileBrowser: New folder confirmed: %s", folderName)
			// The actual folder creation will be handled by the callback setter
		})
	}
}
