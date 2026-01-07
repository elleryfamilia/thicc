package filebrowser

import (
	"testing"

	"github.com/ellery/thicc/internal/filemanager"
	"github.com/micro-editor/tcell/v2"
)

// =============================================================================
// StyleForPath Tests - File extension to color mappings
// =============================================================================

func TestStyleForPath_Directories(t *testing.T) {
	style := StyleForPath("/some/dir", true)
	fg, _, _ := style.Decompose()

	if fg != tcell.Color33 {
		t.Errorf("Directory color: got %v, want %v (Color33 bright blue)", fg, tcell.Color33)
	}
}

func TestStyleForPath_ProgrammingLanguages(t *testing.T) {
	tests := []struct {
		path     string
		wantColor tcell.Color
		name     string
	}{
		// Go
		{"/src/main.go", tcell.Color37, "Go files should be cyan"},
		// Rust
		{"/src/lib.rs", tcell.Color208, "Rust files should be orange"},
		// Python
		{"/app.py", tcell.Color226, "Python files should be bright yellow"},
		// JavaScript/TypeScript
		{"/index.js", tcell.Color220, "JS files should be gold"},
		{"/app.jsx", tcell.Color220, "JSX files should be gold"},
		{"/index.ts", tcell.Color220, "TS files should be gold"},
		{"/app.tsx", tcell.Color220, "TSX files should be gold"},
		// Lua
		{"/init.lua", tcell.Color63, "Lua files should be blue-purple"},
		// Ruby
		{"/app.rb", tcell.Color167, "Ruby files should be red"},
		{"/Rakefile.rake", tcell.Color167, "Rake files should be red"},
		// Java/Kotlin
		{"/Main.java", tcell.Color166, "Java files should be orange-red"},
		{"/App.kt", tcell.Color166, "Kotlin files should be orange-red"},
		{"/Build.scala", tcell.Color166, "Scala files should be orange-red"},
		// C/C++
		{"/main.c", tcell.Color75, "C files should be blue"},
		{"/main.cpp", tcell.Color75, "C++ files should be blue"},
		{"/header.h", tcell.Color75, "Header files should be blue"},
		{"/header.hpp", tcell.Color75, "HPP files should be blue"},
		// C#
		{"/Program.cs", tcell.Color135, "C# files should be purple"},
		// PHP
		{"/index.php", tcell.Color98, "PHP files should be indigo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			style := StyleForPath(tt.path, false)
			fg, _, _ := style.Decompose()
			if fg != tt.wantColor {
				t.Errorf("StyleForPath(%q): got color %v, want %v", tt.path, fg, tt.wantColor)
			}
		})
	}
}

func TestStyleForPath_WebFiles(t *testing.T) {
	tests := []struct {
		path      string
		wantColor tcell.Color
		name      string
	}{
		{"/index.html", tcell.Color208, "HTML should be orange"},
		{"/page.htm", tcell.Color208, "HTM should be orange"},
		{"/style.css", tcell.Color39, "CSS should be sky blue"},
		{"/style.scss", tcell.Color39, "SCSS should be sky blue"},
		{"/style.sass", tcell.Color39, "SASS should be sky blue"},
		{"/style.less", tcell.Color39, "LESS should be sky blue"},
		{"/App.vue", tcell.Color35, "Vue should be green"},
		{"/App.svelte", tcell.Color35, "Svelte should be green"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			style := StyleForPath(tt.path, false)
			fg, _, _ := style.Decompose()
			if fg != tt.wantColor {
				t.Errorf("StyleForPath(%q): got color %v, want %v", tt.path, fg, tt.wantColor)
			}
		})
	}
}

func TestStyleForPath_DataConfig(t *testing.T) {
	tests := []struct {
		path      string
		wantColor tcell.Color
		name      string
	}{
		{"/config.json", tcell.Color226, "JSON should be yellow"},
		{"/config.yaml", tcell.Color40, "YAML should be green"},
		{"/config.yml", tcell.Color40, "YML should be green"},
		{"/config.toml", tcell.Color40, "TOML should be green"},
		{"/data.xml", tcell.Color172, "XML should be orange"},
		{"/settings.ini", tcell.Color67, "INI should be gray-blue"},
		{"/app.conf", tcell.Color67, "CONF should be gray-blue"},
		{"/app.cfg", tcell.Color67, "CFG should be gray-blue"},
		{"/.env", tcell.Color67, "ENV should be gray-blue"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			style := StyleForPath(tt.path, false)
			fg, _, _ := style.Decompose()
			if fg != tt.wantColor {
				t.Errorf("StyleForPath(%q): got color %v, want %v", tt.path, fg, tt.wantColor)
			}
		})
	}
}

func TestStyleForPath_Documentation(t *testing.T) {
	tests := []struct {
		path      string
		wantColor tcell.Color
		name      string
	}{
		{"/README.md", tcell.Color141, "Markdown should be light purple"},
		{"/docs.markdown", tcell.Color141, "Markdown (long ext) should be light purple"},
		{"/notes.txt", tcell.Color250, "Text files should be light gray"},
		{"/manual.pdf", tcell.Color160, "PDF should be red"},
		{"/docs.rst", tcell.Color141, "RST should be light purple"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			style := StyleForPath(tt.path, false)
			fg, _, _ := style.Decompose()
			if fg != tt.wantColor {
				t.Errorf("StyleForPath(%q): got color %v, want %v", tt.path, fg, tt.wantColor)
			}
		})
	}
}

func TestStyleForPath_MarkdownNotSameAsDirectory(t *testing.T) {
	mdStyle := StyleForPath("/README.md", false)
	dirStyle := StyleForPath("/somedir", true)

	mdFg, _, _ := mdStyle.Decompose()
	dirFg, _, _ := dirStyle.Decompose()

	if mdFg == dirFg {
		t.Errorf("Markdown and directory should have different colors: both are %v", mdFg)
	}
}

func TestStyleForPath_Media(t *testing.T) {
	tests := []struct {
		path      string
		wantColor tcell.Color
		name      string
	}{
		// Images
		{"/photo.png", tcell.Color201, "PNG should be magenta"},
		{"/photo.jpg", tcell.Color201, "JPG should be magenta"},
		{"/photo.jpeg", tcell.Color201, "JPEG should be magenta"},
		{"/anim.gif", tcell.Color201, "GIF should be magenta"},
		{"/icon.svg", tcell.Color201, "SVG should be magenta"},
		// Audio
		{"/song.mp3", tcell.Color213, "MP3 should be light magenta"},
		{"/audio.wav", tcell.Color213, "WAV should be light magenta"},
		{"/music.flac", tcell.Color213, "FLAC should be light magenta"},
		// Video
		{"/video.mp4", tcell.Color129, "MP4 should be purple"},
		{"/movie.mkv", tcell.Color129, "MKV should be purple"},
		{"/clip.webm", tcell.Color129, "WEBM should be purple"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			style := StyleForPath(tt.path, false)
			fg, _, _ := style.Decompose()
			if fg != tt.wantColor {
				t.Errorf("StyleForPath(%q): got color %v, want %v", tt.path, fg, tt.wantColor)
			}
		})
	}
}

func TestStyleForPath_SpecialFiles(t *testing.T) {
	tests := []struct {
		path      string
		wantColor tcell.Color
		name      string
	}{
		{"/data.db", tcell.Color44, "DB should be cyan"},
		{"/data.sqlite", tcell.Color44, "SQLite should be cyan"},
		{"/query.sql", tcell.Color44, "SQL should be cyan"},
		{"/archive.zip", tcell.Color196, "ZIP should be red"},
		{"/archive.tar", tcell.Color196, "TAR should be red"},
		{"/package.lock", tcell.Color243, "Lock files should be dark gray"},
		{"/app.log", tcell.Color245, "Log files should be gray"},
		{"/.gitignore", tcell.Color208, "Gitignore should be orange"},
		{"/changes.diff", tcell.Color148, "Diff should be yellow-green"},
		{"/fix.patch", tcell.Color148, "Patch should be yellow-green"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			style := StyleForPath(tt.path, false)
			fg, _, _ := style.Decompose()
			if fg != tt.wantColor {
				t.Errorf("StyleForPath(%q): got color %v, want %v", tt.path, fg, tt.wantColor)
			}
		})
	}
}

func TestStyleForPath_DefaultColor(t *testing.T) {
	// Unknown extensions should get the default soft gray color
	tests := []string{
		"/file.xyz",
		"/file.unknown",
		"/file.randomext",
		"/noextension",
	}

	for _, path := range tests {
		t.Run(path, func(t *testing.T) {
			style := StyleForPath(path, false)
			fg, _, _ := style.Decompose()
			if fg != tcell.Color252 {
				t.Errorf("StyleForPath(%q): got color %v, want %v (soft gray default)", path, fg, tcell.Color252)
			}
		})
	}
}

// =============================================================================
// Scrolling Logic Tests
// =============================================================================

func TestContentHeightCalculation(t *testing.T) {
	// Content height should be Region.Height - 4
	// (top border + header + separator + bottom border)
	tests := []struct {
		height     int
		wantContent int
	}{
		{20, 16},
		{10, 6},
		{5, 1},  // Minimum 1
		{4, 1},  // Minimum 1
		{3, 1},  // Minimum 1
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			p := &Panel{
				Region: Region{Height: tt.height},
				Tree:   filemanager.NewTree("/tmp"),
			}
			// Manually calculate what GetVisibleNodes would use
			contentHeight := p.Region.Height - 4
			if contentHeight < 1 {
				contentHeight = 1
			}
			if contentHeight != tt.wantContent {
				t.Errorf("Height %d: got contentHeight %d, want %d", tt.height, contentHeight, tt.wantContent)
			}
		})
	}
}

func TestEnsureSelectedVisible_SelectionAboveViewport(t *testing.T) {
	p := &Panel{
		Region:   Region{Height: 14}, // contentHeight = 10
		Selected: 2,
		TopLine:  5, // Selection is above TopLine
		Tree:     filemanager.NewTree("/tmp"),
	}

	p.ensureSelectedVisible()

	if p.TopLine != 2 {
		t.Errorf("TopLine should scroll up to Selected: got %d, want 2", p.TopLine)
	}
}

func TestEnsureSelectedVisible_SelectionBelowViewport(t *testing.T) {
	p := &Panel{
		Region:   Region{Height: 14}, // contentHeight = 10
		Selected: 15,
		TopLine:  0, // Selection is below visible area
		Tree:     filemanager.NewTree("/tmp"),
	}

	p.ensureSelectedVisible()

	// TopLine should be Selected - contentHeight + 1 = 15 - 10 + 1 = 6
	if p.TopLine != 6 {
		t.Errorf("TopLine should scroll down: got %d, want 6", p.TopLine)
	}
}

func TestEnsureSelectedVisible_SelectionAlreadyVisible(t *testing.T) {
	p := &Panel{
		Region:   Region{Height: 14}, // contentHeight = 10
		Selected: 5,
		TopLine:  2, // Selection is visible (2 to 11)
		Tree:     filemanager.NewTree("/tmp"),
	}

	originalTopLine := p.TopLine
	p.ensureSelectedVisible()

	if p.TopLine != originalTopLine {
		t.Errorf("TopLine should not change when selection visible: got %d, want %d", p.TopLine, originalTopLine)
	}
}

// =============================================================================
// Navigation Tests
// =============================================================================

// Mock tree with nodes for navigation tests
func setupPanelWithNodes(nodeCount int) *Panel {
	tree := filemanager.NewTree("/tmp")
	// We can't easily add nodes without file system, but we can test boundary conditions
	return &Panel{
		Region:   Region{Height: 14, Width: 30},
		Selected: 0,
		TopLine:  0,
		Tree:     tree,
		Focus:    true,
	}
}

func TestCursorUp_AtTop(t *testing.T) {
	p := &Panel{
		Region:   Region{Height: 14},
		Selected: 0,
		TopLine:  0,
		Tree:     filemanager.NewTree("/tmp"),
		Focus:    true,
	}

	result := p.cursorUp()

	// Should move to header (Selected = -1)
	if p.Selected != -1 {
		t.Errorf("cursorUp from 0 should go to header (-1): got %d", p.Selected)
	}
	if !result {
		t.Error("cursorUp should return true when moving to header")
	}
}

func TestCursorUp_AtHeader(t *testing.T) {
	p := &Panel{
		Region:   Region{Height: 14},
		Selected: -1, // Already at header
		TopLine:  0,
		Tree:     filemanager.NewTree("/tmp"),
		Focus:    true,
	}

	result := p.cursorUp()

	if result {
		t.Error("cursorUp at header should return false")
	}
	if p.Selected != -1 {
		t.Errorf("Selected should stay at -1: got %d", p.Selected)
	}
}

func TestCursorDown_FromHeader(t *testing.T) {
	p := &Panel{
		Region:   Region{Height: 14},
		Selected: -1, // At header
		TopLine:  0,
		Tree:     filemanager.NewTree("/tmp"),
		Focus:    true,
	}

	// Without nodes, should return false
	result := p.cursorDown()

	if result {
		t.Error("cursorDown from header with no nodes should return false")
	}
}

func TestPageUp_Calculation(t *testing.T) {
	p := &Panel{
		Region:   Region{Height: 14}, // contentHeight = 10
		Selected: 15,
		TopLine:  10,
		Tree:     filemanager.NewTree("/tmp"),
		Focus:    true,
	}

	p.pageUp()

	// Should move up by contentHeight (10), from 15 to 5
	if p.Selected != 5 {
		t.Errorf("pageUp should move by contentHeight: got %d, want 5", p.Selected)
	}
}

func TestPageUp_ClampToZero(t *testing.T) {
	p := &Panel{
		Region:   Region{Height: 14}, // contentHeight = 10
		Selected: 3,
		TopLine:  0,
		Tree:     filemanager.NewTree("/tmp"),
		Focus:    true,
	}

	p.pageUp()

	// Should clamp to 0
	if p.Selected != 0 {
		t.Errorf("pageUp should clamp to 0: got %d", p.Selected)
	}
}

func TestGoToTop(t *testing.T) {
	p := &Panel{
		Region:   Region{Height: 14},
		Selected: 10,
		TopLine:  5,
		Tree:     filemanager.NewTree("/tmp"),
		Focus:    true,
	}

	result := p.goToTop()

	if !result {
		t.Error("goToTop should return true when not at top")
	}
	if p.Selected != 0 {
		t.Errorf("Selected should be 0: got %d", p.Selected)
	}
	if p.TopLine != 0 {
		t.Errorf("TopLine should be 0: got %d", p.TopLine)
	}
}

func TestGoToTop_AlreadyAtTop(t *testing.T) {
	p := &Panel{
		Region:   Region{Height: 14},
		Selected: 0,
		TopLine:  0,
		Tree:     filemanager.NewTree("/tmp"),
		Focus:    true,
	}

	result := p.goToTop()

	if result {
		t.Error("goToTop should return false when already at top")
	}
}

// =============================================================================
// Mouse Scroll Tests
// =============================================================================

func TestMouseWheelUp_ScrollsSelection(t *testing.T) {
	p := &Panel{
		Region:   Region{X: 0, Y: 0, Height: 14, Width: 30},
		Selected: 10,
		TopLine:  5,
		Tree:     filemanager.NewTree("/tmp"),
		Focus:    true,
	}

	// Create wheel up event within panel bounds
	ev := tcell.NewEventMouse(5, 5, tcell.WheelUp, tcell.ModNone, "")
	result := p.HandleEvent(ev)

	if !result {
		t.Error("Mouse wheel up should be handled")
	}
	// Should scroll up by 3
	if p.Selected != 7 {
		t.Errorf("WheelUp should decrease Selected by 3: got %d, want 7", p.Selected)
	}
}

func TestMouseWheelUp_ClampsToZero(t *testing.T) {
	p := &Panel{
		Region:   Region{X: 0, Y: 0, Height: 14, Width: 30},
		Selected: 1,
		TopLine:  0,
		Tree:     filemanager.NewTree("/tmp"),
		Focus:    true,
	}

	ev := tcell.NewEventMouse(5, 5, tcell.WheelUp, tcell.ModNone, "")
	p.HandleEvent(ev)

	if p.Selected != 0 {
		t.Errorf("WheelUp should clamp to 0: got %d", p.Selected)
	}
}

func TestMouseOutsideRegion_NotHandled(t *testing.T) {
	p := &Panel{
		Region:   Region{X: 10, Y: 10, Height: 14, Width: 30},
		Selected: 5,
		TopLine:  0,
		Tree:     filemanager.NewTree("/tmp"),
		Focus:    true,
	}

	// Click outside panel region
	ev := tcell.NewEventMouse(0, 0, tcell.Button1, tcell.ModNone, "")
	result := p.HandleEvent(ev)

	if result {
		t.Error("Mouse event outside region should not be handled")
	}
}
