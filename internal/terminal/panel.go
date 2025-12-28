package terminal

import (
	"io"
	"os"
	"os/exec"
	"sync"

	"github.com/creack/pty"
	"github.com/hinshun/vt10x"
	"github.com/ellery/thock/internal/aiterminal"
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
	Tool    *aiterminal.AITool
	Running bool
	mu      sync.Mutex
}

// NewPanel creates a new terminal panel
func NewPanel(x, y, w, h int, tool *aiterminal.AITool) (*Panel, error) {
	// Create VT emulator with specified size
	vt := vt10x.New(vt10x.WithSize(w, h))

	// Get command line from tool
	cmdLine := tool.GetCommandLine()
	cmd := exec.Command(cmdLine[0], cmdLine[1:]...)

	// Set up environment
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, "TERM=xterm-256color")

	// Start command with PTY
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return nil, err
	}

	// Set initial PTY size
	_ = pty.Setsize(ptmx, &pty.Winsize{
		Rows: uint16(h),
		Cols: uint16(w),
	})

	p := &Panel{
		VT:      vt,
		PTY:     ptmx,
		Cmd:     cmd,
		Region:  Region{X: x, Y: y, Width: w, Height: h},
		Tool:    tool,
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

	// Copy PTY output to VT emulator
	// This blocks until PTY is closed
	_, _ = io.Copy(p.VT, p.PTY)
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

	// Resize PTY
	if p.PTY != nil {
		_ = pty.Setsize(p.PTY, &pty.Winsize{
			Rows: uint16(h),
			Cols: uint16(w),
		})
	}

	// Resize VT emulator
	p.VT.Resize(w, h)

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
