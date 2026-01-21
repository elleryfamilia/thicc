package session

import (
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/ellery/thicc/internal/util"
	"golang.org/x/sys/unix"
)

// Client connects to an existing THICC session via PTY multiplexing.
type Client struct {
	session *Session
	config  *Config
	conn    net.Conn

	// Terminal state
	origTermios *unix.Termios
	rows, cols  int

	// Session info from server
	serverVersion string
	sessionName   string

	// Disconnect info
	disconnectReason string

	// Control
	mu       sync.Mutex
	stopChan chan struct{}
	stopped  bool
}

// NewClient creates a new session client.
func NewClient(sess *Session) (*Client, error) {
	if sess.IsServer {
		return nil, fmt.Errorf("session is in server mode, not client mode")
	}

	return &Client{
		session:  sess,
		stopChan: make(chan struct{}),
	}, nil
}

// Connect establishes connection to the session server.
func (c *Client) Connect() error {
	// Get terminal size
	rows, cols, err := getTerminalSize()
	if err != nil {
		log.Printf("Session client: Failed to get terminal size: %v", err)
		rows, cols = 24, 80 // Default
	}
	c.rows = rows
	c.cols = cols

	// Connect to socket
	log.Printf("Session client: Connecting to %s", c.session.SocketPath)
	conn, err := net.DialTimeout("unix", c.session.SocketPath, 5*time.Second)
	if err != nil {
		return fmt.Errorf("failed to connect to session: %w", err)
	}
	c.conn = conn

	// Send hello
	helloPayload := EncodeHello(rows, cols, util.Version)
	if err := WriteFrame(conn, FrameHello, helloPayload); err != nil {
		conn.Close()
		return fmt.Errorf("failed to send hello: %w", err)
	}

	// Read welcome with timeout
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	frame, err := ReadFrame(conn)
	if err != nil {
		conn.Close()
		return fmt.Errorf("failed to read welcome: %w", err)
	}
	conn.SetReadDeadline(time.Time{})

	if frame.Type != FrameWelcome {
		conn.Close()
		return fmt.Errorf("expected welcome, got frame type %d", frame.Type)
	}

	accepted, sessionName, serverVersion, reason, err := DecodeWelcome(frame.Payload)
	if err != nil {
		conn.Close()
		return fmt.Errorf("failed to decode welcome: %w", err)
	}

	if !accepted {
		conn.Close()
		return fmt.Errorf("connection rejected: %s", reason)
	}

	c.sessionName = sessionName
	c.serverVersion = serverVersion

	log.Printf("Session client: Connected to session '%s' (server version: %s)",
		c.sessionName, c.serverVersion)

	return nil
}

// Run runs the client, forwarding stdin to server and server output to stdout.
// This blocks until the session ends.
func (c *Client) Run() error {
	// Put terminal in raw mode
	if err := c.makeRaw(); err != nil {
		return fmt.Errorf("failed to set raw mode: %w", err)
	}
	defer c.restore()

	// Set up signal handler for resize
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGWINCH)
	defer signal.Stop(sigChan)

	// Start goroutines
	errChan := make(chan error, 2)

	// Forward stdin to server
	go func() {
		errChan <- c.stdinLoop()
	}()

	// Forward server output to stdout
	go func() {
		errChan <- c.serverLoop()
	}()

	// Handle resize signals
	go func() {
		for {
			select {
			case <-sigChan:
				c.handleResize()
			case <-c.stopChan:
				return
			}
		}
	}()

	// Wait for either goroutine to finish
	err := <-errChan

	// Stop everything
	c.stop()

	return err
}

// stdinLoop reads from stdin and forwards to server.
func (c *Client) stdinLoop() error {
	buf := make([]byte, 4096) // Larger buffer for mouse sequences

	for {
		// Read from stdin (this will block)
		n, err := os.Stdin.Read(buf)
		if err != nil {
			if err == io.EOF {
				return nil
			}
			return fmt.Errorf("stdin read error: %w", err)
		}

		if n > 0 {
			if err := WriteFrame(c.conn, FrameData, buf[:n]); err != nil {
				return fmt.Errorf("failed to send data: %w", err)
			}
		}
	}
}

// serverLoop reads from server and writes to stdout.
func (c *Client) serverLoop() error {
	for {
		frame, err := ReadFrame(c.conn)
		if err != nil {
			// Check if we're shutting down
			select {
			case <-c.stopChan:
				return nil
			default:
			}
			if err == io.EOF {
				c.disconnectReason = "Connection closed"
				return nil
			}
			return fmt.Errorf("server read error: %w", err)
		}

		switch frame.Type {
		case FrameData:
			// Write to stdout
			if _, err := os.Stdout.Write(frame.Payload); err != nil {
				return fmt.Errorf("stdout write error: %w", err)
			}

		case FrameClose:
			c.disconnectReason = string(frame.Payload)
			return nil

		default:
			log.Printf("Session client: Unknown frame type: %d", frame.Type)
		}
	}
}

// handleResize handles terminal resize events.
func (c *Client) handleResize() {
	rows, cols, err := getTerminalSize()
	if err != nil {
		log.Printf("Session client: Failed to get terminal size: %v", err)
		return
	}

	if rows == c.rows && cols == c.cols {
		return // No change
	}

	c.rows = rows
	c.cols = cols

	resizePayload := EncodeResize(rows, cols)
	if err := WriteFrame(c.conn, FrameResize, resizePayload); err != nil {
		log.Printf("Session client: Failed to send resize: %v", err)
	}
}

// makeRaw puts the terminal in raw mode.
func (c *Client) makeRaw() error {
	fd := int(os.Stdin.Fd())

	termios, err := unix.IoctlGetTermios(fd, unix.TCGETS)
	if err != nil {
		return err
	}

	c.origTermios = termios

	// Make a copy and modify for raw mode
	raw := *termios
	raw.Iflag &^= unix.IGNBRK | unix.BRKINT | unix.PARMRK | unix.ISTRIP | unix.INLCR | unix.IGNCR | unix.ICRNL | unix.IXON
	raw.Oflag &^= unix.OPOST
	raw.Lflag &^= unix.ECHO | unix.ECHONL | unix.ICANON | unix.ISIG | unix.IEXTEN
	raw.Cflag &^= unix.CSIZE | unix.PARENB
	raw.Cflag |= unix.CS8
	raw.Cc[unix.VMIN] = 1
	raw.Cc[unix.VTIME] = 0

	if err := unix.IoctlSetTermios(fd, unix.TCSETS, &raw); err != nil {
		return err
	}

	return nil
}

// restore restores the terminal to its original state.
func (c *Client) restore() {
	if c.origTermios != nil {
		fd := int(os.Stdin.Fd())
		unix.IoctlSetTermios(fd, unix.TCSETS, c.origTermios)
	}
}

// stop signals all goroutines to stop.
func (c *Client) stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.stopped {
		return
	}
	c.stopped = true

	close(c.stopChan)
}

// Disconnect closes the connection to the server.
func (c *Client) Disconnect() {
	c.stop()

	if c.conn != nil {
		// Send close frame
		WriteFrame(c.conn, FrameClose, []byte("Client disconnecting"))
		c.conn.Close()
	}

	c.restore()
}

// DisconnectReason returns the reason for disconnection.
func (c *Client) DisconnectReason() string {
	return c.disconnectReason
}

// SessionName returns the session name.
func (c *Client) SessionName() string {
	return c.sessionName
}

// ShowDisconnectMessage displays a disconnect message and waits for a keypress.
func (c *Client) ShowDisconnectMessage() {
	c.restore()

	reason := c.DisconnectReason()
	isPaused := reason == CloseReasons.NewClient || reason == "Local user took over"

	fmt.Printf("\n\r")
	if isPaused {
		fmt.Printf("\033[7m") // Reverse video
		fmt.Printf("                                                                \r\n")
		fmt.Printf("  Session taken over by another terminal                        \r\n")
		fmt.Printf("  Reconnect with: thicc (in the same project directory)         \r\n")
		fmt.Printf("                                                                \r\n")
		fmt.Printf("\033[0m")
	} else {
		if reason == "" {
			reason = "Connection closed"
		}
		fmt.Printf("Session disconnected: %s\n\r", reason)
	}

	fmt.Printf("\n\rPress Enter to exit...\n\r")
	buf := make([]byte, 1)
	os.Stdin.Read(buf)
}

// getTerminalSize returns the current terminal size.
func getTerminalSize() (rows, cols int, err error) {
	ws, err := unix.IoctlGetWinsize(int(os.Stdout.Fd()), unix.TIOCGWINSZ)
	if err != nil {
		return 0, 0, err
	}
	return int(ws.Row), int(ws.Col), nil
}
