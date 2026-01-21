# PTY Session Multiplexing - Bug Report

## Overview

The PTY-based session multiplexing system was implemented to replace the tcell screen-capture approach. The architecture spawns THICC as a child process in a PTY and forwards raw terminal bytes between the PTY and connected clients.

## Architecture

```
Server Process (first instance):
  ┌─────────────────────────────────────────┐
  │  THICC child (--no-session, in PTY)     │
  └────────────────┬────────────────────────┘
                   │ PTY (ptyFile)
  ┌────────────────┴────────────────────────┐
  │  Server                                  │
  │  - Local mode: stdin→PTY, PTY→stdout    │
  │  - Client mode: socket→PTY, PTY→socket  │
  └────────────────┬────────────────────────┘
                   │ Unix Socket
                   ▼
  ┌─────────────────────────────────────────┐
  │  Client (raw terminal mode)             │
  │  - stdin→socket, socket→stdout          │
  └─────────────────────────────────────────┘
```

## Files Modified

- `internal/session/session.go` - Session identification, socket paths
- `internal/session/protocol.go` - Simple binary framing protocol
- `internal/session/server.go` - PTY server with local+socket client support
- `internal/session/client.go` - PTY client (raw terminal mode)
- `internal/session/screen.go` - DELETED (old tcell capture)
- `cmd/thicc/micro.go` - Startup flow changes
- `run-thicc.sh` - Path conversion fix

## Current Bugs

### Bug 1: Mouse Events Don't Work

**Symptoms:**
- Mouse clicks do not trigger actions in THICC
- Mouse worked before PTY session changes

**Likely Causes:**
1. Terminal mouse mode escape sequences from child tcell may not be reaching the real terminal
2. Mouse input escape sequences from terminal may not be forwarded correctly to PTY
3. PTY terminal attributes may not be set correctly for mouse support

**Relevant Code:**
- `server.go:Start()` - Copies parent termios to PTY (line ~120)
- `server.go:localStdinLoop()` - Forwards stdin to PTY
- `server.go:ptyReadLoop()` - Forwards PTY output to stdout

**Debug Approach:**
1. Check if tcell's mouse enable sequences (`\x1b[?1000h` etc.) reach the real terminal
2. Verify mouse event escape sequences are being read from stdin
3. Test with `xxd` or similar to see raw bytes flowing

### Bug 2: Passing Path Still Opens Dashboard

**Symptoms:**
- Running `./thicc .` or `./run-thicc.sh .` still shows the dashboard
- Should skip dashboard and open the project directly

**Expected Flow:**
1. Parent: `args=["/path"]`, `wouldShowDashboard=false`
2. Parent becomes server, spawns child with `--no-session /path`
3. Child: `args=["/path"]`, `flagNoSession=true`, `wouldShowDashboard=false`
4. Child should NOT show dashboard

**Debug Info:**
Check `log.txt` for lines like:
```
THICC Startup: PID=X args=[...] flagNoSession=X wouldShowDashboard=X cwd=X
```

**Relevant Code:**
- `micro.go:~733` - `wouldShowDashboard` calculation
- `micro.go:~739` - Session check and server spawn
- `server.go:Start()` - Child command construction

**Possible Issues:**
1. `run-thicc.sh` path conversion may not work correctly
2. Args may not be passed correctly to child process
3. `isatty.IsTerminal()` may return unexpected values in PTY

### Bug 3: Paused Message Should Show on Both Terminals

**Symptoms:**
- When switching between terminals, the "paused" message only shows on original
- Should show on whichever terminal loses control

**Expected Behavior:**
- Terminal A starts server, shows THICC
- Terminal B connects as client, Terminal A shows "paused" message
- Terminal B disconnects (Ctrl+Q), Terminal A resumes
- If Terminal A's local user is active and Terminal B connects again, Terminal A should show paused

**Current Implementation:**
- `showLocalPausedMessage()` is called when socket client connects
- `showLocalResumedMessage()` is called when socket client disconnects
- Only the server's local terminal shows these messages
- Socket clients just get disconnected with a frame message

**Fix Needed:**
- The "paused" concept doesn't apply to socket clients the same way
- Socket clients are kicked when a new client connects
- Maybe show a "kicked" message on the client side before disconnect

## Key Code Locations

### Server Startup (server.go)
```go
func (s *Server) Start(thiccArgs []string) error {
    // Line ~90: Build command with --no-session
    args := append([]string{"--no-session"}, thiccArgs...)
    s.cmd = exec.Command(thiccPath, args...)
    s.cmd.Dir = s.config.ProjectRoot

    // Line ~111: Start PTY
    s.ptyFile, err = pty.StartWithSize(s.cmd, &pty.Winsize{...})

    // Line ~125: Copy terminal attributes (for mouse support)
    if parentTermios, err := unix.IoctlGetTermios(int(os.Stdin.Fd()), unix.TCGETS); err == nil {
        unix.IoctlSetTermios(int(s.ptyFile.Fd()), unix.TCSETS, parentTermios)
    }
}
```

### Session Detection (micro.go)
```go
// Line ~733
args := flag.Args()
wouldShowDashboard := len(args) == 0 && isatty.IsTerminal(os.Stdin.Fd()) && isatty.IsTerminal(os.Stdout.Fd())

// Line ~739
if !*flagNoSession && !wouldShowDashboard {
    // Session handling - becomes server or client
}
```

### Client Input Forwarding (server.go)
```go
// Line ~367: Socket client input - Ctrl+Q intercepted
for _, b := range frame.Payload {
    if b == 0x11 { // Ctrl+Q
        // Disconnect client instead of forwarding
        return
    }
}
// Forward to PTY
s.ptyFile.Write(frame.Payload)
```

### Local Input Forwarding (server.go)
```go
// Line ~401: localStdinLoop
n, err := os.Stdin.Read(buf)
if isLocal && s.ptyFile != nil {
    s.ptyFile.Write(buf[:n])
}
```

## Testing Commands

```bash
# Clean up sessions
rm -f ~/.thicc/sessions/* log.txt

# Test with absolute path
./thicc /home/ellery/_git/thicc-mobile

# Check debug log
cat log.txt | grep "THICC Startup"

# Test mouse (after THICC starts)
# Click should focus panels, scroll should work

# Test session sharing
# Terminal 1: ./thicc /path/to/project
# Terminal 2: ./thicc /path/to/project
# Terminal 2 should connect to Terminal 1's session
```

## Protocol (protocol.go)

Frame format: `[type:1][length:4][payload:N]`

Types:
- `FrameData (1)` - Raw terminal data
- `FrameResize (2)` - Terminal size change
- `FrameClose (3)` - Session ending
- `FrameHello (4)` - Client handshake
- `FrameWelcome (5)` - Server response

## Dependencies

- `github.com/creack/pty` - PTY creation
- `golang.org/x/sys/unix` - Terminal raw mode, termios
- `github.com/mattn/go-isatty` - Terminal detection
