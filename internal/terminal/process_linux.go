//go:build linux

package terminal

import (
	"fmt"
	"log"
	"os"
	"strings"

	"golang.org/x/sys/unix"
)

// getForegroundProcessName returns the name of the foreground process in the PTY.
// Uses TIOCGPGRP ioctl to get the foreground process group, then reads /proc/<pid>/comm.
func getForegroundProcessName(pty *os.File) string {
	if pty == nil {
		log.Println("THICC: getForegroundProcessName: pty is nil")
		return ""
	}

	// Get foreground process group via TIOCGPGRP
	pgrp, err := unix.IoctlGetInt(int(pty.Fd()), unix.TIOCGPGRP)
	if err != nil {
		log.Printf("THICC: TIOCGPGRP ioctl failed: %v", err)
		return ""
	}
	if pgrp <= 0 {
		log.Printf("THICC: TIOCGPGRP returned invalid pgrp: %d", pgrp)
		return ""
	}

	log.Printf("THICC: TIOCGPGRP returned pgrp: %d", pgrp)

	// Read process name from /proc/<pid>/comm
	commPath := fmt.Sprintf("/proc/%d/comm", pgrp)
	data, err := os.ReadFile(commPath)
	if err != nil {
		log.Printf("THICC: Failed to read %s: %v", commPath, err)
		return ""
	}

	procName := strings.TrimSpace(string(data))
	log.Printf("THICC: Read process name from %s: %q", commPath, procName)
	return procName
}
