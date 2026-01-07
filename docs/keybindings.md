# Keybindings Reference

This is a complete reference of thicc-specific keybindings. These shortcuts work on top of the standard editor keybindings inherited from micro.

## Panel Navigation

The most important shortcuts for moving between panels:

| Shortcut | Action |
|----------|--------|
| `Ctrl+Space` | Cycle focus: File Browser → Editor → Terminal(s) |
| `Tab` | Jump from file browser to editor |
| `Shift+Tab` | Jump from editor to file browser |
| `Alt+1` | Toggle file browser visibility |
| `Alt+2` | Toggle editor visibility |
| `Alt+3` | Toggle terminal 1 visibility |
| `Alt+4` | Toggle terminal 2 visibility |
| `Alt+5` | Toggle terminal 3 visibility |

## Quick File Finder

| Shortcut | Action |
|----------|--------|
| `Ctrl+P` | Open quick file finder (fuzzy search) |

In the quick file finder:
- Type to filter files by name (fuzzy matching)
- `↑` / `↓` to navigate results
- `Enter` to open selected file
- `Escape` to cancel

## File Browser

When the file browser panel has focus:

### Navigation

| Shortcut | Action |
|----------|--------|
| `↑` / `k` | Move selection up |
| `↓` / `j` | Move selection down |
| `←` / `h` | Collapse directory (or go to parent) |
| `→` / `l` | Expand directory (or move into it) |
| `Enter` | Open file in editor / toggle directory |
| `Page Up` | Move up one page |
| `Page Down` | Move down one page |
| `Home` / `g` | Go to top of list |
| `End` / `G` | Go to bottom of list |

### File Operations

When the file browser is focused, use quick command mode (`Ctrl+?`) for file operations:

| Shortcut | Action |
|----------|--------|
| `n` / `N` | Create new file |
| `f` / `F` | Create new folder |
| `d` / `D` | Delete selected file/folder |
| `r` | Refresh tree / Rename (context-dependent) |
| `R` | Rename selected file/folder |

### Mouse

| Action | Effect |
|--------|--------|
| Click on file | Open file in editor |
| Click on folder | Expand/collapse folder |
| Click on project path (header) | Open project picker |
| Scroll wheel | Scroll file list |

## Terminal

When a terminal panel has focus, most keys are sent directly to the shell. Special thicc keys:

| Shortcut | Action |
|----------|--------|
| `Ctrl+Space` | Cycle to next panel |
| `Ctrl+Q` (twice) | Exit thicc from terminal |
| `Ctrl+E` (twice) | Enter command mode from terminal |
| `Ctrl+W` (twice) | Next split from terminal |

### Terminal Scrolling

| Shortcut | Action |
|----------|--------|
| `Shift+PageUp` | Scroll up through terminal history |
| `Shift+PageDown` | Scroll down through terminal history |
| Mouse wheel | Scroll (3 lines per tick) |

Any keypress while scrolled up snaps back to the live terminal output.

### Quick Command Mode

Press `Ctrl+\` to enter quick command mode in the terminal. A banner appears with options:

| Key | Action |
|-----|--------|
| `q` | Quit thicc |
| `w` | Switch to next pane |
| `p` | Enter passthrough mode |
| `Escape` | Cancel |

**Passthrough mode** sends all keys directly to the terminal, bypassing thicc shortcuts. Useful for programs that use `Ctrl+Space` or other thicc shortcuts.

## Dashboard

When the dashboard is showing:

### Menu Navigation

| Shortcut | Action |
|----------|--------|
| `↑` / `↓` or `j` / `k` | Navigate menu items |
| `←` / `→` or `h` / `l` | Switch between menu and recent projects |
| `Tab` | Cycle between panes |
| `Enter` | Select menu item or open recent project |
| `Escape` or `q` | Exit thicc |

### Quick Actions

| Shortcut | Action |
|----------|--------|
| `n` | New file |
| `f` | Open file picker |
| `o` | Open project picker |
| `d` | New folder |
| `?` | Show onboarding guide |
| `1-9` | Open recent project by number (1 = most recent) |
| `Delete` / `Backspace` | Remove selected recent project from list |

## Quick Command Mode (Editor)

Press `Ctrl+\` to enter quick command mode. A hint bar appears at the bottom showing available shortcuts:

### In File Browser
```
N File   F Folder   D Delete   R Rename   Q Quit   [Space] Next   ESC Cancel
```

### In Editor
```
S Save   W Close   Q Quit   [Space] Next   ESC Cancel
```

### In Terminal
```
P Passthrough   Q Quit   [Space] Next   ESC Cancel
```

## Keybinding Hints

Press `Alt+G` to toggle a persistent hint bar showing common keybindings at the bottom of the screen.

## Editor Keybindings

The editor uses standard keybindings. Here are the most common:

### Basic Editing

| Shortcut | Action |
|----------|--------|
| `Ctrl+S` | Save file |
| `Ctrl+Q` | Quit |
| `Ctrl+Z` | Undo |
| `Ctrl+Y` | Redo |
| `Ctrl+C` | Copy (or copy line if no selection) |
| `Ctrl+X` | Cut (or cut line if no selection) |
| `Ctrl+V` | Paste |
| `Ctrl+D` | Duplicate line |

### Search

| Shortcut | Action |
|----------|--------|
| `Ctrl+F` | Find (regex) |
| `Alt+F` | Find literal |
| `Ctrl+N` | Find next |
| `Ctrl+Shift+P` | Find previous |
| `Ctrl+L` | Go to line |

### Selection

| Shortcut | Action |
|----------|--------|
| `Shift+Arrow` | Extend selection |
| `Ctrl+A` | Select all |
| `Alt+A` | Select line |

### Multiple Cursors

| Shortcut | Action |
|----------|--------|
| `Alt+N` | Spawn cursor at next occurrence |
| `Alt+M` | Spawn cursor (select word) |
| `Alt+Shift+Up/Down` | Add cursor above/below |
| `Alt+P` | Remove last cursor |
| `Alt+C` | Remove all extra cursors |
| `Escape` | Clear cursors and selection |

### Tabs

| Shortcut | Action |
|----------|--------|
| `Ctrl+T` | New tab |
| `Alt+,` | Previous tab |
| `Alt+.` | Next tab |
| `Ctrl+W` | Close tab / next split |

---

**Tip**: If a keybinding isn't working, make sure the correct panel has focus. Look for the highlighted border to see which panel is active.
