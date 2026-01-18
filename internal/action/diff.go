package action

import (
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ellery/thicc/internal/buffer"
)

// Unified diff line types
const (
	DiffLineNone    byte = 0
	DiffLineAdded   byte = 1
	DiffLineDeleted byte = 2
	DiffLineContext byte = 3
	DiffLineHeader  byte = 4
)

// ShowUnifiedDiff shows a unified diff for the given file in a read-only buffer
// The +/- prefixes are stripped and shown in the gutter instead, allowing
// proper syntax highlighting of the actual code content.
// Returns the created buffer (for tab bar integration) and success status.
func ShowUnifiedDiff(filePath string) (*buffer.Buffer, bool) {
	log.Printf("THICC Diff: ShowUnifiedDiff called for %s", filePath)

	// Get current pane
	curPane := MainTab().CurPane()
	if curPane == nil {
		log.Println("THICC Diff: No current pane")
		return nil, false
	}

	// Make path absolute
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		absPath = filePath
	}

	// Get relative path from git root for display
	relPath := getRelativeGitPath(absPath)
	log.Printf("THICC Diff: Relative path: %s", relPath)

	// Run git diff
	diffOutput, err := getGitDiff(absPath)
	if err != nil {
		log.Printf("THICC Diff: git diff failed: %v", err)
		return nil, false
	}

	// Handle no changes case
	var cleanContent string
	var lineTypes map[int]byte
	if diffOutput == "" {
		// Check if this is an untracked file
		if isFileUntracked(absPath) {
			log.Printf("THICC Diff: File is untracked, showing as new file")
			cleanContent, lineTypes, err = getNewFileContent(absPath)
			if err != nil {
				log.Printf("THICC Diff: Failed to read untracked file: %v", err)
				cleanContent = "Error reading untracked file"
				lineTypes = map[int]byte{0: DiffLineNone}
			}
		} else {
			cleanContent = "No changes (file matches HEAD)"
			lineTypes = map[int]byte{0: DiffLineNone}
		}
	} else {
		log.Printf("THICC Diff: Got diff output (%d bytes)", len(diffOutput))
		// Parse the diff to extract clean code and line metadata
		cleanContent, lineTypes = parseDiffContent(diffOutput)
	}

	// Detect file type from extension for syntax highlighting
	ext := filepath.Ext(relPath)
	fileType := extToFileType(ext)
	log.Printf("THICC Diff: Detected filetype '%s' from extension '%s'", fileType, ext)

	// Create read-only buffer with clean content (no +/- prefixes)
	// Use "[diff]" suffix to indicate this is a diff view
	baseName := filepath.Base(relPath)
	bufName := baseName + " [diff]"
	diffBuf := buffer.NewBufferFromString(cleanContent, bufName, buffer.BTHelp)
	if diffBuf == nil {
		log.Println("THICC Diff: Failed to create buffer")
		return nil, false
	}

	// Store the diff line metadata for gutter rendering
	diffBuf.UnifiedDiffLines = lineTypes

	// Set filetype for proper syntax highlighting of the actual code
	if fileType != "" {
		diffBuf.SetOptionNative("filetype", fileType)
	}

	// Open in current pane
	curPane.OpenBuffer(diffBuf)

	log.Printf("THICC Diff: Successfully opened diff view for %s with %d lines", filePath, len(lineTypes))
	return diffBuf, true
}

// parseDiffContent parses git diff output and returns:
// 1. Clean content with +/- prefixes stripped from code lines (headers excluded)
// 2. Map of line number -> diff line type
func parseDiffContent(diffOutput string) (string, map[int]byte) {
	lines := strings.Split(diffOutput, "\n")
	var cleanLines []string
	lineTypes := make(map[int]byte)

	lineNum := 0
	inHunk := false

	for _, line := range lines {
		// Skip diff metadata headers - just show actual content
		if strings.HasPrefix(line, "diff ") ||
			strings.HasPrefix(line, "index ") ||
			strings.HasPrefix(line, "--- ") ||
			strings.HasPrefix(line, "+++ ") ||
			strings.HasPrefix(line, "new file") ||
			strings.HasPrefix(line, "deleted file") {
			inHunk = false
			continue
		}

		// Skip hunk headers but mark that we're in a hunk
		if strings.HasPrefix(line, "@@") {
			inHunk = true
			continue
		}

		// Inside a hunk - process content lines
		if inHunk {
			if len(line) == 0 {
				// Empty line in hunk - treat as context
				cleanLines = append(cleanLines, "")
				lineTypes[lineNum] = DiffLineContext
				lineNum++
				continue
			}

			prefix := line[0]
			content := line[1:] // Strip the prefix

			switch prefix {
			case '+':
				cleanLines = append(cleanLines, content)
				lineTypes[lineNum] = DiffLineAdded
			case '-':
				cleanLines = append(cleanLines, content)
				lineTypes[lineNum] = DiffLineDeleted
			case ' ':
				cleanLines = append(cleanLines, content)
				lineTypes[lineNum] = DiffLineContext
			default:
				// Unknown prefix, keep the whole line
				cleanLines = append(cleanLines, line)
				lineTypes[lineNum] = DiffLineNone
			}
			lineNum++
			continue
		}

		// Outside hunk - skip (shouldn't happen in normal diffs)
	}

	return strings.Join(cleanLines, "\n"), lineTypes
}

// extToFileType maps file extensions to syntax highlighting file types
func extToFileType(ext string) string {
	ext = strings.ToLower(ext)
	switch ext {
	case ".go":
		return "go"
	case ".js":
		return "javascript"
	case ".ts":
		return "typescript"
	case ".tsx":
		return "typescript"
	case ".jsx":
		return "javascript"
	case ".py":
		return "python"
	case ".rb":
		return "ruby"
	case ".rs":
		return "rust"
	case ".c", ".h":
		return "c"
	case ".cpp", ".cc", ".cxx", ".hpp":
		return "c++"
	case ".java":
		return "java"
	case ".sh", ".bash":
		return "shell"
	case ".zsh":
		return "zsh"
	case ".md":
		return "markdown"
	case ".json":
		return "json"
	case ".yaml", ".yml":
		return "yaml"
	case ".toml":
		return "toml"
	case ".xml":
		return "xml"
	case ".html", ".htm":
		return "html"
	case ".css":
		return "css"
	case ".sql":
		return "sql"
	case ".lua":
		return "lua"
	case ".vim":
		return "vim"
	case ".dockerfile":
		return "dockerfile"
	default:
		return ""
	}
}

// getGitDiff runs git diff and returns the output
func getGitDiff(absPath string) (string, error) {
	// Get git root
	gitRoot, err := getGitRoot(absPath)
	if err != nil {
		return "", err
	}

	// Get relative path from git root
	relPath, err := filepath.Rel(gitRoot, absPath)
	if err != nil {
		relPath = filepath.Base(absPath)
	}

	// Run git diff with color disabled for clean output
	cmd := exec.Command("git", "diff", "--no-color", "HEAD", "--", relPath)
	cmd.Dir = gitRoot
	output, err := cmd.Output()
	if err != nil {
		// Check if it's a new file (not in HEAD)
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 128 {
			// Try without HEAD for new files
			cmd = exec.Command("git", "diff", "--no-color", "--", relPath)
			cmd.Dir = gitRoot
			output, err = cmd.Output()
			if err != nil {
				return "", err
			}
		} else {
			return "", err
		}
	}
	return string(output), nil
}

// getGitRoot returns the root directory of the git repository
func getGitRoot(absPath string) (string, error) {
	dir := filepath.Dir(absPath)
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// getRelativeGitPath returns the path relative to git root
func getRelativeGitPath(absPath string) string {
	gitRoot, err := getGitRoot(absPath)
	if err != nil {
		return filepath.Base(absPath)
	}
	relPath, err := filepath.Rel(gitRoot, absPath)
	if err != nil {
		return filepath.Base(absPath)
	}
	return relPath
}

// isFileUntracked checks if a file is untracked by git
func isFileUntracked(absPath string) bool {
	gitRoot, err := getGitRoot(absPath)
	if err != nil {
		return false
	}

	relPath, err := filepath.Rel(gitRoot, absPath)
	if err != nil {
		return false
	}

	cmd := exec.Command("git", "status", "--porcelain", "--", relPath)
	cmd.Dir = gitRoot
	output, err := cmd.Output()
	if err != nil {
		return false
	}

	// Untracked files start with "??"
	return strings.HasPrefix(string(output), "??")
}

// getNewFileContent reads an untracked file and returns its content with line types
// marking all lines as added (green + markers in gutter)
func getNewFileContent(absPath string) (string, map[int]byte, error) {
	content, err := os.ReadFile(absPath)
	if err != nil {
		return "", nil, err
	}

	lines := strings.Split(string(content), "\n")
	// Remove trailing empty line if file ends with newline
	if len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	lineTypes := make(map[int]byte)
	for i := range lines {
		lineTypes[i] = DiffLineAdded
	}

	return strings.Join(lines, "\n"), lineTypes, nil
}

// ShowCommitDiff shows the diff for a file in a specific commit
// The commit hash and file path are used to get the diff for that specific change.
// If filePath is empty, shows the full commit diff (all files).
// repoRoot is the path to the git repository root.
func ShowCommitDiff(commitHash, filePath, repoRoot string) (*buffer.Buffer, bool) {
	log.Printf("THICC Diff: ShowCommitDiff called for '%s' in commit %s (repo: %s)", filePath, commitHash, repoRoot)

	// Get current pane
	curPane := MainTab().CurPane()
	if curPane == nil {
		log.Println("THICC Diff: No current pane")
		return nil, false
	}

	// Use provided repo root
	gitRoot := repoRoot

	// Run git show to get the diff
	var cmd *exec.Cmd
	if filePath == "" {
		// Show full commit diff (all files)
		cmd = exec.Command("git", "show", "--no-color", "--stat", "--patch", commitHash)
	} else {
		// Show diff for specific file
		cmd = exec.Command("git", "show", "--no-color", commitHash, "--", filePath)
	}
	cmd.Dir = gitRoot
	output, err := cmd.Output()
	if err != nil {
		log.Printf("THICC Diff: git show failed: %v", err)
		return nil, false
	}

	diffOutput := string(output)
	if diffOutput == "" {
		diffOutput = "No diff available for this commit"
	}
	log.Printf("THICC Diff: Got commit diff output (%d bytes)", len(diffOutput))

	// Parse the diff to extract clean code and line metadata
	cleanContent, lineTypes := parseDiffContent(diffOutput)

	// Detect file type from extension for syntax highlighting
	var fileType string
	if filePath != "" {
		ext := filepath.Ext(filePath)
		fileType = extToFileType(ext)
		log.Printf("THICC Diff: Detected filetype '%s' from extension '%s'", fileType, ext)
	}

	// Create read-only buffer with clean content
	shortHash := commitHash
	if len(shortHash) > 7 {
		shortHash = shortHash[:7]
	}
	var bufName string
	if filePath == "" {
		bufName = "commit " + shortHash
	} else {
		baseName := filepath.Base(filePath)
		bufName = baseName + " [" + shortHash + "]"
	}
	diffBuf := buffer.NewBufferFromString(cleanContent, bufName, buffer.BTHelp)
	if diffBuf == nil {
		log.Println("THICC Diff: Failed to create buffer")
		return nil, false
	}

	// Store the diff line metadata for gutter rendering
	diffBuf.UnifiedDiffLines = lineTypes

	// Set filetype for proper syntax highlighting of the actual code
	if fileType != "" {
		diffBuf.SetOptionNative("filetype", fileType)
	}

	// Open in current pane
	curPane.OpenBuffer(diffBuf)

	log.Printf("THICC Diff: Successfully opened commit diff view for '%s' at %s", filePath, shortHash)
	return diffBuf, true
}

// CloseDiffView closes the diff view (for compatibility)
func (h *BufPane) CloseDiffView() bool {
	// Clear sync scroll peer if any
	if h.SyncScrollPeer != nil {
		h.SyncScrollPeer.SyncScrollPeer = nil
		h.SyncScrollPeer = nil
	}
	return h.Unsplit()
}
