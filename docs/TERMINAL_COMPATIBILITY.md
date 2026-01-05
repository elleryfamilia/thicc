# Terminal Compatibility Guide

This document captures learnings about cross-terminal compatibility to ensure thicc provides a consistent experience across all terminal emulators.

## Key Learnings

### Alt/Option Key Handling

Terminals handle Alt (or Option on macOS) keys in three different ways:

| Method | Description | Terminals |
|--------|-------------|-----------|
| **ModAlt flag** | Terminal sets the `ModAlt` modifier on key events | Some X11 terminals |
| **ESC-prefix** | Terminal sends `ESC` (0x1b) followed by the key | Kitty (Linux), Alacritty, xterm |
| **macOS Option chars** | Option+key generates Unicode characters | Kitty (macOS), iTerm2, Terminal.app |

#### macOS Option Key Mappings

On macOS, Option+number generates special Unicode characters instead of Alt+number:

| Shortcut | Character | Unicode |
|----------|-----------|---------|
| Option+1 | ¡ | U+00A1 |
| Option+2 | ™ | U+2122 |
| Option+3 | £ | U+00A3 |
| Option+4 | ¢ | U+00A2 |
| Option+5 | ∞ | U+221E |

**Implementation**: We handle all three methods in `internal/layout/manager.go`:
1. Check `ev.Modifiers()&tcell.ModAlt` for ModAlt flag
2. Handle `*tcell.EventRaw` for ESC-prefix sequences
3. Match specific Unicode runes for macOS Option characters

### TERM Environment Variable

The `TERM` environment variable affects how terminals behave:

- **`xterm-256color`**: Forces xterm compatibility mode. Some terminals (like Kitty) may not send native escape sequences.
- **`kitty`**: Kitty's native mode with extended key encoding.
- **Terminal-specific values**: Each terminal may have its own TERM value with specific capabilities.

**Current approach**: We set `TERM=xterm-256color` for maximum compatibility, but this means we must handle Alt keys through multiple methods rather than relying on native terminal protocols.

### tcell Library Notes

We use `github.com/micro-editor/tcell/v2` (micro-editor's fork):

- **Raw sequences**: Register with `Screen.RegisterRawSeq(seq)` to receive `*tcell.EventRaw` events
- **Meta to Alt**: tcell converts `ModMeta` to `ModAlt` via `metaToAlt()` in `internal/action/events.go`
- **Event types**: Key events come as `*tcell.EventKey`, raw sequences as `*tcell.EventRaw`

## Terminal-Specific Notes

### Kitty

- On **Linux**: Sends ESC-prefix sequences for Alt+key
- On **macOS**: Sends Option characters by default
- **Config option**: `macos_option_as_alt yes` changes behavior to send escape sequences
- **Native protocol**: Supports extended key encoding when `TERM=kitty`

### iTerm2

- Sends Option characters by default on macOS
- **Config option**: Preferences → Profiles → Keys → "Left/Right Option key" → "Esc+"
- With Esc+ enabled, sends ESC-prefix sequences

### Terminal.app (macOS)

- Sends Option characters by default
- **Config option**: Preferences → Profiles → Keyboard → "Use Option as Meta key"

### Alacritty

- Generally sends ESC-prefix sequences for Alt+key
- Behavior may vary based on TERM setting

## Adding New Keyboard Shortcuts

When adding shortcuts that use modifier keys:

### Do

1. **Test on multiple terminals**: At minimum, test on Kitty, iTerm2, and Terminal.app
2. **Handle multiple input methods**: For Alt+key, handle ModAlt flag, ESC-prefix, and macOS chars
3. **Log key events for debugging**: Use the debug logging pattern to see what events terminals actually send
4. **Check for conflicts**: Some key combinations are used by the terminal itself or generate special characters

### Don't

1. **Assume ModAlt works everywhere**: Many terminals don't set this flag
2. **Rely solely on ESC-prefix**: macOS terminals often send characters instead
3. **Use Option+letter on macOS**: These generate accented characters (e.g., Option+e = ´)

### Debug Pattern

To debug what a terminal sends for a key combination:

```go
// Add temporarily to HandleEvent()
switch e := event.(type) {
case *tcell.EventRaw:
    log.Printf("EventRaw: seq=%q", e.EscSeq())
case *tcell.EventKey:
    log.Printf("EventKey: Key=%v Rune=%q Mod=%v", e.Key(), e.Rune(), e.Modifiers())
}
```

Then check `log.txt` in the working directory.

## Testing Checklist

Before releasing keyboard shortcut changes:

- [ ] **Kitty (macOS)**: Test with default settings
- [ ] **Kitty (Linux)**: If available, test ESC-prefix behavior
- [ ] **iTerm2**: Test with default settings
- [ ] **Terminal.app**: Test with default settings
- [ ] **VS Code terminal**: Test integrated terminal
- [ ] **tmux/screen**: Test within terminal multiplexers (may intercept keys)

## Known Limitations

1. **tmux/screen**: May intercept certain key combinations before they reach the app
2. **SSH sessions**: Key handling depends on both local and remote terminal settings
3. **Option+letter on macOS**: Generates accented/special characters, not usable as shortcuts
4. **Ctrl+number**: Often doesn't generate distinct key codes in terminals

## References

- [Kitty keyboard protocol](https://sw.kovidgoyal.net/kitty/keyboard-protocol/)
- [XTerm control sequences](https://invisible-island.net/xterm/ctlseqs/ctlseqs.html)
- [tcell documentation](https://github.com/gdamore/tcell)
