package terminal

import (
	"io"
	"os"
	"os/exec"
	"sync"
	"time"

	"github.com/creack/pty"
	"github.com/hinshun/vt10x"
)

// Region defines a rectangular screen region (same as filebrowser)
type Region struct {
	X      int
	Y      int
	Width  int
	Height int
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
}

// NewPanel creates a new terminal panel
// cmdArgs is the command to run (defaults to user's shell if nil/empty)
func NewPanel(x, y, w, h int, cmdArgs []string) (*Panel, error) {
	// Determine if we should auto-respawn shell when process exits
	// (true when running an AI tool, false when running default shell)
	autoRespawn := cmdArgs != nil && len(cmdArgs) > 0

	// Default to user's shell if no command specified
	if cmdArgs == nil || len(cmdArgs) == 0 {
		cmdArgs = []string{getDefaultShell()}
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
	}

	// Start reading from PTY in background
	go p.readLoop()

	return p, nil
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
