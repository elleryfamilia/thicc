package dashboard

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/micro-editor/tcell/v2"
)

// FolderCreatorStep represents the current step in folder creation
type FolderCreatorStep int

const (
	StepSelectLocation FolderCreatorStep = iota
	StepEnterName
)

// FolderCreator is a modal for creating a new folder
type FolderCreator struct {
	Active bool
	Screen tcell.Screen
	Step   FolderCreatorStep

	// Location selection (step 1)
	InputPath  string
	CursorPos  int
	CurrentDir string
	Entries    []DirEntry
	FilteredList []DirEntry
	SelectedIdx int
	TopLine    int

	// Folder name input (step 2)
	FolderName     string
	FolderNameCursor int
	ErrorMessage   string

	// Dimensions
	Width      int
	Height     int
	ListHeight int

	// Callbacks
	OnCreate func(path string) // Called when folder is created
	OnCancel func()
}

// NewFolderCreator creates a new folder creator
func NewFolderCreator(screen tcell.Screen, onCreate func(path string), onCancel func()) *FolderCreator {
	homeDir, _ := os.UserHomeDir()

	fc := &FolderCreator{
		Screen:     screen,
		OnCreate:   onCreate,
		OnCancel:   onCancel,
		Width:      60,
		Height:     20,
		ListHeight: 10,
		Step:       StepSelectLocation,
	}

	// Start at home directory
	fc.InputPath = "~/"
	fc.CursorPos = len(fc.InputPath)
	fc.CurrentDir = homeDir
	fc.loadDirectory(homeDir)

	return fc
}

// Show activates the folder creator
func (fc *FolderCreator) Show() {
	fc.Active = true
	fc.Step = StepSelectLocation
	fc.FolderName = ""
	fc.FolderNameCursor = 0
	fc.ErrorMessage = ""
	// Reload directory
	expanded := fc.expandTilde(fc.InputPath)
	if info, err := os.Stat(expanded); err == nil && info.IsDir() {
		fc.CurrentDir = expanded
		fc.loadDirectory(expanded)
	}
}

// Hide deactivates the folder creator
func (fc *FolderCreator) Hide() {
	fc.Active = false
}

// expandTilde expands ~ to home directory
func (fc *FolderCreator) expandTilde(path string) string {
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
func (fc *FolderCreator) collapseTilde(path string) string {
	homeDir, _ := os.UserHomeDir()
	if strings.HasPrefix(path, homeDir) {
		return "~" + path[len(homeDir):]
	}
	return path
}

// loadDirectory reads directory contents (folders only)
func (fc *FolderCreator) loadDirectory(path string) {
	fc.Entries = nil
	fc.FilteredList = nil
	fc.SelectedIdx = 0
	fc.TopLine = 0

	entries, err := os.ReadDir(path)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() && !strings.HasPrefix(entry.Name(), ".") {
			fc.Entries = append(fc.Entries, DirEntry{
				Name:     entry.Name(),
				FullPath: filepath.Join(path, entry.Name()),
				IsDir:    true,
			})
		}
	}

	sort.Slice(fc.Entries, func(i, j int) bool {
		return strings.ToLower(fc.Entries[i].Name) < strings.ToLower(fc.Entries[j].Name)
	})

	fc.filterEntries()
}

// filterEntries filters the directory list based on input
func (fc *FolderCreator) filterEntries() {
	fc.FilteredList = nil

	normalizedInput := fc.expandTilde(fc.InputPath)
	normalizedCurrent := fc.CurrentDir

	inputDir := strings.TrimSuffix(normalizedInput, "/")
	currentDir := strings.TrimSuffix(normalizedCurrent, "/")

	if inputDir != currentDir {
		if info, err := os.Stat(inputDir); err == nil && info.IsDir() {
			fc.CurrentDir = inputDir
			fc.loadDirectory(inputDir)
			return
		}
	}

	if !strings.HasSuffix(normalizedInput, "/") {
		normalizedInput += "/"
	}
	if !strings.HasSuffix(normalizedCurrent, "/") {
		normalizedCurrent += "/"
	}

	filter := ""
	if strings.HasPrefix(normalizedInput, normalizedCurrent) {
		filter = strings.ToLower(normalizedInput[len(normalizedCurrent):])
		filter = strings.TrimSuffix(filter, "/")
	}

	for _, entry := range fc.Entries {
		if filter == "" || strings.Contains(strings.ToLower(entry.Name), filter) {
			fc.FilteredList = append(fc.FilteredList, entry)
		}
	}

	if fc.SelectedIdx >= len(fc.FilteredList) {
		fc.SelectedIdx = 0
	}
	fc.TopLine = 0
}

// HandleEvent processes input events
func (fc *FolderCreator) HandleEvent(event tcell.Event) bool {
	if !fc.Active {
		return false
	}

	switch ev := event.(type) {
	case *tcell.EventKey:
		return fc.handleKey(ev)
	case *tcell.EventMouse:
		return fc.handleMouse(ev)
	}
	return false
}

func (fc *FolderCreator) handleKey(ev *tcell.EventKey) bool {
	switch fc.Step {
	case StepSelectLocation:
		return fc.handleKeyLocation(ev)
	case StepEnterName:
		return fc.handleKeyName(ev)
	}
	return false
}

func (fc *FolderCreator) handleKeyLocation(ev *tcell.EventKey) bool {
	switch ev.Key() {
	case tcell.KeyEscape:
		if fc.OnCancel != nil {
			fc.OnCancel()
		}
		return true

	case tcell.KeyEnter:
		// If a folder is selected, use that as the target directory
		if len(fc.FilteredList) > 0 && fc.SelectedIdx < len(fc.FilteredList) {
			selected := fc.FilteredList[fc.SelectedIdx]
			fc.CurrentDir = selected.FullPath
			fc.InputPath = fc.collapseTilde(selected.FullPath) + "/"
			fc.CursorPos = len(fc.InputPath)
		}
		// Move to step 2 - enter folder name
		fc.Step = StepEnterName
		fc.FolderName = ""
		fc.FolderNameCursor = 0
		fc.ErrorMessage = ""
		return true

	case tcell.KeyTab:
		fc.handleTab()
		return true

	case tcell.KeyUp:
		fc.moveSelectionUp()
		return true

	case tcell.KeyDown:
		fc.moveSelectionDown()
		return true

	case tcell.KeyLeft:
		if fc.CursorPos > 0 {
			fc.CursorPos--
		}
		return true

	case tcell.KeyRight:
		if fc.CursorPos < len(fc.InputPath) {
			fc.CursorPos++
		}
		return true

	case tcell.KeyHome:
		fc.CursorPos = 0
		return true

	case tcell.KeyEnd:
		fc.CursorPos = len(fc.InputPath)
		return true

	case tcell.KeyBackspace, tcell.KeyBackspace2:
		fc.handleBackspace()
		return true

	case tcell.KeyDelete:
		if fc.CursorPos < len(fc.InputPath) {
			fc.InputPath = fc.InputPath[:fc.CursorPos] + fc.InputPath[fc.CursorPos+1:]
			fc.filterEntries()
		}
		return true

	case tcell.KeyRune:
		ch := ev.Rune()
		fc.InputPath = fc.InputPath[:fc.CursorPos] + string(ch) + fc.InputPath[fc.CursorPos:]
		fc.CursorPos++
		fc.filterEntries()
		return true
	}

	return false
}

func (fc *FolderCreator) handleKeyName(ev *tcell.EventKey) bool {
	switch ev.Key() {
	case tcell.KeyEscape:
		// Go back to step 1
		fc.Step = StepSelectLocation
		fc.ErrorMessage = ""
		return true

	case tcell.KeyEnter:
		// Create the folder
		if fc.FolderName == "" {
			fc.ErrorMessage = "Folder name cannot be empty"
			return true
		}

		// Validate folder name
		if strings.ContainsAny(fc.FolderName, "/\\:*?\"<>|") {
			fc.ErrorMessage = "Invalid folder name"
			return true
		}

		// Create the folder
		newPath := filepath.Join(fc.CurrentDir, fc.FolderName)
		err := os.MkdirAll(newPath, 0755)
		if err != nil {
			if os.IsExist(err) {
				fc.ErrorMessage = "Folder already exists"
			} else if os.IsPermission(err) {
				fc.ErrorMessage = "Permission denied"
			} else {
				fc.ErrorMessage = "Failed to create folder"
			}
			return true
		}

		// Success - call callback
		if fc.OnCreate != nil {
			fc.OnCreate(newPath)
		}
		return true

	case tcell.KeyLeft:
		if fc.FolderNameCursor > 0 {
			fc.FolderNameCursor--
		}
		return true

	case tcell.KeyRight:
		if fc.FolderNameCursor < len(fc.FolderName) {
			fc.FolderNameCursor++
		}
		return true

	case tcell.KeyHome:
		fc.FolderNameCursor = 0
		return true

	case tcell.KeyEnd:
		fc.FolderNameCursor = len(fc.FolderName)
		return true

	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if fc.FolderNameCursor > 0 {
			fc.FolderName = fc.FolderName[:fc.FolderNameCursor-1] + fc.FolderName[fc.FolderNameCursor:]
			fc.FolderNameCursor--
			fc.ErrorMessage = ""
		}
		return true

	case tcell.KeyDelete:
		if fc.FolderNameCursor < len(fc.FolderName) {
			fc.FolderName = fc.FolderName[:fc.FolderNameCursor] + fc.FolderName[fc.FolderNameCursor+1:]
			fc.ErrorMessage = ""
		}
		return true

	case tcell.KeyRune:
		ch := ev.Rune()
		fc.FolderName = fc.FolderName[:fc.FolderNameCursor] + string(ch) + fc.FolderName[fc.FolderNameCursor:]
		fc.FolderNameCursor++
		fc.ErrorMessage = ""
		return true
	}

	return false
}

func (fc *FolderCreator) handleMouse(ev *tcell.EventMouse) bool {
	if ev.Buttons() != tcell.Button1 {
		return false
	}

	if fc.Step != StepSelectLocation {
		return false
	}

	mouseX, mouseY := ev.Position()
	w, h := fc.Screen.Size()
	x := (w - fc.Width) / 2
	y := (h - fc.Height) / 2

	if mouseX < x || mouseX >= x+fc.Width || mouseY < y || mouseY >= y+fc.Height {
		return false
	}

	localY := mouseY - y
	listStartY := 4
	listEndY := listStartY + fc.ListHeight

	if localY >= listStartY && localY < listEndY {
		itemIndex := fc.TopLine + (localY - listStartY)
		if itemIndex < len(fc.FilteredList) {
			fc.SelectedIdx = itemIndex
			fc.handleTab()
			return true
		}
	}

	return false
}

func (fc *FolderCreator) handleTab() {
	if len(fc.FilteredList) > 0 && fc.SelectedIdx < len(fc.FilteredList) {
		selected := fc.FilteredList[fc.SelectedIdx]
		fc.InputPath = fc.collapseTilde(selected.FullPath) + "/"
		fc.CursorPos = len(fc.InputPath)
		fc.CurrentDir = selected.FullPath
		fc.loadDirectory(selected.FullPath)
		return
	}

	expanded := fc.expandTilde(fc.InputPath)
	if info, err := os.Stat(expanded); err == nil && info.IsDir() {
		if !strings.HasSuffix(fc.InputPath, "/") {
			fc.InputPath += "/"
			fc.CursorPos = len(fc.InputPath)
		}
		fc.CurrentDir = expanded
		fc.loadDirectory(expanded)
		return
	}

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
		completed := filepath.Join(dir, matches[0])
		fc.InputPath = fc.collapseTilde(completed) + "/"
		fc.CursorPos = len(fc.InputPath)
		fc.CurrentDir = completed
		fc.loadDirectory(completed)
	} else if len(matches) > 1 {
		commonPrefix := matches[0]
		for _, m := range matches[1:] {
			commonPrefix = commonPrefixStr(commonPrefix, m)
		}
		if len(commonPrefix) > len(base) {
			completed := filepath.Join(dir, commonPrefix)
			fc.InputPath = fc.collapseTilde(completed)
			fc.CursorPos = len(fc.InputPath)
			fc.CurrentDir = dir
			fc.loadDirectory(dir)
		}
	}
}

func (fc *FolderCreator) handleBackspace() {
	if fc.CursorPos <= 0 {
		return
	}

	if fc.CursorPos > 0 && fc.InputPath[fc.CursorPos-1] == '/' {
		expanded := fc.expandTilde(fc.InputPath[:fc.CursorPos-1])
		parent := filepath.Dir(expanded)
		if parent != expanded {
			fc.InputPath = fc.collapseTilde(parent) + "/"
			fc.CursorPos = len(fc.InputPath)
			fc.CurrentDir = parent
			fc.loadDirectory(parent)
			return
		}
	}

	fc.InputPath = fc.InputPath[:fc.CursorPos-1] + fc.InputPath[fc.CursorPos:]
	fc.CursorPos--
	fc.filterEntries()
}

func (fc *FolderCreator) moveSelectionUp() {
	if len(fc.FilteredList) == 0 {
		return
	}
	fc.SelectedIdx--
	if fc.SelectedIdx < 0 {
		fc.SelectedIdx = len(fc.FilteredList) - 1
	}
	fc.ensureVisible()
}

func (fc *FolderCreator) moveSelectionDown() {
	if len(fc.FilteredList) == 0 {
		return
	}
	fc.SelectedIdx++
	if fc.SelectedIdx >= len(fc.FilteredList) {
		fc.SelectedIdx = 0
	}
	fc.ensureVisible()
}

func (fc *FolderCreator) ensureVisible() {
	if fc.SelectedIdx < fc.TopLine {
		fc.TopLine = fc.SelectedIdx
	}
	if fc.SelectedIdx >= fc.TopLine+fc.ListHeight {
		fc.TopLine = fc.SelectedIdx - fc.ListHeight + 1
	}
}

// Render draws the folder creator
func (fc *FolderCreator) Render(screen tcell.Screen) {
	if !fc.Active {
		return
	}

	w, h := screen.Size()
	x := (w - fc.Width) / 2
	y := (h - fc.Height) / 2

	// Background
	bgStyle := tcell.StyleDefault.Foreground(ColorTextBright).Background(ColorBgDark)
	for dy := 0; dy < fc.Height; dy++ {
		for dx := 0; dx < fc.Width; dx++ {
			screen.SetContent(x+dx, y+dy, ' ', nil, bgStyle)
		}
	}

	// Border
	borderStyle := tcell.StyleDefault.Foreground(ColorCyan).Background(ColorBgDark).Bold(true)

	// Top border with title
	screen.SetContent(x, y, '╔', nil, borderStyle)
	screen.SetContent(x+fc.Width-1, y, '╗', nil, borderStyle)
	for i := 1; i < fc.Width-1; i++ {
		screen.SetContent(x+i, y, '═', nil, borderStyle)
	}

	// Title
	title := " New Folder "
	if fc.Step == StepEnterName {
		title = " Enter Folder Name "
	}
	titleX := x + (fc.Width-len(title))/2
	titleStyle := tcell.StyleDefault.Foreground(ColorCyan).Background(ColorBgDark).Bold(true)
	for i, ch := range title {
		screen.SetContent(titleX+i, y, ch, nil, titleStyle)
	}

	// Separator
	screen.SetContent(x, y+1, '╠', nil, borderStyle)
	screen.SetContent(x+fc.Width-1, y+1, '╣', nil, borderStyle)
	for i := 1; i < fc.Width-1; i++ {
		screen.SetContent(x+i, y+1, '═', nil, borderStyle)
	}

	if fc.Step == StepSelectLocation {
		fc.renderLocationStep(screen, x, y)
	} else {
		fc.renderNameStep(screen, x, y)
	}

	// Left and right borders
	for i := 2; i < fc.Height-3; i++ {
		screen.SetContent(x, y+i, '║', nil, borderStyle)
		screen.SetContent(x+fc.Width-1, y+i, '║', nil, borderStyle)
	}

	// Hints separator
	hintSepY := y + fc.Height - 3
	sepStyle := tcell.StyleDefault.Foreground(ColorCyan).Background(ColorBgDark)
	screen.SetContent(x, hintSepY, '╠', nil, borderStyle)
	screen.SetContent(x+fc.Width-1, hintSepY, '╣', nil, borderStyle)
	for i := 1; i < fc.Width-1; i++ {
		screen.SetContent(x+i, hintSepY, '─', nil, sepStyle)
	}

	// Hints
	hintY := y + fc.Height - 2
	hints := "[Tab] Drill in  [Bksp] Go up  [Enter] Select  [Esc] Cancel"
	if fc.Step == StepEnterName {
		hints = "[Enter] Create  [Esc] Back"
	}
	hintStyle := tcell.StyleDefault.Foreground(ColorTextMuted).Background(ColorBgDark)
	hintX := x + (fc.Width-len(hints))/2
	for i, ch := range hints {
		screen.SetContent(hintX+i, hintY, ch, nil, hintStyle)
	}

	// Bottom border
	screen.SetContent(x, y+fc.Height-1, '╚', nil, borderStyle)
	screen.SetContent(x+fc.Width-1, y+fc.Height-1, '╝', nil, borderStyle)
	for i := 1; i < fc.Width-1; i++ {
		screen.SetContent(x+i, y+fc.Height-1, '═', nil, borderStyle)
	}
}

func (fc *FolderCreator) renderLocationStep(screen tcell.Screen, x, y int) {
	borderStyle := tcell.StyleDefault.Foreground(ColorCyan).Background(ColorBgDark).Bold(true)
	inputStyle := tcell.StyleDefault.Foreground(ColorTextBright).Background(ColorBgDark)
	previewStyle := tcell.StyleDefault.Foreground(ColorTextMuted).Background(ColorBgDark)

	// Input field
	inputY := y + 2
	inputX := x + 2
	inputWidth := fc.Width - 4

	actualPath := fc.InputPath
	previewPath := ""
	if len(fc.FilteredList) > 0 && fc.SelectedIdx < len(fc.FilteredList) {
		if strings.HasSuffix(actualPath, "/") {
			previewPath = fc.FilteredList[fc.SelectedIdx].Name
		} else {
			previewPath = "/" + fc.FilteredList[fc.SelectedIdx].Name
		}
	}
	displayPath := actualPath + previewPath

	inputOffset := 0
	if fc.CursorPos > inputWidth-1 {
		inputOffset = fc.CursorPos - inputWidth + 1
	}

	highlightStart := -1
	highlightEnd := -1
	if fc.CursorPos > 0 && fc.CursorPos <= len(actualPath) && actualPath[fc.CursorPos-1] == '/' {
		segmentEnd := fc.CursorPos - 1
		segmentStart := segmentEnd - 1
		for segmentStart >= 0 && actualPath[segmentStart] != '/' {
			segmentStart--
		}
		segmentStart++
		highlightStart = segmentStart
		highlightEnd = segmentEnd
	}

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

	cursorX := inputX + (fc.CursorPos - inputOffset)
	if cursorX >= inputX && cursorX < inputX+inputWidth {
		cursorStyle := tcell.StyleDefault.Foreground(ColorBgDark).Background(ColorTextBright)
		ch := ' '
		if fc.CursorPos < len(displayPath) {
			ch = rune(displayPath[fc.CursorPos])
		}
		screen.SetContent(cursorX, inputY, ch, nil, cursorStyle)
	}

	// List separator
	listSepY := y + 3
	sepStyle := tcell.StyleDefault.Foreground(ColorCyan).Background(ColorBgDark)
	screen.SetContent(x, listSepY, '╠', nil, borderStyle)
	screen.SetContent(x+fc.Width-1, listSepY, '╣', nil, borderStyle)
	for i := 1; i < fc.Width-1; i++ {
		screen.SetContent(x+i, listSepY, '─', nil, sepStyle)
	}

	// Directory list
	listY := y + 4
	listStyle := tcell.StyleDefault.Foreground(ColorTextDim).Background(ColorBgDark)
	selectedStyle := tcell.StyleDefault.Foreground(ColorBgDark).Background(ColorYellow).Bold(true)

	for i := 0; i < fc.ListHeight; i++ {
		entryIdx := fc.TopLine + i
		lineY := listY + i

		if entryIdx < len(fc.FilteredList) {
			entry := fc.FilteredList[entryIdx]
			style := listStyle
			prefix := "   "

			if entryIdx == fc.SelectedIdx {
				style = selectedStyle
				prefix = " > "
			}

			for j, ch := range prefix {
				screen.SetContent(x+1+j, lineY, ch, nil, style)
			}

			displayName := entry.Name + "/"
			if len(displayName) > fc.Width-8 {
				displayName = displayName[:fc.Width-11] + "..."
			}

			for j, ch := range displayName {
				if x+4+j < x+fc.Width-1 {
					screen.SetContent(x+4+j, lineY, ch, nil, style)
				}
			}

			if entryIdx == fc.SelectedIdx {
				for j := 4 + len(displayName); j < fc.Width-1; j++ {
					screen.SetContent(x+j, lineY, ' ', nil, style)
				}
			}
		}
	}
}

func (fc *FolderCreator) renderNameStep(screen tcell.Screen, x, y int) {
	borderStyle := tcell.StyleDefault.Foreground(ColorCyan).Background(ColorBgDark).Bold(true)
	inputStyle := tcell.StyleDefault.Foreground(ColorTextBright).Background(ColorBgDark)
	sepStyle := tcell.StyleDefault.Foreground(ColorCyan).Background(ColorBgDark)

	// Show selected location
	locationY := y + 2
	locationLabel := "Location: "
	locationStyle := tcell.StyleDefault.Foreground(ColorTextMuted).Background(ColorBgDark)
	for i, ch := range locationLabel {
		screen.SetContent(x+2+i, locationY, ch, nil, locationStyle)
	}

	locationPath := fc.collapseTilde(fc.CurrentDir)
	if len(locationPath) > fc.Width-15 {
		locationPath = "..." + locationPath[len(locationPath)-(fc.Width-18):]
	}
	for i, ch := range locationPath {
		screen.SetContent(x+2+len(locationLabel)+i, locationY, ch, nil, inputStyle)
	}

	// Separator
	sepY := y + 3
	screen.SetContent(x, sepY, '╠', nil, borderStyle)
	screen.SetContent(x+fc.Width-1, sepY, '╣', nil, borderStyle)
	for i := 1; i < fc.Width-1; i++ {
		screen.SetContent(x+i, sepY, '─', nil, sepStyle)
	}

	// Folder name label
	labelY := y + 5
	label := "Folder name:"
	labelStyle := tcell.StyleDefault.Foreground(ColorTextMuted).Background(ColorBgDark)
	for i, ch := range label {
		screen.SetContent(x+2+i, labelY, ch, nil, labelStyle)
	}

	// Folder name input
	inputY := y + 7
	inputX := x + 2
	inputWidth := fc.Width - 4

	for i := 0; i < inputWidth; i++ {
		if i < len(fc.FolderName) {
			screen.SetContent(inputX+i, inputY, rune(fc.FolderName[i]), nil, inputStyle)
		} else {
			screen.SetContent(inputX+i, inputY, ' ', nil, inputStyle)
		}
	}

	// Cursor
	cursorX := inputX + fc.FolderNameCursor
	if cursorX >= inputX && cursorX < inputX+inputWidth {
		cursorStyle := tcell.StyleDefault.Foreground(ColorBgDark).Background(ColorTextBright)
		ch := ' '
		if fc.FolderNameCursor < len(fc.FolderName) {
			ch = rune(fc.FolderName[fc.FolderNameCursor])
		}
		screen.SetContent(cursorX, inputY, ch, nil, cursorStyle)
	}

	// Error message
	if fc.ErrorMessage != "" {
		errorY := y + 9
		errorStyle := tcell.StyleDefault.Foreground(tcell.ColorRed).Background(ColorBgDark)
		for i, ch := range fc.ErrorMessage {
			if i < fc.Width-4 {
				screen.SetContent(x+2+i, errorY, ch, nil, errorStyle)
			}
		}
	}
}
