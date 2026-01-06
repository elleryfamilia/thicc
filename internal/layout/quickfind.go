package layout

import (
	"path/filepath"
	"strings"

	"github.com/ellery/thicc/internal/filemanager"
	"github.com/micro-editor/tcell/v2"
)

// QuickFindPicker is a modal for quick file finding (Cmd+P / Ctrl+P)
type QuickFindPicker struct {
	Active  bool
	Screen  tcell.Screen
	ScreenW int
	ScreenH int

	// Input field
	Query     string
	CursorPos int

	// Results
	Index       *filemanager.FileIndex
	Results     []filemanager.SearchResult
	SelectedIdx int
	TopLine     int

	// Dimensions
	Width      int
	Height     int
	ListHeight int

	// Callbacks
	OnSelect func(path string)
	OnCancel func()
}

// NewQuickFindPicker creates a new quick find picker
func NewQuickFindPicker(screen tcell.Screen, index *filemanager.FileIndex, onSelect func(path string), onCancel func()) *QuickFindPicker {
	return &QuickFindPicker{
		Screen:     screen,
		Index:      index,
		OnSelect:   onSelect,
		OnCancel:   onCancel,
		Width:      70,
		Height:     18,
		ListHeight: 10,
	}
}

// Show activates the picker
func (p *QuickFindPicker) Show() {
	p.Active = true
	p.Query = ""
	p.CursorPos = 0
	p.SelectedIdx = 0
	p.TopLine = 0
	p.updateResults()
}

// Hide deactivates the picker
func (p *QuickFindPicker) Hide() {
	p.Active = false
	p.Query = ""
	p.Results = nil
}

// HandleEvent processes input events
func (p *QuickFindPicker) HandleEvent(event tcell.Event) bool {
	if !p.Active {
		return false
	}

	switch ev := event.(type) {
	case *tcell.EventKey:
		return p.handleKey(ev)
	case *tcell.EventMouse:
		return p.handleMouse(ev)
	}

	return true // Consume all events while active
}

func (p *QuickFindPicker) handleKey(ev *tcell.EventKey) bool {
	switch ev.Key() {
	case tcell.KeyEscape:
		if p.OnCancel != nil {
			p.OnCancel()
		}
		p.Hide()
		return true

	case tcell.KeyEnter:
		if len(p.Results) > 0 && p.SelectedIdx < len(p.Results) {
			if p.OnSelect != nil {
				p.OnSelect(p.Results[p.SelectedIdx].File.Path)
			}
			p.Hide()
		}
		return true

	case tcell.KeyUp:
		if p.SelectedIdx > 0 {
			p.SelectedIdx--
			p.ensureVisible()
		}
		return true

	case tcell.KeyDown:
		if p.SelectedIdx < len(p.Results)-1 {
			p.SelectedIdx++
			p.ensureVisible()
		}
		return true

	case tcell.KeyPgUp:
		p.SelectedIdx -= p.ListHeight
		if p.SelectedIdx < 0 {
			p.SelectedIdx = 0
		}
		p.ensureVisible()
		return true

	case tcell.KeyPgDn:
		p.SelectedIdx += p.ListHeight
		if p.SelectedIdx >= len(p.Results) {
			p.SelectedIdx = len(p.Results) - 1
		}
		if p.SelectedIdx < 0 {
			p.SelectedIdx = 0
		}
		p.ensureVisible()
		return true

	case tcell.KeyBackspace, tcell.KeyBackspace2:
		if p.CursorPos > 0 {
			p.Query = p.Query[:p.CursorPos-1] + p.Query[p.CursorPos:]
			p.CursorPos--
			p.SelectedIdx = 0
			p.TopLine = 0
			p.updateResults()
		}
		return true

	case tcell.KeyDelete:
		if p.CursorPos < len(p.Query) {
			p.Query = p.Query[:p.CursorPos] + p.Query[p.CursorPos+1:]
			p.SelectedIdx = 0
			p.TopLine = 0
			p.updateResults()
		}
		return true

	case tcell.KeyLeft:
		if p.CursorPos > 0 {
			p.CursorPos--
		}
		return true

	case tcell.KeyRight:
		if p.CursorPos < len(p.Query) {
			p.CursorPos++
		}
		return true

	case tcell.KeyHome, tcell.KeyCtrlA:
		p.CursorPos = 0
		return true

	case tcell.KeyEnd, tcell.KeyCtrlE:
		p.CursorPos = len(p.Query)
		return true

	case tcell.KeyCtrlU:
		// Clear input
		p.Query = ""
		p.CursorPos = 0
		p.SelectedIdx = 0
		p.TopLine = 0
		p.updateResults()
		return true

	case tcell.KeyRune:
		// Insert character at cursor
		p.Query = p.Query[:p.CursorPos] + string(ev.Rune()) + p.Query[p.CursorPos:]
		p.CursorPos++
		p.SelectedIdx = 0
		p.TopLine = 0
		p.updateResults()
		return true
	}

	return true
}

func (p *QuickFindPicker) handleMouse(ev *tcell.EventMouse) bool {
	x, y := ev.Position()
	w, h := p.Screen.Size()

	// Calculate modal position
	modalX := (w - p.Width) / 2
	modalY := (h - p.Height) / 2

	// Check if click is within modal
	if x < modalX || x >= modalX+p.Width || y < modalY || y >= modalY+p.Height {
		// Click outside - cancel
		if ev.Buttons() == tcell.Button1 {
			if p.OnCancel != nil {
				p.OnCancel()
			}
			p.Hide()
		}
		return true
	}

	// Check if click is in the list area
	listY := modalY + 4 // List starts after title, separator, input, separator
	if y >= listY && y < listY+p.ListHeight && ev.Buttons() == tcell.Button1 {
		clickedIdx := p.TopLine + (y - listY)
		if clickedIdx < len(p.Results) {
			p.SelectedIdx = clickedIdx
			// Double-click opens (simulate by immediate selection)
			if p.OnSelect != nil {
				p.OnSelect(p.Results[p.SelectedIdx].File.Path)
			}
			p.Hide()
		}
	}

	return true
}

func (p *QuickFindPicker) updateResults() {
	if p.Index == nil {
		return
	}
	p.Results = p.Index.Search(p.Query, 100)
}

func (p *QuickFindPicker) ensureVisible() {
	if p.SelectedIdx < p.TopLine {
		p.TopLine = p.SelectedIdx
	}
	if p.SelectedIdx >= p.TopLine+p.ListHeight {
		p.TopLine = p.SelectedIdx - p.ListHeight + 1
	}
}

// Render draws the quick find picker
func (p *QuickFindPicker) Render(screen tcell.Screen) {
	if !p.Active {
		return
	}

	w, h := screen.Size()

	// Calculate position (centered)
	x := (w - p.Width) / 2
	y := (h - p.Height) / 2

	// Colors (explicit fg AND bg to prevent issues in light mode)
	bgColor := tcell.ColorBlack
	borderColor := tcell.Color51   // Cyan
	titleColor := tcell.Color51    // Cyan
	textColor := tcell.ColorWhite
	dimColor := tcell.Color245     // Gray
	highlightColor := tcell.Color226 // Yellow
	matchColor := tcell.Color205   // Hot pink for matched chars

	bgStyle := tcell.StyleDefault.Foreground(textColor).Background(bgColor)
	borderStyle := tcell.StyleDefault.Foreground(borderColor).Background(bgColor).Bold(true)
	titleStyle := tcell.StyleDefault.Foreground(titleColor).Background(bgColor).Bold(true)
	inputStyle := tcell.StyleDefault.Foreground(textColor).Background(bgColor)
	cursorStyle := tcell.StyleDefault.Foreground(bgColor).Background(textColor)
	listStyle := tcell.StyleDefault.Foreground(dimColor).Background(bgColor)
	selectedStyle := tcell.StyleDefault.Foreground(bgColor).Background(highlightColor).Bold(true)
	pathStyle := tcell.StyleDefault.Foreground(dimColor).Background(bgColor)
	pathSelectedStyle := tcell.StyleDefault.Foreground(bgColor).Background(highlightColor)
	matchStyle := tcell.StyleDefault.Foreground(matchColor).Background(bgColor).Bold(true)
	hintStyle := tcell.StyleDefault.Foreground(dimColor).Background(bgColor)
	indexingStyle := tcell.StyleDefault.Foreground(highlightColor).Background(bgColor)

	// Draw background
	for dy := 0; dy < p.Height; dy++ {
		for dx := 0; dx < p.Width; dx++ {
			screen.SetContent(x+dx, y+dy, ' ', nil, bgStyle)
		}
	}

	// Draw border
	// Top border with title
	screen.SetContent(x, y, '╔', nil, borderStyle)
	screen.SetContent(x+p.Width-1, y, '╗', nil, borderStyle)
	for i := 1; i < p.Width-1; i++ {
		screen.SetContent(x+i, y, '═', nil, borderStyle)
	}

	// Title
	title := " Quick Find "
	titleX := x + (p.Width-len(title))/2
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
	inputX := x + 2
	inputWidth := p.Width - 4

	// Draw search icon
	searchIcon := "> "
	for i, ch := range searchIcon {
		screen.SetContent(inputX+i, inputY, ch, nil, inputStyle)
	}
	inputX += len(searchIcon)
	inputWidth -= len(searchIcon)

	// Draw input with cursor
	for i := 0; i < inputWidth; i++ {
		if i < len(p.Query) {
			screen.SetContent(inputX+i, inputY, rune(p.Query[i]), nil, inputStyle)
		} else {
			screen.SetContent(inputX+i, inputY, ' ', nil, inputStyle)
		}
	}

	// Draw cursor
	cursorX := inputX + p.CursorPos
	if cursorX < inputX+inputWidth {
		ch := ' '
		if p.CursorPos < len(p.Query) {
			ch = rune(p.Query[p.CursorPos])
		}
		screen.SetContent(cursorX, inputY, ch, nil, cursorStyle)
	}

	// Separator before list
	listSepY := y + 3
	screen.SetContent(x, listSepY, '╠', nil, borderStyle)
	screen.SetContent(x+p.Width-1, listSepY, '╣', nil, borderStyle)
	for i := 1; i < p.Width-1; i++ {
		screen.SetContent(x+i, listSepY, '─', nil, borderStyle)
	}

	// Show indexing status if not ready
	listY := y + 4
	if p.Index == nil || !p.Index.IsReady() {
		msg := "Indexing files..."
		msgX := x + (p.Width-len(msg))/2
		for i, ch := range msg {
			screen.SetContent(msgX+i, listY+p.ListHeight/2, ch, nil, indexingStyle)
		}
	} else if len(p.Results) == 0 && p.Query != "" {
		msg := "No matching files"
		msgX := x + (p.Width-len(msg))/2
		for i, ch := range msg {
			screen.SetContent(msgX+i, listY+p.ListHeight/2, ch, nil, listStyle)
		}
	} else {
		// Draw results list
		for i := 0; i < p.ListHeight; i++ {
			entryIdx := p.TopLine + i
			lineY := listY + i

			if entryIdx < len(p.Results) {
				result := p.Results[entryIdx]
				isSelected := entryIdx == p.SelectedIdx

				// Determine styles based on selection
				nameStyle := listStyle
				relPathStyle := pathStyle
				if isSelected {
					nameStyle = selectedStyle
					relPathStyle = pathSelectedStyle
				}

				// Draw selection indicator
				prefix := "   "
				if isSelected {
					prefix = " > "
				}
				for j, ch := range prefix {
					screen.SetContent(x+1+j, lineY, ch, nil, nameStyle)
				}

				// Draw filename with match highlighting
				nameX := x + 4
				name := result.File.Name
				maxNameLen := p.Width/2 - 6

				// Create a set of matched indices for quick lookup
				matchedSet := make(map[int]bool)
				for _, idx := range result.MatchedIdx {
					matchedSet[idx] = true
				}

				// Draw name with highlighting
				for j, ch := range name {
					if j >= maxNameLen {
						// Truncate with ellipsis
						screen.SetContent(nameX+j, lineY, '.', nil, nameStyle)
						screen.SetContent(nameX+j+1, lineY, '.', nil, nameStyle)
						screen.SetContent(nameX+j+2, lineY, '.', nil, nameStyle)
						break
					}

					style := nameStyle
					if matchedSet[j] && !isSelected {
						style = matchStyle
					}
					screen.SetContent(nameX+j, lineY, ch, nil, style)
				}

				// Draw relative path (right-aligned)
				relPath := result.File.RelPath
				// Remove filename from relpath to show just directory
				dir := filepath.Dir(relPath)
				if dir == "." {
					dir = ""
				}

				pathX := x + p.Width - 2 - len(dir)
				if pathX < x+p.Width/2 {
					// Truncate path from the left
					maxPathLen := p.Width/2 - 4
					if len(dir) > maxPathLen {
						dir = "..." + dir[len(dir)-maxPathLen+3:]
					}
					pathX = x + p.Width - 2 - len(dir)
				}

				for j, ch := range dir {
					screen.SetContent(pathX+j, lineY, ch, nil, relPathStyle)
				}

				// Fill rest of selected line
				if isSelected {
					nameEndX := nameX + min(len(name), maxNameLen)
					if len(name) > maxNameLen {
						nameEndX += 3 // For ellipsis
					}
					for j := nameEndX; j < pathX; j++ {
						screen.SetContent(j, lineY, ' ', nil, selectedStyle)
					}
				}
			}
		}
	}

	// Left and right borders for content area
	for i := 2; i < p.Height-2; i++ {
		screen.SetContent(x, y+i, '║', nil, borderStyle)
		screen.SetContent(x+p.Width-1, y+i, '║', nil, borderStyle)
	}

	// Separator before hints
	hintSepY := y + p.Height - 3
	screen.SetContent(x, hintSepY, '╠', nil, borderStyle)
	screen.SetContent(x+p.Width-1, hintSepY, '╣', nil, borderStyle)
	for i := 1; i < p.Width-1; i++ {
		screen.SetContent(x+i, hintSepY, '─', nil, borderStyle)
	}

	// Draw hints
	hintY := y + p.Height - 2
	hints := "[Enter] Open  [Esc] Cancel"
	if p.Index != nil && p.Index.IsReady() {
		count := p.Index.Count()
		hints = strings.Replace(hints, "[Enter]", "[Enter]", 1)
		if count > 0 {
			countStr := " │ " + formatCount(count) + " files"
			hints = hints + countStr
		}
	}
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

func formatCount(n int) string {
	if n >= 1000 {
		return strings.TrimRight(strings.TrimRight(
			strings.Replace(
				strings.Replace(
					string(rune('0'+n/1000))+"."+string(rune('0'+(n%1000)/100))+"k",
					".0k", "k", 1),
				"k", "k", 1),
			"0"), ".")
	}
	// Simple int to string without fmt
	if n == 0 {
		return "0"
	}
	result := ""
	for n > 0 {
		result = string(rune('0'+n%10)) + result
		n /= 10
	}
	return result
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
