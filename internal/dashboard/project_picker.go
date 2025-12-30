package dashboard

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/micro-editor/tcell/v2"
)

// DirEntry represents a directory entry in the picker
type DirEntry struct {
	Name     string
	FullPath string
	IsDir    bool
}

// ProjectPicker is a modal for navigating and selecting project folders
type ProjectPicker struct {
	Active bool
	Screen tcell.Screen

	// Input field
	InputPath string
	CursorPos int

	// Directory listing
	CurrentDir   string
	Entries      []DirEntry
	FilteredList []DirEntry
	SelectedIdx  int
	TopLine      int // For scrolling

	// Dimensions
	Width       int
	Height      int
	ListHeight  int // Height of directory listing area

	// Callbacks
	OnSelect func(path string)
	OnCancel func()
}

// NewProjectPicker creates a new project picker
func NewProjectPicker(screen tcell.Screen, onSelect func(path string), onCancel func()) *ProjectPicker {
	homeDir, _ := os.UserHomeDir()

	p := &ProjectPicker{
		Screen:    screen,
		OnSelect:  onSelect,
		OnCancel:  onCancel,
		Width:     60,
		Height:    20,
		ListHeight: 12,
	}

	// Start at home directory
	p.InputPath = "~/"
	p.CursorPos = len(p.InputPath)
	p.CurrentDir = homeDir
	p.loadDirectory(homeDir)

	return p
}

// Show activates the picker
func (p *ProjectPicker) Show() {
	p.Active = true
	// Reload directory in case it changed
	expanded := p.expandTilde(p.InputPath)
	if info, err := os.Stat(expanded); err == nil && info.IsDir() {
		p.CurrentDir = expanded
		p.loadDirectory(expanded)
	}
}

// Hide deactivates the picker
func (p *ProjectPicker) Hide() {
	p.Active = false
}

// expandTilde expands ~ to home directory
func (p *ProjectPicker) expandTilde(path string) string {
	if strings.HasPrefix(path, "~/") {
		homeDir, _ := os.UserHomeDir()
		return filepath.Join(homeDir, path[2:])
	}
	if path == "~" {
		homeDir, _ := os.UserHomeDir()
		return homeDir
	}
	return path
}

// collapseTilde replaces home directory with ~
func (p *ProjectPicker) collapseTilde(path string) string {
	homeDir, _ := os.UserHomeDir()
	if strings.HasPrefix(path, homeDir) {
		return "~" + path[len(homeDir):]
	}
	return path
}

// loadDirectory reads directory contents (folders only)
func (p *ProjectPicker) loadDirectory(path string) {
	p.Entries = nil
	p.FilteredList = nil
	p.SelectedIdx = 0
	p.TopLine = 0

	entries, err := os.ReadDir(path)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			p.Entries = append(p.Entries, DirEntry{
				Name:     entry.Name(),
				FullPath: filepath.Join(path, entry.Name()),
				IsDir:    true,
			})
		}
	}

	// Sort alphabetically
	sort.Slice(p.Entries, func(i, j int) bool {
		return strings.ToLower(p.Entries[i].Name) < strings.ToLower(p.Entries[j].Name)
	})

	p.filterEntries()
}

// filterEntries filters the directory list based on input
func (p *ProjectPicker) filterEntries() {
	p.FilteredList = nil

	// Get the filter text (part after last /)
	filter := ""
	if idx := strings.LastIndex(p.InputPath, "/"); idx >= 0 {
		filter = strings.ToLower(p.InputPath[idx+1:])
	}

	for _, entry := range p.Entries {
		if filter == "" || strings.Contains(strings.ToLower(entry.Name), filter) {
			p.FilteredList = append(p.FilteredList, entry)
		}
	}

	// Reset selection if out of bounds
	if p.SelectedIdx >= len(p.FilteredList) {
		p.SelectedIdx = 0
	}
	p.TopLine = 0
}

// HandleEvent processes input events
func (p *ProjectPicker) HandleEvent(event tcell.Event) bool {
	if !p.Active {
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

func (p *ProjectPicker) handleKey(ev *tcell.EventKey) bool {
	switch ev.Key() {
	case tcell.KeyEscape:
		if p.OnCancel != nil {
			p.OnCancel()
		}
		return true

	case tcell.KeyEnter:
		// Open the selected folder
		if len(p.FilteredList) > 0 && p.SelectedIdx < len(p.FilteredList) {
			selected := p.FilteredList[p.SelectedIdx]
			if p.OnSelect != nil {
				p.OnSelect(selected.FullPath)
			}
		} else {
			// If no selection, try to open current directory
			expanded := p.expandTilde(p.InputPath)
			if info, err := os.Stat(expanded); err == nil && info.IsDir() {
				if p.OnSelect != nil {
					p.OnSelect(expanded)
				}
			}
		}
		return true

	case tcell.KeyTab:
		p.handleTab()
		return true

	case tcell.KeyUp:
		p.moveSelectionUp()
		return true

	case tcell.KeyDown:
		p.moveSelectionDown()
		return true

	case tcell.KeyLeft:
		if p.CursorPos > 0 {
			p.CursorPos--
		}
		return true

	case tcell.KeyRight:
		if p.CursorPos < len(p.InputPath) {
			p.CursorPos++
		}
		return true

	case tcell.KeyHome:
		p.CursorPos = 0
		return true

	case tcell.KeyEnd:
		p.CursorPos = len(p.InputPath)
		return true

	case tcell.KeyBackspace, tcell.KeyBackspace2:
		p.handleBackspace()
		return true

	case tcell.KeyDelete:
		if p.CursorPos < len(p.InputPath) {
			p.InputPath = p.InputPath[:p.CursorPos] + p.InputPath[p.CursorPos+1:]
			p.filterEntries()
		}
		return true

	case tcell.KeyRune:
		// Insert character at cursor
		ch := ev.Rune()
		p.InputPath = p.InputPath[:p.CursorPos] + string(ch) + p.InputPath[p.CursorPos:]
		p.CursorPos++
		p.filterEntries()
		return true
	}

	return false
}

func (p *ProjectPicker) handleMouse(ev *tcell.EventMouse) bool {
	// Basic mouse support - could be extended
	return false
}

// handleTab handles tab completion and drilling into folders
func (p *ProjectPicker) handleTab() {
	// If there's a selection in the list, drill into it
	if len(p.FilteredList) > 0 && p.SelectedIdx < len(p.FilteredList) {
		selected := p.FilteredList[p.SelectedIdx]

		// Update input to this folder and load its contents
		p.InputPath = p.collapseTilde(selected.FullPath) + "/"
		p.CursorPos = len(p.InputPath)
		p.CurrentDir = selected.FullPath
		p.loadDirectory(selected.FullPath)
		return
	}

	// Otherwise, try path completion
	expanded := p.expandTilde(p.InputPath)

	// Check if current input is a valid directory
	if info, err := os.Stat(expanded); err == nil && info.IsDir() {
		// It's a valid directory - make sure it ends with / and load it
		if !strings.HasSuffix(p.InputPath, "/") {
			p.InputPath += "/"
			p.CursorPos = len(p.InputPath)
		}
		p.CurrentDir = expanded
		p.loadDirectory(expanded)
		return
	}

	// Try to complete partial path
	dir := filepath.Dir(expanded)
	base := filepath.Base(expanded)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}

	var matches []string
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(strings.ToLower(entry.Name()), strings.ToLower(base)) {
			matches = append(matches, entry.Name())
		}
	}

	if len(matches) == 1 {
		// Single match - complete it
		completed := filepath.Join(dir, matches[0])
		p.InputPath = p.collapseTilde(completed) + "/"
		p.CursorPos = len(p.InputPath)
		p.CurrentDir = completed
		p.loadDirectory(completed)
	} else if len(matches) > 1 {
		// Multiple matches - find common prefix
		commonPrefix := matches[0]
		for _, m := range matches[1:] {
			commonPrefix = commonPrefixStr(commonPrefix, m)
		}
		if len(commonPrefix) > len(base) {
			completed := filepath.Join(dir, commonPrefix)
			p.InputPath = p.collapseTilde(completed)
			p.CursorPos = len(p.InputPath)
			// Don't change directory yet, let user see options
			p.CurrentDir = dir
			p.loadDirectory(dir)
		}
	}
}

// handleBackspace handles backspace key
func (p *ProjectPicker) handleBackspace() {
	if p.CursorPos <= 0 {
		return
	}

	// Check if we're at a path boundary (right after a /)
	if p.CursorPos > 0 && p.InputPath[p.CursorPos-1] == '/' {
		// Go up one directory level
		expanded := p.expandTilde(p.InputPath[:p.CursorPos-1])
		parent := filepath.Dir(expanded)
		if parent != expanded {
			p.InputPath = p.collapseTilde(parent) + "/"
			p.CursorPos = len(p.InputPath)
			p.CurrentDir = parent
			p.loadDirectory(parent)
			return
		}
	}

	// Normal backspace - delete character before cursor
	p.InputPath = p.InputPath[:p.CursorPos-1] + p.InputPath[p.CursorPos:]
	p.CursorPos--
	p.filterEntries()
}

// moveSelectionUp moves the selection up in the list
func (p *ProjectPicker) moveSelectionUp() {
	if len(p.FilteredList) == 0 {
		return
	}
	p.SelectedIdx--
	if p.SelectedIdx < 0 {
		p.SelectedIdx = len(p.FilteredList) - 1
	}
	p.ensureVisible()
}

// moveSelectionDown moves the selection down in the list
func (p *ProjectPicker) moveSelectionDown() {
	if len(p.FilteredList) == 0 {
		return
	}
	p.SelectedIdx++
	if p.SelectedIdx >= len(p.FilteredList) {
		p.SelectedIdx = 0
	}
	p.ensureVisible()
}

// ensureVisible adjusts scroll to keep selection visible
func (p *ProjectPicker) ensureVisible() {
	if p.SelectedIdx < p.TopLine {
		p.TopLine = p.SelectedIdx
	}
	if p.SelectedIdx >= p.TopLine+p.ListHeight {
		p.TopLine = p.SelectedIdx - p.ListHeight + 1
	}
}

// Render draws the project picker
func (p *ProjectPicker) Render(screen tcell.Screen) {
	if !p.Active {
		return
	}

	w, h := screen.Size()

	// Calculate position (centered)
	x := (w - p.Width) / 2
	y := (h - p.Height) / 2

	// Draw background
	bgStyle := tcell.StyleDefault.Background(ColorBgDark)
	for dy := 0; dy < p.Height; dy++ {
		for dx := 0; dx < p.Width; dx++ {
			screen.SetContent(x+dx, y+dy, ' ', nil, bgStyle)
		}
	}

	// Draw border
	borderStyle := tcell.StyleDefault.Foreground(ColorCyan).Bold(true)

	// Top border with title
	screen.SetContent(x, y, '╔', nil, borderStyle)
	screen.SetContent(x+p.Width-1, y, '╗', nil, borderStyle)
	for i := 1; i < p.Width-1; i++ {
		screen.SetContent(x+i, y, '═', nil, borderStyle)
	}

	// Title
	title := " Open Project "
	titleX := x + (p.Width-len(title))/2
	titleStyle := tcell.StyleDefault.Foreground(ColorCyan).Bold(true)
	for i, ch := range title {
		screen.SetContent(titleX+i, y, ch, nil, titleStyle)
	}

	// Separator after title
	screen.SetContent(x, y+1, '╠', nil, borderStyle)
	screen.SetContent(x+p.Width-1, y+1, '╣', nil, borderStyle)
	for i := 1; i < p.Width-1; i++ {
		screen.SetContent(x+i, y+1, '═', nil, borderStyle)
	}

	// Input field (line 2)
	inputY := y + 2
	inputStyle := tcell.StyleDefault.Foreground(ColorTextBright)

	// Draw input with cursor
	inputX := x + 2
	inputWidth := p.Width - 4
	displayPath := p.InputPath

	// Scroll input if too long
	inputOffset := 0
	if p.CursorPos > inputWidth-1 {
		inputOffset = p.CursorPos - inputWidth + 1
	}

	for i := 0; i < inputWidth; i++ {
		charIdx := inputOffset + i
		if charIdx < len(displayPath) {
			screen.SetContent(inputX+i, inputY, rune(displayPath[charIdx]), nil, inputStyle)
		} else {
			screen.SetContent(inputX+i, inputY, ' ', nil, inputStyle)
		}
	}

	// Draw cursor
	cursorX := inputX + (p.CursorPos - inputOffset)
	if cursorX >= inputX && cursorX < inputX+inputWidth {
		cursorStyle := tcell.StyleDefault.Foreground(ColorBgDark).Background(ColorTextBright)
		ch := ' '
		if p.CursorPos < len(displayPath) {
			ch = rune(displayPath[p.CursorPos])
		}
		screen.SetContent(cursorX, inputY, ch, nil, cursorStyle)
	}

	// Separator before list
	listSepY := y + 3
	screen.SetContent(x, listSepY, '╠', nil, borderStyle)
	screen.SetContent(x+p.Width-1, listSepY, '╣', nil, borderStyle)
	for i := 1; i < p.Width-1; i++ {
		screen.SetContent(x+i, listSepY, '─', nil, tcell.StyleDefault.Foreground(ColorCyan))
	}

	// Directory list
	listY := y + 4
	listStyle := tcell.StyleDefault.Foreground(ColorTextDim)
	selectedStyle := tcell.StyleDefault.Foreground(ColorBgDark).Background(ColorYellow).Bold(true)

	for i := 0; i < p.ListHeight; i++ {
		entryIdx := p.TopLine + i
		lineY := listY + i

		if entryIdx < len(p.FilteredList) {
			entry := p.FilteredList[entryIdx]
			style := listStyle
			prefix := "   "

			if entryIdx == p.SelectedIdx {
				style = selectedStyle
				prefix = " > "
			}

			// Draw prefix
			for j, ch := range prefix {
				screen.SetContent(x+1+j, lineY, ch, nil, style)
			}

			// Draw folder icon and name
			displayName := entry.Name + "/"
			if len(displayName) > p.Width-8 {
				displayName = displayName[:p.Width-11] + "..."
			}

			for j, ch := range displayName {
				if x+4+j < x+p.Width-1 {
					screen.SetContent(x+4+j, lineY, ch, nil, style)
				}
			}

			// Fill rest of line if selected
			if entryIdx == p.SelectedIdx {
				for j := 4 + len(displayName); j < p.Width-1; j++ {
					screen.SetContent(x+j, lineY, ' ', nil, style)
				}
			}
		}
	}

	// Left and right borders for list area
	for i := 2; i < p.Height-3; i++ {
		screen.SetContent(x, y+i, '║', nil, borderStyle)
		screen.SetContent(x+p.Width-1, y+i, '║', nil, borderStyle)
	}

	// Separator before hints
	hintSepY := y + p.Height - 3
	screen.SetContent(x, hintSepY, '╠', nil, borderStyle)
	screen.SetContent(x+p.Width-1, hintSepY, '╣', nil, borderStyle)
	for i := 1; i < p.Width-1; i++ {
		screen.SetContent(x+i, hintSepY, '─', nil, tcell.StyleDefault.Foreground(ColorCyan))
	}

	// Hints
	hintY := y + p.Height - 2
	hints := "[Tab] Complete  [Enter] Open  [Esc] Cancel"
	hintStyle := tcell.StyleDefault.Foreground(ColorTextMuted)
	hintX := x + (p.Width-len(hints))/2
	for i, ch := range hints {
		screen.SetContent(hintX+i, hintY, ch, nil, hintStyle)
	}

	// Bottom border
	screen.SetContent(x, y+p.Height-1, '╚', nil, borderStyle)
	screen.SetContent(x+p.Width-1, y+p.Height-1, '╝', nil, borderStyle)
	for i := 1; i < p.Width-1; i++ {
		screen.SetContent(x+i, y+p.Height-1, '═', nil, borderStyle)
	}
}

// commonPrefixStr returns the common prefix of two strings
func commonPrefixStr(a, b string) string {
	minLen := len(a)
	if len(b) < minLen {
		minLen = len(b)
	}
	for i := 0; i < minLen; i++ {
		if strings.ToLower(string(a[i])) != strings.ToLower(string(b[i])) {
			return a[:i]
		}
	}
	return a[:minLen]
}
