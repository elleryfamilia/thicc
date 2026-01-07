# AI Workflow

thicc is built for developers who pair with AI. This guide explains how to get the most out of AI-assisted development.

## The Philosophy

> "You in the driver's seat, not the model."

AI coding assistants are powerful, but they work best when you stay engaged. thicc's 3-panel layout lets you:

1. **See the code** — The editor shows what the AI is working on
2. **Guide the AI** — The terminal is where you prompt and interact
3. **Review the diff** — You can read changes as they happen

This is different from IDEs that hide AI output in sidebar panels. In thicc, the AI's work is front and center, right next to your code.

## Supported AI Tools

thicc works with any CLI-based AI tool. The tool selector shows these by default:

| Tool | Command | Description |
|------|---------|-------------|
| Claude Code | `claude` | Anthropic's AI coding assistant |
| Gemini CLI | `gemini` | Google's Gemini CLI |
| Codex CLI | `codex` | OpenAI's coding assistant |
| GitHub Copilot | `copilot` | Copilot CLI |
| Aider | `aider` | AI pair programming |
| Ollama | `ollama` | Run LLMs locally |

Not installed? The tool selector shows install commands for popular tools.

## Setting Up Claude Code

Claude Code is what thicc was originally built around. Here's how to get started:

### 1. Install Claude Code

```sh
npm install -g @anthropic-ai/claude-code
```

Or use the install script from the tool selector.

### 2. Configure API Key

```sh
export ANTHROPIC_API_KEY="your-key-here"
```

Add this to your shell profile (`.zshrc`, `.bashrc`) to persist it.

### 3. Launch in thicc

1. Open thicc
2. Press `Ctrl+Space` to focus the terminal
3. The tool selector appears—choose "Claude Code"
4. Start prompting!

## The Workflow

Here's a typical AI-assisted development session:

### 1. Open Your Project

```sh
thicc ~/my-project
```

The file browser shows your project structure. The editor is ready.

### 2. Focus the Terminal

Press `Ctrl+Space` until the terminal pane is focused. Select Claude Code (or your preferred AI tool) from the tool selector.

### 3. Give the AI Context

AI tools work better with context. Claude Code can read your project files, but you might want to:
- Open relevant files in the editor first
- Point out specific files in your prompt
- Explain your project structure

### 4. Prompt and Review

Ask the AI to make changes. Watch the diff appear in your editor. The 3-panel layout means you can:
- See the code being changed (editor)
- Read the AI's reasoning (terminal)
- Navigate to related files (file browser)

### 5. Stay in Control

Review every change before committing. The AI is your pair programmer, not your autopilot.

## Multi-Terminal Workflows

thicc supports up to three terminals. Use this for:

### Dedicated AI Terminal

- **Terminal 1** (`Alt+3`): Your AI assistant
- **Terminal 2** (`Alt+4`): Build/test commands
- **Terminal 3** (`Alt+5`): Git operations

### Multiple AI Assistants

Want to compare outputs?

- **Terminal 1**: Claude Code
- **Terminal 2**: Aider or Copilot

Each terminal runs independently.

## Tips for Effective AI Pairing

### Be Specific

Instead of:
> "Fix the bug"

Try:
> "The `processOrder` function in `src/orders.js` is throwing a null reference error when `customer.address` is undefined. Add null checking."

### Work Incrementally

Don't ask for huge changes at once. Small, focused prompts lead to better results.

### Review the Diff

The whole point of thicc's layout is to keep you engaged. Read the changes. Understand them. Push back if something looks wrong.

### Use the File Browser

Before prompting, navigate to the relevant files. This helps you understand the context and give better prompts.

### Trust but Verify

AI makes mistakes. Run tests. Check edge cases. The AI is a collaborator, not an infallible oracle.

## Keyboard Shortcuts Summary

| Shortcut | Action |
|----------|--------|
| `Ctrl+Space` | Cycle between panels |
| `Alt+3` | Toggle terminal (where AI runs) |
| `Alt+4` | Toggle second terminal |
| `Alt+5` | Toggle third terminal |
| `Tab` | Jump from file browser to editor |

## Common Issues

### AI tool not found

Make sure the tool is installed and in your `PATH`. Run `which claude` (or your tool name) to verify.

### API key errors

Most AI tools need an API key. Check their documentation for setup instructions.

### Terminal too narrow

If the AI's output is hard to read, try hiding the file browser (`Alt+1`) to give the terminal more space.

---

Next: [Dashboard](dashboard.md) — Learn about the project picker and onboarding
