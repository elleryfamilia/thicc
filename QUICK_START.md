# Quick Start - Continue Terminal Debugging

## Resume Session

```bash
cd /Users/ellery/_git/thicc
claude  # Start new session in correct directory
```

Then tell Claude:
> "Read TERMINAL_DEBUG_STATUS.md - I'm continuing the terminal debugging work"

## Collect Debug Logs (Next Step)

```bash
cd /Users/ellery/_git/thicc

# Clear old logs
rm -f /tmp/thicc-terminal-debug.log

# Run THICC (uses correct config)
./run-thicc.sh

# In THICC's terminal pane (right side):
# 1. Run test: /tmp/test-terminal.sh
# 2. Run Claude CLI: claude
# 3. Let it start, observe rendering issues
# 4. Exit THICC (Ctrl+Q)

# Get logs (first 200 lines)
head -200 /tmp/thicc-terminal-debug.log > /tmp/thicc-debug-snippet.txt

# Share with Claude:
cat /tmp/thicc-debug-snippet.txt
```

## Key Files

- **Status doc**: `TERMINAL_DEBUG_STATUS.md` - Full debugging history
- **Code changes**: `internal/terminal/panel.go` - DSR/CPR fix + logging
- **Test script**: `/tmp/test-terminal.sh` - Terminal capability tests
- **Logs**: `/tmp/thicc-terminal-debug.log` - Debug output
- **Binary**: `./thicc` - Built with all fixes

## Current State

✅ **Completed**:
- DSR/CPR fix applied (vt10x.WithWriter)
- Debug logging added
- Test script created
- Test results: colors, box-drawing, Unicode all WORK

⚠️ **Issue Found**:
- DSR/CPR response appears as visible output (^[[54;19R)
- Response should go to app stdin, not terminal output
- This might cause Claude CLI timeouts/wrong layout

❌ **Still Broken**:
- Claude CLI missing borders, headings, content
- Large gaps in output

🎯 **Next**: Collect logs to see exact escape sequences and responses
