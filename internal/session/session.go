// Package session provides PTY-based session multiplexing for THICC.
// This allows multiple terminals to connect to the same THICC session,
// enabling access from different devices without a terminal multiplexer.
//
// Architecture:
// - Server spawns THICC child process in a PTY
// - Raw terminal bytes are forwarded between PTY and connected clients
// - Only one client connected at a time (new connection kicks previous)
package session

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// Session represents a THICC session that can be shared across clients.
type Session struct {
	// Name is the session identifier (either a hash of project path or custom name)
	Name string

	// SocketPath is the full path to the Unix socket
	SocketPath string

	// IsServer indicates whether this process is the server (true) or client (false)
	IsServer bool

	// ProjectRoot is the original project path (only set for project-based sessions)
	ProjectRoot string
}

// Config contains session configuration options.
type Config struct {
	// SessionName is an explicit session name (overrides project-based naming)
	// If empty, uses project root path hash
	SessionName string

	// ProjectRoot is the project directory path
	ProjectRoot string

	// Disabled completely disables session sharing
	Disabled bool
}

var (
	// ErrDisabled is returned when session sharing is disabled
	ErrDisabled = errors.New("session sharing is disabled")

	// ErrNoSession is returned when no existing session was found
	ErrNoSession = errors.New("no existing session found")
)

// GetSessionDir returns the directory where session sockets are stored.
// Creates the directory if it doesn't exist.
func GetSessionDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}

	dir := filepath.Join(home, ".thicc", "sessions")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("failed to create sessions directory: %w", err)
	}

	return dir, nil
}

// hashProjectPath creates a short hash of the project path for socket naming.
func hashProjectPath(path string) string {
	// Normalize the path
	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	// Create SHA256 hash and take first 16 chars
	hash := sha256.Sum256([]byte(absPath))
	return hex.EncodeToString(hash[:])[:16]
}

// GetSocketPath returns the socket path for a session.
func GetSocketPath(cfg *Config) (string, error) {
	dir, err := GetSessionDir()
	if err != nil {
		return "", err
	}

	var name string
	if cfg.SessionName != "" {
		// Custom named session
		name = cfg.SessionName
	} else {
		// Project-based session (hash of project path)
		name = hashProjectPath(cfg.ProjectRoot)
	}

	return filepath.Join(dir, name+".sock"), nil
}

// CheckExistingSession checks if a session already exists for the given config.
// Returns the socket path if a valid session exists, empty string otherwise.
func CheckExistingSession(cfg *Config) (string, error) {
	if cfg.Disabled {
		return "", ErrDisabled
	}

	socketPath, err := GetSocketPath(cfg)
	if err != nil {
		return "", err
	}

	// Check if socket file exists
	info, err := os.Stat(socketPath)
	if os.IsNotExist(err) {
		return "", ErrNoSession
	}
	if err != nil {
		return "", fmt.Errorf("failed to stat socket: %w", err)
	}

	// Verify it's a socket
	if info.Mode()&os.ModeSocket == 0 {
		// Not a socket - stale file, remove it
		os.Remove(socketPath)
		return "", ErrNoSession
	}

	// Try to connect to verify the socket is active
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		// Socket exists but no one is listening - stale socket
		os.Remove(socketPath)
		return "", ErrNoSession
	}
	conn.Close()

	return socketPath, nil
}

// NewSession creates a new session based on the config.
// It determines whether this process should be the server or client.
func NewSession(cfg *Config) (*Session, error) {
	if cfg.Disabled {
		return nil, ErrDisabled
	}

	socketPath, err := GetSocketPath(cfg)
	if err != nil {
		return nil, err
	}

	session := &Session{
		SocketPath:  socketPath,
		ProjectRoot: cfg.ProjectRoot,
	}

	// Set name based on config
	if cfg.SessionName != "" {
		session.Name = cfg.SessionName
	} else {
		session.Name = hashProjectPath(cfg.ProjectRoot)
	}

	// Check if session already exists
	existingSocket, err := CheckExistingSession(cfg)
	if err == nil && existingSocket != "" {
		// Existing session found - we're a client
		session.IsServer = false
	} else {
		// No existing session - we're the server
		session.IsServer = true
	}

	return session, nil
}

// CleanupStaleSocket removes a stale socket file if it exists.
// This is useful for cleaning up after crashes.
func CleanupStaleSocket(socketPath string) error {
	// Check if socket file exists
	_, err := os.Stat(socketPath)
	if os.IsNotExist(err) {
		return nil // Nothing to clean up
	}
	if err != nil {
		return err
	}

	// Try to connect - if it fails, the socket is stale
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		// Stale socket - remove it
		return os.Remove(socketPath)
	}
	conn.Close()

	// Socket is active, don't remove
	return nil
}

// IsSocketInUse checks if a socket file is actively being used.
func IsSocketInUse(socketPath string) bool {
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// AcquireSessionLock attempts to acquire an exclusive lock for the session.
// This prevents race conditions when multiple clients start simultaneously.
func AcquireSessionLock(socketPath string) (*os.File, error) {
	lockPath := socketPath + ".lock"

	// Open or create the lock file
	f, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, fmt.Errorf("failed to open lock file: %w", err)
	}

	// Try to acquire exclusive lock (non-blocking)
	err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("failed to acquire session lock: %w", err)
	}

	return f, nil
}

// ReleaseSessionLock releases the session lock.
func ReleaseSessionLock(f *os.File) error {
	if f == nil {
		return nil
	}

	lockPath := f.Name()

	// Release the lock
	syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
	f.Close()

	// Remove the lock file
	os.Remove(lockPath)

	return nil
}

// ListSessions returns all active sessions.
func ListSessions() ([]*Session, error) {
	dir, err := GetSessionDir()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var sessions []*Session
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		if !strings.HasSuffix(name, ".sock") {
			continue
		}

		socketPath := filepath.Join(dir, name)

		// Check if socket is active
		if !IsSocketInUse(socketPath) {
			// Stale socket - clean up
			os.Remove(socketPath)
			continue
		}

		sessionName := name[:len(name)-5] // Remove .sock suffix
		sessions = append(sessions, &Session{
			Name:       sessionName,
			SocketPath: socketPath,
			IsServer:   false, // We're just listing, not determining role
		})
	}

	return sessions, nil
}
