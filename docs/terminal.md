# Terminal

The terminal pane on the right side of thicc is where you run commands, build your project, and interact with AI assistants.

<!-- TODO: screenshot of terminal pane with Claude CLI running -->

## Overview

The terminal is a full PTY (pseudo-terminal) that supports:
- Colors and ANSI escape sequences
- Interactive programs (vim, htop, etc.)
- Shell completions and history
- Unicode and special characters

It's not just a command output viewer—it's a real terminal.

## Multiple Terminals

thicc supports up to three independent terminal panes:

| Terminal | Toggle | Default |
|----------|--------|---------|
| Terminal 1 | `Alt+3` | Visible |
| Terminal 2 | `Alt+4` | Hidden |
| Terminal 3 | `Alt+5` | Hidden |

Each terminal runs its own shell session. This is useful for:
- Running your dev server in one terminal
- Using an AI assistant in another
- Keeping a third for ad-hoc commands

## The Powerline Prompt

When you open a terminal in thicc, you'll see a custom Powerline-style prompt with a Spider-Verse inspired color scheme:

```
 ~/project  main   2s ❯
```

The prompt shows:
- **Current directory** (magenta background)
- **Git branch** (cyan if clean, orange if dirty)
- **Command timing** (gold, shows elapsed time for long-running commands)

This prompt is automatically set up—no configuration needed.

## Tool Selector

<!-- TODO: screenshot of tool selector modal -->

When you first open a terminal pane, thicc shows a **tool selector** letting you choose what to run:

- **Default Shell** — Your normal shell (zsh, bash, etc.)
- **Claude Code** — Anthropic's AI coding assistant
- **Gemini CLI** — Google's Gemini CLI
- **GitHub Copilot** — Copilot CLI
- **Aider** — AI pair programming
- **And more...**

The selector shows which tools are installed (checkmark) and which aren't (with install instructions).

## Keybindings

When the terminal has focus, most keys go directly to the shell. But some thicc shortcuts still work:

| Shortcut | Action |
|----------|--------|
| `Ctrl+Space` | Cycle to next panel |
| `Ctrl+Q` (twice) | Exit thicc |
| `Ctrl+E` (twice) | Enter command mode |
| `Ctrl+W` (twice) | Next split |

**Why twice?** Single presses of these keys are sent to the terminal (they might be used by programs running in the shell). Double-press tells thicc you want the action.

### Quick Command Mode

Press `Ctrl+\` in the terminal to enter quick command mode. A banner appears with options:

| Key | Action |
|-----|--------|
| `q` | Quit thicc |
| `w` | Switch to next pane |
| `p` | Enter passthrough mode |
| `Escape` | Cancel |

### Passthrough Mode

If you need to send *all* keys to the terminal (including `Ctrl+Space`), enter passthrough mode:

1. Press `Ctrl+\` to enter quick command mode
2. Press `p` to enable passthrough
3. The terminal now receives all keystrokes

This is useful for terminal programs that use thicc's shortcuts (like vim or tmux).

## Scrolling

Navigate through terminal history:

| Shortcut | Effect |
|----------|--------|
| `Shift+PageUp` | Scroll up through history |
| `Shift+PageDown` | Scroll down through history |
| Mouse wheel | Scroll (3 lines per tick) |

**Note**: Any keypress while scrolled up automatically snaps back to the live terminal output.

## Auto-Hide on Exit

When a program exits in the terminal (like when you quit an AI assistant), the terminal pane can automatically hide. This returns focus to the editor, giving you more screen space.

## Shell Detection

thicc automatically detects your default shell from the `SHELL` environment variable. It supports:
- zsh (default on macOS)
- bash
- fish
- sh

The custom prompt is configured for zsh and bash. Other shells will use their default prompts.

## Multiplexer Detection

If you're running thicc inside tmux, Zellij, or screen, thicc automatically detects this. This helps avoid conflicts between thicc's terminal and the outer multiplexer.

## Tips

1. **Use multiple terminals**: Keep one for builds, one for AI—switching is just `Ctrl+Space`
2. **Hide when not needed**: `Alt+3` toggles the terminal to give you more editor space
3. **Tool selector**: You don't have to use the default shell—launch directly into Claude or another AI tool
4. **Watch the prompt**: The git status color tells you if you have uncommitted changes

---

Next: [AI Workflow](ai-workflow.md) — Get the most out of AI-assisted development
