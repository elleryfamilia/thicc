# THOCK Build Instructions

## Prerequisites

1. **Install Go** (version 1.19 or later)
   ```bash
   # macOS
   brew install go

   # Or download from https://go.dev/dl/
   ```

2. **Verify Go installation**
   ```bash
   go version
   # Should output: go version go1.xx.x darwin/amd64
   ```

## Building THOCK

### Option 1: Quick Build

```bash
cd ~/_git/thock
go build -o thock ./cmd/thock
```

This creates a `thock` binary in the current directory.

### Option 2: Build with Make

```bash
cd ~/_git/thock
make build
```

### Option 3: Install to PATH

```bash
cd ~/_git/thock
go install ./cmd/thock
```

This installs `thock` to `$GOPATH/bin` (usually `~/go/bin`).

## Running THOCK

### Run directly

```bash
./thock
```

### Run with a specific file

```bash
./thock README.md
```

### Run with debug logging

```bash
./thock -debug
# Check ./log.txt for debug output
```

## Testing the Integration

### What to Test

1. **Launch thock**
   ```bash
   ./thock
   ```

2. **Check the log file**
   ```bash
   tail -f log.txt
   ```

   You should see:
   - "THOCK: Setting up layout with root: /path/to/dir"
   - "THOCK layout initialized with root: /path/to/dir"
   - "THOCK file tree initialized - Use Alt-t to focus tree"

3. **Verify micro still works**
   - File editing should work normally
   - Try navigating with arrow keys
   - Try saving a file

## Current Implementation Status

### ✅ Completed (Phase 1 & 2)

- **Repository Setup**
  - Forked micro v2.0.14
  - Renamed to `thock`
  - Module renamed to `github.com/ellery/thock`
  - All imports migrated

- **Filemanager Package** (`internal/filemanager/`)
  - `tree.go`: Core tree data structures (350 lines)
  - `git.go`: Git integration (140 lines)
  - `icons.go`: Nerd Font icons (180 lines)
  - `pane.go`: TreePane implementation (280 lines)

- **Layout Package** (`internal/layout/`)
  - `manager.go`: Basic layout manager (55 lines)

- **Integration**
  - Modified `cmd/thock/micro.go` to call `InitThockLayout()`
  - Added THOCK-specific initialization

### ⚠️ Known Limitations (Current Build)

1. **Tree pane not yet visible**
   - The tree data structure is created
   - Integration with micro's display system is pending
   - Logs confirm initialization is working

2. **No keybindings yet**
   - Alt-t, Alt-e focus switching not implemented
   - Tree navigation not hooked up to key events

3. **File operations pending**
   - Open file in editor (from tree) - not implemented
   - Preview on select - not implemented
   - File creation/deletion - not implemented

### 🎯 Next Steps (Phase 3)

1. **Display Integration**
   - Add tree pane to micro's tab pane list
   - Hook into micro's display loop
   - Render tree in left column

2. **Keybindings**
   - Register Alt-t/Alt-e actions
   - Implement tree navigation (up/down/enter)
   - Implement expand/collapse

3. **File Operations**
   - Open file on Enter
   - Preview on cursor move
   - Create/rename/delete files

## Troubleshooting

### Build Errors

**"package X is not in GOROOT"**
```bash
go mod tidy
go mod download
```

**"undefined: X"**
- Check that all imports are correct
- Verify the package exists in `internal/`

### Runtime Errors

**"Fatal: Micro could not initialize a Screen."**
- Terminal emulator not supported
- Try a different terminal (iTerm2, Alacazitty, kitty)

**Log shows "Warning: No active tab for THOCK layout"**
- Normal if no files are opened
- Try: `./thock README.md`

## File Structure

```
~/_git/thock/
├── cmd/thock/
│   ├── micro.go          ← Modified: Added InitThockLayout()
│   ├── initlua.go
│   └── ...
├── internal/
│   ├── filemanager/      ← NEW: Tree implementation
│   │   ├── tree.go
│   │   ├── git.go
│   │   ├── icons.go
│   │   └── pane.go
│   ├── layout/           ← NEW: Layout manager
│   │   └── manager.go
│   ├── action/
│   ├── buffer/
│   ├── display/
│   └── ...
├── go.mod               ← Modified: Module renamed
├── go.sum
├── BUILD.md             ← This file
└── README.md
```

## Build Logs

When building, you might see warnings about unused variables/functions. This is normal for the current state - we've created structures that aren't fully integrated yet.

Expected warnings:
- `thockLayout declared but not used` (in main)
- `TreePane` field unused (in layout manager)

These will be resolved in Phase 3 when we complete the display integration.
