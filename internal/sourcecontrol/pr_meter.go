package sourcecontrol

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ellery/thicc/internal/config"
	"github.com/ellery/thicc/internal/thicc"
)

// PRFileStats holds line count stats for a single file
type PRFileStats struct {
	Path      string
	Additions int
	Deletions int
	Weight    float64 // Multiplier based on file type
}

// PRStats holds the overall PR diff statistics
type PRStats struct {
	Additions   int           // Total lines added
	Deletions   int           // Total lines deleted
	FilesCount  int           // Number of files changed
	FileStats   []PRFileStats // Per-file breakdown
	BaseBranch  string        // Branch we're comparing against
	IsOnMain    bool          // True if on main/master branch
	HasRemote   bool          // True if remote tracking branch exists
}

// PRMeterState holds the calculated meter display state
type PRMeterState struct {
	Patience    float64 // 0.0 to 1.0 (0 = huge PR, 1 = tiny PR)
	TotalPellets int    // Total pellets to display (fixed at 16)
	EatenPellets int    // Number of pellets eaten (from left)
	WeightedLines int   // Lines after applying file type weights
	RawLines     int    // Raw total lines changed
	SpreadFactor float64 // Spread multiplier (0.85 focused, 1.0 normal, 1.25 scattered)
}

// GetFileTypeWeight returns the weight multiplier for a file based on its extension
func GetFileTypeWeight(path string) float64 {
	// Get the filename and extension
	filename := filepath.Base(path)
	ext := strings.ToLower(filepath.Ext(path))

	// Check for test files first (by naming convention)
	lowerFilename := strings.ToLower(filename)
	if strings.HasSuffix(lowerFilename, "_test.go") ||
		strings.HasSuffix(lowerFilename, ".test.ts") ||
		strings.HasSuffix(lowerFilename, ".test.tsx") ||
		strings.HasSuffix(lowerFilename, ".test.js") ||
		strings.HasSuffix(lowerFilename, ".test.jsx") ||
		strings.HasSuffix(lowerFilename, ".spec.ts") ||
		strings.HasSuffix(lowerFilename, ".spec.tsx") ||
		strings.HasSuffix(lowerFilename, ".spec.js") ||
		strings.HasSuffix(lowerFilename, ".spec.jsx") ||
		strings.Contains(lowerFilename, "_test") ||
		strings.HasPrefix(lowerFilename, "test_") {
		return 0.5 // Test files: 0.5x
	}

	// Check for lock files by name
	if lowerFilename == "package-lock.json" ||
		lowerFilename == "yarn.lock" ||
		lowerFilename == "pnpm-lock.yaml" ||
		lowerFilename == "composer.lock" ||
		lowerFilename == "Gemfile.lock" ||
		lowerFilename == "poetry.lock" ||
		lowerFilename == "Pipfile.lock" {
		return 0.1 // Lock files: 0.1x
	}

	// Check for generated/vendor paths
	lowerPath := strings.ToLower(path)
	if strings.Contains(lowerPath, "vendor/") ||
		strings.Contains(lowerPath, "node_modules/") ||
		strings.Contains(lowerPath, "generated/") ||
		strings.Contains(lowerPath, ".gen.") ||
		strings.HasSuffix(lowerPath, ".pb.go") ||
		strings.HasSuffix(lowerPath, ".generated.go") ||
		strings.HasSuffix(lowerPath, ".min.js") ||
		strings.HasSuffix(lowerPath, ".min.css") {
		return 0.1 // Generated/vendor: 0.1x
	}

	// Check by extension
	switch ext {
	// Core source files: 1.0x
	case ".go", ".ts", ".tsx", ".js", ".jsx", ".py", ".rs", ".java", ".c", ".cpp", ".h", ".hpp",
		".cs", ".rb", ".php", ".swift", ".kt", ".scala", ".ex", ".exs", ".erl", ".hs", ".ml",
		".clj", ".vue", ".svelte":
		return 1.0

	// Config files: 0.3x
	case ".json", ".yaml", ".yml", ".toml", ".ini", ".conf", ".xml", ".env":
		return 0.3

	// Documentation: 0.2x
	case ".md", ".txt", ".rst", ".adoc":
		return 0.2

	// Lock files: 0.1x (essentially noise)
	case ".lock":
		return 0.1

	// CSS/styling: 0.5x
	case ".css", ".scss", ".sass", ".less":
		return 0.5

	// HTML templates: 0.4x
	case ".html", ".htm", ".tmpl", ".tpl":
		return 0.4

	// SQL/DB: 0.7x
	case ".sql":
		return 0.7

	// Shell scripts: 0.6x
	case ".sh", ".bash", ".zsh":
		return 0.6

	default:
		return 0.8 // Unknown files: 0.8x (reasonable middle ground)
	}
}

// CalculateSpreadFactor returns the spread multiplier based on file distribution
// Focused (few files, more changes): 0.85x
// Normal: 1.0x
// Scattered (many files, few changes): 1.25x
func CalculateSpreadFactor(filesCount int, totalLines int) float64 {
	if totalLines == 0 || filesCount == 0 {
		return 1.0
	}

	// spread_score = files_changed / (total_lines / 50)
	// This normalizes to roughly 1.0 when there's 50 lines per file
	linesPerFile := float64(totalLines) / 50.0
	if linesPerFile == 0 {
		linesPerFile = 1
	}
	spreadScore := float64(filesCount) / linesPerFile

	if spreadScore < 0.5 {
		return 0.85 // Focused
	} else if spreadScore > 2.0 {
		return 1.25 // Scattered
	}
	return 1.0 // Normal
}

// GetPRSizeMultiplier returns the multiplier based on the user's preferred PR size.
// "small": 1.5x (stricter - meter fills faster)
// "medium": 1.0x (default behavior)
// "large": 0.6x (more lenient - meter fills slower)
func GetPRSizeMultiplier() float64 {
	// Check THICC settings first
	prsize := ""
	if thicc.GlobalThiccSettings != nil && thicc.GlobalThiccSettings.Editor.PRSize != "" {
		prsize = thicc.GlobalThiccSettings.Editor.PRSize
	} else {
		// Fall back to micro-style config
		if opt := config.GetGlobalOption("prsize"); opt != nil {
			prsize = opt.(string)
		}
	}

	switch prsize {
	case "small":
		return 1.5
	case "large":
		return 0.6
	default:
		return 1.0
	}
}

// CalculatePatience converts weighted lines to patience percentage
func CalculatePatience(weightedLines int) float64 {
	// Base thresholds:
	// 0-100 lines: 100-80% patience
	// 100-250 lines: 80-60% patience
	// 250-400 lines: 60-40% patience
	// 400-600 lines: 40-20% patience
	// 600-1000 lines: 20-5% patience
	// 1000+ lines: 5-0% patience

	lines := float64(weightedLines)

	if lines <= 0 {
		return 1.0
	} else if lines <= 100 {
		// 100% -> 80% over 0-100 lines
		return 1.0 - (lines/100)*0.2
	} else if lines <= 250 {
		// 80% -> 60% over 100-250 lines
		return 0.8 - ((lines-100)/150)*0.2
	} else if lines <= 400 {
		// 60% -> 40% over 250-400 lines
		return 0.6 - ((lines-250)/150)*0.2
	} else if lines <= 600 {
		// 40% -> 20% over 400-600 lines
		return 0.4 - ((lines-400)/200)*0.2
	} else if lines <= 1000 {
		// 20% -> 5% over 600-1000 lines
		return 0.2 - ((lines-600)/400)*0.15
	} else {
		// 5% -> 0% over 1000-2000+ lines
		patience := 0.05 - ((lines-1000)/1000)*0.05
		if patience < 0 {
			patience = 0
		}
		return patience
	}
}

// CalculateMeterState computes the full meter state from PR stats
func CalculateMeterState(stats *PRStats) *PRMeterState {
	if stats == nil {
		return &PRMeterState{
			Patience:     1.0,
			TotalPellets: 16,
			EatenPellets: 0,
		}
	}

	// Calculate weighted lines
	var weightedTotal float64
	for _, fs := range stats.FileStats {
		lines := float64(fs.Additions + fs.Deletions)
		weightedTotal += lines * fs.Weight
	}

	rawLines := stats.Additions + stats.Deletions
	spreadFactor := CalculateSpreadFactor(stats.FilesCount, rawLines)
	weightedLines := int(weightedTotal * spreadFactor * GetPRSizeMultiplier())

	patience := CalculatePatience(weightedLines)

	// Calculate eaten pellets (16 total)
	// 100% patience = 0 eaten, 0% patience = 16 eaten
	totalPellets := 16
	eatenPellets := int(float64(totalPellets) * (1.0 - patience))
	if eatenPellets > totalPellets {
		eatenPellets = totalPellets
	}
	if eatenPellets < 0 {
		eatenPellets = 0
	}

	// Ensure at least 1 pellet eaten if there are ANY changes
	// This makes even tiny PRs visible on the meter
	if rawLines > 0 && eatenPellets == 0 {
		eatenPellets = 1
	}

	return &PRMeterState{
		Patience:      patience,
		TotalPellets:  totalPellets,
		EatenPellets:  eatenPellets,
		WeightedLines: weightedLines,
		RawLines:      rawLines,
		SpreadFactor:  spreadFactor,
	}
}

// GetPRDiffStats runs git diff --numstat and parses the results
func GetPRDiffStats(repoRoot string) (*PRStats, error) {
	if repoRoot == "" {
		return nil, nil
	}

	// First, detect the current branch
	currentBranch := ""
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = repoRoot
	output, err := cmd.Output()
	if err != nil {
		log.Printf("THICC PRMeter: Failed to get current branch: %v", err)
		return nil, err
	}
	currentBranch = strings.TrimSpace(string(output))

	// Determine if we're on main/master
	isOnMain := currentBranch == "main" || currentBranch == "master"

	// Detect the base branch for comparison
	baseBranch := detectBaseBranch(repoRoot)
	if baseBranch == "" {
		// No remote or no base branch found
		return &PRStats{
			IsOnMain:  isOnMain,
			HasRemote: false,
		}, nil
	}

	// Build the diff command
	var diffSpec string
	if isOnMain {
		// On main: compare unpushed commits
		diffSpec = "origin/" + currentBranch + "..HEAD"
	} else {
		// On feature branch: compare to base using three-dot (merge-base)
		diffSpec = baseBranch + "...HEAD"
	}

	cmd = exec.Command("git", "diff", "--numstat", diffSpec)
	cmd.Dir = repoRoot
	output, err = cmd.Output()
	if err != nil {
		// Might fail if remote doesn't exist or no common ancestor
		log.Printf("THICC PRMeter: git diff --numstat %s failed: %v", diffSpec, err)
		// Try two-dot syntax as fallback
		if !isOnMain {
			diffSpec = baseBranch + "..HEAD"
			cmd = exec.Command("git", "diff", "--numstat", diffSpec)
			cmd.Dir = repoRoot
			output, err = cmd.Output()
			if err != nil {
				return nil, err
			}
		} else {
			return nil, err
		}
	}

	// Parse the numstat output
	// Format: additions<tab>deletions<tab>path
	// Binary files show - - path
	stats := &PRStats{
		BaseBranch: baseBranch,
		IsOnMain:   isOnMain,
		HasRemote:  true,
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}

		parts := strings.Split(line, "\t")
		if len(parts) < 3 {
			continue
		}

		// Handle binary files (shown as -)
		additions := 0
		deletions := 0
		if parts[0] != "-" {
			additions, _ = strconv.Atoi(parts[0])
		}
		if parts[1] != "-" {
			deletions, _ = strconv.Atoi(parts[1])
		}

		path := parts[2]
		// Handle renames: old => new
		if strings.Contains(path, " => ") {
			pathParts := strings.Split(path, " => ")
			if len(pathParts) == 2 {
				path = pathParts[1]
			}
		}

		weight := GetFileTypeWeight(path)

		stats.FileStats = append(stats.FileStats, PRFileStats{
			Path:      path,
			Additions: additions,
			Deletions: deletions,
			Weight:    weight,
		})

		stats.Additions += additions
		stats.Deletions += deletions
	}

	stats.FilesCount = len(stats.FileStats)

	// Also include uncommitted changes (staged + unstaged)
	// This ensures the meter reflects work-in-progress, not just commits
	addUncommittedChanges(repoRoot, stats)

	log.Printf("THICC PRMeter: %d files, +%d/-%d lines, base=%s, isOnMain=%v",
		stats.FilesCount, stats.Additions, stats.Deletions, baseBranch, isOnMain)

	return stats, nil
}

// addUncommittedChanges adds staged, unstaged, and untracked changes to the stats
func addUncommittedChanges(repoRoot string, stats *PRStats) {
	// Track files we've already counted from commits to avoid double-counting
	existingFiles := make(map[string]bool)
	for _, fs := range stats.FileStats {
		existingFiles[fs.Path] = true
	}

	// Get unstaged changes: git diff --numstat
	cmd := exec.Command("git", "diff", "--numstat")
	cmd.Dir = repoRoot
	output, err := cmd.Output()
	if err == nil {
		parseNumstatIntoStats(string(output), stats, existingFiles)
	}

	// Get staged changes: git diff --numstat --cached
	cmd = exec.Command("git", "diff", "--numstat", "--cached")
	cmd.Dir = repoRoot
	output, err = cmd.Output()
	if err == nil {
		parseNumstatIntoStats(string(output), stats, existingFiles)
	}

	// Get untracked files and count their lines
	addUntrackedFiles(repoRoot, stats, existingFiles)

	// Update file count
	stats.FilesCount = len(stats.FileStats)
}

// addUntrackedFiles counts lines in untracked code files
func addUntrackedFiles(repoRoot string, stats *PRStats, existingFiles map[string]bool) {
	// Get untracked files: git status --porcelain
	cmd := exec.Command("git", "status", "--porcelain")
	cmd.Dir = repoRoot
	output, err := cmd.Output()
	if err != nil {
		return
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if len(line) < 3 {
			continue
		}

		// Untracked files start with "??"
		if line[0] != '?' || line[1] != '?' {
			continue
		}

		path := strings.TrimSpace(line[3:])
		if path == "" {
			continue
		}

		// Skip if already counted
		if existingFiles[path] {
			continue
		}

		// Check if it's a code file worth counting
		weight := GetFileTypeWeight(path)
		if weight < 0.2 {
			// Skip very low-weight files (docs, generated, etc.)
			continue
		}

		// Skip binary/non-text files by extension
		if isBinaryFile(path) {
			continue
		}

		// Count lines in the file
		fullPath := filepath.Join(repoRoot, path)
		lineCount, err := countFileLines(fullPath)
		if err != nil {
			continue
		}

		existingFiles[path] = true

		stats.FileStats = append(stats.FileStats, PRFileStats{
			Path:      path,
			Additions: lineCount,
			Deletions: 0,
			Weight:    weight,
		})

		stats.Additions += lineCount
	}
}

// isBinaryFile returns true if the file extension suggests a binary file
func isBinaryFile(path string) bool {
	ext := strings.ToLower(filepath.Ext(path))
	binaryExts := map[string]bool{
		".exe": true, ".dll": true, ".so": true, ".dylib": true, ".a": true,
		".o": true, ".obj": true, ".class": true, ".jar": true,
		".zip": true, ".tar": true, ".gz": true, ".bz2": true, ".xz": true, ".7z": true,
		".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".bmp": true, ".ico": true, ".webp": true,
		".mp3": true, ".mp4": true, ".avi": true, ".mov": true, ".wav": true, ".flac": true,
		".pdf": true, ".doc": true, ".docx": true, ".xls": true, ".xlsx": true, ".ppt": true, ".pptx": true,
		".ttf": true, ".otf": true, ".woff": true, ".woff2": true, ".eot": true,
		".pyc": true, ".pyo": true, ".beam": true,
		".db": true, ".sqlite": true, ".sqlite3": true,
	}
	return binaryExts[ext]
}

// countFileLines counts the number of lines in a file
func countFileLines(path string) (int, error) {
	file, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	// Check file size - skip very large files (likely not code)
	stat, err := file.Stat()
	if err != nil {
		return 0, err
	}
	if stat.Size() > 1024*1024 { // Skip files > 1MB
		return 0, fmt.Errorf("file too large")
	}

	count := 0
	buf := make([]byte, 32*1024)
	for {
		n, err := file.Read(buf)
		if n > 0 {
			for _, b := range buf[:n] {
				if b == '\n' {
					count++
				}
			}
		}
		if err != nil {
			break
		}
	}

	// Count last line if file doesn't end with newline
	if count == 0 && stat.Size() > 0 {
		count = 1
	}

	return count, nil
}

// parseNumstatIntoStats parses git diff --numstat output and adds to stats
func parseNumstatIntoStats(output string, stats *PRStats, existingFiles map[string]bool) {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if line == "" {
			continue
		}

		parts := strings.Split(line, "\t")
		if len(parts) < 3 {
			continue
		}

		// Handle binary files (shown as -)
		additions := 0
		deletions := 0
		if parts[0] != "-" {
			additions, _ = strconv.Atoi(parts[0])
		}
		if parts[1] != "-" {
			deletions, _ = strconv.Atoi(parts[1])
		}

		path := parts[2]
		// Handle renames: old => new
		if strings.Contains(path, " => ") {
			pathParts := strings.Split(path, " => ")
			if len(pathParts) == 2 {
				path = pathParts[1]
			}
		}

		// Skip if we already counted this file from commits
		if existingFiles[path] {
			continue
		}
		existingFiles[path] = true

		weight := GetFileTypeWeight(path)

		stats.FileStats = append(stats.FileStats, PRFileStats{
			Path:      path,
			Additions: additions,
			Deletions: deletions,
			Weight:    weight,
		})

		stats.Additions += additions
		stats.Deletions += deletions
	}
}

// detectBaseBranch finds the appropriate base branch for comparison
func detectBaseBranch(repoRoot string) string {
	// Check if remote exists
	cmd := exec.Command("git", "remote")
	cmd.Dir = repoRoot
	output, err := cmd.Output()
	if err != nil || len(strings.TrimSpace(string(output))) == 0 {
		return ""
	}

	// Try origin/main first
	cmd = exec.Command("git", "rev-parse", "--verify", "origin/main")
	cmd.Dir = repoRoot
	if err := cmd.Run(); err == nil {
		return "origin/main"
	}

	// Try origin/master
	cmd = exec.Command("git", "rev-parse", "--verify", "origin/master")
	cmd.Dir = repoRoot
	if err := cmd.Run(); err == nil {
		return "origin/master"
	}

	// Try to get the default branch from remote
	cmd = exec.Command("git", "symbolic-ref", "refs/remotes/origin/HEAD")
	cmd.Dir = repoRoot
	output, err = cmd.Output()
	if err == nil {
		// Output is like refs/remotes/origin/main
		ref := strings.TrimSpace(string(output))
		if strings.HasPrefix(ref, "refs/remotes/") {
			return strings.TrimPrefix(ref, "refs/remotes/")
		}
	}

	return ""
}
