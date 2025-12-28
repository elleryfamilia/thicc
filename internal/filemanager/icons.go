package filemanager

import (
	"path/filepath"
	"strings"
)

// Icon constants
const (
	DefaultFileIcon     = "󰈙"
	DefaultFolderIcon   = "󰉋"
	ExpandedFolderIcon  = "󰝰"
	CollapsedFolderIcon = "󰉋"
)

// extensionIcons maps file extensions to Nerd Font icons
var extensionIcons = map[string]string{
	// Programming languages
	".lua":  "󰢱",
	".go":   "󰟓",
	".rs":   "󱘗",
	".py":   "󰌠",
	".rb":   "󰴭",
	".java": "󰬷",
	".c":    "",
	".cpp":  "",
	".h":    "",
	".hpp":  "",
	".cs":   "󰌛",
	".php":  "󰌟",
	".sh":   "󰆍",
	".zsh":  "󰆍",
	".bash": "󰆍",
	".fish": "󰈺",
	".vim":  "",
	".diff": "",
	".patch": "",

	// Web
	".js":   "󰌞",
	".ts":   "󰛦",
	".jsx":  "󰜈",
	".tsx":  "󰛦",
	".html": "󰌝",
	".css":  "󰌜",
	".scss": "󰌜",
	".sass": "󰌜",
	".less": "󰌜",
	".vue":  "󰡄",
	".svelte": "󰡄",

	// Data/Config
	".json": "󰘦",
	".yaml": "󰘦",
	".yml":  "󰘦",
	".toml": "󰘦",
	".xml":  "󰗀",
	".ini":  "󰘦",
	".conf": "󰘦",
	".cfg":  "󰘦",
	".env":  "󰙪",

	// Documentation
	".md":  "󰍔",
	".txt": "󰈙",
	".rst": "󰍔",
	".tex": "󰙩",
	".pdf": "󰈦",

	// Images
	".png":  "󰈟",
	".jpg":  "󰈟",
	".jpeg": "󰈟",
	".gif":  "󰈟",
	".svg":  "󰈟",
	".ico":  "󰈟",
	".bmp":  "󰈟",
	".webp": "󰈟",

	// Archives
	".zip": "󰆧",
	".tar": "󰆧",
	".gz":  "󰆧",
	".bz2": "󰆧",
	".xz":  "󰆧",
	".7z":  "󰆧",
	".rar": "󰆧",

	// Audio
	".mp3":  "󰎆",
	".wav":  "󰎆",
	".flac": "󰎆",
	".ogg":  "󰎆",
	".m4a":  "󰎆",

	// Video
	".mp4": "󰎁",
	".mov": "󰎁",
	".avi": "󰎁",
	".mkv": "󰎁",
	".webm": "󰎁",

	// Database
	".db":     "󰆼",
	".sqlite": "󰆼",
	".sql":    "󰆼",

	// Other
	".lock": "󰌾",
	".log":  "󰌱",
	".git":  "",
	".gitignore": "",
	".gitmodules": "",
	".gitattributes": "",
}

// nameIcons maps specific filenames to Nerd Font icons
var nameIcons = map[string]string{
	// Documentation
	"readme":     "󰈙",
	"readme.md":  "󰈙",
	"readme.txt": "󰈙",
	"changelog":  "󰄭",
	"changelog.md": "󰄭",
	"license":    "󰿃",
	"license.md": "󰿃",
	"license.txt": "󰿃",
	"authors":    "󰒃",
	"contributors": "󰒃",

	// Build files
	"makefile":   "󰙲",
	"dockerfile": "󰡨",
	"docker-compose.yml": "󰡨",
	"docker-compose.yaml": "󰡨",
	"vagrantfile": "󰯁",
	"rakefile":   "",
	"gemfile":    "",
	"gruntfile":  "󰛓",
	"gulpfile":   "󰛓",

	// Config files
	".editorconfig": "󰘦",
	".eslintrc":     "󰘦",
	".prettierrc":   "󰘦",
	".babelrc":      "󰘦",
	".npmrc":        "󰘦",
	".nvmrc":        "󰘦",

	// Package files
	"package.json":    "󰏗",
	"package-lock.json": "󰏗",
	"yarn.lock":       "󰛓",
	"cargo.toml":      "󱘗",
	"cargo.lock":      "󱘗",
	"go.mod":          "󰟓",
	"go.sum":          "󰟓",
	"requirements.txt": "󰌠",
	"pipfile":         "󰌠",
	"gemfile.lock":    "",

	// CI/CD
	".travis.yml":       "",
	".gitlab-ci.yml":    "",
	"jenkinsfile":       "",
	".circleci/config.yml": "",
}

// IconForPath returns the appropriate Nerd Font icon for a given path
func IconForPath(path string, isDir bool) string {
	if isDir {
		return DefaultFolderIcon
	}

	// Get basename for name-based lookup
	basename := filepath.Base(path)
	basenameLower := strings.ToLower(basename)

	// Check name-based icons first
	if icon, ok := nameIcons[basenameLower]; ok {
		return icon
	}

	// Check extension-based icons
	ext := strings.ToLower(filepath.Ext(path))
	if icon, ok := extensionIcons[ext]; ok {
		return icon
	}

	// Default file icon
	return DefaultFileIcon
}

// IconForNode returns the icon for a tree node
func IconForNode(node *TreeNode) string {
	if node.IsDir {
		if node.Expanded {
			return ExpandedFolderIcon
		}
		return CollapsedFolderIcon
	}

	return IconForPath(node.Path, false)
}

// ExpanderSymbol returns the expander symbol for a directory node
func ExpanderSymbol(node *TreeNode) string {
	if !node.IsDir {
		return "  " // Two spaces for files
	}

	if node.Expanded {
		return "- "
	}
	return "+ "
}
