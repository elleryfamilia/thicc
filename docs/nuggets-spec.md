# Agent Nuggets Specification

An open standard for capturing **nuggets** (valuable insights, decisions, patterns) from AI coding sessions.

## Overview

Nuggets are valuable insights extracted from AI-assisted coding sessions. Instead of storing full conversation logs, only the distilled knowledge is persisted - the signal without the noise.

```
AI Session → Chunked Extraction → Draft Nuggets → UI Review → Persisted Nuggets
     ↓              ↓                  ↓            ↓            ↓
 (streaming)   (every N tokens)   (pending)    (curate)     (JSON)
```

## Storage Layout

```
project-root/
  .agent-nuggets.json              # Persisted nuggets (committed to git)
  .agent-history/               # Local/ephemeral (gitignored)
    sessions/                   # Raw session chunks
    pending-nuggets.json           # Nuggets awaiting review
    extraction-state.json       # Tracks extraction progress
```

Only `.agent-nuggets.json` is committed. Everything else is local and ephemeral.

## What Is a Nugget?

A nugget is a **valuable insight worth documenting** for future reference by humans or AI.

### Nugget Types

| Type | Description | Example |
|------|-------------|---------|
| `decision` | Architectural or design choice with lasting impact | "Chose PostgreSQL over MongoDB for ACID compliance" |
| `discovery` | Unexpected finding during development | "React 18 concurrent mode breaks our custom hooks" |
| `gotcha` | Non-obvious pitfall or edge case | "Must call `cleanup()` before unmount or memory leaks" |
| `pattern` | Reusable solution or approach | "Use middleware chain pattern for request validation" |
| `issue` | Bug or problem encountered and how it was resolved | "CORS errors when API on different subdomain - fixed with proxy" |
| `context` | Important background info for understanding code | "Legacy auth system requires JWT in header AND cookie" |

### What Is NOT a Nugget

- Routine code changes (typo fixes, formatting)
- Simple syntax questions ("how do I do X in Go?")
- Standard implementations with no novel insight
- Temporary debugging steps
- Obvious patterns documented elsewhere

### Extraction Criteria

The summarizer should ask: **"Would a future developer (or AI) benefit from knowing this?"**

**Extract if:**
- Decision has trade-offs that aren't obvious from code
- Behavior is surprising or counterintuitive
- Solution took significant debugging to find
- Pattern could be reused elsewhere
- Context would otherwise be lost

**Skip if:**
- Information is obvious from reading the code
- It's a one-off fix with no broader relevance
- Standard library/framework usage

## JSON Format

### File Structure (`.agent-nuggets.json`)

```json
{
  "version": 1,
  "nuggets": [...]
}
```

### Nugget Object

```json
{
  "id": "nugget-abc123",
  "type": "decision",
  "title": "JWT Authentication Design",
  "summary": "Chose JWT over session cookies for stateless auth",
  "significance": "This decision affects scalability and security - stateless auth enables horizontal scaling without shared session storage, but requires careful token management",
  "created": "2024-01-11T10:30:00Z",
  "commit": "a1b2c3d",
  "client": "claude-code",
  "model": "claude-opus-4",
  "tags": ["architecture", "auth", "go"],
  "files": ["src/auth/middleware.go", "src/auth/jwt.go"],
  "content": {
    "rationale": [
      "Stateless: no server-side session storage needed",
      "Scales horizontally without shared state",
      "Standard format, works with external services"
    ],
    "implementation": [
      "Tokens expire after 1 hour",
      "Refresh tokens stored in httpOnly cookies"
    ],
    "gotchas": [
      "Remember to check exp claim before trusting token",
      "Use RS256 not HS256 for production (asymmetric keys)"
    ]
  },
  "user_notes": "Also consider key rotation strategy"
}
```

### Field Reference

**Core Fields:**
| Field | Type | Description |
|-------|------|-------------|
| `id` | string | Unique identifier (auto-generated) |
| `type` | enum | One of: `decision`, `discovery`, `gotcha`, `pattern`, `issue`, `context` |
| `title` | string | Short descriptive title (< 60 chars) |
| `summary` | string | One-line summary of the insight |
| `significance` | string | Why this nugget is worth keeping - what makes it non-obvious or valuable |
| `created` | ISO 8601 | Timestamp when nugget was created |
| `commit` | string? | Git commit SHA (null if uncommitted) |
| `client` | string | AI tool (claude-code, aider, cursor, etc.) |
| `model` | string | LLM model used |
| `tags` | string[] | Categorization tags |
| `files` | string[] | Related files |

**Content Fields (vary by type):**
| Field | Used By | Description |
|-------|---------|-------------|
| `rationale` | decision, pattern | Why this approach was chosen |
| `implementation` | decision, pattern | How it was implemented |
| `gotchas` | all types | Pitfalls to avoid |
| `cause` | issue | Root cause of the problem |
| `resolution` | issue | How it was fixed |
| `pattern` | pattern | Reusable code/approach |

**User Fields:**
| Field | Type | Description |
|-------|------|-------------|
| `user_notes` | string? | Manual additions from review |

## Extraction Workflow

### Continuous Extraction

1. AI session running, text accumulating
2. Every N tokens (e.g., 2000), trigger extraction:
   - Send chunk + previous nuggets to summarizer LLM
   - LLM returns new nuggets found (if any)
   - LLM indicates if conversation seems incomplete
3. New nuggets added to `pending-nuggets.json`
4. UI shows pending count for review

### Incomplete Conversation Handling

If extraction returns "incomplete":
- Mark chunk boundary
- On next extraction, include previous chunk for context
- Re-extract with fuller picture
- Merge/dedupe with previously found nuggets

### Review Flow

1. User sees nuggets indicator in status bar
2. Opens nugget review panel
3. For each pending nugget:
   - View extracted content
   - Edit/refine
   - Add user notes
   - Accept → moves to `.agent-nuggets.json`
   - Reject → discarded
   - Skip → stays pending
4. Accepted nuggets auto-link to current/next commit

## CLI Commands

```bash
thicc nugget list                 # List all committed nuggets
thicc nugget pending              # Show pending nuggets awaiting review
thicc nugget show <id>            # Show nugget details
thicc nugget accept <id>          # Accept a pending nugget
thicc nugget reject <id>          # Reject a pending nugget
thicc nugget add                  # Manually add a nugget (interactive)
thicc nugget search <query>       # Search nuggets by text
thicc nugget extract [options]    # Extract nuggets from session file
```

### Extract Options

```bash
thicc nugget extract --file session.txt                     # Ollama (local)
thicc nugget extract --file session.txt --provider groq     # Groq (fast)
thicc nugget extract --file session.txt --provider together # Together AI
thicc nugget extract --file session.txt --summarizer openai \
  --host "https://api.deepinfra.com/v1/openai" \
  --model "deepseek-ai/DeepSeek-V3" --api-key "$KEY"        # DeepInfra
```

| Option | Description |
|--------|-------------|
| `--file <path>` | Session file to extract from (required) |
| `--summarizer` | Summarizer type: `ollama`, `anthropic`, `openai` |
| `--provider` | Provider preset: `openai`, `groq`, `together`, `openrouter` |
| `--model` | Model name (default varies by provider) |
| `--host` | API endpoint URL (for custom deployments) |
| `--api-key` | API key (or set via environment variable) |

## Use Cases

1. **Onboarding**: New developer reads nuggets to understand architectural decisions
2. **Code review**: "Why did you do it this way?" → link to nugget
3. **Future self**: "Why did I choose JWT?" → nugget explains rationale
4. **Team knowledge**: Nuggets become searchable project documentation
5. **AI context**: Future AI sessions can read nuggets for project-specific patterns

## Summarizer Configuration

The summarizer LLM is configurable. Any OpenAI-compatible API works.

**Local:**
- Ollama (default)
- llama.cpp
- LocalAI

**Cloud Providers:**
- Anthropic (Claude)
- OpenAI (GPT-4o)
- Groq (fast inference)
- Together AI
- DeepInfra
- OpenRouter

**Environment Variables:**
```bash
ANTHROPIC_API_KEY=...   # For Anthropic
OPENAI_API_KEY=...      # For OpenAI and compatible APIs
GROQ_API_KEY=...        # For Groq
```

Configuration stored in thicc settings:
```json
{
  "nuggets.summarizer": "ollama",
  "nuggets.model": "llama3.2",
  "nuggets.extractionThreshold": 2000
}
```

## Compatibility

This spec is designed to be:
- **Tool-agnostic**: Works with any AI coding assistant
- **Editor-agnostic**: Can be implemented by any IDE/editor
- **LLM-agnostic**: Summarizer can be any capable model
- **Git-friendly**: JSON format, one file, easy to merge
