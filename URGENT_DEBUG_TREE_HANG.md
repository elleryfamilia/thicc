# URGENT: Debug Tree Hang Issue

## Current Blocker
**Tree creation hangs the entire application when user types `:tree` or presses `Alt-t`**

## Last Known State
- Debug build created: Dec 27, 2024 17:43
- Extensive logging added to `toggleTree()` function
- User tried to run debug build but no log.txt was created
- Application is CURRENTLY HANGING while user waits

## Immediate Next Steps

### 1. Verify Debug Build is Running
```bash
cd /Users/ellery/_git/thock
ls -lh thock  # Should show Dec 27 17:43 or later
./thock       # This should create log.txt on startup
```

### 2. Capture Log During Hang
**User needs to:**
1. Kill current hanging instance (if still running)
2. Run: `cd /Users/ellery/_git/thock && ./thock`
3. When editor opens, press `Ctrl-e`, type `tree`, press Enter
4. In ANOTHER terminal: `cat /Users/ellery/_git/thock/log.txt | grep "THOCK:"`
5. Report the LAST line that appears (this is where it hung)

### 3. Expected Log Output
Should see lines like:
```
THOCK: toggleTree called
THOCK: Checking for existing tree pane
THOCK: Creating new tree pane
THOCK: Tree root: /path/to/dir
THOCK: Got pane node
THOCK: Creating VSplit
THOCK: VSplit created, ID: 12345
THOCK: Calling NewTreePane
[HANG OCCURS HERE - NEED TO KNOW WHICH LINE IS LAST]
```

## Likely Hang Points

### A. Hangs at "Calling NewTreePane"
**Problem:** `action.NewTreePane()` is blocking
**Location:** `/Users/ellery/_git/thock/internal/action/treepane.go:23-44`
**Likely cause:** `filemanager.NewTreePane()` blocking on tree creation

### B. Hangs at "Creating VSplit"
**Problem:** `node.VSplit(false)` is blocking
**Location:** `/Users/ellery/_git/thock/cmd/thock/micro.go:719`
**Likely cause:** Node tree manipulation deadlock

### C. Hangs at "Resizing tab"
**Problem:** `tab.Resize()` is blocking
**Location:** `/Users/ellery/_git/thock/cmd/thock/micro.go:741`
**Likely cause:** Display recalculation deadlock

## Files to Investigate Based on Hang Point

### If hangs in NewTreePane:
- `/Users/ellery/_git/thock/internal/filemanager/pane.go:25-51` - NewTreePane()
- `/Users/ellery/_git/thock/internal/filemanager/tree.go:66-100` - Tree.Refresh()
- `/Users/ellery/_git/thock/internal/filemanager/pane.go:54-78` - RenderTree()

### If hangs in VSplit:
- Check micro's views package for Node.VSplit implementation
- May need to avoid VSplit and use simpler approach

### If hangs in Resize:
- Check tab.Resize() implementation
- May need to defer resize or skip it

## Quick Fixes to Try

### Fix 1: Create Tree in Goroutine
```go
// In toggleTree() before line 724:
done := make(chan *action.TreePane, 1)
go func() {
    pane := action.NewTreePane(0, 0, 30, 100, root, treeSplitID, tab)
    done <- pane
}()

select {
case treePane := <-done:
    // Continue with tree pane
case <-time.After(2 * time.Second):
    action.InfoBar.Error("Tree creation timed out")
    return
}
```

### Fix 2: Simplify to Basic Buffer
Replace TreePane with simple file list:
```go
files, _ := os.ReadDir(root)
var content strings.Builder
for _, f := range files {
    content.WriteString(f.Name() + "\n")
}
buf := buffer.NewBufferFromString(content.String(), "filetree", buffer.BTInfo)
// Use buf in vsplit instead of TreePane
```

### Fix 3: Skip VSplit, Use Existing Pane
Don't create new pane, just replace current buffer with tree:
```go
bp.Buf = treeBuffer  // Replace editor buffer with tree
```

## Code Locations Reference

### Main Toggle Function
`/Users/ellery/_git/thock/cmd/thock/micro.go:674-747`

### TreePane Wrapper
`/Users/ellery/_git/thock/internal/action/treepane.go:22-44`

### Filemanager TreePane
`/Users/ellery/_git/thock/internal/filemanager/pane.go:25-51`

### Tree Implementation
`/Users/ellery/_git/thock/internal/filemanager/tree.go`

## Build Commands

```bash
cd /Users/ellery/_git/thock

# Debug build (with logging)
make build-dbg

# Quick build (no logging)
make quick

# Clean build
make clean && make build
```

## Run Commands

```bash
cd /Users/ellery/_git/thock

# Run with launcher script
./run-thock.sh

# Run directly
./thock

# Check if it's debug build (should create log.txt)
ls -la log.txt  # Should appear after launch
```

## What We Need from User

1. **Exact last log line before hang:**
   ```bash
   cat /Users/ellery/_git/thock/log.txt | grep "THOCK:" | tail -1
   ```

2. **Confirmation debug build is running:**
   - Does log.txt get created when launching thock?
   - What's the timestamp on the thock binary?

3. **Can user try simple workaround:**
   - Instead of `:tree`, try `:vsplit` to see if splits work at all
   - This tells us if it's VSplit or TreePane causing the hang

---

**Once we have the log output showing where it hangs, we can implement a targeted fix!**
