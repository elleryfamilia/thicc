# File Browser

The file browser lives on the left side of thicc, giving you quick access to your project files without leaving the editor.

<!-- TODO: screenshot of file browser focused -->

## Navigation

### Keyboard

| Key | Action |
|-----|--------|
| `↑` / `k` | Move up |
| `↓` / `j` | Move down |
| `←` / `h` | Collapse folder / go to parent |
| `→` / `l` | Expand folder / enter folder |
| `Enter` | Open file / toggle folder |
| `Page Up/Down` | Move by page |
| `Home` / `g` | Go to top |
| `End` / `G` | Go to bottom |

### Mouse

- **Click a file**: Open it in the editor
- **Click a folder**: Expand or collapse it
- **Scroll wheel**: Scroll the file list
- **Click the project path** (header): Open the project picker

## File Operations

When the file browser is focused, you can create, rename, and delete files:

| Shortcut | Action |
|----------|--------|
| `n` | Create new file |
| `f` | Create new folder |
| `d` | Delete selected item |
| `r` | Refresh the tree |
| `R` | Rename selected item |

When you create or rename a file, a modal will appear for you to enter the name.

**Note**: Delete prompts for confirmation before removing files.

## Git Integration

The file browser shows git status for tracked files with colored icons:

| Icon | Meaning |
|------|---------|
| ✎ (pencil) | Modified |
| ✓ (check) | Staged |
| ? (question) | Untracked |
| ✗ (x mark) | Deleted |
| → (arrow) | Renamed |
| ⚠ (warning) | Conflict |

These indicators help you see at a glance which files have changed since your last commit.

## File Icons

thicc uses Nerd Font icons to show file types at a glance. You'll see different icons for:

- Folders (open/closed states)
- Source files (Go, Python, JavaScript, etc.)
- Config files (JSON, YAML, TOML)
- Documents (Markdown, text)
- And many more

**Icons not showing?** Make sure you have a Nerd Font installed and set as your terminal font. See the [FAQ](faq.md) for setup instructions.

## Auto-Refresh

The file browser watches your project directory for changes. If you create, modify, or delete files outside of thicc (for example, from the terminal pane or another tool), the file browser will automatically update to reflect those changes.

This is powered by [fsnotify](https://github.com/fsnotify/fsnotify), which watches for filesystem events in the background.

## Project Header

At the top of the file browser, you'll see the current project path. Click on it (or press `Enter` when it's selected) to open the **project picker**, which lets you switch to a different directory.

## Performance

The file browser is designed to handle large projects efficiently:

- **Depth limiting**: Very deeply nested directories are truncated
- **Skip list**: Common directories like `node_modules`, `.git`, and `vendor` are skipped by default
- **Background scanning**: The tree loads in the background so thicc stays responsive
- **Lazy loading**: Directories are only scanned when you expand them

## Tips

1. **Quick preview**: As you navigate with arrow keys, files are automatically previewed in the editor
2. **Toggle visibility**: Press `Alt+1` to hide/show the file browser when you need more editor space
3. **Jump to file**: Use `Tab` to jump from the file browser directly to the editor
4. **Focused expansion**: When the file browser is the only visible panel, it expands to 40 columns for easier reading

---

Next: [Terminal](terminal.md) — Learn about the terminal pane and AI integration
