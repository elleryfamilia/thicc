# Dashboard Package

The dashboard package provides a welcome screen displayed when thock starts without file/directory arguments. It features a Spider-Verse inspired color theme and provides quick access to common actions.

## Features

- **New File** - Create an empty buffer and enter the editor
- **Open Project** - Navigate to and open a project folder via the Project Picker
- **Recent Projects** - Quick access to recently opened files and folders (1-9 shortcuts)
- **Exit** - Quit thock

## Visual Design

```
                    THOCK LOGO (Pink/Cyan two-tone)

    ╔══════════════════════════════════════╗
    ║  > New File              n           ║
    ║    Open Project          o           ║
    ║                                      ║
    ║    Recent                            ║
    ║    ──────────────────────            ║
    ║    1. myproject/                     ║
    ║    2. config.go                      ║
    ║    3. another-folder/                ║
    ║                                      ║
    ║    Exit                  q           ║
    ╚══════════════════════════════════════╝

    [n] New  [o] Open  [q] Quit           v0.1.0
```

## Color Scheme (Spider-Verse)

| Color | Usage |
|-------|-------|
| **Cyan (Color51)** | Borders, section dividers, tagline |
| **Yellow (Color226)** | Selection highlights |
| **Pink (Color205)** | Logo solid blocks, accents |
| **White** | Body text |
| **Dark gray (Color16)** | Background |

## Architecture

### Files

| File | Purpose |
|------|---------|
| `dashboard.go` | Main Dashboard struct, state management, callbacks |
| `render.go` | All rendering methods |
| `events.go` | Keyboard and mouse event handling |
| `recent.go` | Recent projects persistence (JSON) |
| `project_picker.go` | Project folder navigation modal |
| `ascii_art.go` | THOCK logo (Figlet Rebel font) |
| `colors.go` | Spider-Verse color definitions and styles |

### Key Data Structures

```go
// Dashboard is the main welcome screen
type Dashboard struct {
    Screen        tcell.Screen
    ScreenW, ScreenH int

    MenuItems     []MenuItem
    SelectedIdx   int

    RecentStore   *RecentStore
    RecentIdx     int
    InRecentPane  bool

    ProjectPicker *ProjectPicker

    // Callbacks
    OnNewFile     func()
    OnOpenProject func(path string)
    OnOpenFile    func(path string)
    OnOpenFolder  func(path string)
    OnExit        func()
}

// ProjectPicker is a modal for navigating folders
type ProjectPicker struct {
    Active      bool
    InputPath   string
    CursorPos   int
    CurrentDir  string
    Entries     []DirEntry
    FilteredList []DirEntry
    SelectedIdx int
    // ...
}

// RecentProject represents a recently opened file/folder
type RecentProject struct {
    Path       string    `json:"path"`
    Name       string    `json:"name"`
    IsFolder   bool      `json:"is_folder"`
    LastOpened time.Time `json:"last_opened"`
}
```

## Keyboard Navigation

### Dashboard

| Key | Action |
|-----|--------|
| `n` | New File |
| `o` | Open Project Picker |
| `1-9` | Open recent project by number |
| `↑/↓` or `j/k` | Navigate menu |
| `←/→` or `h/l` | Switch menu ↔ recent pane |
| `Tab` | Cycle between panes |
| `Enter` | Activate selection |
| `q` / `Esc` | Exit thock |
| `Delete` | Remove selected recent item |

### Project Picker

| Key | Action |
|-----|--------|
| Type | Edit path in input, filter directory list |
| `←/→` | Move cursor in input field |
| `↑/↓` | Navigate directory list |
| `Tab` | Complete path OR drill into selected folder |
| `Backspace` | Delete char, OR at `/` go up one directory |
| `Enter` | Open highlighted folder as project |
| `Esc` | Cancel and return to dashboard |

## Project Picker Behavior

The Project Picker provides an elegant way to navigate to project folders:

1. **Start**: Shows home directory (`~/`) contents
2. **Type path**: Input field accepts paths like `~/_git/`
3. **Tab completion**: Completes partial paths like bash
4. **Fuzzy filtering**: Typing filters the directory list
5. **Drill down**: Tab on a selected folder enters it
6. **Go up**: Backspace at a `/` boundary goes to parent
7. **Open**: Enter opens the selected folder as a project

Example flow:
```
~/_git/█           ← Type "_git", Tab to complete
~/_git/            ← Shows _git contents
  thock/
> thock-dashboard-screen/   ← Press ↓ to select
  other-project/
                   ← Press Enter to open
```

## Recent Projects Persistence

Recent projects are stored at `~/.config/micro/thock/recent.json`:

```json
{
  "projects": [
    {
      "path": "/Users/ellery/_git/thock",
      "name": "thock",
      "is_folder": true,
      "last_opened": "2024-01-15T10:30:00Z"
    }
  ]
}
```

- Maximum 10 items
- Sorted by last opened (most recent first)
- Non-existent paths are cleaned up on load

## Integration

The dashboard is shown when thock starts without arguments:

```go
// In cmd/thock/micro.go
if len(args) == 0 && isatty.IsTerminal(os.Stdin.Fd()) && isatty.IsTerminal(os.Stdout.Fd()) {
    showDashboard = true
}

if showDashboard {
    InitDashboard()
    for showDashboard {
        DoDashboardEvent()
    }
}
// Then transition to normal editor...
```

Callbacks are wired in `InitDashboard()` to handle transitions:

```go
thockDashboard.OnNewFile = func() {
    showDashboard = false
    TransitionToEditor(nil, "")
}

thockDashboard.OnOpenProject = func(path string) {
    showDashboard = false
    os.Chdir(path)
    TransitionToEditor(nil, "")
    thockDashboard.RecentStore.AddProject(path, true)
}
```
