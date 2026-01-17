//go:build darwin

package terminal

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"

	"golang.org/x/sys/unix"
)

// getForegroundProcessName returns the name of the foreground process in the PTY.
// Uses TIOCGPGRP ioctl to get the foreground process group, then uses ps to get the process name.
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

	// Use ps on macOS (no /proc filesystem)
	out, err := exec.Command("ps", "-p", fmt.Sprintf("%d", pgrp), "-o", "comm=").Output()
	if err != nil {
		log.Printf("THICC: ps command failed for pid %d: %v", pgrp, err)
		return ""
	}

	procName := strings.TrimSpace(string(out))
	log.Printf("THICC: ps returned process name: %q", procName)
	return procName
}
