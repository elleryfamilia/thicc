# The 3-Panel Layout

thicc's layout is one of its core opinions: three panels, side by side, optimized for AI-assisted development.

## Why This Layout?

```
┌─────────────┬─────────────────────┬─────────────────┐
│             │                     │                 │
│    File     │       Editor        │    Terminal     │
│   Browser   │                     │                 │
│   (30 col)  │      (dynamic)      │     (45%)       │
│             │                     │                 │
└─────────────┴─────────────────────┴─────────────────┘
```

The idea is simple:
- **Browse** files on the left
- **Edit** code in the middle
- **Run commands** (and AI tools) on the right

When you're pairing with an AI assistant like Claude CLI, this layout shines. You can see the AI's output while reading the diff it produced—all without switching windows or tabs.

## Panel Dimensions

- **File Browser**: Fixed at 30 columns (expands to 40 when it's the only visible panel)
- **Editor**: Takes remaining space
- **Terminal**: 45% of screen width

The layout automatically adapts when you hide panels—remaining panels expand to fill the space.

## Focus and Navigation

### Cycling Focus

Press **`Ctrl+Space`** to cycle focus between visible panels:

```
File Browser → Editor → Terminal → File Browser → ...
```

The active panel has a highlighted border so you always know where your input will go.

### Direct Panel Focus

You can jump directly to a panel:

| Shortcut | Action |
|----------|--------|
| `Tab` | Jump from file browser to editor |
| `Shift+Tab` | Jump from editor to file browser |
| `Ctrl+Space` | Cycle to next panel |

## Showing and Hiding Panels

Toggle individual panels with `Alt` + number:

| Shortcut | Panel |
|----------|-------|
| `Alt+1` | Toggle file browser |
| `Alt+2` | Toggle editor |
| `Alt+3` | Toggle terminal |
| `Alt+4` | Toggle terminal 2 |
| `Alt+5` | Toggle terminal 3 |

### Example Workflows

**Full focus on code:**
- Press `Alt+1` to hide the file browser
- Press `Alt+3` to hide the terminal
- Now you have just the editor, fullscreen

**Side-by-side editing and terminal:**
- Press `Alt+1` to hide the file browser
- Editor and terminal split the screen

**Multiple terminals:**
- Press `Alt+4` to show a second terminal
- Press `Alt+5` for a third
- Each terminal runs independently

## Multiple Terminals

thicc supports up to three terminal panes:

| Terminal | Shortcut | Default Visibility |
|----------|----------|-------------------|
| Terminal 1 | `Alt+3` | Visible |
| Terminal 2 | `Alt+4` | Hidden |
| Terminal 3 | `Alt+5` | Hidden |

This is useful when you want to:
- Run your dev server in one terminal
- Run an AI assistant in another
- Keep a third for ad-hoc commands

Each terminal is fully independent with its own shell session.

## Visual Indicators

### Focus Border

The active panel has a visible border. When you press `Ctrl+Space`, watch for the border to move—that's how you know which panel has focus.

### Tab Bar

Above the editor, you'll see a tab bar showing all open files. The active file is highlighted.

## Layout Tips

1. **Start simple**: Use the default layout until you know what you need
2. **Hide what you don't need**: `Alt+1/2/3` are muscle-memory shortcuts
3. **Use multiple terminals**: One for builds, one for AI—no need to switch
4. **Ctrl+Space is your friend**: It's the fastest way to move around

## Multiplexer Detection

If you're running thicc inside tmux, Zellij, or screen, thicc automatically detects this. This helps with proper terminal handling and avoids conflicts with multiplexer keybindings.

---

Next: [File Browser](file-browser.md) — Learn to navigate and manage files
