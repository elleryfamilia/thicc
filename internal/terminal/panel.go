package terminal

import (
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/creack/pty"
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
	Selection     [2]Loc // Start and end of selection
	mouseReleased bool   // Track mouse button state for drag detection
}

// NewPanel creates a new terminal panel
// cmdArgs is the command to run (defaults to user's shell if nil/empty)
func NewPanel(x, y, w, h int, cmdArgs []string) (*Panel, error) {
	// Determine if we should auto-respawn shell when process exits
	// (true when running an AI tool, false when running default shell)
	autoRespawn := cmdArgs != nil && len(cmdArgs) > 0

	// Track if we're starting a default shell (to inject sexy prompt)
	injectPrompt := false

	log.Printf("THICC: NewPanel called with cmdArgs=%v (nil=%v, len=%d)", cmdArgs, cmdArgs == nil, len(cmdArgs))

	// Default to user's shell if no command specified
	if cmdArgs == nil || len(cmdArgs) == 0 {
		shell := getDefaultShell()
		cmdArgs = []string{shell, "-i"} // Just start interactive shell
		injectPrompt = true             // We'll inject sexy prompt after shell starts
		log.Printf("THICC: Using default shell: %s", shell)
	} else {
		log.Printf("THICC: Using provided cmdArgs: %v", cmdArgs)
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
	}

	// Start reading from PTY in background
	go p.readLoop()

	// Inject sexy prompt after shell fully initializes
	if injectPrompt {
		log.Printf("THICC: Will inject sexy prompt for shell")
		go p.injectSexyPrompt()
	} else {
		log.Printf("THICC: Not injecting prompt (cmdArgs provided)")
	}

	return p, nil
}

// injectSexyPrompt sends prompt initialization to the shell after it starts
// This ensures our prompt overrides anything set in rc files
func (p *Panel) injectSexyPrompt() {
	log.Printf("THICC: injectSexyPrompt starting, waiting 500ms...")
	// Wait for shell to fully initialize (load rc files, oh-my-zsh, etc.)
	time.Sleep(500 * time.Millisecond)

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
	log.Printf("THICC: injectSexyPrompt - sending command: %s", initCmd)

	// Send command to override prompt, then clear screen for clean look
	fullCmd := initCmd + "\nclear\n"
	n, err := pty.Write([]byte(fullCmd))
	log.Printf("THICC: injectSexyPrompt - wrote %d bytes, err=%v", n, err)
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
			}
			return
		}

		if n > 0 {
			// Write to VT emulator
			p.VT.Write(buf[:n])

			// Schedule throttled redraw
			p.scheduleRedraw()
		}
	}
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

	// Start new read loop
	go p.readLoop()

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

	var result string
	for y := start.Y; y <= end.Y && y < rows; y++ {
		lineStart := 0
		lineEnd := cols

		if y == start.Y {
			lineStart = start.X
		}
		if y == end.Y {
			lineEnd = end.X
		}

		for x := lineStart; x < lineEnd && x < cols; x++ {
			glyph := p.VT.Cell(x, y)
			r := glyph.Char
			if r == 0 {
				r = ' '
			}
			result += string(r)
		}

		// Add newline between lines (but not at the end)
		if y < end.Y {
			result += "\n"
		}
	}

	return result
}

// isSelected returns true if the given cell position is within the selection
func (p *Panel) isSelected(x, y int) bool {
	if !p.HasSelection() {
		return false
	}

	loc := Loc{X: x, Y: y}
	start := p.Selection[0]
	end := p.Selection[1]

	// Handle selection in either direction
	if start.GreaterEqual(end) {
		start, end = end, start
	}

	return loc.GreaterEqual(start) && loc.LessThan(end)
}
