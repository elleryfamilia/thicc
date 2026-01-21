package session

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
	"github.com/ellery/thicc/internal/util"
	"golang.org/x/sys/unix"
)

// Server manages a PTY-based session server that accepts client connections.
// It spawns THICC as a child process in a PTY and forwards I/O.
// The server also acts as the "local client" - when no socket client is connected,
// it forwards its own stdin/stdout to the PTY.
type Server struct {
	session  *Session
	config   *Config
	listener net.Listener
	lockFile *os.File

	// PTY and child process
	ptyFile *os.File
	cmd     *exec.Cmd

	// Current socket client (single client at a time)
	mu         sync.RWMutex
	client     net.Conn
	clientRows int
	clientCols int

	// Local terminal state (for when no socket client is connected)
	localMode   bool // true = using local terminal, false = socket client active
	origTermios *unix.Termios

	// Control
	stopChan  chan struct{}
	stopped   bool
	childDone chan struct{}
}

// NewServer creates a new PTY-based session server.
func NewServer(sess *Session, cfg *Config) (*Server, error) {
	log.Printf("Session: NewServer called - SocketPath=%s", sess.SocketPath)

	if !sess.IsServer {
		return nil, fmt.Errorf("session is not in server mode")
	}

	// Acquire lock to prevent race conditions
	log.Printf("Session: Acquiring lock for %s", sess.SocketPath)
	lockFile, err := AcquireSessionLock(sess.SocketPath)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire session lock: %w", err)
	}
	log.Printf("Session: Lock acquired")

	// Clean up any stale socket
	os.Remove(sess.SocketPath)

	// Create listener
	log.Printf("Session: Creating Unix socket listener at %s", sess.SocketPath)
	listener, err := net.Listen("unix", sess.SocketPath)
	if err != nil {
		ReleaseSessionLock(lockFile)
		return nil, fmt.Errorf("failed to create socket listener: %w", err)
	}
	log.Printf("Session: Socket listener created successfully")

	return &Server{
		session:   sess,
		config:    cfg,
		listener:  listener,
		lockFile:  lockFile,
		localMode: true, // Start in local mode
		stopChan:  make(chan struct{}),
		childDone: make(chan struct{}),
	}, nil
}

// Start spawns the THICC child process and begins serving.
func (s *Server) Start(thiccArgs []string) error {
	// Get the path to our own executable
	thiccPath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("failed to get executable path: %w", err)
	}

	// Get current terminal size for PTY
	rows, cols := 24, 80
	if ws, err := unix.IoctlGetWinsize(int(os.Stdout.Fd()), unix.TIOCGWINSZ); err == nil {
		rows, cols = int(ws.Row), int(ws.Col)
	}

	// Build command: thicc --no-session --skip-dashboard [original args]
	args := append([]string{"--no-session", "--skip-dashboard"}, thiccArgs...)
	s.cmd = exec.Command(thiccPath, args...)
	s.cmd.Dir = s.config.ProjectRoot
	s.cmd.Env = os.Environ()

	// Start with a PTY
	log.Printf("Session: Starting THICC child with args: %v in dir: %s", args, s.config.ProjectRoot)
	s.ptyFile, err = pty.StartWithSize(s.cmd, &pty.Winsize{
		Rows: uint16(rows),
		Cols: uint16(cols),
	})
	if err != nil {
		return fmt.Errorf("failed to start PTY: %w", err)
	}
	log.Printf("Session: PTY started, child PID: %d, size: %dx%d", s.cmd.Process.Pid, cols, rows)

	// Put local terminal in raw mode
	if err := s.makeRaw(); err != nil {
		log.Printf("Session: Warning: failed to set raw mode: %v", err)
	}

	// Set up signal handler for local terminal resize
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGWINCH)
	go s.handleLocalResize(sigChan)

	// Monitor child process
	go s.monitorChild()

	// Start accepting socket connections
	go s.acceptLoop()

	// Forward PTY output to local terminal or socket client
	go s.ptyReadLoop()

	// Forward local stdin to PTY (when in local mode)
	go s.localStdinLoop()

	// Enable mouse mode on local terminal
	// tcell in the child will also send these, but we send them explicitly
	// to ensure mouse works from the start (PTY might delay tcell's sequences)
	mouseEnableSeq := "\x1b[?1000h" + // Basic mouse tracking
		"\x1b[?1002h" + // Button-event tracking
		"\x1b[?1003h" + // Any-event tracking
		"\x1b[?1006h" // SGR extended mode
	os.Stdout.WriteString(mouseEnableSeq)

	return nil
}

// makeRaw puts the local terminal in raw mode.
func (s *Server) makeRaw() error {
	fd := int(os.Stdin.Fd())

	termios, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		return err
	}

	s.origTermios = termios

	// Make a copy and modify for raw mode
	raw := *termios
	raw.Iflag &^= unix.IGNBRK | unix.BRKINT | unix.PARMRK | unix.ISTRIP | unix.INLCR | unix.IGNCR | unix.ICRNL | unix.IXON
	raw.Oflag &^= unix.OPOST
	raw.Lflag &^= unix.ECHO | unix.ECHONL | unix.ICANON | unix.ISIG | unix.IEXTEN
	raw.Cflag &^= unix.CSIZE | unix.PARENB
	raw.Cflag |= unix.CS8
	raw.Cc[unix.VMIN] = 1
	raw.Cc[unix.VTIME] = 0

	return unix.IoctlSetTermios(fd, unix.TCSETS, &raw)
}

// restore restores the local terminal to its original state.
func (s *Server) restore() {
	if s.origTermios != nil {
		fd := int(os.Stdin.Fd())
		unix.IoctlSetTermios(fd, unix.TCSETS, s.origTermios)
	}
}

// handleLocalResize handles SIGWINCH for local terminal resize.
func (s *Server) handleLocalResize(sigChan <-chan os.Signal) {
	for {
		select {
		case <-sigChan:
			s.mu.RLock()
			isLocal := s.localMode
			s.mu.RUnlock()

			if isLocal {
				if ws, err := unix.IoctlGetWinsize(int(os.Stdout.Fd()), unix.TIOCGWINSZ); err == nil {
					s.resizePTY(int(ws.Row), int(ws.Col))
				}
			}
		case <-s.stopChan:
			return
		case <-s.childDone:
			return
		}
	}
}

// monitorChild waits for the child process to exit.
func (s *Server) monitorChild() {
	err := s.cmd.Wait()
	log.Printf("Session: Child process exited: %v", err)

	// Signal that child is done
	close(s.childDone)

	// Notify connected socket client
	s.mu.Lock()
	if s.client != nil {
		WriteFrame(s.client, FrameClose, []byte(CloseReasons.ChildExited))
		// Give client time to receive the close message before closing connection
		time.Sleep(50 * time.Millisecond)
		s.client.Close()
		s.client = nil
	}
	s.mu.Unlock()

	// Trigger server shutdown
	s.Stop()
}

// acceptLoop accepts incoming socket client connections.
func (s *Server) acceptLoop() {
	for {
		select {
		case <-s.stopChan:
			return
		case <-s.childDone:
			return
		default:
		}

		// Set accept timeout so we can check stop channels
		if ul, ok := s.listener.(*net.UnixListener); ok {
			ul.SetDeadline(time.Now().Add(1 * time.Second))
		}

		conn, err := s.listener.Accept()
		if err != nil {
			if netErr, ok := err.(net.Error); ok && netErr.Timeout() {
				continue
			}
			select {
			case <-s.stopChan:
				return
			case <-s.childDone:
				return
			default:
				log.Printf("Session: Accept error: %v", err)
				continue
			}
		}

		go s.handleClient(conn)
	}
}

// handleClient processes a new socket client connection.
func (s *Server) handleClient(conn net.Conn) {
	log.Printf("Session: New socket client connected")

	// Read hello frame with timeout
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	frame, err := ReadFrame(conn)
	if err != nil {
		log.Printf("Session: Failed to read hello: %v", err)
		conn.Close()
		return
	}
	conn.SetReadDeadline(time.Time{})

	if frame.Type != FrameHello {
		log.Printf("Session: Expected hello, got frame type %d", frame.Type)
		conn.Close()
		return
	}

	rows, cols, clientVersion, err := DecodeHello(frame.Payload)
	if err != nil {
		log.Printf("Session: Failed to decode hello: %v", err)
		conn.Close()
		return
	}

	log.Printf("Session: Client hello: %dx%d, version=%s", cols, rows, clientVersion)

	// Version check (for now, require exact match)
	if clientVersion != util.Version {
		log.Printf("Session: Version mismatch: client=%s server=%s", clientVersion, util.Version)
		welcomePayload := EncodeWelcome(false, s.session.Name, util.Version,
			fmt.Sprintf("version mismatch: client=%s server=%s", clientVersion, util.Version))
		WriteFrame(conn, FrameWelcome, welcomePayload)
		conn.Close()
		return
	}

	// Kick previous socket client if any, and disable local mode
	s.mu.Lock()
	if s.client != nil {
		log.Printf("Session: Kicking previous socket client")
		WriteFrame(s.client, FrameClose, []byte(CloseReasons.NewClient))
		// Give client time to receive the close message before closing connection
		time.Sleep(50 * time.Millisecond)
		s.client.Close()
	}
	s.client = conn
	s.clientRows = rows
	s.clientCols = cols
	wasLocalMode := s.localMode
	s.localMode = false // Disable local terminal
	s.mu.Unlock()

	// Show "paused" message on local terminal
	if wasLocalMode {
		s.showLocalPausedMessage()
	}

	// Send welcome
	welcomePayload := EncodeWelcome(true, s.session.Name, util.Version, "")
	if err := WriteFrame(conn, FrameWelcome, welcomePayload); err != nil {
		log.Printf("Session: Failed to send welcome: %v", err)
		s.clearClient(conn)
		return
	}

	// Send mouse-enable escape sequences to the new client's terminal
	// These sequences enable various mouse tracking modes that tcell uses
	mouseEnableSeq := "\x1b[?1000h" + // Basic mouse tracking
		"\x1b[?1002h" + // Button-event tracking
		"\x1b[?1003h" + // Any-event tracking
		"\x1b[?1006h" // SGR extended mode
	WriteFrame(conn, FrameData, []byte(mouseEnableSeq))

	// Resize PTY to match socket client
	s.resizePTY(rows, cols)

	// Read loop for socket client input
	s.clientReadLoop(conn)

	// Socket client disconnected - re-enable local mode
	// Note: The server is still running. The LOCAL terminal can now control THICC.
	// To quit completely, press Ctrl+Q from the local terminal.
	log.Printf("Session: Socket client disconnected, resuming local mode")
	s.mu.Lock()
	if s.client == conn {
		s.client = nil
		s.localMode = true
		s.mu.Unlock()

		// Re-enable mouse mode on local terminal
		s.showLocalResumedMessage()

		// Force redraw by doing a resize (even if same size, SIGWINCH triggers tcell redraw)
		if ws, err := unix.IoctlGetWinsize(int(os.Stdout.Fd()), unix.TIOCGWINSZ); err == nil {
			// Do a "resize dance" - resize to different size then back to force redraw
			s.resizePTY(int(ws.Row)-1, int(ws.Col))
			time.Sleep(10 * time.Millisecond)
			s.resizePTY(int(ws.Row), int(ws.Col))
		}
	} else {
		s.mu.Unlock()
	}
}

// clientReadLoop reads frames from the socket client and forwards input to PTY.
func (s *Server) clientReadLoop(conn net.Conn) {
	for {
		// Read frame - no timeout for data, we want low latency
		frame, err := ReadFrame(conn)
		if err != nil {
			// Check if we're shutting down
			select {
			case <-s.stopChan:
				return
			case <-s.childDone:
				return
			default:
			}
			if err == io.EOF {
				return
			}
			log.Printf("Session: Client read error: %v", err)
			return
		}

		switch frame.Type {
		case FrameData:
			// Check for Ctrl+Q (0x11) from socket clients - disconnect them instead of forwarding
			// This allows clients to "detach" without killing the session
			for _, b := range frame.Payload {
				if b == 0x11 { // Ctrl+Q
					log.Printf("Session: Socket client pressed Ctrl+Q, disconnecting")
					WriteFrame(conn, FrameClose, []byte("Detached from session (Ctrl+Q)"))
					return
				}
			}

			// Forward to PTY
			if s.ptyFile != nil {
				_, err := s.ptyFile.Write(frame.Payload)
				if err != nil {
					log.Printf("Session: PTY write error: %v", err)
					return
				}
			}

		case FrameResize:
			rows, cols, err := DecodeResize(frame.Payload)
			if err != nil {
				log.Printf("Session: Invalid resize frame: %v", err)
				continue
			}
			log.Printf("Session: Socket client resize to %dx%d", cols, rows)
			s.mu.Lock()
			s.clientRows = rows
			s.clientCols = cols
			s.mu.Unlock()
			s.resizePTY(rows, cols)

		case FrameClose:
			log.Printf("Session: Socket client sent close")
			return

		default:
			log.Printf("Session: Unknown frame type: %d", frame.Type)
		}
	}
}

// localStdinLoop reads from local stdin and forwards to PTY when in local mode.
// When a client is connected (not in local mode), Ctrl+C will force disconnect the client.
func (s *Server) localStdinLoop() {
	buf := make([]byte, 4096) // Larger buffer for mouse sequences

	for {
		// Read from stdin (this blocks)
		n, err := os.Stdin.Read(buf)
		if err != nil {
			// Check if we're shutting down
			select {
			case <-s.stopChan:
				return
			case <-s.childDone:
				return
			default:
			}
			if err == io.EOF {
				return
			}
			log.Printf("Session: Local stdin read error: %v", err)
			return
		}

		if n > 0 {
			s.mu.RLock()
			isLocal := s.localMode
			client := s.client
			s.mu.RUnlock()

			if isLocal {
				// Forward to PTY
				if s.ptyFile != nil {
					_, err := s.ptyFile.Write(buf[:n])
					if err != nil {
						log.Printf("Session: PTY write error from local: %v", err)
						return
					}
				}
			} else if client != nil {
				// Check for Ctrl+C (0x03) to force disconnect client
				for i := 0; i < n; i++ {
					if buf[i] == 0x03 { // Ctrl+C
						log.Printf("Session: Ctrl+C received, forcing client disconnect")
						s.mu.Lock()
						if s.client != nil {
							WriteFrame(s.client, FrameClose, []byte("Local user took over"))
							// Give client time to receive the close message before closing connection
							time.Sleep(50 * time.Millisecond)
							s.client.Close()
							s.client = nil
							s.localMode = true
						}
						s.mu.Unlock()

						// Re-enable mouse mode and trigger redraw
						s.showLocalResumedMessage()
						if ws, err := unix.IoctlGetWinsize(int(os.Stdout.Fd()), unix.TIOCGWINSZ); err == nil {
							// Do a "resize dance" to force redraw
							s.resizePTY(int(ws.Row)-1, int(ws.Col))
							time.Sleep(10 * time.Millisecond)
							s.resizePTY(int(ws.Row), int(ws.Col))
						}
						break
					}
				}
			}
		}
	}
}

// ptyReadLoop reads from PTY and forwards to local terminal or socket client.
func (s *Server) ptyReadLoop() {
	buf := make([]byte, 32*1024) // 32KB buffer

	for {
		n, err := s.ptyFile.Read(buf)
		if err != nil {
			// Check if we're shutting down
			select {
			case <-s.stopChan:
				return
			case <-s.childDone:
				return
			default:
			}
			if err == io.EOF {
				log.Printf("Session: PTY EOF")
				return
			}
			log.Printf("Session: PTY read error: %v", err)
			return
		}

		if n > 0 {
			s.mu.RLock()
			isLocal := s.localMode
			client := s.client
			s.mu.RUnlock()

			if isLocal {
				// Write to local stdout
				_, err := os.Stdout.Write(buf[:n])
				if err != nil {
					log.Printf("Session: Local stdout write error: %v", err)
				}
			} else if client != nil {
				// Write to socket client
				if err := WriteFrame(client, FrameData, buf[:n]); err != nil {
					log.Printf("Session: Failed to write to socket client: %v", err)
					// Don't return - the client might disconnect and we resume local
				}
			}
		}
	}
}

// showLocalPausedMessage shows a message on the local terminal when a client takes over.
func (s *Server) showLocalPausedMessage() {
	log.Printf("Session: showLocalPausedMessage called")
	// Clear screen and show message
	// Using ANSI escape codes since we're in raw mode
	msg := "\033[2J\033[H" + // Clear screen, move to top-left
		"\033[7m" + // Reverse video
		"                                                                \r\n" +
		"  Session taken over by remote client                          \r\n" +
		"  This terminal is paused until the client disconnects         \r\n" +
		"                                                                \r\n" +
		"  Press Ctrl+C to force disconnect the client and take over    \r\n" +
		"                                                                \r\n" +
		"\033[0m" // Reset

	os.Stdout.WriteString(msg)
}

// showLocalResumedMessage shows a message when resuming local control.
func (s *Server) showLocalResumedMessage() {
	log.Printf("Session: showLocalResumedMessage called")

	// Re-enable mouse mode on local terminal BEFORE clearing
	// These sequences enable various mouse tracking modes that tcell uses
	mouseEnableSeq := "\x1b[?1000h" + // Basic mouse tracking
		"\x1b[?1002h" + // Button-event tracking
		"\x1b[?1003h" + // Any-event tracking
		"\x1b[?1006h" // SGR extended mode
	os.Stdout.WriteString(mouseEnableSeq)

	// Don't clear screen - let the resize trigger a proper redraw from tcell
	// Clearing the screen can cause a blank screen if the redraw doesn't happen immediately
}

// resizePTY resizes the PTY to the given dimensions.
func (s *Server) resizePTY(rows, cols int) {
	if s.ptyFile == nil {
		return
	}

	winSize := &pty.Winsize{
		Rows: uint16(rows),
		Cols: uint16(cols),
	}

	if err := pty.Setsize(s.ptyFile, winSize); err != nil {
		log.Printf("Session: Failed to resize PTY: %v", err)
	} else {
		log.Printf("Session: PTY resized to %dx%d", cols, rows)
	}

	// Explicitly send SIGWINCH to child to force redraw and re-enable mouse mode
	if s.cmd != nil && s.cmd.Process != nil {
		s.cmd.Process.Signal(syscall.SIGWINCH)
	}
}

// clearClient clears the socket client if it matches the given connection.
func (s *Server) clearClient(conn net.Conn) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.client == conn {
		s.client = nil
		s.localMode = true
	}
}

// Stop shuts down the server.
func (s *Server) Stop() {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return
	}
	s.stopped = true
	s.mu.Unlock()

	log.Printf("Session: Stopping server")

	// Restore terminal
	s.restore()

	// Close stop channel
	select {
	case <-s.stopChan:
		// Already closed
	default:
		close(s.stopChan)
	}

	// Notify and close socket client
	s.mu.Lock()
	if s.client != nil {
		WriteFrame(s.client, FrameClose, []byte(CloseReasons.ServerShutdown))
		// Give client time to receive the close message before closing connection
		time.Sleep(50 * time.Millisecond)
		s.client.Close()
		s.client = nil
	}
	s.mu.Unlock()

	// Close PTY (this will cause child to receive SIGHUP)
	if s.ptyFile != nil {
		s.ptyFile.Close()
	}

	// Kill child process if still running
	if s.cmd != nil && s.cmd.Process != nil {
		s.cmd.Process.Kill()
	}

	// Close listener
	if s.listener != nil {
		s.listener.Close()
	}

	// Clean up socket
	os.Remove(s.session.SocketPath)

	// Release lock
	ReleaseSessionLock(s.lockFile)

	log.Printf("Session: Server stopped")
}

// Wait blocks until the child process exits.
func (s *Server) Wait() error {
	<-s.childDone
	return nil
}

// SocketPath returns the path to the session socket.
func (s *Server) SocketPath() string {
	return s.session.SocketPath
}

// SessionName returns the session name.
func (s *Server) SessionName() string {
	return s.session.Name
}

// HasClient returns true if a socket client is currently connected.
func (s *Server) HasClient() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.client != nil
}
