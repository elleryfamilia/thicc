package filebrowser

import (
	"github.com/micro-editor/tcell/v2"
)

// Region defines a rectangular screen region
type Region struct {
	X      int
	Y      int
	Width  int
	Height int
}

// Contains checks if a point is within this region
func (r Region) Contains(x, y int) bool {
	return x >= r.X && x < r.X+r.Width &&
		y >= r.Y && y < r.Y+r.Height
}

// DefaultStyle is the default style for the file browser
var DefaultStyle = tcell.StyleDefault.
	Foreground(tcell.ColorWhite).
	Background(tcell.ColorBlack)

// FocusedStyle is the style for focused items
var FocusedStyle = tcell.StyleDefault.
	Foreground(tcell.ColorBlack).
	Background(tcell.ColorWhite)

// DirectoryStyle is the style for directories
// Using 256-color palette (Color33 = bright blue) for consistent rendering across terminals
var DirectoryStyle = tcell.StyleDefault.
	Foreground(tcell.Color33).
	Background(tcell.ColorBlack).
	Bold(true)

// FileStyle is the style for regular files
var FileStyle = tcell.StyleDefault.
	Foreground(tcell.ColorWhite).
	Background(tcell.ColorBlack)

// DividerStyle is the style for panel dividers
var DividerStyle = tcell.StyleDefault.
	Foreground(tcell.ColorGray).
	Background(tcell.ColorBlack)

// File type specific colors - using 256-color palette for consistent rendering
var (
	// Programming languages
	GoFileStyle         = tcell.StyleDefault.Foreground(tcell.Color37).Background(tcell.ColorBlack)  // Cyan
	RustFileStyle       = tcell.StyleDefault.Foreground(tcell.Color208).Background(tcell.ColorBlack) // Orange
	PythonFileStyle     = tcell.StyleDefault.Foreground(tcell.Color226).Background(tcell.ColorBlack) // Bright yellow
	JavaScriptFileStyle = tcell.StyleDefault.Foreground(tcell.Color220).Background(tcell.ColorBlack) // Gold yellow
	LuaFileStyle        = tcell.StyleDefault.Foreground(tcell.Color63).Background(tcell.ColorBlack)  // Blue-purple

	// Web files
	HTMLFileStyle = tcell.StyleDefault.Foreground(tcell.Color208).Background(tcell.ColorBlack) // Orange
	CSSFileStyle  = tcell.StyleDefault.Foreground(tcell.Color39).Background(tcell.ColorBlack)  // Deep sky blue

	// Data/Config
	JSONFileStyle = tcell.StyleDefault.Foreground(tcell.Color226).Background(tcell.ColorBlack) // Bright yellow
	YAMLFileStyle = tcell.StyleDefault.Foreground(tcell.Color40).Background(tcell.ColorBlack)  // Green

	// Documentation
	MarkdownFileStyle = tcell.StyleDefault.Foreground(tcell.Color33).Background(tcell.ColorBlack) // Bright blue (same as directories)

	// Images
	ImageFileStyle = tcell.StyleDefault.Foreground(tcell.Color201).Background(tcell.ColorBlack) // Magenta/fuchsia

	// Archives
	ArchiveFileStyle = tcell.StyleDefault.Foreground(tcell.Color196).Background(tcell.ColorBlack) // Bright red

	// Executables
	ExecutableFileStyle = tcell.StyleDefault.Foreground(tcell.Color40).Background(tcell.ColorBlack).Bold(true) // Green
)

// StyleForPath returns the appropriate style for a file path
func StyleForPath(path string, isDir bool) tcell.Style {
	if isDir {
		return DirectoryStyle
	}

	// Get file extension
	ext := ""
	for i := len(path) - 1; i >= 0 && path[i] != '/'; i-- {
		if path[i] == '.' {
			ext = path[i:]
			break
		}
	}

	// Debug logging for markdown files
	// if ext == ".md" || ext == ".markdown" {
	// 	log.Printf("THOCK FileBrowser: Markdown file detected: %s (ext=%s)", path, ext)
	// }

	// Return style based on extension
	switch ext {
	// Go
	case ".go":
		return GoFileStyle
	// Rust
	case ".rs":
		return RustFileStyle
	// Python
	case ".py":
		return PythonFileStyle
	// JavaScript/TypeScript
	case ".js", ".jsx", ".ts", ".tsx":
		return JavaScriptFileStyle
	// Lua
	case ".lua":
		return LuaFileStyle
	// Web
	case ".html", ".htm":
		return HTMLFileStyle
	case ".css", ".scss", ".sass", ".less":
		return CSSFileStyle
	// Data/Config
	case ".json":
		return JSONFileStyle
	case ".yaml", ".yml":
		return YAMLFileStyle
	// Documentation
	case ".md", ".markdown":
		return MarkdownFileStyle
	// Images
	case ".png", ".jpg", ".jpeg", ".gif", ".svg", ".ico", ".bmp", ".webp":
		return ImageFileStyle
	// Archives
	case ".zip", ".tar", ".gz", ".bz2", ".xz", ".7z", ".rar":
		return ArchiveFileStyle
	// Executables (by extension)
	case ".sh", ".bash", ".zsh", ".fish":
		return ExecutableFileStyle
	default:
		return FileStyle
	}
}
