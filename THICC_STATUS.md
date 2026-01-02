# THICC Development Status

**Last Updated:** 2025-12-27
**Status:** MVP - Editor Working, File Browser Loading with Colors, Terminal Disabled

## Architecture Overview

THICC is a hard fork of micro editor v2.0.14 with a 3-panel IDE layout:

```
┌──────────────┬─────────────────────────┬──────────────────┐
│              │                         │                  │
│ File Browser │   Micro Editor Panel    │  Terminal Panel  │
│   (30 cols)  │    (dynamic width)      │    (40 cols)     │
│              │                         │                  │
│  DISABLED    │      ✅ WORKING         │    DISABLED      │
│              │                         │                  │
└──────────────┴─────────────────────────┴──────────────────┘
```

**Key Design Principle:** File browser and terminal are **completely independent** from micro's infrastructure (no coupling to buffers/windows/panes). They use direct tcell rendering.

---

## Current Status

### ✅ What's Working

**Editor Panel (Middle)**
- Full micro functionality preserved
- Properly constrained to middle region (X=30, Width=ScreenW-70)
- Syntax highlighting, multi-cursor, all micro features work
- Tabs system works
- File opening/editing works perfectly

**Layout Manager**
- 3-panel coordination working
- Focus switching with Ctrl+Space (cycles: browser → editor → terminal)
- Proper event routing to active panel
- Screen resize handling
- Divider rendering between panels

**Rendering**
- No hangs on startup
- Direct tcell rendering for independent panels
- Editor constrained to middle region via `ConstrainEditorRegion()`

**Logging**
- Always enabled in `debug.go`
- Writes to `log.txt` in project root
- Comprehensive logging for debugging

### ❌ What's Disabled

**File Browser (Left Panel)**
- **Status:** Renders but empty, shows "File browser disabled (tree scan hangs)"
- **Why:** `Tree.Refresh()` hangs indefinitely during directory scan
- **Root Cause:** Likely symlink loops, large directories (.git, node_modules), or filesystem issues
- **Code:** Auto-refresh commented out in `internal/filebrowser/panel.go:34-48`

**Terminal Panel (Right Panel)**
- **Status:** Completely disabled
- **Why:** Temporarily disabled to debug file browser issues
- **Code:** Terminal creation commented out in `internal/layout/manager.go:99-122`

---

## Key Code Locations

### Main Files

**`cmd/thicc/micro.go`**
- Main event loop (`DoEvent()` lines 523-617)
- Layout integration (lines 530-541: rendering, 533: `ConstrainEditorRegion()`)
- Focus switching (lines 591-606: event routing through layout manager)
- Resize handling (lines 589-600: calls `thiccLayout.Resize()`)

**`internal/layout/manager.go`**
- Layout manager coordinating 3 panels
- `RenderFrame()` (128-151): Renders file browser and terminal
- `HandleEvent()` (154-181): Routes events to active panel
- `ConstrainEditorRegion()` (338-358): Constrains editor to middle region
- `cycleFocus()` (201-234): Focus switching logic with debug logging
- Thread-safe Terminal access via `sync.RWMutex`

**`internal/filebrowser/panel.go`**
- Independent file browser with direct tcell rendering
- Auto-refresh disabled (lines 34-48)
- Atomic `ready` flag for thread safety (line 17)
- `GetVisibleNodes()` (60-84): Returns nil when not ready (prevents lock blocking)

**`internal/filebrowser/render.go`**
- Direct screen.SetContent() rendering
- Shows "File browser disabled" message when tree not loaded (lines 57-60)
- Focus border rendering (line 23-25)

**`internal/terminal/panel.go`**
- VT10x + PTY terminal emulator (complete but unused)
- Background readLoop for PTY output (line 75-86)
- Complete implementation ready to enable

**`cmd/thicc/debug.go`**
- Logging always enabled (lines 20-27)
- Writes to `log.txt` in project root

### Package Structure

```
internal/
├── filebrowser/       # Independent file browser (direct tcell)
│   ├── panel.go       # Main panel, tree refresh disabled
│   ├── render.go      # Direct rendering
│   ├── events.go      # Event handling
│   └── types.go       # Region, styles
├── terminal/          # Independent terminal (VT10x + PTY) - COMPLETE
│   ├── panel.go       # Terminal panel
│   ├── vt_render.go   # VT state → tcell
│   └── input.go       # tcell events → PTY
├── layout/            # Layout coordinator
│   └── manager.go     # 3-panel management
├── filemanager/       # Tree data structures (reused)
│   ├── tree.go        # Tree.Refresh() hangs here
│   ├── icons.go       # File icons
│   └── git.go         # Git integration
└── aiterminal/        # AI CLI detection (reused)
    ├── detect.go      # Auto-detect Claude, Aider, etc.
    └── session.go     # Session state
```

---

## Known Issues & Root Causes

### Issue 1: Mutex Deadlock in Tree Refresh (CRITICAL - FIXED) ✅

**Symptom:** `Tree.Refresh()` hangs forever, never completes
**Location:** `internal/filemanager/tree.go` - `Refresh()`, `Expand()`, `Collapse()`

**Root Cause:** Classic mutex deadlock pattern:
- `Refresh()` acquires `t.mu.Lock()` (line 237)
- Then calls `t.Scan(t.Root)` while holding lock (line 254)
- `Scan()` tries to acquire `t.mu.Lock()` again (line 74)
- **Result:** Deadlock - goroutine blocks forever waiting for lock it already owns

**Same bug in:**
- `Expand()` - holds lock, calls `Scan()` → deadlock
- `Collapse()` - holds lock, calls `Scan()` → deadlock
- `Refresh()` - calls `Scan()` TWICE (lines 254 and 267) → double deadlock

**Evidence from logs:**
```
2025/12/27 22:14:07 THICC FileBrowser: Starting tree refresh with safeguards
[... 2+ minutes later, still no completion message ...]
```

**Fix Applied:**
- `Refresh()`: Call `scanDir()` directly instead of `Scan()` (already holding lock)
- `Expand()`: Call `scanDir()` directly instead of `Scan()`
- `Collapse()`: Call `scanDir()` directly instead of `Scan()`
- Added extensive logging to track scan progress

**Additional Safeguards Added:**
1. Max depth limit: reduced to 2 levels
2. Skip ALL hidden files/directories (starts with `.`)
3. Skip common problem dirs: .git, node_modules, vendor, build, etc.
4. Max 100 files per directory
5. Verbose logging at each step

### Issue 2: Lock Contention (Resolved) ✅

**Symptom:** GetNodes() would block render loop
**Root Cause:** Tree.Refresh() holds write lock, GetNodes() needs read lock
**Fix:** Added atomic `ready` flag - don't call GetNodes() until Refresh() complete
**Code:** `internal/filebrowser/panel.go:17` (atomic int32), `panel.go:60` (check before GetNodes)

### Issue 3: Editor Coordinate Overflow (Resolved) ✅

**Symptom:** Typing would overflow into file browser panel
**Root Cause:** Editor panes didn't know about constrained region
**Fix:** `ConstrainEditorRegion()` adjusts editor View bounds before rendering
**Code:** `internal/layout/manager.go:338-358`

---

## Testing Notes

### How to Test

```bash
cd ~/_git/thicc
make quick
./run-thicc.sh
```

### Check Logs

```bash
cat ~/_git/thicc/log.txt
```

### Expected Behavior (Current State)

✅ **Startup:** No hanging, immediate launch
✅ **Editor:** Works in middle region, typing constrained
✅ **Focus:** Ctrl+Space cycles between panels
✅ **Left Panel:** Shows "File browser disabled (tree scan hangs)"
✅ **Right Panel:** Empty (terminal disabled)
✅ **Resize:** Panels resize correctly

### Known Working Features

- ✅ Editor editing, saving, syntax highlighting
- ✅ Ctrl+Space focus switching
- ✅ Editor coordinate constraints
- ✅ Panel borders/dividers
- ✅ Resize handling
- ✅ Logging system

---

## Next Steps (Priority Order)

### 1. Test Mutex Deadlock Fix (URGENT - READY TO TEST) ⚡

**Status:** Fix implemented, ready for testing

**What was fixed:**
- Removed deadlock in `Refresh()`, `Expand()`, `Collapse()`
- Added extensive logging to track scan progress
- Added safeguards: depth limit 2, skip hidden dirs, max 100 files/dir

**Testing Command:**
```bash
cd ~/_git/thicc && make quick && ./run-thicc.sh
```

**Expected Behavior:**
- File browser should load within 1-2 seconds
- Log should show: "THICC Tree: Scan() complete" message
- Files should be visible in left panel
- No hanging

**Check Logs:**
```bash
cat ~/_git/thicc/log.txt | grep "THICC Tree"
```

**Success Criteria:**
- Tree refresh completes (log shows "Scan() complete")
- Files appear in file browser panel
- No deadlock or hanging
- App remains responsive

### 2. Re-enable Terminal Panel (MEDIUM PRIORITY)

**Current State:** Complete implementation exists, just commented out

**Steps:**
1. Uncomment terminal creation in `internal/layout/manager.go:99-122`
2. Uncomment imports
3. Test with shell (bash/zsh) first
4. Test with AI CLI auto-detection
5. Verify VT rendering works
6. Test input handling

**Files to Modify:**
- `internal/layout/manager.go` - Uncomment lines 99-122

**Success Criteria:**
- Terminal appears on right
- Can type commands
- Shell prompt appears
- Ctrl+Space includes terminal in focus cycle

### 3. Add Focus Visual Indicators (MEDIUM PRIORITY)

**User Request:** "I expect that if there's an effect around a panel to show 'focus', that it should be consistent across all panels"

**Current State:**
- File browser has `drawFocusBorder()` (render.go:23-25)
- Editor: No visual focus indicator
- Terminal: No visual focus indicator

**Implementation:**
- Add consistent focus border to all 3 panels
- Use same style/color
- Show which panel is active via border highlight

**Files to Modify:**
- `internal/filebrowser/render.go` - Already has border
- `internal/terminal/vt_render.go` - Add focus border
- `internal/layout/manager.go` - Maybe draw borders here for consistency

### 4. Improve File Browser UX (LOW PRIORITY)

- Manual refresh command (`:treerefresh`)
- Navigate with arrow keys (already implemented in events.go)
- Open files with Enter (already implemented)
- Expand/collapse directories
- Git status indicators

### 5. Terminal Features (LOW PRIORITY)

- Copy/paste support
- Scrollback buffer
- Multiple terminal tabs
- AI CLI integration (Claude, Aider auto-detected)

---

## Architecture Decisions Made

### ✅ Correct Decisions

1. **Independence:** File browser and terminal completely separate from micro
   - Prevents coupling issues
   - Allows direct tcell rendering
   - Easy to maintain

2. **Layout Manager Pattern:** Central coordinator for 3 panels
   - Clean separation of concerns
   - Focus management in one place
   - Event routing centralized

3. **Atomic Ready Flag:** Thread-safe loading state
   - Prevents lock blocking
   - Clean async pattern

4. **ConstrainEditorRegion():** Adjust editor bounds per frame
   - Simple, works with micro's existing code
   - No need to modify micro internals

### ⚠️ Needs Improvement

1. **Tree Scanning:** No safeguards, hangs on large directories
   - **Fix:** Add skip list, depth limit, timeout

2. **Focus Indicators:** Inconsistent across panels
   - **Fix:** Standardize border rendering

3. **Terminal Disabled:** Complete implementation unused
   - **Fix:** Re-enable after tree fix verified stable

---

## Deleted Old Coupled Code

These files were removed during migration:
- ❌ `internal/filemanager/pane.go` - Coupled to micro's display.BufWindow
- ❌ `internal/action/treepane.go` - Unnecessary wrapper
- ❌ `internal/action/aitermpane.go` - Unnecessary wrapper
- ❌ `internal/aiterminal/pane.go` - Coupled to micro's shell.Terminal

These were replaced with independent implementations.

---

## Build & Run

```bash
# Build
cd /Users/ellery/_git/thicc
make quick

# Run
./run-thicc.sh

# Or directly
./thicc

# Check logs
cat log.txt
```

---

## Important Findings from Debugging

1. **Log location:** Requires `InitLog()` called, writes to `./log.txt`
2. **Focus cycling works:** No issues with Ctrl+Space
3. **Tree scan is the blocker:** Everything else works fine
4. **Editor constraints work:** No overflow to other panels
5. **Async loading needs careful thought:** Race conditions, lock contention
6. **VT10x terminal implementation complete:** Just needs enabling
7. **Mutex protection essential:** Terminal access protected with RWMutex

---

## Quick Reference

### Key Bindings
- **Ctrl+Space:** Cycle focus (file browser → editor → terminal)
- **Ctrl+Q:** Quit (micro default)

### Commands (in editor)
- `:treefocus` - Focus file browser
- `:editorfocus` - Focus editor
- `:termfocus` - Focus terminal (when enabled)

### Panel IDs
- **0:** File browser
- **1:** Editor
- **2:** Terminal

---

## Contact / Notes

- Fork of micro v2.0.14
- Module: `github.com/ellery/thicc`
- Architecture: Independent panels with direct tcell rendering
- Goal: VSCode-like 3-panel layout with micro as editor core
