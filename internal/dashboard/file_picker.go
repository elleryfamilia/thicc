package dashboard

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/micro-editor/tcell/v2"
)

// FilePicker is a modal for navigating and selecting files or folders
type FilePicker struct {
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
	Width      int
	Height     int
	ListHeight int // Height of directory listing area

	// Callbacks
	OnSelectFile   func(path string) // Called when a file is selected
	OnSelectFolder func(path string) // Called when a folder is selected (Enter on folder)
	OnCancel       func()
}

// NewFilePicker creates a new file picker
func NewFilePicker(screen tcell.Screen, onSelectFile func(path string), onSelectFolder func(path string), onCancel func()) *FilePicker {
	homeDir, _ := os.UserHomeDir()

	p := &FilePicker{
		Screen:         screen,
		OnSelectFile:   onSelectFile,
		OnSelectFolder: onSelectFolder,
		OnCancel:       onCancel,
		Width:          60,
		Height:         20,
		ListHeight:     12,
	}

	// Start at home directory
	p.InputPath = "~/"
	p.CursorPos = len(p.InputPath)
	p.CurrentDir = homeDir
	p.loadDirectory(homeDir)

	return p
}

// Show activates the picker
func (p *FilePicker) Show() {
	p.Active = true
	// Reload directory in case it changed
	expanded := p.expandTilde(p.InputPath)
	if info, err := os.Stat(expanded); err == nil && info.IsDir() {
		p.CurrentDir = expanded
		p.loadDirectory(expanded)
	}
}

// Hide deactivates the picker
func (p *FilePicker) Hide() {
	p.Active = false
}

// expandTilde expands ~ to home directory
func (p *FilePicker) expandTilde(path string) string {
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
func (p *FilePicker) collapseTilde(path string) string {
	homeDir, _ := os.UserHomeDir()
	if strings.HasPrefix(path, homeDir) {
		return "~" + path[len(homeDir):]
	}
	return path
}

// loadDirectory reads directory contents (files AND folders)
func (p *FilePicker) loadDirectory(path string) {
	p.Entries = nil
	p.FilteredList = nil
	p.SelectedIdx = 0
	p.TopLine = 0

	entries, err := os.ReadDir(path)
	if err != nil {
		return
	}

	// Separate dirs and files
	var dirs, files []DirEntry

	for _, entry := range entries {
		// Skip hidden files
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}

		de := DirEntry{
			Name:     entry.Name(),
			FullPath: filepath.Join(path, entry.Name()),
			IsDir:    entry.IsDir(),
		}

		if entry.IsDir() {
			dirs = append(dirs, de)
		} else {
			files = append(files, de)
		}
	}

	// Sort alphabetically (case-insensitive)
	sort.Slice(dirs, func(i, j int) bool {
		return strings.ToLower(dirs[i].Name) < strings.ToLower(dirs[j].Name)
	})
	sort.Slice(files, func(i, j int) bool {
		return strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name)
	})

	// Directories first, then files
	p.Entries = append(dirs, files...)

	p.filterEntries()
}

// filterEntries filters the directory list based on input
func (p *FilePicker) filterEntries() {
	p.FilteredList = nil

	// Get the filter text by comparing InputPath with CurrentDir
	filter := ""

	// Normalize both paths for comparison
	normalizedInput := p.expandTilde(p.InputPath)
	normalizedCurrent := p.CurrentDir

	// Check if InputPath points to a valid directory different from CurrentDir
	// Remove trailing slash for directory check
	inputDir := strings.TrimSuffix(normalizedInput, "/")
	currentDir := strings.TrimSuffix(normalizedCurrent, "/")

	if inputDir != currentDir {
		// Check if the input path is a valid directory
		if info, err := os.Stat(inputDir); err == nil && info.IsDir() {
			// Load this directory instead of filtering
			p.CurrentDir = inputDir
			p.loadDirectory(inputDir)
			return
		}
	}

	// Ensure both have trailing slashes for consistent comparison
	if !strings.HasSuffix(normalizedInput, "/") {
		normalizedInput += "/"
	}
	if !strings.HasSuffix(normalizedCurrent, "/") {
		normalizedCurrent += "/"
	}

	// If InputPath is within CurrentDir, extract any filter text
	if strings.HasPrefix(normalizedInput, normalizedCurrent) {
		// Filter is anything typed after the current directory path
		filter = strings.ToLower(normalizedInput[len(normalizedCurrent):])
		// Remove trailing slash from filter if present
		filter = strings.TrimSuffix(filter, "/")
	}

	// Apply filter
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
func (p *FilePicker) HandleEvent(event tcell.Event) bool {
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

func (p *FilePicker) handleKey(ev *tcell.EventKey) bool {
	switch ev.Key() {
	case tcell.KeyEscape:
		if p.OnCancel != nil {
			p.OnCancel()
		}
		return true

	case tcell.KeyEnter:
		// Open the selected item
		if len(p.FilteredList) > 0 && p.SelectedIdx < len(p.FilteredList) {
			selected := p.FilteredList[p.SelectedIdx]
			if selected.IsDir {
				// Directory - open as project
				if p.OnSelectFolder != nil {
					p.OnSelectFolder(selected.FullPath)
				}
			} else {
				// File - open file
				if p.OnSelectFile != nil {
					p.OnSelectFile(selected.FullPath)
				}
			}
		} else {
			// If no selection, try to open current directory as project
			expanded := p.expandTilde(p.InputPath)
			if info, err := os.Stat(expanded); err == nil && info.IsDir() {
				if p.OnSelectFolder != nil {
					p.OnSelectFolder(expanded)
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

func (p *FilePicker) handleMouse(ev *tcell.EventMouse) bool {
	// Only handle left clicks
	if ev.Buttons() != tcell.Button1 {
		return false
	}

	mouseX, mouseY := ev.Position()

	// Calculate modal position (same logic as Render method)
	w, h := p.Screen.Size()
	x := (w - p.Width) / 2
	y := (h - p.Height) / 2

	// Check if click is within modal bounds
	if mouseX < x || mouseX >= x+p.Width || mouseY < y || mouseY >= y+p.Height {
		return false
	}

	// Convert to local coordinates
	localY := mouseY - y

	// List area starts at line 4 (after title + separator + input + separator)
	// and spans ListHeight rows
	listStartY := 4
	listEndY := listStartY + p.ListHeight

	if localY >= listStartY && localY < listEndY {
		// Calculate which item was clicked
		itemIndex := p.TopLine + (localY - listStartY)

		// Validate index is within filtered list
		if itemIndex < len(p.FilteredList) {
			// Set selection to clicked item
			p.SelectedIdx = itemIndex
			selected := p.FilteredList[itemIndex]

			if selected.IsDir {
				// Drill into the folder
				p.handleTab()
			} else {
				// Open the file
				if p.OnSelectFile != nil {
					p.OnSelectFile(selected.FullPath)
				}
			}
			return true
		}
	}

	return false
}

// handleTab handles tab completion and drilling into folders
func (p *FilePicker) handleTab() {
	// If there's a selection in the list and it's a directory, drill into it
	if len(p.FilteredList) > 0 && p.SelectedIdx < len(p.FilteredList) {
		selected := p.FilteredList[p.SelectedIdx]

		if selected.IsDir {
			// Update input to this folder and load its contents
			p.InputPath = p.collapseTilde(selected.FullPath) + "/"
			p.CursorPos = len(p.InputPath)
			p.CurrentDir = selected.FullPath
			p.loadDirectory(selected.FullPath)
			return
		}
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
		if strings.HasPrefix(strings.ToLower(entry.Name()), strings.ToLower(base)) {
			matches = append(matches, entry.Name())
		}
	}

	if len(matches) == 1 {
		// Single match - complete it
		completed := filepath.Join(dir, matches[0])
		info, err := os.Stat(completed)
		if err == nil && info.IsDir() {
			p.InputPath = p.collapseTilde(completed) + "/"
			p.CursorPos = len(p.InputPath)
			p.CurrentDir = completed
			p.loadDirectory(completed)
		} else {
			p.InputPath = p.collapseTilde(completed)
			p.CursorPos = len(p.InputPath)
		}
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
func (p *FilePicker) handleBackspace() {
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
func (p *FilePicker) moveSelectionUp() {
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
func (p *FilePicker) moveSelectionDown() {
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
func (p *FilePicker) ensureVisible() {
	if p.SelectedIdx < p.TopLine {
		p.TopLine = p.SelectedIdx
	}
	if p.SelectedIdx >= p.TopLine+p.ListHeight {
		p.TopLine = p.SelectedIdx - p.ListHeight + 1
	}
}

// getContextualHints generates hint text
func (p *FilePicker) getContextualHints() string {
	return "[Tab] Drill in  [Bksp] Go up  [Enter] Open  [Esc] Cancel"
}

// Render draws the file picker
func (p *FilePicker) Render(screen tcell.Screen) {
	if !p.Active {
		return
	}

	w, h := screen.Size()

	// Calculate position (centered)
	x := (w - p.Width) / 2
	y := (h - p.Height) / 2

	// Draw background
	bgStyle := tcell.StyleDefault.Foreground(ColorTextBright).Background(ColorBgDark)
	for dy := 0; dy < p.Height; dy++ {
		for dx := 0; dx < p.Width; dx++ {
			screen.SetContent(x+dx, y+dy, ' ', nil, bgStyle)
		}
	}

	// Draw border
	borderStyle := tcell.StyleDefault.Foreground(ColorCyan).Background(ColorBgDark).Bold(true)

	// Top border with title
	screen.SetContent(x, y, '╔', nil, borderStyle)
	screen.SetContent(x+p.Width-1, y, '╗', nil, borderStyle)
	for i := 1; i < p.Width-1; i++ {
		screen.SetContent(x+i, y, '═', nil, borderStyle)
	}

	// Title
	title := " Open File "
	titleX := x + (p.Width-len(title))/2
	titleStyle := tcell.StyleDefault.Foreground(ColorCyan).Background(ColorBgDark).Bold(true)
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
	inputStyle := tcell.StyleDefault.Foreground(ColorTextBright).Background(ColorBgDark)
	previewStyle := tcell.StyleDefault.Foreground(ColorTextMuted).Background(ColorBgDark)

	// Draw input with cursor
	inputX := x + 2
	inputWidth := p.Width - 4

	// Build display path with selected item preview
	actualPath := p.InputPath
	previewPath := ""
	if len(p.FilteredList) > 0 && p.SelectedIdx < len(p.FilteredList) {
		selected := p.FilteredList[p.SelectedIdx]
		suffix := ""
		if selected.IsDir {
			suffix = "/"
		}
		if strings.HasSuffix(actualPath, "/") {
			previewPath = selected.Name + suffix
		} else {
			previewPath = "/" + selected.Name + suffix
		}
	}
	displayPath := actualPath + previewPath

	// Scroll input if too long
	inputOffset := 0
	if p.CursorPos > inputWidth-1 {
		inputOffset = p.CursorPos - inputWidth + 1
	}

	// Determine if we should highlight a segment
	highlightStart := -1
	highlightEnd := -1
	if p.CursorPos > 0 && p.CursorPos <= len(actualPath) && actualPath[p.CursorPos-1] == '/' {
		segmentEnd := p.CursorPos - 1
		segmentStart := segmentEnd - 1
		for segmentStart >= 0 && actualPath[segmentStart] != '/' {
			segmentStart--
		}
		segmentStart++
		highlightStart = segmentStart
		highlightEnd = segmentEnd
	}

	// Draw input characters with appropriate styling
	highlightStyle := tcell.StyleDefault.Foreground(ColorBgDark).Background(ColorYellow)
	for i := 0; i < inputWidth; i++ {
		charIdx := inputOffset + i
		if charIdx < len(displayPath) {
			style := inputStyle

			if charIdx >= highlightStart && charIdx < highlightEnd {
				style = highlightStyle
			} else if charIdx >= len(actualPath) {
				style = previewStyle
			}

			screen.SetContent(inputX+i, inputY, rune(displayPath[charIdx]), nil, style)
		} else {
			screen.SetContent(inputX+i, inputY, ' ', nil, inputStyle)
		}
	}

	// Draw cursor
	cursorX := inputX + (p.CursorPos - inputOffset)
	if cursorX >= inputX && cursorX < inputX+inputWidth {
		cursorStyle := tcell.StyleDefault.Foreground(ColorBgDark).Background(ColorTextBright)
		ch := ' '
		charIdx := p.CursorPos
		if charIdx < len(displayPath) {
			ch = rune(displayPath[charIdx])
		}
		screen.SetContent(cursorX, inputY, ch, nil, cursorStyle)
	}

	// Separator before list
	listSepY := y + 3
	screen.SetContent(x, listSepY, '╠', nil, borderStyle)
	screen.SetContent(x+p.Width-1, listSepY, '╣', nil, borderStyle)
	sepStyle := tcell.StyleDefault.Foreground(ColorCyan).Background(ColorBgDark)
	for i := 1; i < p.Width-1; i++ {
		screen.SetContent(x+i, listSepY, '─', nil, sepStyle)
	}

	// Directory/file list
	listY := y + 4
	listStyle := tcell.StyleDefault.Foreground(ColorTextDim).Background(ColorBgDark)
	selectedStyle := tcell.StyleDefault.Foreground(ColorBgDark).Background(ColorYellow).Bold(true)
	dirStyle := tcell.StyleDefault.Foreground(ColorCyan).Background(ColorBgDark)

	for i := 0; i < p.ListHeight; i++ {
		entryIdx := p.TopLine + i
		lineY := listY + i

		if entryIdx < len(p.FilteredList) {
			entry := p.FilteredList[entryIdx]
			style := listStyle
			if entry.IsDir {
				style = dirStyle
			}
			prefix := "   "

			if entryIdx == p.SelectedIdx {
				style = selectedStyle
				prefix = " > "
			}

			// Draw prefix
			for j, ch := range prefix {
				screen.SetContent(x+1+j, lineY, ch, nil, style)
			}

			// Draw name with appropriate suffix
			displayName := entry.Name
			if entry.IsDir {
				displayName += "/"
			}
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
		screen.SetContent(x+i, hintSepY, '─', nil, sepStyle)
	}

	// Hints
	hintY := y + p.Height - 2
	hints := p.getContextualHints()
	hintStyle := tcell.StyleDefault.Foreground(ColorTextMuted).Background(ColorBgDark)
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
