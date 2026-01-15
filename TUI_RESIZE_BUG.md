# TUI Resize Bug - Investigation Notes

## Problem Summary

TUI applications (like Claude CLI) display incorrectly in thicc terminal panels:

1. **Initial render is broken**: Input box appears twice (top and bottom), only top is interactive
2. **Shrinking makes it worse**: Vertical pipes clash, layout completely breaks
3. **After shrink+grow cycle**: Renders PERFECTLY - identical to iTerm2

The same TUI apps work flawlessly in iTerm2 and other standard terminals.

---

## Root Cause Analysis

### The vt10x Library Architecture

thicc uses `github.com/hinshun/vt10x` for terminal emulation. Key findings:

1. **Two screen buffers**:
   - `lines` - main buffer (normal terminal output)
   - `altLines` - alternate screen buffer (used by TUI apps like vim, htop, Claude CLI)

2. **`Cell()` always reads from main buffer**:
   ```go
   func (t *State) Cell(x, y int) Glyph {
       cell := t.lines[y][x]  // ALWAYS main buffer, ignores alt screen mode
       // ...
   }
   ```

3. **`altLines` is unexported** (lowercase field name), requiring reflection to access

4. **`ModeAltScreen` flag** indicates when a TUI app has entered alternate screen mode

### The Original Bug (FIXED)

The original `getAltCell()` function used `Interface()` on an unexported field:
```go
altLines := altLinesField.Interface()  // FAILS for unexported fields!
```

This caused `CanInterface()` to return false, falling back to `VT.Cell()` which reads from the **wrong buffer**. This was fixed by using `Index()` directly.

### The Remaining Bug (UNSOLVED)

Even with the reflection fix, TUI apps render incorrectly on initial load but correctly after a resize cycle. This suggests:

1. The `altLines` buffer isn't properly initialized until first resize
2. OR there's a timing/race condition on initial render
3. OR vt10x has a bug in how it handles initial alt screen entry

---

## What We Tried

### 1. Call Resize() in updateLayout() ❌
**File**: `internal/layout/manager.go`
**Change**: When panel width changes due to focus, call `term.Resize()` instead of just updating Region
**Result**: Did not fix the issue

### 2. Reorder VT resize before PTY resize ❌
**File**: `internal/terminal/panel.go`
**Change**: Resize VT emulator before PTY (so VT is ready when app receives SIGWINCH)
**Result**: Did not fix the issue

### 3. Clear screen escape sequence on shrink ❌
**File**: `internal/terminal/panel.go`
**Change**: Write `\x1b[2J\x1b[H` to VT after resize when shrinking
**Result**: Did not fix the issue

### 4. Two-step resize (shrink→min→target) ❌
**File**: `internal/terminal/panel.go`
**Change**: When shrinking, first resize to minimum (10x5), then to target
**Result**: Did not fix the issue

### 5. Always fill entire content area in render ❌
**File**: `internal/terminal/vt_render.go`
**Change**: Modified `renderLiveView()` to iterate over full contentW×contentH, filling blanks for out-of-bounds cells
**Result**: Did not fix the issue

### 6. Clear altLines via reflection ❌
**File**: `internal/terminal/panel.go`
**Change**: Added `clearAltScreen()` to create fresh altLines buffer after resize
**Result**: Could not work - `CanSet()` returns false for unexported fields

### 7. Fix getAltCell() reflection to use Index() ⚠️ PARTIAL
**File**: `internal/terminal/vt_render.go`
**Change**: Use `reflect.Index()` and direct field access instead of `Interface()`
**Result**: This fix is correct and necessary, but didn't solve the initial render issue

### 8. Force resize cycle on panel creation ❌
**File**: `internal/terminal/panel.go`
**Change**: Call `vt.Resize(1,1)` then `vt.Resize(contentW, contentH)` after creating VT
**Result**: Did not fix the issue

---

## Current State of Code

### `internal/terminal/vt_render.go` - getAltCell()
```go
func (p *Panel) getAltCell(x, y int) vt10x.Glyph {
    v := reflect.ValueOf(p.VT)
    if v.Kind() == reflect.Ptr || v.Kind() == reflect.Interface {
        v = v.Elem()
    }

    altLinesField := v.FieldByName("altLines")
    if !altLinesField.IsValid() {
        return p.VT.Cell(x, y)
    }

    if y >= altLinesField.Len() {
        return vt10x.Glyph{}
    }

    lineVal := altLinesField.Index(y)
    if x >= lineVal.Len() {
        return vt10x.Glyph{}
    }

    glyphVal := lineVal.Index(x)

    var glyph vt10x.Glyph
    if charField := glyphVal.FieldByName("Char"); charField.IsValid() {
        glyph.Char = rune(charField.Int())
    }
    if modeField := glyphVal.FieldByName("Mode"); modeField.IsValid() {
        glyph.Mode = int16(modeField.Int())
    }
    if fgField := glyphVal.FieldByName("FG"); fgField.IsValid() {
        glyph.FG = vt10x.Color(fgField.Uint())
    }
    if bgField := glyphVal.FieldByName("BG"); bgField.IsValid() {
        glyph.BG = vt10x.Color(bgField.Uint())
    }

    return glyph
}
```

### `internal/terminal/panel.go` - Resize()
```go
func (p *Panel) Resize(w, h int) error {
    p.mu.Lock()
    defer p.mu.Unlock()

    p.Region.Width = w
    p.Region.Height = h

    contentW := w - 2
    contentH := h - 2
    if contentW < 10 {
        contentW = 10
    }
    if contentH < 5 {
        contentH = 5
    }

    p.VT.Resize(contentW, contentH)

    if p.PTY != nil {
        _ = pty.Setsize(p.PTY, &pty.Winsize{
            Rows: uint16(contentH),
            Cols: uint16(contentW),
        })
    }

    return nil
}
```

---

## Things to Try Next

### 1. Add Debug Logging
Add logging to `getAltCell()` to verify:
- Is `altLinesField.IsValid()` returning true?
- What is `altLinesField.Len()` on initial render vs after resize?
- Are we actually reading data or getting empty glyphs?

```go
log.Printf("getAltCell(%d,%d): altLines.Len()=%d", x, y, altLinesField.Len())
```

### 2. Check ModeAltScreen Timing
Add logging to see when `ModeAltScreen` is set:
```go
mode := p.VT.Mode()
log.Printf("Render: ModeAltScreen=%v", mode&vt10x.ModeAltScreen != 0)
```

Maybe we're rendering before the mode is set, causing us to read from wrong buffer.

### 3. Investigate vt10x Alt Screen Entry
Look at how vt10x handles `ESC[?1049h` (enter alt screen):
- Does it allocate `altLines` at this point?
- Does it clear the alt buffer?
- Is there a race between entering alt screen and our first render?

### 4. Fork vt10x
The cleanest solution might be to fork `github.com/hinshun/vt10x` and:
- Export `altLines` (or add `AltCell()` method)
- Fix any bugs in alt screen buffer initialization
- Ensure `Resize()` properly handles both buffers

### 5. Use unsafe.Pointer
Instead of reflection, use `unsafe.Pointer` to directly access `altLines`:
```go
// Get pointer to State struct
statePtr := (*vt10xState)(unsafe.Pointer(reflect.ValueOf(p.VT).Elem().UnsafeAddr()))
// Access altLines directly
cell := statePtr.altLines[y][x]
```

This is risky but would bypass reflection limitations.

### 6. Delayed Initial Render
Add a short delay before first render to ensure vt10x has processed initial escape sequences:
```go
if !p.hasRenderedOnce {
    time.Sleep(50 * time.Millisecond)
    p.hasRenderedOnce = true
}
```

### 7. Compare Buffer States
Add a debug mode that dumps both `lines` and `altLines` contents to a file after initial load and after resize, to see exactly what's different.

### 8. Check vt10x Version/Alternatives
- Current version: `v0.0.0-20220301184237-5011da428d02` (March 2022)
- Check if there are newer commits with fixes
- Consider alternative VT emulators that might handle alt screen better

---

## Key Observations

1. **Works perfectly after resize cycle** - This proves the alt screen rendering CAN work
2. **Initial render is consistently broken** - Something about initial state is wrong
3. **iTerm2 works perfectly** - The Claude CLI app is correct, the bug is in thicc/vt10x
4. **Duplicate content suggests buffer mixing** - Maybe reading from wrong buffer or wrong coordinates

---

## Files Involved

| File | Purpose |
|------|---------|
| `internal/terminal/panel.go` | Panel creation, resize handling |
| `internal/terminal/vt_render.go` | Rendering logic, `getAltCell()` |
| `internal/layout/manager.go` | Layout updates, calls `Resize()` |
| `go.mod` | vt10x dependency version |

---

## Useful Commands

```bash
# Build
go build ./...

# Run with debug logging
THICC_DEBUG=1 ./thicc

# Test with Claude CLI
# 1. Run thicc
# 2. Type: claude
# 3. Observe initial render (broken)
# 4. Click file browser (shrink terminal)
# 5. Click terminal (grow back)
# 6. Observe render (should be fixed)
```

---

## Contact / Resources

- vt10x repo: https://github.com/hinshun/vt10x
- vt10x source for Cell(): https://raw.githubusercontent.com/hinshun/vt10x/master/state.go
- VT100 escape sequences: https://vt100.net/docs/vt100-ug/chapter3.html
