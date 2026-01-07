# Getting Started

This guide will get you up and running with thicc in about 5 minutes.

## Launching thicc

There are three ways to start thicc:

```sh
thicc              # Opens the dashboard (project picker)
thicc .            # Opens the current directory
thicc path/to/file # Opens a specific file
```

When you run `thicc` without arguments, you'll see the **dashboard**—a welcome screen where you can open recent projects, create new files, or set up AI tools.

![thicc dashboard](../assets/thicc_dashboard.png)

When you open a directory or file, you'll see the **main view**—three panels side by side.

## The Layout

thicc's interface is divided into panels:

```
┌─────────────┬─────────────────────┬─────────────────┐
│             │                     │                 │
│    File     │       Editor        │   Terminal(s)   │
│   Browser   │                     │    (1, 2, 3)    │
│             │                     │                 │
└─────────────┴─────────────────────┴─────────────────┘
```

- **Left**: File browser for navigating your project
- **Middle**: Editor for writing and reading code
- **Right**: Terminal for running commands—**up to 3 terminals** can be shown

By default, one terminal is visible. Press `Alt+4` for a second terminal, `Alt+5` for a third. This lets you run your dev server in one, an AI assistant in another, and keep a third for git commands—all visible at once.

## Moving Between Panels

The most important shortcut to learn:

**`Ctrl+Space`** cycles focus between panels: File Browser → Editor → Terminal → File Browser

You'll see a visual border around the active panel. Try pressing `Ctrl+Space` a few times to get a feel for it.

## Opening Files

**From the file browser:**
1. Use **arrow keys** to navigate up and down
2. Press **Enter** to:
   - Open a folder (expand/collapse)
   - Open a file in the editor

Press `Tab` to jump from the file browser directly to the editor.

**Quick file finder:**
Press **`Ctrl+P`** from anywhere to open the quick file finder. Start typing a filename and use arrow keys to select from the fuzzy-matched results. Press `Enter` to open.

## Editing

Once a file is open in the editor:
- Type to insert text (no special mode needed)
- **`Ctrl+S`** saves the file
- **`Ctrl+Z`** undoes changes
- **`Ctrl+Y`** redoes changes

The editor supports syntax highlighting for 100+ languages, multiple cursors, and all the features you'd expect.

## Using the Terminal

Press `Ctrl+Space` until the terminal panel is focused, then:
- Type commands as you normally would
- Run your build tools, git commands, or AI assistants
- The terminal is a full PTY—it supports colors, interactive programs, and more

To return to editing, press `Ctrl+Space` to cycle back to the editor.

## Creating New Files

From the dashboard:
- Select **New File** and enter a filename

From the file browser:
- Press `n` to create a new file
- Press `N` to create a new folder

## Exiting thicc

Press **`Ctrl+Q`** to quit.

If you have unsaved changes, thicc will prompt you to save or discard them.

## What's Next?

Now that you know the basics:

- [The Layout](layout.md) — Understand how to toggle and arrange panels
- [File Browser](file-browser.md) — Learn file browser features like git integration
- [Terminal](terminal.md) — Set up your terminal for AI tools
- [Keybindings](keybindings.md) — See all available shortcuts

---

**Tip**: Press `Alt+G` to toggle a hints bar at the bottom showing common keybindings.
