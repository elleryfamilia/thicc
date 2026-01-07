package filebrowser

import (
	"github.com/ellery/thicc/internal/config"
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
	return config.DefStyle.Foreground(tcell.Color252) // Soft gray
}

// DefaultStyle is kept for backwards compatibility (used at render time)
var DefaultStyle = tcell.StyleDefault.Foreground(tcell.Color252)

// GetFocusedStyle returns the style for focused/selected items
// Uses config.DefStyle to ensure correct background with dark theme
func GetFocusedStyle() tcell.Style {
	return config.DefStyle.
		Foreground(tcell.ColorBlack).
		Background(tcell.ColorWhite)
}

// GetDirectoryStyle returns the style for directories, using the editor's background
func GetDirectoryStyle() tcell.Style {
	return config.DefStyle.Foreground(tcell.Color33).Bold(true)
}

// DirectoryStyle is kept for backwards compatibility
var DirectoryStyle = tcell.StyleDefault.Foreground(tcell.Color33).Bold(true)

// GetFileStyle returns the style for regular files, using the editor's background
func GetFileStyle() tcell.Style {
	return config.DefStyle.Foreground(tcell.Color252) // Soft gray
}

// FileStyle is kept for backwards compatibility
var FileStyle = tcell.StyleDefault.Foreground(tcell.Color252)

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
	rubyFileColor       = tcell.Color167 // Red
	javaFileColor       = tcell.Color166 // Orange-red
	cFileColor          = tcell.Color75  // Blue
	csharpFileColor     = tcell.Color135 // Purple
	phpFileColor        = tcell.Color98  // Indigo

	// Web files
	htmlFileColor       = tcell.Color208 // Orange
	cssFileColor        = tcell.Color39  // Deep sky blue
	vueFileColor        = tcell.Color35  // Green

	// Data/Config
	jsonFileColor   = tcell.Color226 // Bright yellow
	yamlFileColor   = tcell.Color40  // Green
	xmlFileColor    = tcell.Color172 // Orange
	configFileColor = tcell.Color67  // Gray-blue

	// Documentation
	markdownFileColor = tcell.Color141 // Light purple
	textFileColor     = tcell.Color250 // Light gray
	pdfFileColor      = tcell.Color160 // Red

	// Media
	imageFileColor = tcell.Color201 // Magenta/fuchsia
	audioFileColor = tcell.Color213 // Light magenta
	videoFileColor = tcell.Color129 // Purple

	// Database
	databaseFileColor = tcell.Color44 // Cyan

	// Archives
	archiveFileColor = tcell.Color196 // Bright red

	// Executables
	executableFileColor = tcell.Color40 // Green

	// Other
	lockFileColor   = tcell.Color243 // Dark gray
	logFileColor    = tcell.Color245 // Gray
	gitFileColor    = tcell.Color208 // Orange
	diffFileColor   = tcell.Color148 // Yellow-green
	defaultFileColor = tcell.Color252 // Soft gray
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
	// Ruby
	case ".rb", ".rake", ".gemspec":
		return config.DefStyle.Foreground(rubyFileColor)
	// Java/Kotlin
	case ".java", ".kt", ".kts", ".scala":
		return config.DefStyle.Foreground(javaFileColor)
	// C/C++
	case ".c", ".cpp", ".cc", ".cxx", ".h", ".hpp", ".hxx":
		return config.DefStyle.Foreground(cFileColor)
	// C#
	case ".cs":
		return config.DefStyle.Foreground(csharpFileColor)
	// PHP
	case ".php":
		return config.DefStyle.Foreground(phpFileColor)
	// Web - HTML
	case ".html", ".htm":
		return config.DefStyle.Foreground(htmlFileColor)
	// Web - CSS
	case ".css", ".scss", ".sass", ".less":
		return config.DefStyle.Foreground(cssFileColor)
	// Web - Vue/Svelte
	case ".vue", ".svelte":
		return config.DefStyle.Foreground(vueFileColor)
	// Data - JSON
	case ".json":
		return config.DefStyle.Foreground(jsonFileColor)
	// Data - YAML/TOML
	case ".yaml", ".yml", ".toml":
		return config.DefStyle.Foreground(yamlFileColor)
	// Data - XML
	case ".xml", ".xsl", ".xslt":
		return config.DefStyle.Foreground(xmlFileColor)
	// Config files
	case ".ini", ".conf", ".cfg", ".env":
		return config.DefStyle.Foreground(configFileColor)
	// Documentation - Markdown
	case ".md", ".markdown", ".rst":
		return config.DefStyle.Foreground(markdownFileColor)
	// Documentation - Plain text
	case ".txt":
		return config.DefStyle.Foreground(textFileColor)
	// Documentation - PDF
	case ".pdf":
		return config.DefStyle.Foreground(pdfFileColor)
	// Images
	case ".png", ".jpg", ".jpeg", ".gif", ".svg", ".ico", ".bmp", ".webp":
		return config.DefStyle.Foreground(imageFileColor)
	// Audio
	case ".mp3", ".wav", ".flac", ".ogg", ".m4a", ".aac":
		return config.DefStyle.Foreground(audioFileColor)
	// Video
	case ".mp4", ".mov", ".avi", ".mkv", ".webm", ".flv":
		return config.DefStyle.Foreground(videoFileColor)
	// Database
	case ".db", ".sqlite", ".sqlite3", ".sql":
		return config.DefStyle.Foreground(databaseFileColor)
	// Archives
	case ".zip", ".tar", ".gz", ".bz2", ".xz", ".7z", ".rar":
		return config.DefStyle.Foreground(archiveFileColor)
	// Executables (shell scripts)
	case ".sh", ".bash", ".zsh", ".fish":
		return config.DefStyle.Foreground(executableFileColor).Bold(true)
	// Lock files
	case ".lock":
		return config.DefStyle.Foreground(lockFileColor)
	// Log files
	case ".log":
		return config.DefStyle.Foreground(logFileColor)
	// Git files
	case ".gitignore", ".gitmodules", ".gitattributes":
		return config.DefStyle.Foreground(gitFileColor)
	// Diff/Patch
	case ".diff", ".patch":
		return config.DefStyle.Foreground(diffFileColor)
	// Vim
	case ".vim":
		return config.DefStyle.Foreground(vueFileColor) // Green like Vue
	// TeX
	case ".tex":
		return config.DefStyle.Foreground(markdownFileColor) // Same as documentation
	default:
		return config.DefStyle.Foreground(defaultFileColor)
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
