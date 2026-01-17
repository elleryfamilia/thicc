package terminal

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/creack/pty"
	"github.com/ellery/thicc/internal/screen"
	"github.com/hinshun/vt10x"
	"github.com/micro-editor/tcell/v2"
)

// Debug logging for PTY output - set THICC_DEBUG_PTY=1 to enable
var debugPTY bool
var debugFile *os.File

func init() {
	debugPTY = os.Getenv("THICC_DEBUG_PTY") == "1"
	log.Printf("THICC: Debug PTY check - THICC_DEBUG_PTY=%q, debugPTY=%v", os.Getenv("THICC_DEBUG_PTY"), debugPTY)
	if debugPTY {
		var err error
		debugFile, err = os.Create("/tmp/thicc_pty_debug.log")
		if err != nil {
			log.Printf("THICC: Failed to create debug file: %v", err)
			debugPTY = false
		} else {
			log.Printf("THICC: PTY debug logging enabled -> /tmp/thicc_pty_debug.log")
			fmt.Fprintf(debugFile, "=== THICC PTY DEBUG LOG STARTED ===\n\n")
			debugFile.Sync()
		}
	}
}

// hasStarship checks if the starship prompt is installed
func hasStarship() bool {
	path, err := exec.LookPath("starship")
	found := err == nil
	log.Printf("THICC: hasStarship check - found=%v, path=%s, err=%v", found, path, err)
	return found
}

// getPromptInitCommand returns the command to initialize a sexy prompt
// This is sent to the shell AFTER it fully starts (so it overrides rc files)
func getPromptInitCommand() string {
	shell := getDefaultShell()
	shellName := filepath.Base(shell)

	// Skip starship - use our custom Powerline prompt instead
	_ = hasStarship() // keep function for future use

	// Powerline-style prompt with Spider-Verse theme colors
	// Glyphs defined via printf with UTF-8 hex bytes (avoids encoding issues)
	if shellName == "zsh" {
		return `
# Clear existing hooks
precmd_functions=()
chpwd_functions=()
preexec_functions=()
setopt PROMPT_SUBST

# Define glyphs via printf (UTF-8 hex bytes)
_g_folder=$(printf '\xef\x81\xbc')      # U+F07C folder
_g_arrow=$(printf '\xee\x82\xb0')       # U+E0B0 powerline arrow
_g_branch=$(printf '\xee\x82\xa0')      # U+E0A0 git branch
_g_prompt=$(printf '\xe2\x9d\xaf')      # U+276F chevron
_g_clock=$(printf '\xef\x80\x97')       # U+F017 clock

# Command timing (returns powerline segment if elapsed > 0)
_cmd_start=0
preexec() { _cmd_start=$SECONDS }
_cmd_elapsed() {
  # Determine previous segment color for arrow transition
  # Not in git repo: use folder segment color (24=navy)
  # In git repo clean: use git segment color (30=teal)
  # In git repo dirty: use git segment color (#60fdff=aqua)
  local dirty_git=""
  if git rev-parse --git-dir &>/dev/null; then
    [[ -n "$(git status --porcelain 2>/dev/null | head -1)" ]] && dirty_git=1
  fi

  if (( _cmd_start > 0 )) && (( SECONDS - _cmd_start > 0 )); then
    # prev_bg -> white segment -> default
    if [[ -n "$dirty_git" ]]; then
      echo "%{\e[0m\e[38;2;96;253;255m\e[48;5;231m%}${_g_arrow}%F{black} ${_g_clock} $((SECONDS - _cmd_start))s %k%F{231}${_g_arrow}%f"
    elif git rev-parse --git-dir &>/dev/null; then
      echo "%k%F{30}%K{231}${_g_arrow}%F{black} ${_g_clock} $((SECONDS - _cmd_start))s %k%F{231}${_g_arrow}%f"
    else
      echo "%k%F{24}%K{231}${_g_arrow}%F{black} ${_g_clock} $((SECONDS - _cmd_start))s %k%F{231}${_g_arrow}%f"
    fi
  else
    # prev_bg -> default (just the ending arrow)
    if [[ -n "$dirty_git" ]]; then
      echo "%{\e[0m\e[38;2;96;253;255m%}${_g_arrow}%f"
    elif git rev-parse --git-dir &>/dev/null; then
      echo "%k%F{30}${_g_arrow}%f"
    else
      echo "%k%F{24}${_g_arrow}%f"
    fi
  fi
  _cmd_start=0
}

# Git info with dynamic segment color based on state
# Returns full segment: [arrow into][content][arrow out or to time]
_git_info() {
  local branch=$(git symbolic-ref --short HEAD 2>/dev/null)
  [[ -z "$branch" ]] && return

  # Check dirty state
  local dirty=$(git status --porcelain 2>/dev/null | head -1)

  # Check ahead/behind
  local ab=""
  local ahead=$(git rev-list --count @{upstream}..HEAD 2>/dev/null)
  local behind=$(git rev-list --count HEAD..@{upstream} 2>/dev/null)
  [[ "$ahead" -gt 0 ]] 2>/dev/null && ab="+${ahead}"
  [[ "$behind" -gt 0 ]] 2>/dev/null && ab="${ab}-${behind}"

  # Truncate branch to 12 chars
  branch="${branch:0:12}"

  # Segment color: 30=teal (clean), #60fdff=aqua (dirty)
  if [[ -n "$dirty" ]]; then
    # Dirty: aqua background (#60fdff) with black text
    echo "%F{24}%{\e[48;2;96;253;255m%}${_g_arrow}%f%F{black} ${_g_branch} ${branch}${ab} %{\e[0m%}%F{#60fdff}"
  else
    # Clean: teal background with white text
    echo "%F{24}%K{30}${_g_arrow}%f%F{231} ${_g_branch} ${branch}${ab} "
  fi
}

# Build prompt: [navy] dir [dynamic git segment] [muted gold time if >0] [arrow]
PROMPT='%K{24}%F{231} ${_g_folder} %1~ %k$(_git_info)$(_cmd_elapsed) '
RPROMPT=''
`
	}
	// bash: Similar but with bash syntax
	return `
# Define glyphs via printf
_g_folder=$(printf '\xef\x81\xbc')
_g_arrow=$(printf '\xee\x82\xb0')
_g_branch=$(printf '\xee\x82\xa0')
_g_prompt=$(printf '\xe2\x9d\xaf')
_g_clock=$(printf '\xef\x80\x97')

_cmd_start=0
_timer_start() { _cmd_start=$SECONDS; }
_timer_show() {
  # Determine previous segment color for arrow transition
  local dirty_git=""
  if git rev-parse --git-dir &>/dev/null; then
    [[ -n "$(git status --porcelain 2>/dev/null | head -1)" ]] && dirty_git=1
  fi

  if (( _cmd_start > 0 )) && (( SECONDS - _cmd_start > 0 )); then
    if [[ -n "$dirty_git" ]]; then
      printf "\e[0m\e[38;2;96;253;255m\e[48;5;231m${_g_arrow}\e[30m ${_g_clock} %ss \e[0m\e[38;5;231m${_g_arrow}\e[0m" "$((SECONDS - _cmd_start))"
    elif git rev-parse --git-dir &>/dev/null; then
      printf "\e[0m\e[38;5;30m\e[48;5;231m${_g_arrow}\e[30m ${_g_clock} %ss \e[0m\e[38;5;231m${_g_arrow}\e[0m" "$((SECONDS - _cmd_start))"
    else
      printf "\e[0m\e[38;5;24m\e[48;5;231m${_g_arrow}\e[30m ${_g_clock} %ss \e[0m\e[38;5;231m${_g_arrow}\e[0m" "$((SECONDS - _cmd_start))"
    fi
  else
    if [[ -n "$dirty_git" ]]; then
      printf "\e[0m\e[38;2;96;253;255m${_g_arrow}\e[0m"
    elif git rev-parse --git-dir &>/dev/null; then
      printf "\e[0m\e[38;5;30m${_g_arrow}\e[0m"
    else
      printf "\e[0m\e[38;5;24m${_g_arrow}\e[0m"
    fi
  fi
  _cmd_start=0
}
trap '_timer_start' DEBUG

_git_info() {
  local branch=$(git symbolic-ref --short HEAD 2>/dev/null)
  [[ -z "$branch" ]] && return
  local dirty=$(git status --porcelain 2>/dev/null | head -1)
  branch="${branch:0:12}"
  if [[ -n "$dirty" ]]; then
    # Dirty: aqua background (#60fdff) with black text
    printf "\e[38;5;24m\e[48;2;96;253;255m${_g_arrow}\e[30m ${_g_branch} ${branch} "
  else
    # Clean: teal background with white text
    printf "\e[38;5;24m\e[48;5;30m${_g_arrow}\e[97m ${_g_branch} ${branch} "
  fi
}

PS1='\[\e[48;5;24m\e[97m\] ${_g_folder} \W \[\e[0m\]$(_git_info)$(_timer_show) '
`
}

// Region defines a rectangular screen region (same as filebrowser)
type Region struct {
	X      int
	Y      int
	Width  int
	Height int
}

// Loc represents a position in the terminal (for selection)
type Loc struct {
	X, Y int
}

// LessThan returns true if this location is before other
func (l Loc) LessThan(other Loc) bool {
	if l.Y < other.Y {
		return true
	}
	if l.Y > other.Y {
		return false
	}
	return l.X < other.X
}

// GreaterEqual returns true if this location is at or after other
func (l Loc) GreaterEqual(other Loc) bool {
	return !l.LessThan(other)
}

// Panel is a standalone terminal emulator with VT10x + PTY
type Panel struct {
	VT      vt10x.Terminal
	PTY     *os.File
	Cmd     *exec.Cmd
	Region  Region
	Focus   bool
	Running bool
	mu      sync.Mutex

	// OnRedraw is called when new data arrives and screen needs refresh
	OnRedraw func()

	// Render throttling
	pendingRedraw bool
	redrawTimer   *time.Timer
	throttleDelay time.Duration

	// Auto-respawn shell when process exits
	autoRespawn bool

	// Selection state for copy/paste
	Selection          [2]Loc // Start and end of selection (Y is lineIndex into scrollback+live buffer)
	mouseReleased      bool   // Track mouse button state for drag detection
	keyboardSelecting  bool   // True when actively selecting with Shift+Arrow

	// Scrollback support
	Scrollback     *ScrollbackBuffer // Circular buffer for terminal history
	scrollOffset   int               // 0 = live view, >0 = scrolled up N lines
	previousScreen [][]vt10x.Glyph   // All rows before VT.Write for scroll detection

	// Startup state - for showing loading indicator
	hasReceivedOutput bool // True once terminal has received any output

	// Quick command mode - shows hint bar, single key triggers action
	QuickCommandMode bool
	// Passthrough mode - all keys sent directly to PTY, bypassing thicc shortcuts
	PassthroughMode bool
	// LastCtrlBackslash tracks timing for double-tap exit from passthrough mode
	LastCtrlBackslash time.Time
	// OnShowMessage callback to display messages (set by layout manager)
	OnShowMessage func(msg string)
	// OnQuit callback when user triggers quit from quick command mode
	OnQuit func()
	// OnNextPane callback when user triggers next pane from quick command mode
	OnNextPane func()
	// OnSessionEnd callback when terminal process exits (for auto-hiding the pane)
	OnSessionEnd func()

	// Auto-scroll state for drag-to-select
	autoScrollDirection int         // -1=up, 0=none, 1=down
	autoScrollTimer     *time.Timer // Timer for continuous scrolling
}

// isShellCommand returns true if the command is a shell (bash, zsh, sh, fish, etc.)
func isShellCommand(cmdArgs []string) bool {
	if len(cmdArgs) == 0 {
		return false
	}
	base := filepath.Base(cmdArgs[0])
	switch base {
	case "bash", "zsh", "sh", "fish", "ksh", "csh", "tcsh", "dash":
		return true
	}
	return false
}

// NewPanel creates a new terminal panel
// cmdArgs is the command to run (defaults to user's shell if nil/empty)
func NewPanel(x, y, w, h int, cmdArgs []string) (*Panel, error) {
	// Determine if we should auto-respawn shell when process exits
	// (true when running an AI tool, false when running default shell)
	autoRespawn := cmdArgs != nil && len(cmdArgs) > 0

	// Track if we're starting a default shell (to inject sexy prompt)
	// We inject prompt for: nil/empty cmdArgs, OR when cmdArgs is a shell command
	injectPrompt := false

	log.Printf("THICC: NewPanel called with cmdArgs=%v (nil=%v, len=%d, autoRespawn=%v)", cmdArgs, cmdArgs == nil, len(cmdArgs), autoRespawn)

	// Default to user's shell if no command specified
	if cmdArgs == nil || len(cmdArgs) == 0 {
		shell := getDefaultShell()
		cmdArgs = []string{shell, "-i"} // Just start interactive shell
		injectPrompt = true             // We'll inject sexy prompt after shell starts
		autoRespawn = false             // Default shell doesn't auto-respawn
		log.Printf("THICC: Using default shell: %s, injectPrompt=true", shell)
	} else if isShellCommand(cmdArgs) {
		// User selected a shell as their "AI tool" - still inject the sexy prompt
		injectPrompt = true
		autoRespawn = false // Shells don't auto-respawn
		log.Printf("THICC: Using shell from cmdArgs: %v, injectPrompt=true (detected as shell)", cmdArgs)
	} else {
		log.Printf("THICC: Using AI tool cmdArgs: %v, injectPrompt=false", cmdArgs)
	}

	// Content area is inside the border (1 cell on each side)
	contentW := w - 2
	contentH := h - 2
	if contentW < 10 {
		contentW = 10
	}
	if contentH < 5 {
		contentH = 5
	}

	// Create VT emulator with content size (not full region)
	// IMPORTANT: Must create command/PTY first, then pass PTY as writer to vt10x
	// This is a two-step process due to initialization order

	// Create command
	cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)

	// Set up environment
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "TERM=xterm-256color")
	cmd.Env = append(cmd.Env, "THICC_TERM=1") // Marker so users can customize prompt in their shell config

	// Start command with PTY at the correct size from the beginning
	// This prevents apps from rendering at default size then re-rendering on SIGWINCH
	winSize := &pty.Winsize{
		Rows: uint16(contentH),
		Cols: uint16(contentW),
	}
	ptmx, err := pty.StartWithSize(cmd, winSize)
	if err != nil {
		return nil, err
	}

	if debugPTY && debugFile != nil {
		fmt.Fprintf(debugFile, "=== TERMINAL CREATED ===\n")
		fmt.Fprintf(debugFile, "Command: %v\n", cmdArgs)
		fmt.Fprintf(debugFile, "Region: x=%d y=%d w=%d h=%d\n", x, y, w, h)
		fmt.Fprintf(debugFile, "Content size: %dx%d (cols x rows)\n", contentW, contentH)
		fmt.Fprintf(debugFile, "PTY Winsize: %d cols x %d rows\n", winSize.Cols, winSize.Rows)
		fmt.Fprintf(debugFile, "=== END ===\n\n")
		debugFile.Sync()
	}

	// Create VT emulator with PTY as writer (enables DSR/CPR responses)
	vt := vt10x.New(vt10x.WithSize(contentW, contentH), vt10x.WithWriter(ptmx))

	// Load settings and create scrollback buffer
	settings := LoadSettings()

	p := &Panel{
		VT:            vt,
		PTY:           ptmx,
		Cmd:           cmd,
		Region:        Region{X: x, Y: y, Width: w, Height: h},
		Running:       true,
		Focus:         false,
		throttleDelay: 16 * time.Millisecond, // 60fps max
		autoRespawn:   autoRespawn,
		mouseReleased: true, // Start with mouse released
		Scrollback:    NewScrollbackBuffer(settings.ScrollbackLines),
		scrollOffset:  0,
	}

	// Start reading from PTY in background
	go p.readLoop()

	// Start loading animation ticker (triggers redraws for spinner)
	go p.loadingAnimationLoop()

	// Inject sexy prompt after shell fully initializes
	// Check THICC_SIMPLE_PROMPT env var to disable fancy prompt for testing
	if injectPrompt && os.Getenv("THICC_SIMPLE_PROMPT") == "" {
		log.Printf("THICC: Will inject sexy prompt for shell")
		go p.injectSexyPrompt()
	} else {
		log.Printf("THICC: Not injecting prompt (cmdArgs provided or THICC_SIMPLE_PROMPT set)")
	}

	return p, nil
}

// injectSexyPrompt sends prompt initialization to the shell after it starts
// This ensures our prompt overrides anything set in rc files
func (p *Panel) injectSexyPrompt() {
	log.Printf("THICC: injectSexyPrompt starting, waiting 1000ms...")
	// Wait for shell frameworks (oh-my-zsh, powerlevel10k, etc.) to fully initialize
	// This needs to be long enough for the shell to be at a prompt
	time.Sleep(1000 * time.Millisecond)

	p.mu.Lock()
	if !p.Running || p.PTY == nil {
		log.Printf("THICC: injectSexyPrompt - shell not running or PTY nil, aborting")
		p.mu.Unlock()
		return
	}
	pty := p.PTY
	p.mu.Unlock()

	// Get the prompt init command
	initCmd := getPromptInitCommand()

	// Write prompt config to temp file, then source it
	// This is more reliable than sending raw commands which might get
	// mangled by shell line editing or overwritten by shell frameworks
	tmpFile := "/tmp/.thicc_prompt_init"
	if err := os.WriteFile(tmpFile, []byte(initCmd), 0644); err != nil {
		log.Printf("THICC: injectSexyPrompt - failed to write temp file: %v", err)
		return
	}

	// Source the file and clear screen
	// Using exec to bypass any shell aliases or functions
	sourceCmd := fmt.Sprintf("source %s && clear\n", tmpFile)
	log.Printf("THICC: injectSexyPrompt - sourcing prompt from %s", tmpFile)

	_, err := pty.Write([]byte(sourceCmd))
	if err != nil {
		log.Printf("THICC: injectSexyPrompt - write error: %v", err)
	}
}

// loadingAnimationLoop triggers screen redraws while loading indicator is visible
// This allows the spinner animation to update
func (p *Panel) loadingAnimationLoop() {
	ticker := time.NewTicker(80 * time.Millisecond)
	defer ticker.Stop()

	for range ticker.C {
		p.mu.Lock()
		done := p.hasReceivedOutput || !p.Running
		p.mu.Unlock()

		if done {
			return
		}

		// Trigger a screen redraw to update the spinner
		screen.Redraw()
	}
}

// getDefaultShell returns the user's default shell
func getDefaultShell() string {
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/bash"
	}
	return shell
}

// readLoop continuously reads from PTY and writes to VT emulator
func (p *Panel) readLoop() {
	buf := make([]byte, 4096)
	for {
		n, err := p.PTY.Read(buf)
		if err != nil {
			// EOF or error - terminal closed
			p.mu.Lock()
			shouldRespawn := p.autoRespawn
			p.Running = false
			p.mu.Unlock()

			// If auto-respawn is enabled (AI tool was running), spawn a new shell
			if shouldRespawn {
				// Small delay to let the process fully exit
				time.Sleep(100 * time.Millisecond)
				p.RespawnShell()
			} else {
				// Notify layout manager to hide this terminal pane
				if p.OnSessionEnd != nil {
					p.OnSessionEnd()
				}
			}
			return
		}

		if n > 0 {
			// Debug: log raw PTY output
			if debugPTY && debugFile != nil {
				fmt.Fprintf(debugFile, "=== PTY OUTPUT (%d bytes) ===\n", n)
				// Log hex dump for escape sequences
				for i, b := range buf[:n] {
					if b < 32 || b > 126 {
						fmt.Fprintf(debugFile, "\\x%02x", b)
					} else {
						fmt.Fprintf(debugFile, "%c", b)
					}
					if (i+1)%80 == 0 {
						fmt.Fprintf(debugFile, "\n")
					}
				}
				fmt.Fprintf(debugFile, "\n=== END ===\n\n")
				debugFile.Sync()
			}

			// Hold mutex during VT operations to prevent race with Resize
			p.mu.Lock()

			// Mark that we've received output (clears loading indicator)
			if !p.hasReceivedOutput {
				p.hasReceivedOutput = true
			}

			// Capture screen before write (for scroll detection)
			p.captureScreenBefore()

			// Filter out CPR responses before writing to VT emulator
			// (CPR = Cursor Position Report, response to ESC[6n query)
			filtered := filterCPRResponses(buf[:n])

			// Write to VT emulator
			p.VT.Write(filtered)

			// Check if scroll occurred and capture scrolled lines
			p.captureScrolledLines()

			p.mu.Unlock()

			// Schedule throttled redraw
			p.scheduleRedraw()
		}
	}
}

// captureScreenBefore captures all rows before VT.Write for scroll detection
func (p *Panel) captureScreenBefore() {
	cols, rows := p.VT.Size()
	p.previousScreen = make([][]vt10x.Glyph, rows)
	for y := 0; y < rows; y++ {
		p.previousScreen[y] = make([]vt10x.Glyph, cols)
		for x := 0; x < cols; x++ {
			p.previousScreen[y][x] = p.VT.Cell(x, y)
		}
	}
}

// rowsMatch checks if two rows have the same content
func rowsMatch(row1, row2 []vt10x.Glyph) bool {
	if len(row1) != len(row2) {
		return false
	}
	for x := 0; x < len(row1); x++ {
		if row1[x].Char != row2[x].Char {
			return false
		}
	}
	return true
}

// captureScrolledLines detects scrolled lines and adds them to scrollback
func (p *Panel) captureScrolledLines() {
	if len(p.previousScreen) == 0 {
		return
	}

	cols, rows := p.VT.Size()
	if rows != len(p.previousScreen) || (rows > 0 && cols != len(p.previousScreen[0])) {
		return // Size changed, skip
	}

	// Get current top row
	currentTopRow := make([]vt10x.Glyph, cols)
	for x := 0; x < cols; x++ {
		currentTopRow[x] = p.VT.Cell(x, 0)
	}

	// Check if top row changed at all
	if rowsMatch(currentTopRow, p.previousScreen[0]) {
		return // No scroll, top row unchanged
	}

	// Strategy 1: Look for old row 0 somewhere in new screen
	// If found at newY, then newY lines scrolled off
	for newY := 1; newY < rows; newY++ {
		newRow := make([]vt10x.Glyph, cols)
		for x := 0; x < cols; x++ {
			newRow[x] = p.VT.Cell(x, newY)
		}

		if rowsMatch(newRow, p.previousScreen[0]) {
			// Verify with next row
			if newY+1 < rows && 1 < len(p.previousScreen) {
				nextRow := make([]vt10x.Glyph, cols)
				for x := 0; x < cols; x++ {
					nextRow[x] = p.VT.Cell(x, newY+1)
				}
				if rowsMatch(nextRow, p.previousScreen[1]) {
					// Confirmed scroll - capture newY lines
					for i := 0; i < newY; i++ {
						line := ScrollbackLine{
							Cells: make([]vt10x.Glyph, len(p.previousScreen[i])),
						}
						copy(line.Cells, p.previousScreen[i])
						p.Scrollback.Push(line)
					}
					return
				}
			}
		}
	}

	// Strategy 2: Old row 0 not visible - check if any old row is now at new row 0
	// If old row N is at new row 0, then N lines scrolled off entirely
	for oldY := 1; oldY < len(p.previousScreen); oldY++ {
		if rowsMatch(currentTopRow, p.previousScreen[oldY]) {
			// Verify with next row
			if oldY+1 < len(p.previousScreen) {
				nextNewRow := make([]vt10x.Glyph, cols)
				for x := 0; x < cols; x++ {
					nextNewRow[x] = p.VT.Cell(x, 1)
				}
				if rowsMatch(nextNewRow, p.previousScreen[oldY+1]) {
					// Confirmed large scroll - capture oldY lines
					for i := 0; i < oldY; i++ {
						line := ScrollbackLine{
							Cells: make([]vt10x.Glyph, len(p.previousScreen[i])),
						}
						copy(line.Cells, p.previousScreen[i])
						p.Scrollback.Push(line)
					}
					return
				}
			}
		}
	}

	// No scroll pattern detected - might be a screen clear or redraw
}

// RespawnShell starts a new shell in the terminal after the previous process exited
func (p *Panel) RespawnShell() error {
	log.Printf("THICC: RespawnShell called")
	p.mu.Lock()
	defer p.mu.Unlock()

	// Close old PTY if still open
	if p.PTY != nil {
		p.PTY.Close()
		p.PTY = nil
	}

	// Get content dimensions
	contentW := p.Region.Width - 2
	contentH := p.Region.Height - 2
	if contentW < 10 {
		contentW = 10
	}
	if contentH < 5 {
		contentH = 5
	}

	// Create new shell command
	shell := getDefaultShell()
	cmd := exec.Command(shell)
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "TERM=xterm-256color")
	cmd.Env = append(cmd.Env, "THICC_TERM=1")

	// Start with new PTY
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return err
	}

	// Set PTY size
	_ = pty.Setsize(ptmx, &pty.Winsize{
		Rows: uint16(contentH),
		Cols: uint16(contentW),
	})

	// Create new VT emulator (clear screen for fresh start)
	p.VT = vt10x.New(vt10x.WithSize(contentW, contentH), vt10x.WithWriter(ptmx))

	p.PTY = ptmx
	p.Cmd = cmd
	p.Running = true
	p.autoRespawn = false // Don't auto-respawn again (now running shell)

	// Clear scrollback on respawn
	p.Scrollback.Clear()
	p.scrollOffset = 0
	p.hasReceivedOutput = false // Reset for new loading indicator

	// Start new read loop
	go p.readLoop()

	// Start loading animation ticker
	go p.loadingAnimationLoop()

	// Inject sexy prompt after shell initializes
	go p.injectSexyPrompt()

	// Trigger redraw
	if p.OnRedraw != nil {
		p.OnRedraw()
	}

	return nil
}

// scheduleRedraw batches rapid redraw requests using a timer
func (p *Panel) scheduleRedraw() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.pendingRedraw = true

	if p.redrawTimer == nil {
		p.redrawTimer = time.AfterFunc(p.throttleDelay, func() {
			p.mu.Lock()
			if p.pendingRedraw && p.OnRedraw != nil {
				p.pendingRedraw = false
				p.mu.Unlock()
				p.OnRedraw()
			} else {
				p.mu.Unlock()
			}
		})
	} else {
		p.redrawTimer.Reset(p.throttleDelay)
	}
}

// Write sends data to the PTY (used for user input)
func (p *Panel) Write(data []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.Running || p.PTY == nil {
		return 0, io.ErrClosedPipe
	}

	return p.PTY.Write(data)
}

// Resize changes the terminal size
func (p *Panel) Resize(w, h int) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Skip if size hasn't changed (avoids unnecessary SIGWINCH to apps)
	if p.Region.Width == w && p.Region.Height == h {
		if debugPTY && debugFile != nil {
			fmt.Fprintf(debugFile, "=== RESIZE SKIPPED (same size %dx%d) ===\n\n", w, h)
			debugFile.Sync()
		}
		return nil
	}

	if debugPTY && debugFile != nil {
		fmt.Fprintf(debugFile, "=== RESIZE %dx%d -> %dx%d ===\n\n", p.Region.Width, p.Region.Height, w, h)
		debugFile.Sync()
	}

	p.Region.Width = w
	p.Region.Height = h

	// Content area is inside the border
	contentW := w - 2
	contentH := h - 2
	if contentW < 10 {
		contentW = 10
	}
	if contentH < 5 {
		contentH = 5
	}

	// Resize VT emulator first, then PTY (which sends SIGWINCH to app)
	p.VT.Resize(contentW, contentH)

	if p.PTY != nil {
		_ = pty.Setsize(p.PTY, &pty.Winsize{
			Rows: uint16(contentH),
			Cols: uint16(contentW),
		})
	}

	return nil
}

// Close cleans up the terminal resources
func (p *Panel) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.Running = false

	if p.redrawTimer != nil {
		p.redrawTimer.Stop()
	}

	// Stop auto-scroll timer
	p.stopAutoScrollLocked()

	if p.PTY != nil {
		p.PTY.Close()
		p.PTY = nil
	}

	if p.Cmd != nil && p.Cmd.Process != nil {
		_ = p.Cmd.Process.Kill()
	}
}

// IsRunning returns whether the terminal command is still running
func (p *Panel) IsRunning() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.Running
}

// HasSelection returns true if there is an active text selection
func (p *Panel) HasSelection() bool {
	return p.Selection[0] != p.Selection[1]
}

// ClearSelection clears the current selection
func (p *Panel) ClearSelection() {
	p.Selection[0] = Loc{}
	p.Selection[1] = Loc{}
}

// GetSelection returns the selected text from the terminal
func (p *Panel) GetSelection() string {
	if !p.HasSelection() {
		return ""
	}

	start := p.Selection[0]
	end := p.Selection[1]

	// Normalize so start is before end
	if start.GreaterEqual(end) {
		start, end = end, start
	}

	// Get content dimensions
	cols, rows := p.VT.Size()
	scrollbackCount := p.Scrollback.Count()

	// Helper to get a cell at a given line index
	getCell := func(x, lineIndex int) rune {
		if lineIndex < 0 {
			// Above scrollback - empty
			return ' '
		} else if lineIndex < scrollbackCount {
			// In scrollback buffer
			line := p.Scrollback.Get(lineIndex)
			if line != nil && x < len(line.Cells) {
				r := line.Cells[x].Char
				if r == 0 {
					return ' '
				}
				return r
			}
			return ' '
		} else {
			// In live terminal
			liveY := lineIndex - scrollbackCount
			if liveY < rows && x < cols {
				glyph := p.VT.Cell(x, liveY)
				r := glyph.Char
				if r == 0 {
					return ' '
				}
				return r
			}
			return ' '
		}
	}

	var result string
	for lineIndex := start.Y; lineIndex <= end.Y; lineIndex++ {
		lineStart := 0
		lineEnd := cols

		if lineIndex == start.Y {
			lineStart = start.X
		}
		if lineIndex == end.Y {
			lineEnd = end.X
		}

		for x := lineStart; x < lineEnd && x < cols; x++ {
			result += string(getCell(x, lineIndex))
		}

		// Add newline between lines (but not at the end)
		if lineIndex < end.Y {
			result += "\n"
		}
	}

	return result
}

// isSelected returns true if the given cell position is within the selection
// x and y are screen coordinates; selection Y is stored as lineIndex
func (p *Panel) isSelected(x, y int) bool {
	if !p.HasSelection() {
		return false
	}

	// Convert screen Y to line index (same coordinate space as selection)
	scrollbackCount := p.Scrollback.Count()
	lineIndex := scrollbackCount - p.scrollOffset + y
	loc := Loc{X: x, Y: lineIndex}

	start := p.Selection[0]
	end := p.Selection[1]

	// Handle selection in either direction
	if start.GreaterEqual(end) {
		start, end = end, start
	}

	return loc.GreaterEqual(start) && loc.LessThan(end)
}

// ScrollUp scrolls the view up (into history)
func (p *Panel) ScrollUp(lines int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	maxScroll := p.Scrollback.Count()
	p.scrollOffset += lines
	if p.scrollOffset > maxScroll {
		p.scrollOffset = maxScroll
	}

	if p.OnRedraw != nil {
		p.OnRedraw()
	}
}

// ScrollDown scrolls the view down (toward live)
func (p *Panel) ScrollDown(lines int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.scrollOffset -= lines
	if p.scrollOffset < 0 {
		p.scrollOffset = 0
	}

	if p.OnRedraw != nil {
		p.OnRedraw()
	}
}

// ScrollToBottom returns to live view
func (p *Panel) ScrollToBottom() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.scrollOffset == 0 {
		return // Already at bottom
	}

	p.scrollOffset = 0

	if p.OnRedraw != nil {
		p.OnRedraw()
	}
}

// IsScrolledUp returns true if viewing scrollback history
func (p *Panel) IsScrolledUp() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.scrollOffset > 0
}

// ScrollOffset returns the current scroll offset (0 = live view)
func (p *Panel) ScrollOffset() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.scrollOffset
}

// Auto-scroll constants
const (
	autoScrollInterval = 50 * time.Millisecond // How often to scroll when holding at edge
	autoScrollLines    = 1                     // Lines to scroll per tick
)

// StartAutoScroll begins continuous scrolling in the given direction (-1=up, 1=down)
func (p *Panel) StartAutoScroll(direction int) {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.autoScrollDirection == direction {
		return // Already scrolling in this direction
	}

	p.stopAutoScrollLocked()
	p.autoScrollDirection = direction

	p.autoScrollTimer = time.AfterFunc(autoScrollInterval, func() {
		p.autoScrollTick()
	})
}

// StopAutoScroll stops continuous scrolling
func (p *Panel) StopAutoScroll() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.stopAutoScrollLocked()
}

func (p *Panel) stopAutoScrollLocked() {
	p.autoScrollDirection = 0
	if p.autoScrollTimer != nil {
		p.autoScrollTimer.Stop()
		p.autoScrollTimer = nil
	}
}

func (p *Panel) autoScrollTick() {
	p.mu.Lock()
	direction := p.autoScrollDirection
	p.mu.Unlock()

	if direction == 0 {
		return
	}

	// Scroll in the appropriate direction (these methods handle their own locking)
	if direction < 0 {
		p.ScrollUp(autoScrollLines)
	} else {
		p.ScrollDown(autoScrollLines)
	}

	// Extend selection while scrolling
	p.extendSelectionWhileScrolling(direction)

	// Schedule next tick
	p.mu.Lock()
	if p.autoScrollDirection != 0 {
		p.autoScrollTimer = time.AfterFunc(autoScrollInterval, func() {
			p.autoScrollTick()
		})
	}
	p.mu.Unlock()
}

func (p *Panel) extendSelectionWhileScrolling(direction int) {
	p.mu.Lock()
	scrollbackCount := p.Scrollback.Count()
	_, rows := p.VT.Size()
	cols, _ := p.VT.Size()

	if direction < 0 {
		// Scrolling up - extend selection to top of visible area
		lineIndex := scrollbackCount - p.scrollOffset
		p.Selection[1] = Loc{X: 0, Y: lineIndex}
	} else {
		// Scrolling down - extend selection to bottom of visible area
		lineIndex := scrollbackCount - p.scrollOffset + rows - 1
		p.Selection[1] = Loc{X: cols - 1, Y: lineIndex}
	}
	p.mu.Unlock()

	if p.OnRedraw != nil {
		p.OnRedraw()
	}
}

// ExtendSelectionByKey extends the current selection based on arrow key direction
func (p *Panel) ExtendSelectionByKey(key tcell.Key) {
	p.mu.Lock()
	defer p.mu.Unlock()

	cols, rows := p.VT.Size()
	scrollbackCount := p.Scrollback.Count()
	totalLines := scrollbackCount + rows
	current := p.Selection[1]

	switch key {
	case tcell.KeyLeft:
		if current.X > 0 {
			current.X--
		} else if current.Y > 0 {
			current.Y--
			current.X = cols - 1 // Wrap to previous line
		}
	case tcell.KeyRight:
		if current.X < cols-1 {
			current.X++
		} else if current.Y < totalLines-1 {
			current.Y++
			current.X = 0 // Wrap to next line
		}
	case tcell.KeyUp:
		if current.Y > 0 {
			current.Y--
		}
	case tcell.KeyDown:
		if current.Y < totalLines-1 {
			current.Y++
		}
	}

	p.Selection[1] = current
}

// filterCPRResponses removes Cursor Position Report responses (ESC[row;colR) from output.
// These are responses to DSR (Device Status Report) queries sent by applications like
// Claude CLI, and should not be displayed as visible text.
func filterCPRResponses(data []byte) []byte {
	result := make([]byte, 0, len(data))
	i := 0
	for i < len(data) {
		// Check for ESC[
		if i+2 < len(data) && data[i] == 0x1b && data[i+1] == '[' {
			// Look for pattern: digits ; digits R
			j := i + 2
			// Read first number (row)
			for j < len(data) && data[j] >= '0' && data[j] <= '9' {
				j++
			}
			// Check for semicolon
			if j < len(data) && data[j] == ';' {
				k := j + 1
				// Read second number (col)
				for k < len(data) && data[k] >= '0' && data[k] <= '9' {
					k++
				}
				// Check for R (CPR terminator)
				if k < len(data) && data[k] == 'R' {
					// This is a CPR response, skip it entirely
					i = k + 1
					continue
				}
			}
		}
		result = append(result, data[i])
		i++
	}
	return result
}
