# THOCK Project Status & Plan

**Last Updated:** December 27, 2024
**Status:** In Development - TreePane Integration Blocked by Hang Issue

## Project Overview

**THOCK** is a hard fork of micro editor v2.0.14 designed to be an IDE-like terminal editor with:
- Built-in file tree browser (native Go, not Lua plugin)
- AI terminal integration (auto-detects Claude, Aider, Gemini, etc.)
- 3-pane layout: Tree + Editor + AI Terminal
- Optimized for AI-assisted development workflows

**Repository:** `/Users/ellery/_git/thock` (forked from micro v2.0.14)
**Module:** `github.com/ellery/thock`
**Config:** `~/.config/thock/` (separate from micro)

---

## Architecture Decisions

### Key Design Choices

1. **Hard Fork vs Plugin**
   - Decision: Hard fork with native Go components
   - Reason: Full control, tighter integration, better performance
   - Trade-off: Maintenance burden for upstream merges

2. **Native Go Filemanager**
   - Replaced Lua filemanager plugin (2,162 lines) with native Go
   - Location: `internal/filemanager/` package (~960 lines)
   - Benefits: Performance, maintainability, deeper integration

3. **AI Terminal Auto-Detection**
   - Detects installed AI CLIs: Claude, Aider, GitHub Copilot, Gemini, ChatGPT, OpenAI
   - Uses `exec.LookPath()` to check availability
   - Presents menu for selection
   - Location: `internal/aiterminal/detect.go`

4. **User Feedback: Filemanager Architecture**
   - User preference: "filemanager should be its own thing and not be another panel of micro"
   - Current: Integrated into micro's pane system
   - Future consideration: Standalone component managed separately from panes

---

## Completed Work

### Phase 1: Repository Setup ✅
- [x] Forked micro v2.0.14 to `/Users/ellery/_git/thock`
- [x] Renamed module: `github.com/zyedidia/micro/v2` → `github.com/ellery/thock`
- [x] Updated all import paths across codebase
- [x] Renamed binary: `micro` → `thock`
- [x] Created separate config directory: `~/.config/thock/`

### Phase 2: Native Filemanager Implementation ✅
- [x] Created `internal/filemanager/` package:
  - `tree.go` (350 lines) - Tree data structures, operations
  - `git.go` (140 lines) - Git integration, .gitignore support
  - `icons.go` (180 lines) - Nerd Font icon mappings (70+ file types)
  - `pane.go` (280 lines) - TreePane display component
- [x] Implemented tree operations:
  - Directory scanning with sorting (folders first)
  - Expand/collapse nodes
  - Navigate with j/k/h/l
  - Open files with Enter
  - Git-aware filtering
  - Auto-refresh capability

### Phase 3: AI Terminal Infrastructure ✅
- [x] Created `internal/aiterminal/` package:
  - `detect.go` (165 lines) - Auto-detect AI CLIs
  - `session.go` (150 lines) - PTY session management
  - `pane.go` (55 lines) - Terminal pane using shell.Terminal
  - `menu.go` (70 lines) - Tool selection menu
- [x] Detects 7 AI tools: Claude, Aider, GitHub Copilot, Gemini, ChatGPT, OpenAI, Default Shell
- [x] Integrated with micro's shell.Terminal for PTY handling
- [x] Created `action.AITermPane` wrapper for pane system

### Phase 4: Commands & Keybindings ✅
- [x] Registered commands:
  - `:tree` - Toggle file tree (CURRENTLY HANGS - SEE ISSUES)
  - `:treefocus` - Focus tree pane
  - `:editorfocus` - Focus editor pane
  - `:treerefresh` - Refresh file tree
  - `:aiterm` - Quick launch first AI CLI
  - `:ailauncher` - Show AI tool selection menu
- [x] Created keybindings (`~/.config/thock/bindings.json`):
  - `Alt-t` → Toggle tree
  - `Alt-e` → Focus editor
  - `Ctrl-Q` → Quit all

### Phase 5: Plugin Conflict Resolution ✅
- [x] Disabled Lua filemanager plugin:
  - Renamed: `~/.config/lazymicro/micro/plug/filemanager` → `.disabled`
  - Removed keybindings from bindings.json
  - Disabled init.lua auto-loading
- [x] Updated init.lua to not interfere with native layout

### Phase 6: Build System ✅
- [x] Updated Makefile:
  - Changed binary output to `thock`
  - Updated module paths
  - Added `build-dbg` target for debug builds
- [x] Created launcher script: `run-thock.sh`
- [x] Verified compilation works on macOS

---

## Current Issues & Blockers

### 🔴 CRITICAL: TreePane Creation Hangs Application

**Status:** BLOCKING - Application becomes unresponsive when creating tree pane

**Symptoms:**
- User types `:tree` or presses `Alt-t`
- Application hangs completely
- Terminal must be killed (Ctrl-C doesn't work)
- No log output generated (even with debug build)

**Investigation Done:**
1. Added extensive logging to `toggleTree()` function (lines 674-747 in micro.go)
2. Built debug version with `-X github.com/ellery/thock/internal/util.Debug=ON`
3. Attempted to capture logs during hang - no log.txt created

**Suspected Causes:**
1. `action.NewTreePane()` blocking in initialization
2. `node.VSplit(false)` causing deadlock
3. Tree rendering blocking the event loop
4. Display system integration issue

**Files Involved:**
- `/Users/ellery/_git/thock/cmd/thock/micro.go:674-747` - toggleTree function
- `/Users/ellery/_git/thock/internal/action/treepane.go` - TreePane wrapper
- `/Users/ellery/_git/thock/internal/filemanager/pane.go` - TreePane implementation

**Next Steps to Debug:**
1. ✅ Ensure debug build is actually running (timestamp: Dec 27 17:43)
2. ⏳ Capture log output showing where hang occurs
3. ⏳ Add goroutine to create tree async with timeout
4. ⏳ Simplify tree creation (use basic buffer instead of TreePane)
5. ⏳ Test tree creation in isolation outside event loop

**Workaround Attempted:**
- Disabled tree auto-loading on startup (prevents hang at launch)
- Made tree creation on-demand via command (still hangs)

---

## Technical Challenges Faced

### Challenge 1: Lua Plugin Conflicts
**Problem:** Old Lua filemanager plugin conflicted with native Go implementation
**Solution:** Disabled plugin, cleaned up config files, created separate config dir
**Learning:** Micro loads plugins from multiple locations, need clean separation

### Challenge 2: Debug Logging Not Working
**Problem:** Log statements not appearing in log.txt
**Root Cause:** Debug logging requires compile-time flag, not runtime flag
**Solution:** Use `make build-dbg` instead of `./thock -debug`
**Learning:** `util.Debug == "ON"` must be set via `-ldflags` during build

### Challenge 3: Buffer Creation Signature Changed
**Problem:** `buffer.NewBufferFromString()` required BufType parameter
**Error:** `not enough arguments in call`
**Solution:** Added `buffer.BTInfo` parameter for tree buffer
**Learning:** Check function signatures when porting code between versions

### Challenge 4: Import Path Migration
**Problem:** All imports needed updating from zyedidia/micro to ellery/thock
**Solution:** Systematic find/replace across entire codebase
**Files Changed:** ~50 Go files

### Challenge 5: Terminal Initialization Without TTY
**Problem:** Nil pointer panic when running in background
**Root Cause:** `screen.Init()` requires actual terminal
**Solution:** Only run interactively, not in background for testing
**Learning:** Can't test terminal apps without TTY

### Challenge 6: User Confusion - Command Entry
**Problem:** User didn't know how to enter commands (typed `:tree` in buffer)
**Solution:** Explained `Ctrl-e` opens command prompt, added `Alt-t` keybinding
**Learning:** Need better onboarding/help for micro's command system

---

## File Structure

```
/Users/ellery/_git/thock/
├── cmd/thock/
│   ├── micro.go              # Main entry, InitThockLayout, toggleTree
│   └── debug.go              # Debug logging setup
├── internal/
│   ├── filemanager/          # Native Go file tree (NEW)
│   │   ├── tree.go           # TreeNode, Tree data structures
│   │   ├── git.go            # Git integration, .gitignore
│   │   ├── icons.go          # Nerd Font icon mappings
│   │   └── pane.go           # TreePane display component
│   ├── aiterminal/           # AI terminal integration (NEW)
│   │   ├── detect.go         # Auto-detect AI CLIs
│   │   ├── session.go        # PTY session management
│   │   ├── pane.go           # Terminal pane wrapper
│   │   └── menu.go           # Tool selection menu
│   ├── layout/               # Layout management (NEW)
│   │   └── manager.go        # LayoutManager (minimal, for future use)
│   ├── action/
│   │   ├── treepane.go       # TreePane action wrapper (NEW)
│   │   └── aitermpane.go     # AITermPane action wrapper (NEW)
│   ├── buffer/               # Text buffers (EXISTING)
│   ├── display/              # Display system (EXISTING)
│   └── screen/               # Terminal screen (EXISTING)
├── runtime/
│   ├── plugins/              # Built-in plugins (no filemanager)
│   └── help/                 # Help documentation
├── Makefile                  # Build system (MODIFIED)
├── run-thock.sh              # Launcher script (NEW)
├── THOCK_PROJECT_STATUS.md   # This file (NEW)
└── log.txt                   # Debug log (generated by debug builds)
```

**Config Location:**
```
~/.config/thock/
├── bindings.json             # Keybindings (Alt-t, Ctrl-Q)
└── settings.json             # Settings (auto-generated)
```

---

## Build Instructions

### Standard Build
```bash
cd /Users/ellery/_git/thock
make quick                    # Fast build without generate step
```

### Debug Build (with logging)
```bash
cd /Users/ellery/_git/thock
make build-dbg                # Enables log.txt output
```

### Clean Build
```bash
cd /Users/ellery/_git/thock
make clean
make build                    # Full build with generate step
```

### Run
```bash
cd /Users/ellery/_git/thock
./run-thock.sh                # Uses clean config, clears old logs
# OR directly:
./thock
```

**Important:** After making code changes, always rebuild before testing!

---

## Commands Reference

### In THOCK Editor

**Open Command Prompt:** `Ctrl-e`

**Available Commands:**
- `:tree` - Toggle file tree (⚠️ CURRENTLY HANGS)
- `:aiterm` - Launch first available AI CLI
- `:ailauncher` - Show menu to select AI tool
- `:treefocus` - Focus tree pane (when working)
- `:editorfocus` - Focus editor pane
- `:treerefresh` - Refresh tree display
- `:quit` - Quit current pane
- `:quitall` - Quit all panes

**Keybindings:**
- `Ctrl-e` - Open command prompt
- `Alt-t` - Toggle tree (⚠️ CURRENTLY HANGS)
- `Alt-e` - Focus editor
- `Ctrl-Q` - Quit all

### Tree Navigation (when working)
- `j` / `Down` - Move down
- `k` / `Up` - Move up
- `h` / `Left` - Collapse folder
- `l` / `Right` - Expand folder
- `Enter` - Open file or toggle folder

---

## Next Steps & TODO

### Immediate Priority: Fix Tree Hang 🔴

#### Option 1: Debug Current Implementation
- [ ] Get log output during hang to identify exact blocking line
- [ ] Check if `NewTreePane()` is blocking on tree.Refresh()
- [ ] Check if `node.VSplit()` is blocking on layout calculation
- [ ] Add timeout wrapper around tree creation
- [ ] Try creating tree in goroutine with channel communication

#### Option 2: Simplify Tree Implementation
- [ ] Replace TreePane with simple buffer containing file list
- [ ] Use micro's standard vsplit instead of manual pane manipulation
- [ ] Render tree as plain text without BufWindow complexity
- [ ] Test if basic file list works without hanging

#### Option 3: Alternative Architecture
- [ ] Implement user's suggestion: "filemanager as its own thing"
- [ ] Create standalone file browser (not integrated into pane system)
- [ ] Use overlay/modal approach instead of persistent pane
- [ ] Investigate if micro's plugin system could work after all

### Phase 7: Complete Tree Integration
Once hang is fixed:
- [ ] Verify tree displays correctly
- [ ] Test tree navigation (j/k/h/l)
- [ ] Test file opening from tree
- [ ] Test tree refresh
- [ ] Add mouse support (hover highlight, click to open)
- [ ] Handle tree resize events
- [ ] Persist tree state (expanded nodes) across sessions

### Phase 8: AI Terminal Features
- [ ] Test AI terminal launch with actual AI CLIs
- [ ] Implement session persistence (save/restore)
- [ ] Add context bridge (send file to AI)
- [ ] Add commands:
  - `:sendfile` - Send current file to AI
  - `:sendselection` - Send selected code to AI
- [ ] Add keybindings:
  - `Alt-c` - Launch Claude
  - `Alt-g` - Launch Gemini
  - `Alt-s` - Send to AI

### Phase 9: Polish & UX
- [ ] Create welcome screen on first launch
- [ ] Add `:help thock` documentation
- [ ] Improve error messages
- [ ] Add progress indicators for slow operations
- [ ] Create installation script
- [ ] Add keyboard shortcut reference (`:shortcuts`)

### Phase 10: Testing & Stabilization
- [ ] Test on large directories (1000+ files)
- [ ] Test with different AI CLIs
- [ ] Test terminal resize handling
- [ ] Test session persistence
- [ ] Memory leak testing
- [ ] Performance profiling

### Future Enhancements
- [ ] Dashboard mode (project switcher)
- [ ] Project templates
- [ ] Git status integration in tree
- [ ] Syntax highlighting in tree preview
- [ ] Multiple AI terminal sessions
- [ ] Terminal tabs/split
- [ ] Custom layouts (save/load)

---

## Known Issues

### Critical
1. **Tree creation hangs application** (BLOCKING ALL TREE FEATURES)
   - Cannot open file tree
   - Must kill terminal to recover
   - No error messages or logs

### High
2. **No hover highlighting in tree** (user feedback)
   - Mouse hover doesn't highlight items
   - Makes it hard to see what you're about to click

### Medium
3. **Tree not shown on startup**
   - Currently requires manual `:tree` command
   - Should have option to auto-open on launch
   - Disabled to avoid hang on startup

4. **AI terminal not tested**
   - Built but not verified with actual AI CLIs
   - Unknown if session persistence works
   - Context bridge not implemented

### Low
5. **No installation script**
   - Manual build required
   - No automated setup
   - Config directory created manually

6. **Limited documentation**
   - No built-in help for THOCK features
   - Keybindings not discoverable
   - No tutorial or quickstart

---

## Dependencies

### Go Packages (in go.mod)
- `github.com/creack/pty` - PTY interface for terminals
- `github.com/micro-editor/terminal` - VT emulation
- `github.com/micro-editor/tcell/v2` - Terminal display
- `github.com/yuin/gopher-lua` - Lua plugin support
- All original micro dependencies

### External Tools (Optional)
User installs these for AI features:
- Claude Code CLI: `npm install -g @anthropic-ai/claude-code`
- Aider: `pip install aider-chat`
- GitHub Copilot CLI: Via GitHub CLI
- Gemini CLI: From Google
- ChatGPT CLI: `npm install -g chatgpt-cli`

### System Requirements
- macOS (tested) or Linux
- Go 1.19+ (for building)
- Terminal with Nerd Font (for icons)
- Git (for .gitignore support)

---

## Environment Variables

```bash
# Config directory (defaults to ~/.config/micro or ~/.config/thock)
export MICRO_CONFIG_HOME=~/.config/thock

# AI API Keys (for AI terminal features)
export ANTHROPIC_API_KEY="your_key"      # Claude
export OPENAI_API_KEY="your_key"         # ChatGPT/Codex
# Gemini uses Google account auth (no key needed)
```

---

## Debug Information

### Log Locations
- Debug log: `/Users/ellery/_git/thock/log.txt` (only with debug build)
- Must build with: `make build-dbg`
- Log created on startup if debug flag set

### Debug Build Verification
```bash
# Check if debug build
strings thock | grep "Debug=ON"  # Should output if debug build
ls -lh thock                      # Check timestamp
```

### Common Debug Tasks
```bash
# View recent logs
tail -f /Users/ellery/_git/thock/log.txt

# Search for errors
grep -i error /Users/ellery/_git/thock/log.txt

# Find where execution stopped
grep "THOCK:" /Users/ellery/_git/thock/log.txt | tail -20

# Clear logs before test
rm /Users/ellery/_git/thock/log.txt
```

---

## Lessons Learned

1. **Always verify debug builds are actually running**
   - Check binary timestamp
   - Verify log.txt is created
   - Don't assume `-debug` flag enables logging

2. **Terminal apps can't be tested in background**
   - Requires real TTY
   - Background tests cause nil pointer panics
   - Use interactive testing only

3. **Micro's pane system is complex**
   - VSplit/HSplit involve node tree manipulation
   - Pane indices shift when adding/removing
   - Need to understand views.Node system

4. **User experience matters**
   - Explaining `:tree` vs just typing isn't obvious
   - Keybindings are better than commands
   - Status messages help guide users

5. **Incremental integration is safer**
   - Starting with auto-tree caused hangs
   - On-demand tree creation isolated the issue
   - Can test components independently

6. **Logging is essential for debugging hangs**
   - Can't debug without seeing where it stops
   - Strategic log placement around blocking operations
   - Debug builds must be verified before trusting logs

---

## Questions for Future Sessions

1. **Tree Hang Issue:**
   - Where exactly is the code blocking?
   - Is it in tree rendering, VSplit, or pane creation?
   - Should we abandon current approach and simplify?

2. **Architecture:**
   - Should filemanager be standalone vs integrated pane?
   - Is TreePane complexity worth it vs simple buffer?
   - Can we use overlay instead of persistent pane?

3. **AI Terminal:**
   - How to test with actual AI CLIs?
   - Should terminal be always-on or on-demand like tree?
   - How to implement context bridge effectively?

4. **Performance:**
   - How does tree handle 1000+ files?
   - Is auto-refresh causing issues?
   - Should we implement lazy loading?

---

## Contact & Workflow Notes

**User Preferences:**
- Wants filemanager as "its own thing, not another panel"
- Prefers Ctrl-Q to quit all
- Uses mouse for tree interaction
- Expects hover highlighting

**Workflow:**
1. User runs: `/Users/ellery/_git/thock/run-thock.sh`
2. Types commands via `Ctrl-e` prompt
3. Uses `Ctrl-Q` to quit
4. Reports issues via terminal output
5. Checks logs when debugging

**Build Process:**
- User runs `make quick` or `make build-dbg`
- Testing from `/Users/ellery/_git/thock`
- Uses separate config in `~/.config/thock`

---

## Success Criteria

### MVP Complete When:
- [x] THOCK binary builds and runs
- [x] Editor works (typing, saving, basic editing)
- [x] Commands system working (Ctrl-e)
- [x] AI detection working
- [ ] **Tree pane opens without hanging** ← CURRENT BLOCKER
- [ ] Tree navigation works (j/k/h/l/Enter)
- [ ] AI terminal can be launched
- [ ] Ctrl-Q quits reliably

### Production Ready When:
- [ ] All MVP criteria met
- [ ] Tree stable with large directories
- [ ] AI terminals tested with real CLIs
- [ ] Context bridge functional
- [ ] Documentation complete
- [ ] Installation script available
- [ ] No critical bugs
- [ ] Performance acceptable

---

**End of Status Document**
