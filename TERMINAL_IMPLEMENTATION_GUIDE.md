# Terminal Package Implementation Guide

## Overview
This document contains complete implementation details for the standalone terminal package using hinshun/vt10x + creack/pty.

## Architecture

```
Terminal Panel
├─ PTY Management (creack/pty)
│  ├─ Start command with PTY
│  ├─ Read loop: PTY output → VT emulator
│  └─ Write loop: User input → PTY
├─ VT Emulation (hinshun/vt10x)
│  ├─ Parse ANSI escape sequences
│  ├─ Maintain terminal state (cursor, colors, cells)
│  └─ Provide state for rendering
└─ tcell Rendering
   ├─ Convert VT state to tcell screen
   ├─ Render each cell with proper style
   └─ Show cursor when focused
```

## Key Types

### Panel struct
```go
type Panel struct {
    VT       vt10x.Terminal   // VT emulator
    PTY      *os.File         // PTY file
    Cmd      *exec.Cmd        // Running command
    Region   Region           // Screen region
    Focus    bool             // Is focused?
    Tool     *aiterminal.AITool  // Which AI CLI
    Running  bool             // Is command running?
    mu       sync.Mutex       // Protect state
}
```

### Region struct (from filebrowser)
```go
type Region struct {
    X, Y, Width, Height int
}
```

## Implementation Files

### 1. panel.go - Main Terminal Panel

**Key Functions:**
- `NewPanel(x, y, w, h int, tool *AITool) (*Panel, error)`
  - Creates VT emulator: `vt10x.Create(vt10x.WithSize(w, h))`
  - Starts command with PTY: `pty.Start(cmd)`
  - Launches read loop goroutine
  - Returns configured panel

- `readLoop()`
  - Goroutine that copies PTY output to VT
  - `io.Copy(p.VT, p.PTY)`
  - Runs until PTY closes

- `Write(data []byte) (int, error)`
  - Sends data to PTY
  - Used for user input

- `Close()`
  - Closes PTY
  - Kills process
  - Cleans up resources

- `Resize(w, h int) error`
  - Resizes PTY: `pty.Setsize(p.PTY, &pty.Winsize{Rows: h, Cols: w})`
  - Resizes VT: `p.VT.Resize(w, h)`

### 2. vt_render.go - VT State to tcell Rendering

**Key Function:**
- `Render(screen tcell.Screen)`
  - Gets VT state: `state := p.VT.State()`
  - Iterates through state cells
  - For each cell:
    - Get rune: `cell.Rune()`
    - Get attributes: `cell.Attr()`
    - Convert VT attributes to tcell style
    - Call `screen.SetContent(x, y, rune, nil, style)`
  - If focused, show cursor: `screen.ShowCursor(cx, cy)`

**Attribute Conversion:**
```go
func vtAttrToTcellStyle(attr vt10x.Attr) tcell.Style {
    style := tcell.StyleDefault

    // Foreground color
    if attr.FG != vt10x.DefaultFG {
        style = style.Foreground(vtColorToTcell(attr.FG))
    }

    // Background color
    if attr.BG != vt10x.DefaultBG {
        style = style.Background(vtColorToTcell(attr.BG))
    }

    // Bold
    if attr.Mode&vt10x.AttrBold != 0 {
        style = style.Bold(true)
    }

    // Underline
    if attr.Mode&vt10x.AttrUnderline != 0 {
        style = style.Underline(true)
    }

    // Reverse
    if attr.Mode&vt10x.AttrReverse != 0 {
        style = style.Reverse(true)
    }

    return style
}

func vtColorToTcell(c vt10x.Color) tcell.Color {
    // VT10x uses 256-color palette
    // Map to tcell colors
    if c < 16 {
        // Standard 16 colors
        return tcell.Color(c)
    }
    // 256-color mode
    return tcell.Color(c)
}
```

### 3. input.go - tcell Events to PTY Input

**Key Function:**
- `HandleEvent(event tcell.Event) bool`
  - If not focused, return false
  - Handle `*tcell.EventKey`:
    - Convert key to bytes
    - Write to PTY
    - Return true
  - Handle mouse events (future)

**Key Conversion:**
```go
func keyToBytes(ev *tcell.EventKey) []byte {
    switch ev.Key() {
    case tcell.KeyEnter, tcell.KeyCR:
        return []byte{'\r'}
    case tcell.KeyTab:
        return []byte{'\t'}
    case tcell.KeyBackspace, tcell.KeyBackspace2:
        return []byte{0x7f}
    case tcell.KeyEscape:
        return []byte{0x1b}
    case tcell.KeyUp:
        return []byte{0x1b, '[', 'A'}
    case tcell.KeyDown:
        return []byte{0x1b, '[', 'B'}
    case tcell.KeyRight:
        return []byte{0x1b, '[', 'C'}
    case tcell.KeyLeft:
        return []byte{0x1b, '[', 'D'}
    case tcell.KeyHome:
        return []byte{0x1b, '[', 'H'}
    case tcell.KeyEnd:
        return []byte{0x1b, '[', 'F'}
    case tcell.KeyPgUp:
        return []byte{0x1b, '[', '5', '~'}
    case tcell.KeyPgDn:
        return []byte{0x1b, '[', '6', '~'}
    case tcell.KeyDelete:
        return []byte{0x1b, '[', '3', '~'}
    case tcell.KeyInsert:
        return []byte{0x1b, '[', '2', '~'}
    case tcell.KeyCtrlA:
        return []byte{0x01}
    case tcell.KeyCtrlB:
        return []byte{0x02}
    // ... more ctrl keys
    case tcell.KeyRune:
        r := ev.Rune()
        if r < 128 {
            return []byte{byte(r)}
        }
        // UTF-8 encoding for runes >= 128
        buf := make([]byte, 4)
        n := utf8.EncodeRune(buf, r)
        return buf[:n]
    }
    return nil
}
```

### 4. Clipboard/Paste Handling

**Problem:** When terminal has focus, Cmd-V/Ctrl-V must paste to terminal, not editor.

**Challenge:** tcell can generate paste events in multiple ways:
1. `tcell.EventPaste` - Bracketed paste from outer terminal
2. `tcell.EventKey` with `KeyCtrlV` - Direct Ctrl-V keypress
3. `tcell.EventKey` with `KeyRune` 'v' + `ModMeta` - Cmd-V on Mac

**Solution:** Focus-aware clipboard routing in the layout manager.

**Implementation (layout/manager.go):**

```go
// handleTerminalPaste intercepts paste commands when terminal has focus
// Returns true if event was a paste that was handled
func (lm *LayoutManager) handleTerminalPaste(event tcell.Event) bool {
    lm.mu.RLock()
    term := lm.Terminal
    lm.mu.RUnlock()

    if term == nil {
        return false
    }

    // Handle tcell.EventPaste (bracketed paste from outer terminal)
    if ev, ok := event.(*tcell.EventPaste); ok {
        term.Write([]byte(ev.Text()))
        return true
    }

    // Handle Cmd-V (Mac) or Ctrl-V paste keybinding
    if ev, ok := event.(*tcell.EventKey); ok {
        isPaste := false

        // Check for Ctrl-V
        if ev.Key() == tcell.KeyCtrlV {
            isPaste = true
        }

        // Check for Cmd-V (Mac) or Ctrl-V as rune with modifier
        if ev.Key() == tcell.KeyRune && ev.Rune() == 'v' {
            if ev.Modifiers()&tcell.ModMeta != 0 || ev.Modifiers()&tcell.ModCtrl != 0 {
                isPaste = true
            }
        }

        if isPaste {
            clip, err := clipboard.Read(clipboard.ClipboardReg)
            if err != nil {
                return true // Still consume to prevent editor paste
            }
            term.Write([]byte(clip))
            return true
        }
    }

    return false
}
```

**Call site in HandleEvent():**
```go
func (lm *LayoutManager) HandleEvent(event tcell.Event) bool {
    // Handle modal dialog first
    if lm.Modal != nil && lm.Modal.Active {
        return lm.Modal.HandleEvent(event)
    }

    // Handle paste for terminal BEFORE other routing
    if lm.ActivePanel == 2 {
        if lm.handleTerminalPaste(event) {
            return true
        }
    }
    // ... rest of event handling
}
```

**Defensive backup in terminal/input.go:**
```go
case *tcell.EventPaste:
    // Handle paste events directly (backup if layout manager doesn't catch it)
    _, err := p.Write([]byte(ev.Text()))
    return err == nil
```

**Why this approach:**
1. **Early interception**: Paste checked before regular event routing
2. **Focus-aware**: Only intercepts when `ActivePanel == 2` (terminal)
3. **Multiple paste methods**: Handles EventPaste, Cmd-V, and Ctrl-V
4. **Clipboard integration**: Reads actual system clipboard via micro's clipboard package
5. **Editor unchanged**: When editor focused, returns false → micro handles normally
6. **Defensive depth**: Terminal also handles EventPaste directly as backup

### 5. pty.go - PTY Management Helpers

**Functions:**
- `startCommand(cmdLine []string) (*os.File, *exec.Cmd, error)`
  - Creates exec.Cmd
  - Sets environment variables
  - Starts with PTY
  - Returns PTY file and cmd

- `setTerminalSize(ptmx *os.File, w, h int) error`
  - Wrapper around pty.Setsize

## Integration with AI Tools

The terminal panel takes an `*aiterminal.AITool` which has:
```go
type AITool struct {
    Name        string
    Command     string
    Args        []string
    Description string
    Available   bool
}

func (t *AITool) GetCommandLine() []string {
    if len(t.Args) > 0 {
        return append([]string{t.Command}, t.Args...)
    }
    return []string{t.Command}
}
```

For example:
- Claude: `{"claude", "", nil, ...}` → runs `claude`
- Aider: `{"aider", "", nil, ...}` → runs `aider`
- Default shell: `{"/bin/bash", "", nil, ...}` → runs `bash`

## Error Handling

1. **PTY Start Failure:**
   - Return error from NewPanel
   - Layout manager should handle gracefully
   - Can continue without terminal panel

2. **Command Exit:**
   - Read loop ends naturally when PTY closes
   - Set Running = false
   - Could restart or show exit status

3. **Write Errors:**
   - If PTY is closed, writes will fail
   - Check Running before writing

## Testing

### Manual Test (Standalone):
```go
package main

import (
    "github.com/ellery/thicc/internal/terminal"
    "github.com/ellery/thicc/internal/aiterminal"
    "github.com/micro-editor/tcell/v2"
)

func main() {
    screen, _ := tcell.NewScreen()
    screen.Init()
    defer screen.Fini()

    // Create bash terminal
    tool := &aiterminal.AITool{
        Name:    "Bash",
        Command: "/bin/bash",
    }

    w, h := screen.Size()
    panel, err := terminal.NewPanel(0, 0, w, h, tool)
    if err != nil {
        panic(err)
    }
    defer panel.Close()

    panel.Focus = true

    for {
        screen.Clear()
        panel.Render(screen)
        screen.Show()

        ev := screen.PollEvent()
        switch ev := ev.(type) {
        case *tcell.EventKey:
            if ev.Key() == tcell.KeyCtrlQ {
                return
            }
            panel.HandleEvent(ev)
        case *tcell.EventResize:
            w, h := screen.Size()
            panel.Resize(w, h)
        }
    }
}
```

## Known Issues & Solutions

### Issue 1: VT state might not match PTY size
**Solution:** Always resize both PTY and VT together in Resize()

### Issue 2: Some programs (vim, less) might not render correctly
**Solution:**
- Ensure TERM is set correctly (export TERM=xterm-256color)
- VT10x should handle most escape sequences

### Issue 3: Mouse events in terminal programs
**Solution:**
- Terminal programs can request mouse events
- VT10x handles this via escape sequences
- For MVP, can be left for future

### Issue 4: Unicode rendering
**Solution:**
- tcell handles UTF-8 automatically
- Just ensure runes are passed correctly from VT state

## Performance Considerations

1. **Rendering:**
   - Only render when VT state changes
   - Could add dirty tracking
   - For MVP: render every frame is fine

2. **Read Loop:**
   - Runs in goroutine
   - Uses io.Copy which is efficient
   - No additional buffering needed

3. **Memory:**
   - VT state is bounded by terminal size
   - PTY buffers are kernel-managed
   - No leaks expected

## Security Considerations

1. **Command Execution:**
   - Only execute commands from AITool
   - No shell interpretation of user input
   - Direct exec.Command usage

2. **PTY Access:**
   - PTY is process-local
   - No network exposure
   - Standard OS security applies

## Future Enhancements

1. **Scrollback Buffer:**
   - VT10x supports scrollback
   - Add scrolling with PageUp/PageDown
   - Store history in VT state

2. **Copy/Paste:** ✅ IMPLEMENTED
   - Paste (Cmd-V/Ctrl-V) routes to focused panel
   - See "Clipboard/Paste Handling" section above
   - TODO: Copy from terminal (mouse selection → clipboard)

3. **Multiple Sessions:**
   - Tab between different AI tools
   - Each has own Panel instance
   - Layout manager coordinates

4. **Session Persistence:**
   - Save PTY state on exit
   - Restore on restart
   - Use screen/tmux approach

## Complete Code Skeleton

```go
// panel.go
package terminal

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

func NewPanel(x, y, w, h int, tool *aiterminal.AITool) (*Panel, error)
func (p *Panel) readLoop()
func (p *Panel) Write(data []byte) (int, error)
func (p *Panel) Close()
func (p *Panel) Resize(w, h int) error

// vt_render.go
func (p *Panel) Render(screen tcell.Screen)
func vtAttrToTcellStyle(attr vt10x.Attr) tcell.Style
func vtColorToTcell(c vt10x.Color) tcell.Color

// input.go
func (p *Panel) HandleEvent(event tcell.Event) bool
func keyToBytes(ev *tcell.EventKey) []byte

// pty.go (optional helpers)
func startCommand(cmdLine []string) (*os.File, *exec.Cmd, error)
func setTerminalSize(ptmx *os.File, w, h int) error
```

## References

- hinshun/vt10x: https://github.com/hinshun/vt10x
- creack/pty: https://github.com/creack/pty
- tcell: https://github.com/gdamore/tcell
- ANSI escape codes: https://en.wikipedia.org/wiki/ANSI_escape_code
- VT100 reference: https://vt100.net/docs/vt100-ug/

## Migration Notes

**Files Being Replaced:**
- `internal/aiterminal/pane.go` (old, coupled to micro's shell.Terminal)
- `internal/action/aitermpane.go` (old, action wrapper)

**Files Being Reused:**
- `internal/aiterminal/detect.go` (AI CLI detection - keep this!)
- `internal/aiterminal/session.go` (session state - keep for future)

**Integration Point:**
- Layout manager will create Panel in `Initialize()`
- Pass detected AI tool from `aiterminal.QuickLaunchFirst()`
- Terminal panel is completely independent of micro
