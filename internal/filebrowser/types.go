package filebrowser

import (
	"github.com/ellery/thock/internal/config"
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

// GetDefaultStyle returns the default style for the file browser, using the editor's background
func GetDefaultStyle() tcell.Style {
	return config.DefStyle.Foreground(tcell.ColorWhite)
}

// DefaultStyle is kept for backwards compatibility (used at render time)
var DefaultStyle = tcell.StyleDefault.Foreground(tcell.ColorWhite)

// FocusedStyle is the style for focused items
var FocusedStyle = tcell.StyleDefault.
	Foreground(tcell.ColorBlack).
	Background(tcell.ColorWhite)

// GetDirectoryStyle returns the style for directories, using the editor's background
func GetDirectoryStyle() tcell.Style {
	return config.DefStyle.Foreground(tcell.Color33).Bold(true)
}

// DirectoryStyle is kept for backwards compatibility
var DirectoryStyle = tcell.StyleDefault.Foreground(tcell.Color33).Bold(true)

// GetFileStyle returns the style for regular files, using the editor's background
func GetFileStyle() tcell.Style {
	return config.DefStyle.Foreground(tcell.ColorWhite)
}

// FileStyle is kept for backwards compatibility
var FileStyle = tcell.StyleDefault.Foreground(tcell.ColorWhite)

// GetDividerStyle returns the style for panel dividers, using the editor's background
func GetDividerStyle() tcell.Style {
	return config.DefStyle.Foreground(tcell.ColorGray)
}

// DividerStyle is kept for backwards compatibility
var DividerStyle = tcell.StyleDefault.Foreground(tcell.ColorGray)

// File type foreground colors - using 256-color palette for consistent rendering
var (
	// Programming languages
	goFileColor         = tcell.Color37  // Cyan
	rustFileColor       = tcell.Color208 // Orange
	pythonFileColor     = tcell.Color226 // Bright yellow
	javaScriptFileColor = tcell.Color220 // Gold yellow
	luaFileColor        = tcell.Color63  // Blue-purple

	// Web files
	htmlFileColor = tcell.Color208 // Orange
	cssFileColor  = tcell.Color39  // Deep sky blue

	// Data/Config
	jsonFileColor = tcell.Color226 // Bright yellow
	yamlFileColor = tcell.Color40  // Green

	// Documentation
	markdownFileColor = tcell.Color33 // Bright blue (same as directories)

	// Images
	imageFileColor = tcell.Color201 // Magenta/fuchsia

	// Archives
	archiveFileColor = tcell.Color196 // Bright red

	// Executables
	executableFileColor = tcell.Color40 // Green
)

// StyleForPath returns the appropriate style for a file path
// Uses config.DefStyle as the base to match the editor background
func StyleForPath(path string, isDir bool) tcell.Style {
	if isDir {
		return GetDirectoryStyle()
	}

	// Get file extension
	ext := ""
	for i := len(path) - 1; i >= 0 && path[i] != '/'; i-- {
		if path[i] == '.' {
			ext = path[i:]
			break
		}
	}

	// Return style based on extension, using config.DefStyle as base
	switch ext {
	// Go
	case ".go":
		return config.DefStyle.Foreground(goFileColor)
	// Rust
	case ".rs":
		return config.DefStyle.Foreground(rustFileColor)
	// Python
	case ".py":
		return config.DefStyle.Foreground(pythonFileColor)
	// JavaScript/TypeScript
	case ".js", ".jsx", ".ts", ".tsx":
		return config.DefStyle.Foreground(javaScriptFileColor)
	// Lua
	case ".lua":
		return config.DefStyle.Foreground(luaFileColor)
	// Web
	case ".html", ".htm":
		return config.DefStyle.Foreground(htmlFileColor)
	case ".css", ".scss", ".sass", ".less":
		return config.DefStyle.Foreground(cssFileColor)
	// Data/Config
	case ".json":
		return config.DefStyle.Foreground(jsonFileColor)
	case ".yaml", ".yml":
		return config.DefStyle.Foreground(yamlFileColor)
	// Documentation
	case ".md", ".markdown":
		return config.DefStyle.Foreground(markdownFileColor)
	// Images
	case ".png", ".jpg", ".jpeg", ".gif", ".svg", ".ico", ".bmp", ".webp":
		return config.DefStyle.Foreground(imageFileColor)
	// Archives
	case ".zip", ".tar", ".gz", ".bz2", ".xz", ".7z", ".rar":
		return config.DefStyle.Foreground(archiveFileColor)
	// Executables (by extension)
	case ".sh", ".bash", ".zsh", ".fish":
		return config.DefStyle.Foreground(executableFileColor).Bold(true)
	default:
		return GetFileStyle()
	}
}

// Git status colors
var (
	gitModifiedColor  = tcell.Color208 // Orange
	gitStagedColor    = tcell.Color40  // Green
	gitUntrackedColor = tcell.Color243 // Gray
	gitDeletedColor   = tcell.Color196 // Red
	gitConflictColor  = tcell.Color226 // Yellow
)

// Git status icon constants (must match filemanager.GitIcon* constants)
const (
	gitIconModified  = "\uf040" // Pencil
	gitIconStaged    = "\uf00c" // Check
	gitIconUntracked = "\uf059" // Question
	gitIconDeleted   = "\uf00d" // X mark
	gitIconRenamed   = "\uf061" // Arrow
	gitIconConflict  = "\uf071" // Warning
)

// GetGitStatusStyle returns the style for a git status icon
func GetGitStatusStyle(icon string) tcell.Style {
	switch icon {
	case gitIconModified:
		return config.DefStyle.Foreground(gitModifiedColor)
	case gitIconStaged:
		return config.DefStyle.Foreground(gitStagedColor)
	case gitIconUntracked:
		return config.DefStyle.Foreground(gitUntrackedColor)
	case gitIconDeleted:
		return config.DefStyle.Foreground(gitDeletedColor)
	case gitIconRenamed:
		return config.DefStyle.Foreground(gitStagedColor)
	case gitIconConflict:
		return config.DefStyle.Foreground(gitConflictColor)
	default:
		return config.DefStyle
	}
}
