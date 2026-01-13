package terminal

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/creack/pty"
	"github.com/ellery/thicc/internal/llmhistory"
	"github.com/ellery/thicc/internal/screen"
	"github.com/hinshun/vt10x"
)

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
  # Not in git repo: use folder segment color (53=magenta)
  # In git repo clean: use git segment color (30=cyan)
  # In git repo dirty: use git segment color (208=orange)
  local gbg=53
  if git rev-parse --git-dir &>/dev/null; then
    gbg=30
    [[ -n "$(git status --porcelain 2>/dev/null | head -1)" ]] && gbg=208
  fi

  if (( _cmd_start > 0 )) && (( SECONDS - _cmd_start > 0 )); then
    # git_bg -> gold segment -> default
    echo "%k%F{${gbg}}%K{178}${_g_arrow}%F{black} ${_g_clock} $((SECONDS - _cmd_start))s %k%F{178}${_g_arrow}%f"
  else
    # git_bg -> default (just the ending arrow)
    echo "%k%F{${gbg}}${_g_arrow}%f"
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

  # Segment bg color: 30=cyan (clean), 208=orange (dirty)
  local bg=30
  [[ -n "$dirty" ]] && bg=208

  # Truncate branch to 12 chars
  branch="${branch:0:12}"

  # Output: [magenta arrow on git_bg][white text on git_bg][content]
  echo "%F{53}%K{${bg}}${_g_arrow}%f%F{231} ${_g_branch} ${branch}${ab} "
}

# Build prompt: [magenta] dir [dynamic git segment] [yellow time if >0] [arrow]
PROMPT='%K{53}%F{231} ${_g_folder} %1~ %k$(_git_info)$(_cmd_elapsed) '
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
  # Not in git repo: use folder segment color (53=magenta)
  # In git repo clean: use git segment color (30=cyan)
  # In git repo dirty: use git segment color (208=orange)
  local gbg=53
  if git rev-parse --git-dir &>/dev/null; then
    gbg=30
    [[ -n "$(git status --porcelain 2>/dev/null | head -1)" ]] && gbg=208
  fi

  if (( _cmd_start > 0 )) && (( SECONDS - _cmd_start > 0 )); then
    printf "\e[0m\e[38;5;${gbg}m\e[48;5;178m${_g_arrow}\e[30m ${_g_clock} %ss \e[0m\e[38;5;178m${_g_arrow}\e[0m" "$((SECONDS - _cmd_start))"
  else
    printf "\e[0m\e[38;5;${gbg}m${_g_arrow}\e[0m"
  fi
  _cmd_start=0
}
trap '_timer_start' DEBUG

_git_info() {
  local branch=$(git symbolic-ref --short HEAD 2>/dev/null)
  [[ -z "$branch" ]] && return
  local dirty=$(git status --porcelain 2>/dev/null | head -1)
  local bg=30
  [[ -n "$dirty" ]] && bg=208
  printf "\e[38;5;53m\e[48;5;${bg}m${_g_arrow}\e[97m ${_g_branch} ${branch:0:12} "
}

PS1='\[\e[48;5;53m\e[97m\] ${_g_folder} \W \[\e[0m\]$(_git_info)$(_timer_show) '
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
	Selection     [2]Loc // Start and end of selection (Y is lineIndex into scrollback+live buffer)
	mouseReleased bool   // Track mouse button state for drag detection

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

	// LLM History recording
	Recorder    *llmhistory.Recorder // Session recorder (nil if not recording)
	AIToolName  string               // Name of AI tool (e.g., "claude", "aider")
	AIToolCmd   string               // Full command string
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
	cmd.Env = append(cmd.Env, "THOCK_TERM=1") // Marker so users can customize prompt in their shell config

	// Start command with PTY
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, err
	}

	// NOW create VT emulator with PTY as writer (enables DSR/CPR responses)
	vt := vt10x.New(vt10x.WithSize(contentW, contentH), vt10x.WithWriter(ptmx))

	// Set initial PTY size (content area, not full region)
	_ = pty.Setsize(ptmx, &pty.Winsize{
		Rows: uint16(contentH),
		Cols: uint16(contentW),
	})

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

			// Stop LLM history recording if active
			if p.Recorder != nil {
				// Flush remaining live screen content to recorder
				p.flushLiveScreenToRecorder()
				p.Recorder.Stop()
				p.Recorder = nil
			}
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
			// Hold mutex during VT operations to prevent race with Resize
			p.mu.Lock()

			// Mark that we've received output (clears loading indicator)
			if !p.hasReceivedOutput {
				p.hasReceivedOutput = true
			}

			// Capture screen before write (for scroll detection)
			p.captureScreenBefore()

			// Write to VT emulator
			p.VT.Write(buf[:n])

			// Check if scroll occurred and capture scrolled lines
			// (this also feeds clean lines to the LLM history recorder)
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

						// Feed to LLM history recorder if active
						if p.Recorder != nil {
							p.Recorder.OnScrolledLine(line.ToString())
						}
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

						// Feed to LLM history recorder if active
						if p.Recorder != nil {
							p.Recorder.OnScrolledLine(line.ToString())
						}
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
	cmd.Env = append(cmd.Env, "THOCK_TERM=1")

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

	// Resize PTY to content area
	if p.PTY != nil {
		_ = pty.Setsize(p.PTY, &pty.Winsize{
			Rows: uint16(contentH),
			Cols: uint16(contentW),
		})
	}

	// Resize VT emulator to content area
	p.VT.Resize(contentW, contentH)

	return nil
}

// Close cleans up the terminal resources
func (p *Panel) Close() {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.Running = false

	// Stop LLM history recording if active
	if p.Recorder != nil {
		// Flush remaining live screen content to recorder
		p.flushLiveScreenToRecorder()
		p.Recorder.Stop()
		p.Recorder = nil
	}

	if p.redrawTimer != nil {
		p.redrawTimer.Stop()
	}

	if p.PTY != nil {
		p.PTY.Close()
		p.PTY = nil
	}

	if p.Cmd != nil && p.Cmd.Process != nil {
		_ = p.Cmd.Process.Kill()
	}
}

// SetRecorder sets the LLM history recorder for this terminal
// Call this after creating the panel but before it receives output
func (p *Panel) SetRecorder(recorder *llmhistory.Recorder) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.Recorder = recorder
}

// flushLiveScreenToRecorder captures remaining visible content and sends to recorder
// This should be called before stopping the recorder to capture content that hasn't scrolled
// Caller must hold p.mu
func (p *Panel) flushLiveScreenToRecorder() {
	if p.Recorder == nil {
		return
	}

	cols, rows := p.VT.Size()
	var lines []string

	for y := 0; y < rows; y++ {
		var line strings.Builder
		for x := 0; x < cols; x++ {
			glyph := p.VT.Cell(x, y)
			if glyph.Char == 0 {
				line.WriteRune(' ')
			} else {
				line.WriteRune(glyph.Char)
			}
		}
		// Trim trailing spaces
		trimmed := strings.TrimRight(line.String(), " ")
		if trimmed != "" {
			lines = append(lines, trimmed)
		}
	}

	if len(lines) > 0 {
		p.Recorder.FlushLiveScreen(lines)
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
