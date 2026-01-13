# LLM History

Thicc automatically records AI coding sessions (Claude, Aider, Cursor, etc.) and provides searchable history via MCP.

## Features

- **Automatic recording** - Captures terminal output from AI tools
- **Full-text search** - SQLite FTS5 for fast queries
- **MCP integration** - Query history from Claude Code or other MCP clients
- **Size management** - Configurable limits with auto-cleanup
- **Tool use tracking** - Records tool invocations via Claude CLI hooks

## How Recording Works

Thicc captures AI session output via the terminal's **scrollback buffer**:

1. AI tool output flows through the PTY to the VT10x terminal emulator
2. When lines scroll off the top of the visible terminal, they're pushed to the scrollback buffer
3. These clean, finalized lines are then saved to the history database
4. When the session ends, any remaining visible content is also captured

### What Gets Captured
- All text that appears in the terminal and scrolls off-screen
- Final screen content when the AI tool exits
- Tool use metadata (via Claude CLI hooks)

### What Doesn't Get Captured
- Spinner animations and progress bars (overwritten by `\r`)
- Alternate screen buffer content (vim, less, etc.)
- Content that's cleared before scrolling (e.g., `clear` command)

### Debugging Recording Issues

Enable debug logging by rebuilding with:
```bash
go build -ldflags "-X github.com/ellery/thicc/internal/util.Debug=ON" ./cmd/thicc
```

Then check `log.txt` for `LLMHISTORY:` prefixed messages.

## Quick Start

### 1. Enable MCP Server

Add to Claude Code (global):
```bash
claude mcp add -s user llm-history thicc mcp serve
```

Or add to `~/.claude/mcp.json`:
```json
{
  "mcpServers": {
    "llm-history": {
      "command": "thicc",
      "args": ["mcp", "serve"]
    }
  }
}
```

### 2. (Optional) Enable Tool Use Hooks

To capture structured tool use data from Claude CLI, add to `~/.claude/settings.json`:
```json
{
  "hooks": {
    "PostToolUse": [
      {
        "matcher": "",
        "hooks": [{"type": "command", "command": "thicc mcp hook post-tool-use"}]
      }
    ]
  }
}
```

## CLI Commands

```bash
thicc mcp sessions [limit]   # List recent sessions (default: 20)
thicc mcp session <id>       # Show session details and tool uses
thicc mcp output <id>        # Show raw session output text
thicc mcp search <query>     # Search history for a term
thicc mcp stats              # Show database statistics
thicc mcp compact            # Vacuum database to reclaim space
thicc mcp cleanup [days]     # Delete sessions older than N days (default: 90)
thicc mcp limit [size_mb]    # Enforce size limit (default: 100MB)
thicc mcp rebuild-fts        # Rebuild full-text search index
```

Session IDs support prefix matching (minimum 8 characters), so `thicc mcp session abc12345` works.

## Configuration

Settings in `~/.config/thicc/settings.json`:

```json
{
  "llmhistory.enabled": true,      // Master switch - disable all AI session logging
  "llmhistory.maxsizemb": 100,     // Max database size in MB (0 = unlimited)
  "llmhistory.mcpenabled": true    // Enable/disable MCP server
}
```

## MCP Tools

When connected via MCP, the following tools are available:

| Tool | Description |
|------|-------------|
| `search_history` | Full-text search across all sessions |
| `list_sessions` | List recent AI sessions |
| `get_session` | Get details for a specific session |
| `get_file_history` | Get history of tool uses for a file |

All tools accept an optional `project` parameter to filter by project directory.

## Database

Location: `~/.config/thicc/thicc/llm_history.db`

SQLite with:
- WAL mode for concurrent reads
- FTS5 full-text search index
- Automatic size enforcement

### Size Management

The database automatically enforces size limits:
- Default limit: 100MB
- Cleanup triggers at 90% of limit
- Cleans down to 70% of limit
- Oldest sessions deleted first

### Troubleshooting

**Search returns no results but data exists:**
Run `thicc mcp rebuild-fts` to rebuild the full-text search index. This can happen if data was imported or if the FTS triggers got out of sync.

---

## Future: Project-Scoped Storage

> **Status**: Design phase - not yet implemented

A planned enhancement to store history per-project in a `.agent-history/` directory at the git root.

### Motivation

- History scoped to project by default
- Shareable via git (team can see AI activity)
- Portable (clone repo, get history)
- Per-tool isolation (separate files for Claude, Aider, etc.)

### Proposed Structure

```
project/
  .agent-history/
    sessions/
      abc123.jsonl    # One file per session
      def456.jsonl
    .index.db         # Local SQLite index (gitignored)
    .gitignore        # Contains: .index.db
```

### Session File Format

```jsonl
{"type":"session_start","id":"abc123","tool":"claude","timestamp":"2024-01-11T10:00:00Z"}
{"type":"output","line":"User: How do I fix this bug?"}
{"type":"tool_use","tool":"Read","input":{"file_path":"src/main.go"}}
{"type":"session_end","timestamp":"2024-01-11T10:30:00Z"}
```

### Open Questions

1. **Storage format**: Per-session JSONL files vs single append-only file
2. **Migration**: How to migrate from global DB to per-project storage
3. **Fallback**: What to do for non-git directories
