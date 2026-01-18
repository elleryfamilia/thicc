//go:build !linux && !darwin

package terminal

import (
	"os"
)

// getForegroundProcessName returns the name of the foreground process in the PTY.
// This is a stub for unsupported platforms - returns empty string.
func getForegroundProcessName(pty *os.File) string {
	return ""
}
