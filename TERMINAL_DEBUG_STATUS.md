# THICC Terminal Debugging Status - Dec 30, 2025

## Original Problem

When running CLI tools (Claude, Codex, Gemini) in THICC's terminal pane:
- ❌ Missing borders (box-drawing characters don't appear)
- ❌ Missing headings/section titles
- ❌ Large whitespace gaps (~100px) before input cursor
- ❌ Only partial content renders (e.g., "Welcome back ellery" but missing ASCII art, formatted sections)
- ✅ Basic text does appear

Example: Claude CLI should show:
- Welcome message with ASCII art
- Bordered sections with tips
- Recent activity in formatted boxes
- Proper spacing

Actually shows:
- Welcome message only
- Tips without headings
- Huge gap before cursor
- No borders/boxes

## Root Cause Hypothesis

Initially suspected **DSR/CPR (Device Status Report/Cursor Position Report)** issue:
- CLI tools send `ESC[6n` to query cursor position
- Terminal emulator should respond with `ESC[row;colR`
- Without response, tools make wrong layout assumptions → gaps, missing content

## Changes Made So Far

### 1. Fixed DSR/CPR Response (COMPLETED)

**File**: `/Users/ellery/_git/thicc/internal/terminal/panel.go`

**Change** (line 95):
```go
// BEFORE (broken):
vt := vt10x.New(vt10x.WithSize(contentW, contentH))

// AFTER (fixed):
vt := vt10x.New(vt10x.WithSize(contentW, contentH), vt10x.WithWriter(ptmx))
```

**Why**: Connects vt10x response writer to PTY, enabling DSR/CPR responses to flow back to applications.

**Status**: ✅ Applied and built

### 2. Added Debug Logging (COMPLETED)

**File**: `/Users/ellery/_git/thicc/internal/terminal/panel.go`

**Added**:
- Direct file logging to `/tmp/thicc-terminal-debug.log` (bypasses stderr issues)
- Logs all PTY→VT data (program output)
- Logs all User→PTY data (keyboard input)
- Logs VT state after each update (cursor position, size, visibility)
- Logs terminal panel creation

**Key functions added**:
```go
var debugLog *os.File

func init() {
    debugLog, err = os.OpenFile("/tmp/thicc-terminal-debug.log",
        os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
}

func logDebug(format string, args ...interface{}) {
    if debugLog != nil {
        fmt.Fprintf(debugLog, format+"\n", args...)
        debugLog.Sync() // Force write
    }
}
```

**Logging points**:
- `NewPanel()`: Logs panel creation with size and command
- `readLoop()`: Logs every PTY→VT data transfer and VT state
- `Write()`: Logs every User→PTY input
- `DumpState()`: Dumps entire VT buffer (available but not auto-called)

**Status**: ✅ Applied and built

### 3. Created Test Script (COMPLETED)

**File**: `/tmp/test-terminal.sh`

Tests 5 terminal features:
1. **Colors** - Red, green, bold blue text
2. **Box-drawing characters** - Unicode borders (┌─┐│└┘)
3. **DSR/CPR** - Cursor position query `ESC[6n`
4. **Unicode** - Chinese, emojis, special chars
5. **DEC line drawing** - Alternative charset graphics

**Status**: ✅ Created and executable

### 4. Updated README (COMPLETED)

**File**: `/Users/ellery/_git/thicc/README.md`

Added THICC-specific section documenting Nerd Font requirement with installation instructions for macOS/Linux and terminal configuration examples.

**Status**: ✅ Applied

## Test Results So Far

### Test Script Results (`/tmp/test-terminal.sh`)

**Screenshot analysis from user**:

✅ **Test 1: Colors** - WORKING
- Red, green, bold blue all render correctly
- Color escape sequences work perfectly

✅ **Test 2: Box-drawing characters** - WORKING
- Perfect box with borders (┌─┐│└┘) renders correctly
- Unicode box-drawing range (U+2500-U+257F) works!

⚠️ **Test 3: DSR/CPR** - PARTIALLY WORKING
```
Sending ESC[6n... ^[[54;19RNo response (timeout)
```

**Critical finding**: The response `^[[54;19R` (cursor at row 54, col 19) **IS being sent**, BUT:
- It appears as **visible output** on screen (`^[[54;19R`)
- It's NOT being consumed by the shell's `read` command
- The `read` command times out

**What this means**:
- `vt10x.WithWriter(ptmx)` IS working (response is sent)
- BUT the response goes to PTY master → read by THICC as output → displayed
- Should go: PTY master → PTY slave → application's stdin

**Possible issue**: Response feedback loop - vt10x writes to ptmx, THICC reads from ptmx as output instead of it going to app's stdin.

✅ **Test 4: Unicode** - WORKING
- Chinese characters (你好), emojis (🎨), checkmarks (✓✗) all display

⚠️ **Test 5: DEC line drawing** - PARTIALLY WORKING
- Shows box but some characters corrupted
- Alternative charset mode may not be fully supported

### Key Discoveries

1. **Box-drawing Unicode WORKS** ✅
   - Since Test 2 shows perfect boxes, the issue is NOT Unicode rendering
   - Claude CLI's missing borders are NOT due to Unicode support

2. **DSR/CPR response is sent but goes to wrong place** ⚠️
   - Response is being written (we see `^[[54;19R`)
   - But it's appearing as terminal output instead of going to app stdin
   - This might still be causing Claude CLI to timeout/make wrong assumptions

3. **Colors and basic escape sequences work** ✅
   - SGR (Select Graphic Rendition) codes work
   - 256-color support confirmed

4. **vt10x is processing escape sequences** ✅
   - VT emulator is parsing and handling most sequences
   - Issue is more about response routing than parsing

## Current Status: AWAITING LOGS

### What We Need

**Logs from running Claude CLI** to see:
1. What escape sequences Claude CLI actually sends
2. What responses (if any) it receives
3. Whether box-drawing characters appear in PTY output
4. Cursor position tracking during startup
5. Any timing issues or sequence ordering problems

### How to Get Logs

**In regular terminal** (not Claude Code):

```bash
cd /Users/ellery/_git/thicc

# Clear old logs
rm -f /tmp/thicc-terminal-debug.log

# Run THICC
./run-thicc.sh

# In THICC's terminal pane:
# 1. Run test script first: /tmp/test-terminal.sh
# 2. Then run: claude
# 3. Let it start (don't interact much)
# 4. Exit THICC

# Check logs
cat /tmp/thicc-terminal-debug.log

# If logs are huge, get first 200 lines
head -200 /tmp/thicc-terminal-debug.log > /tmp/thicc-debug-snippet.txt
```

**Expected log format**:
```
=== Terminal Panel Created: size=80x24 (content: 78x22), cmd=[/bin/zsh] ===
THICC PTY->VT: 45 bytes: "\x1b[?1034h\x1b[?2004h..."
THICC VT State: size=78x22 cursor=0,0 visible=true
THICC User->PTY: 1 bytes: "c"
THICC PTY->VT: 12 bytes: "claude\r\n..."
...
```

## Files Modified

| File | Status | Description |
|------|--------|-------------|
| `internal/terminal/panel.go` | ✅ Modified | Added DSR/CPR fix + debug logging |
| `README.md` | ✅ Modified | Added Nerd Font documentation |
| `/tmp/test-terminal.sh` | ✅ Created | Terminal capabilities test script |
| `thicc` binary | ✅ Built | Contains all fixes and logging |

## Build Status

✅ **THICC built successfully** with all changes

**Built binary**: `/Users/ellery/_git/thicc/thicc`

**Verify changes in binary**:
```bash
cd /Users/ellery/_git/thicc
./run-thicc.sh  # Uses correct config (MICRO_CONFIG_HOME=~/.config/thicc)
```

**DO NOT run** `./thicc` directly without setting `MICRO_CONFIG_HOME` or you'll get old micro config.

## Next Steps

### Step 1: Collect Debug Logs
Run THICC and capture logs as described above.

### Step 2: Analyze Logs
Look for:
- **DSR/CPR sequences**: Search for `ESC[6n` in output and `ESC[` followed by numbers and `R` in responses
- **Box-drawing characters**: Search for bytes in range `\xe2\x94\x80` to `\xe2\x95\xbf` (UTF-8 encoded box chars)
- **Missing content**: Compare what Claude CLI sends vs what appears in VT buffer
- **Timing issues**: Large gaps between sequences might indicate timeouts

### Step 3: Potential Fixes Based on Logs

**If DSR/CPR is still the issue**:
- May need to handle response routing differently
- PTY echo might need configuration
- Response might need to go to a different file descriptor

**If box-drawing chars are in PTY output but not rendering**:
- Check `vt_render.go` - ensure glyphs are converted correctly
- May need to verify tcell's Unicode handling
- Check if Nerd Font is configured in outer terminal

**If escape sequences aren't being parsed**:
- May need to upgrade vt10x library
- Add custom sequence handlers
- Check vt10x documentation for supported sequences

**If terminal size mismatch**:
- Verify PTY size == VT size
- Check COLUMNS/LINES environment variables
- Log size after each resize event

## Alternative Debugging Approaches

### If logs don't help, try:

1. **Capture raw PTY output to file**:
   - Modify `readLoop()` to also write raw bytes to `/tmp/pty-raw.bin`
   - Compare with output from regular terminal using `script`

2. **Run Claude CLI in regular terminal and capture**:
   ```bash
   script -q /tmp/claude-real.txt
   claude
   # Interact
   exit
   ```
   Then compare escape sequences with what THICC receives

3. **Test simpler TUI apps first**:
   - `htop` - should show colors and boxes
   - `vim` - should handle cursor movement
   - `less` - should handle scrolling

   If these work but Claude CLI doesn't, issue is Claude-specific.

4. **Add VT buffer dump on exit**:
   Call `p.DumpState()` when Claude CLI exits to see final buffer state

## Known Working vs Broken

### ✅ Working:
- Colors (SGR codes)
- Box-drawing Unicode characters
- Basic Unicode (emojis, international chars)
- Cursor positioning (basic ESC[row;colH)
- Text rendering
- Keyboard input
- PTY management
- VT emulator initialization

### ❌ Broken:
- DSR/CPR response routing (response visible instead of consumed)
- Claude CLI full rendering
- Possibly: DEC line drawing mode
- Unknown: Other TUI applications

### ⚠️ Unknown/Untested:
- Mouse events in terminal programs
- Scrollback buffer
- Alternative screen buffer
- Clipboard integration in terminal apps
- True color (24-bit) support
- Other complex escape sequences

## Technical Details

### Architecture
```
User Input → THICC (tcell) → PTY Master → PTY Slave → Shell/App
                                ↓                         ↓
                          VT Emulator ← ← ← ← ← ← ← ← ← Output
                                ↓
                          Render to tcell
```

### The DSR/CPR Flow
```
1. App sends: ESC[6n → PTY Slave → PTY Master → THICC reads
2. THICC → VT Emulator processes ESC[6n
3. VT Emulator → ptmx.Write(ESC[row;colR) → PTY Master
4. PTY Master → PTY Slave → App SHOULD receive response
   BUT: THICC also reads from PTY Master, sees response as output
```

**This is the suspected issue**: The response goes BOTH to the app AND back to THICC as output.

### PTY Setup Details
```go
// Current setup (panel.go lines 61-95):
cmd := exec.Command(cmdArgs[0], cmdArgs[1:]...)
cmd.Env = append(os.Environ(), "TERM=xterm-256color", "THICC_TERM=1")
ptmx, _ := pty.Start(cmd)  // ptmx is PTY master
vt := vt10x.New(vt10x.WithSize(contentW, contentH), vt10x.WithWriter(ptmx))
```

**Potential issue**: `vt10x.WithWriter(ptmx)` writes responses to the same FD we read output from.

**Possible solutions to investigate**:
1. Use a different writer for DSR responses
2. Filter DSR responses from output stream
3. Configure PTY with different echo settings
4. Investigate if vt10x has a response callback instead of writer

## Environment

- **THICC version**: Fork of micro v2.0.14
- **VT emulator**: hinshun/vt10x
- **PTY library**: creack/pty
- **Rendering**: micro-editor/tcell/v2
- **Terminal**: xterm-256color (set in env)
- **Platform**: macOS (Darwin 24.4.0)

## Session Restart Instructions

**When starting new Claude Code session**:

```bash
# Start in correct directory
cd /Users/ellery/_git/thicc
claude

# In new session, tell Claude:
"I'm continuing work on THICC terminal debugging.
Please read /Users/ellery/_git/thicc/TERMINAL_DEBUG_STATUS.md
for the current status."
```

## Questions to Answer with Logs

1. **Are box-drawing characters in the raw PTY output?**
   - Search logs for `\xe2\x94` (UTF-8 box-drawing prefix)
   - If YES but not visible → rendering issue
   - If NO → Claude CLI not sending them (why?)

2. **Is DSR/CPR actually working?**
   - Search for `\x1b[6n` in PTY→VT logs (query sent)
   - Search for `\x1b[` + numbers + `R` in User→PTY logs (response received by app)
   - If response only in PTY→VT → feedback loop confirmed

3. **What's the cursor position history?**
   - Track "cursor=X,Y" in VT State logs
   - Large jumps might indicate missing positioning sequences
   - Stuck cursor might indicate sequence parsing failures

4. **Are there timing gaps?**
   - Compare timestamps (if we add them) between sequences
   - Large gaps might indicate app waiting for responses

5. **What's actually in the VT buffer?**
   - Run `DumpState()` after Claude CLI starts
   - Compare buffer contents with what should be displayed
   - Missing content → not sent or not parsed
   - Wrong position → positioning issue

## Success Criteria

✅ **Terminal is fully working when**:
1. Claude CLI renders complete UI with borders
2. No large whitespace gaps
3. All sections/headings visible
4. ASCII art displays
5. Interactive prompts work correctly
6. Colors render properly (already works)
7. DSR/CPR responses consumed by app (not visible)

---

**Current Status**: Ready for log collection and analysis
**Next Action**: Run THICC, use terminal, collect logs, share first 200 lines
**Blocked By**: Need user to run THICC in their terminal and collect logs
