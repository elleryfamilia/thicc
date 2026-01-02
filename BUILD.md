# THICC Build Instructions

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

## Building THICC

### Option 1: Quick Build

```bash
cd ~/_git/thicc
go build -o thicc ./cmd/thicc
```

This creates a `thicc` binary in the current directory.

### Option 2: Build with Make

```bash
cd ~/_git/thicc
make build
```

### Option 3: Install to PATH

```bash
cd ~/_git/thicc
go install ./cmd/thicc
```

This installs `thicc` to `$GOPATH/bin` (usually `~/go/bin`).

## Running THICC

### Run directly

```bash
./thicc
```

### Run with a specific file

```bash
./thicc README.md
```

### Run with debug logging

```bash
./thicc -debug
# Check ./log.txt for debug output
```

## Testing the Integration

### What to Test

1. **Launch thicc**
   ```bash
   ./thicc
   ```

2. **Check the log file**
   ```bash
   tail -f log.txt
   ```

   You should see:
   - "THICC: Setting up layout with root: /path/to/dir"
   - "THICC layout initialized with root: /path/to/dir"
   - "THICC file tree initialized - Use Alt-t to focus tree"

3. **Verify micro still works**
   - File editing should work normally
   - Try navigating with arrow keys
   - Try saving a file

## Current Implementation Status

### ✅ Completed (Phase 1 & 2)

- **Repository Setup**
  - Forked micro v2.0.14
  - Renamed to `thicc`
  - Module renamed to `github.com/ellery/thicc`
  - All imports migrated

- **Filemanager Package** (`internal/filemanager/`)
  - `tree.go`: Core tree data structures (350 lines)
  - `git.go`: Git integration (140 lines)
  - `icons.go`: Nerd Font icons (180 lines)
  - `pane.go`: TreePane implementation (280 lines)

- **Layout Package** (`internal/layout/`)
  - `manager.go`: Basic layout manager (55 lines)

- **Integration**
  - Modified `cmd/thicc/micro.go` to call `InitThiccLayout()`
  - Added THICC-specific initialization

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

**Log shows "Warning: No active tab for THICC layout"**
- Normal if no files are opened
- Try: `./thicc README.md`

## File Structure

```
~/_git/thicc/
├── cmd/thicc/
│   ├── micro.go          ← Modified: Added InitThiccLayout()
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
- `thiccLayout declared but not used` (in main)
- `TreePane` field unused (in layout manager)

These will be resolved in Phase 3 when we complete the display integration.
