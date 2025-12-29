package terminal

import (
	"io"
	"os"
	"os/exec"
	"sync"

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
}

// NewPanel creates a new terminal panel
// cmdArgs is the command to run (defaults to user's shell if nil/empty)
func NewPanel(x, y, w, h int, cmdArgs []string) (*Panel, error) {
	// Default to user's shell if no command specified
	if cmdArgs == nil || len(cmdArgs) == 0 {
		shell := os.Getenv("SHELL")
		if shell == "" {
			shell = "/bin/bash"
		}
		cmdArgs = []string{shell}
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
	vt := vt10x.New(vt10x.WithSize(contentW, contentH))

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

	// Set initial PTY size (content area, not full region)
	_ = pty.Setsize(ptmx, &pty.Winsize{
		Rows: uint16(contentH),
		Cols: uint16(contentW),
	})

	p := &Panel{
		VT:      vt,
		PTY:     ptmx,
		Cmd:     cmd,
		Region:  Region{X: x, Y: y, Width: w, Height: h},
		Running: true,
		Focus:   false,
	}

	// Start reading from PTY in background
	go p.readLoop()

	return p, nil
}

// readLoop continuously reads from PTY and writes to VT emulator
func (p *Panel) readLoop() {
	defer func() {
		p.mu.Lock()
		p.Running = false
		p.mu.Unlock()
	}()

	buf := make([]byte, 4096)
	for {
		n, err := p.PTY.Read(buf)
		if err != nil {
			// EOF or error - terminal closed
			return
		}

		if n > 0 {
			// Write to VT emulator
			p.VT.Write(buf[:n])

			// Trigger screen refresh
			if p.OnRedraw != nil {
				p.OnRedraw()
			}
		}
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
