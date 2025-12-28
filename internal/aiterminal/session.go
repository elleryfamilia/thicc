package aiterminal

import (
	"io"
	"os"
	"os/exec"
	"sync"

	"github.com/creack/pty"
)

// Session represents a terminal session with an AI CLI or shell
type Session struct {
	Tool    *AITool
	PTY     *os.File
	Cmd     *exec.Cmd
	Running bool
	mu      sync.Mutex

	// Callbacks
	OnOutput func([]byte) // Called when output is received
	OnExit   func(error)  // Called when process exits
}

// NewSession creates a new terminal session for the given tool
func NewSession(tool *AITool) (*Session, error) {
	s := &Session{
		Tool:    tool,
		Running: false,
	}

	return s, nil
}

// Start launches the AI CLI or shell process
func (s *Session) Start() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.Running {
		return nil // Already running
	}

	// Create the command
	cmdLine := s.Tool.GetCommandLine()
	if len(cmdLine) == 0 {
		return io.EOF // No command to run
	}

	cmd := exec.Command(cmdLine[0], cmdLine[1:]...)

	// Set environment variables
	cmd.Env = os.Environ()

	// Start with a PTY
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return err
	}

	s.Cmd = cmd
	s.PTY = ptmx
	s.Running = true

	// Start output reader
	go s.readOutput()

	// Wait for process to exit
	go s.waitForExit()

	return nil
}

// readOutput reads from PTY and calls OnOutput callback
func (s *Session) readOutput() {
	buf := make([]byte, 8192)
	for {
		n, err := s.PTY.Read(buf)
		if err != nil {
			if err != io.EOF {
				// Error reading
			}
			return
		}

		if n > 0 && s.OnOutput != nil {
			data := make([]byte, n)
			copy(data, buf[:n])
			s.OnOutput(data)
		}
	}
}

// waitForExit waits for the process to exit
func (s *Session) waitForExit() {
	if s.Cmd == nil || s.Cmd.Process == nil {
		return
	}

	err := s.Cmd.Wait()

	s.mu.Lock()
	s.Running = false
	s.mu.Unlock()

	if s.OnExit != nil {
		s.OnExit(err)
	}
}

// Write sends input to the terminal
func (s *Session) Write(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.Running || s.PTY == nil {
		return io.EOF
	}

	_, err := s.PTY.Write(data)
	return err
}

// Resize resizes the terminal
func (s *Session) Resize(rows, cols int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.Running || s.PTY == nil {
		return nil
	}

	return pty.Setsize(s.PTY, &pty.Winsize{
		Rows: uint16(rows),
		Cols: uint16(cols),
	})
}

// Stop stops the session
func (s *Session) Stop() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.Running {
		return nil
	}

	// Close PTY
	if s.PTY != nil {
		s.PTY.Close()
	}

	// Kill process if still running
	if s.Cmd != nil && s.Cmd.Process != nil {
		s.Cmd.Process.Kill()
	}

	s.Running = false
	return nil
}

// IsRunning returns whether the session is active
func (s *Session) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.Running
}
