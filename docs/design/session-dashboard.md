# Session Dashboard Design

Live visualization of AI tool session state via hidden command injection.

## Problem

Users running AI CLI tools (Claude Code, Aider, etc.) lack visibility into session state:
- How much context is being used?
- What files are in context?
- Am I close to the context limit?

Currently, users must manually run `/context` or `/usage` commands to see this information, interrupting their flow.

## Solution

Thicc silently executes status commands when the AI tool is idle, filters the output from the user's view, parses it, and displays the information in an always-visible dashboard panel.

## Scope

**Phase 1 (this doc):** `/context` command for Claude Code only

**Future phases:**
- `/usage` for cost/token tracking
- Multi-tool support (Aider, Cursor CLI, etc.)
- Tool-specific adaptations

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                              Thicc                                  │
├─────────────────────────────────────────────────────────────────────┤
│                                                                     │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │                    Context Dashboard                         │   │
│  │   Context: 45k/200k  ━━━━━━━░░░░ 23%   Files: 5             │   │
│  └─────────────────────────────────────────────────────────────┘   │
│  ┌─────────────────────────────────────────────────────────────┐   │
│  │                                                             │   │
│  │                     Terminal Panel                          │   │
│  │                     (Claude Code)                           │   │
│  │                                                             │   │
│  └─────────────────────────────────────────────────────────────┘   │
│                                                                     │
└─────────────────────────────────────────────────────────────────────┘
```

### Data Flow

```
Normal:
  PTY Output → VT10x → Screen

Hidden command:
  1. Idle detected (no output for 500ms)
  2. filterMode = true
  3. Send "/context\n" to PTY
  4. Output → Filter (buffer, don't render)
  5. Detect completion (prompt returns)
  6. Parse buffer → Update dashboard
  7. filterMode = false
```

---

## Dashboard Placement Options

Three options for dashboard placement:

### Option A: Horizontal Bar (Above Terminal)
```
┌─────────────────────────────────────────────────────────────────┐
│  Context: 45k/200k  ━━━━━━━░░░░ 23%  │  Files: main.go +4     │
├─────────────────────────────────────────────────────────────────┤
│                                                                 │
│                        Terminal                                 │
│                                                                 │
└─────────────────────────────────────────────────────────────────┘
```
- **Pros:** Always visible, doesn't reduce terminal width
- **Cons:** Reduces terminal height by 1-2 rows

### Option B: Horizontal Bar (Below Terminal)
```
┌─────────────────────────────────────────────────────────────────┐
│                                                                 │
│                        Terminal                                 │
│                                                                 │
├─────────────────────────────────────────────────────────────────┤
│  Context: 45k/200k  ━━━━━━━░░░░ 23%  │  Files: main.go +4     │
└─────────────────────────────────────────────────────────────────┘
```
- **Pros:** Near the input area where user focuses
- **Cons:** May feel cramped with prompt

### Option C: Thin Vertical Pane (Code Minimap Style)
```
┌───────────────────────────────────────────────────────────┬────┐
│                                                           │ 23%│
│                                                           │━━━ │
│                        Terminal                           │━━━ │
│                                                           │━━━ │
│                                                           │░░░ │
│                                                           │░░░ │
│                                                           │    │
│                                                           │ 5f │
└───────────────────────────────────────────────────────────┴────┘
```
- **Pros:** Minimal footprint, familiar IDE pattern
- **Cons:** Limited space for text, more abstract

### Recommendation

Start with **Option A (above terminal)** for clarity. Easy to expand with more info. Can add toggle to hide if user prefers.

---

## Implementation Components

### 1. Output Filter (`internal/terminal/filter.go`)

Intercepts PTY output during hidden command execution.

```go
type OutputFilter struct {
    mu           sync.Mutex
    active       bool
    buffer       bytes.Buffer
    onComplete   func(output string)
    timeout      time.Duration
    timeoutTimer *time.Timer

    // Completion detection
    promptPattern *regexp.Regexp
    silenceMs     int  // complete after N ms of no output
}

// Key methods:
func (f *OutputFilter) Start(onComplete func(string), timeout time.Duration)
func (f *OutputFilter) ProcessBytes(data []byte) (passthrough []byte)
func (f *OutputFilter) Abort() string  // return buffered content, stop filtering
```

**Completion detection strategies:**
1. Prompt pattern matching (e.g., `^>` or Claude's specific prompt)
2. Silence timeout (no output for 100ms after initial response)
3. Hard timeout (2s max)

### 2. Idle Detection (`internal/terminal/panel.go`)

Detect when AI tool is waiting for input.

```go
// Add to Panel struct:
lastOutputTime  time.Time
idleThreshold   time.Duration  // default 500ms
idleCallbacks   []func()
idleTimer       *time.Timer

func (p *Panel) onOutput() {
    p.lastOutputTime = time.Now()
    p.resetIdleTimer()
}

func (p *Panel) resetIdleTimer() {
    if p.idleTimer != nil {
        p.idleTimer.Stop()
    }
    p.idleTimer = time.AfterFunc(p.idleThreshold, func() {
        for _, cb := range p.idleCallbacks {
            cb()
        }
    })
}
```

### 3. Context Parser (`internal/terminal/context_parser.go`)

Parse `/context` output into structured data.

#### Actual `/context` Output Format

```
❯ /context
⎿   Context Usage
⛁ ⛁ ⛁ ⛁ ⛁ ⛁ ⛁ ⛁ ⛁ ⛁   claude-opus-4-5-20251101 · 112k/200k tokens (56%)
⛁ ⛁ ⛁ ⛁ ⛁ ⛁ ⛁ ⛁ ⛁ ⛁   ⛁ System prompt: 2.9k tokens (1.5%)
⛁ ⛁ ⛁ ⛁ ⛁ ⛁ ⛁ ⛁ ⛁ ⛁   ⛁ System tools: 16.0k tokens (8.0%)
⛁ ⛁ ⛁ ⛁ ⛁ ⛁ ⛁ ⛁ ⛁ ⛁   ⛁ Messages: 93.2k tokens (46.6%)
⛁ ⛁ ⛁ ⛁ ⛁ ⛁ ⛁ ⛁ ⛁ ⛁   ⛶ Free space: 43k (21.4%)
⛁ ⛁ ⛁ ⛁ ⛁ ⛀ ⛶ ⛶ ⛶ ⛶   ⛝ Autocompact buffer: 45.0k tokens (22.5%)
⛶ ⛶ ⛶ ⛶ ⛶ ⛶ ⛶ ⛶ ⛶ ⛶
⛶ ⛶ ⛶ ⛶ ⛶ ⛶ ⛶ ⛝ ⛝ ⛝
⛝ ⛝ ⛝ ⛝ ⛝ ⛝ ⛝ ⛝ ⛝ ⛝
⛝ ⛝ ⛝ ⛝ ⛝ ⛝ ⛝ ⛝ ⛝ ⛝
```

**Visual grid characters:**
- `⛁` (U+26C1) = filled/used tokens
- `⛀` (U+26C0) = partial
- `⛶` (U+26F6) = free space
- `⛝` (U+26DD) = autocompact buffer

**Parseable lines (regex patterns):**
```
Model:      /(\S+)\s*·\s*(\d+\.?\d*)k\/(\d+\.?\d*)k tokens \((\d+\.?\d*)%\)/
Prompt:     /System prompt:\s*(\d+\.?\d*)k tokens \((\d+\.?\d*)%\)/
Tools:      /System tools:\s*(\d+\.?\d*)k tokens \((\d+\.?\d*)%\)/
Messages:   /Messages:\s*(\d+\.?\d*)k tokens \((\d+\.?\d*)%\)/
Free:       /Free space:\s*(\d+\.?\d*)k \((\d+\.?\d*)%\)/
Autocompact:/Autocompact buffer:\s*(\d+\.?\d*)k tokens \((\d+\.?\d*)%\)/
```

#### Data Structure

```go
type ContextInfo struct {
    Model           string   // "claude-opus-4-5-20251101"
    UsedTokens      int      // 112000
    MaxTokens       int      // 200000
    UsedPercent     float64  // 56.0

    SystemPrompt    TokenInfo
    SystemTools     TokenInfo
    Messages        TokenInfo
    FreeSpace       TokenInfo
    AutocompactBuffer TokenInfo
}

type TokenInfo struct {
    Tokens  int
    Percent float64
}

func ParseClaudeContext(raw string) (*ContextInfo, error)
```

**Note:** Output includes ANSI color codes that must be stripped before parsing.

### 4. Dashboard Panel (`internal/layout/context_bar.go`)

Renders the context information.

```go
type ContextBar struct {
    mu       sync.Mutex
    info     *ContextInfo
    visible  bool
    style    BarStyle  // horizontal_top, horizontal_bottom, vertical
}

func (b *ContextBar) Render(screen tcell.Screen, x, y, width int)
func (b *ContextBar) Update(info *ContextInfo)
func (b *ContextBar) Height() int  // 1 for compact, 2 for expanded
```

**Compact rendering (1 row):**
```
opus-4.5 │ 112k/200k ━━━━━━░░░░ 56% │ msgs: 93k │ free: 43k
```

**Expanded rendering (2-3 rows):**
```
claude-opus-4-5 │ 112k/200k tokens (56%)  ━━━━━━━━━━━░░░░░░░░░
prompt: 2.9k │ tools: 16k │ messages: 93.2k │ free: 43k │ buffer: 45k
```

**Color coding (optional):**
- Green: < 50% usage
- Yellow: 50-75% usage
- Red: > 75% usage

### 5. Hidden Command Executor (`internal/terminal/panel.go`)

Coordinates the injection and capture.

```go
func (p *Panel) ExecuteHidden(cmd string, cb func(output string)) error {
    if p.filter.IsActive() {
        return ErrCommandInProgress
    }

    p.filter.Start(func(output string) {
        cb(output)
    }, 2*time.Second)

    _, err := p.pty.Write([]byte(cmd + "\n"))
    return err
}
```

---

## Integration Points

### Layout Manager Changes

```go
// internal/layout/manager.go

type LayoutManager struct {
    // ... existing fields
    contextBar *ContextBar
}

func (m *LayoutManager) setupContextBar() {
    m.contextBar = NewContextBar()

    // Register idle callback on terminal
    m.terminal.OnIdle(func() {
        m.refreshContext()
    })
}

func (m *LayoutManager) refreshContext() {
    m.terminal.ExecuteHidden("/context", func(output string) {
        if info, err := ParseClaudeContext(output); err == nil {
            m.contextBar.Update(info)
            m.Redraw()
        }
    })
}
```

### Terminal Panel Changes

Add to `internal/terminal/panel.go`:
- `OutputFilter` field
- `lastOutputTime` tracking
- `OnIdle()` callback registration
- `ExecuteHidden()` method
- Filter integration in output processing loop

---

## Edge Cases

| Scenario | Handling |
|----------|----------|
| User types during hidden command | Abort filter, pass all buffered + new content through |
| Tool outputs during filter | Timeout (2s), flush buffer to screen |
| Parse failure | Dashboard shows "---" or last known good value |
| Non-Claude tool running | Disable feature (detect by process name or command) |
| Multiple terminals | Each terminal has own filter/dashboard state |
| Context command not available | Detect error response, disable feature for session |

---

## Multi-Tool Support (Future)

Different tools have different status commands:

| Tool | Context Command | Notes |
|------|-----------------|-------|
| Claude Code | `/context` | Primary target |
| Aider | `/tokens` | Different format |
| Cursor CLI | TBD | May not have equivalent |

Design the parser interface to support multiple implementations:

```go
type ContextProvider interface {
    Command() string
    Parse(output string) (*ContextInfo, error)
    Detect(processName string) bool
}

var providers = []ContextProvider{
    &ClaudeContextProvider{},
    &AiderContextProvider{},
}
```

---

## Configuration

```yaml
# thicc.yaml
dashboard:
  context_bar:
    enabled: true
    position: top  # top, bottom, right
    style: compact  # compact, expanded
    refresh_idle_ms: 500
    refresh_interval_s: 60  # fallback periodic refresh
```

---

## Implementation Order

1. ~~**Capture /context output**~~ - ✅ Done (see Parser section)
2. **Build parser** - Parse the captured output into ContextInfo (strip ANSI, regex match)
3. **Build output filter** - Intercept/buffer PTY output
4. **Add idle detection** - Track output timing, fire callbacks
5. **Build context bar** - Render the dashboard UI
6. **Wire up integration** - Connect filter → parser → dashboard
7. **Handle edge cases** - Timeouts, user interrupts, errors

---

## Open Questions

- [x] Exact format of `/context` output - **RESOLVED** (see Parser section above)
- [ ] How does Claude Code's prompt look? (for completion detection)
- [ ] Should dashboard be hideable via keybind?
- [ ] Color scheme for progress bar (match terminal theme?)

---

## Success Criteria

1. Dashboard updates automatically after each Claude response
2. User never sees `/context` command or its output
3. No noticeable delay or flicker
4. Graceful degradation when feature unavailable
