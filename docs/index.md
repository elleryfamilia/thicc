# thicc Documentation

Welcome to the thicc documentation. thicc is a terminal IDE built for developers who pair with AI but still read the diff.

![thicc main view](../assets/thicc_main.png)

## What is thicc?

File browser. Editor. Terminal. AI tools. One vibe.

thicc combines three essential tools into a single, unified interface:
- A **file browser** on the left for navigating your project
- An **editor** in the middle for writing code
- A **terminal** on the right for running commands (and AI assistants)

We made the decisions so you don't have to. One colorscheme. One layout. Zero config paralysis.

## Philosophy

thicc is opinionated by design. While most editors let you configure everything, we ship with sensible defaults and get out of your way. The goal is less time tweaking, more time shipping.

We're built for the AI-assisted workflow—terminal pane ready for Claude CLI, Copilot CLI, Aider, or whatever you're into. But we put *you* in the driver's seat, not the model.

## Before You Start

You'll need:

1. **A Nerd Font** (for the icons)
   - macOS: `brew install --cask font-jetbrains-mono-nerd-font`
   - Then set "JetBrainsMono Nerd Font" as your terminal font

2. **A terminal with true color support**
   - iTerm2, Kitty, Alacritty, WezTerm, or similar

## Documentation

| Topic | Description |
|-------|-------------|
| [Getting Started](getting-started.md) | Your first 5 minutes with thicc |
| [The Layout](layout.md) | Understanding the 3-panel layout |
| [File Browser](file-browser.md) | Navigating and managing files |
| [Terminal](terminal.md) | Terminal panes and shell integration |
| [AI Workflow](ai-workflow.md) | Using thicc with Claude, Copilot, and more |
| [LLM History](llm-history.md) | AI session recording and MCP search |
| [Gems Spec](gems-spec.md) | Open standard for AI coding insights |
| [Keybindings](keybindings.md) | Complete keyboard shortcut reference |
| [Dashboard](dashboard.md) | Project picker and onboarding |
| [Command Line](command-line.md) | CLI flags and configuration |
| [FAQ](faq.md) | Common questions and troubleshooting |

## Quick Reference

| Action | Shortcut |
|--------|----------|
| Navigate between panes | `Ctrl+Space` |
| Toggle file browser | `Alt+1` |
| Toggle editor | `Alt+2` |
| Toggle terminal | `Alt+3` |
| Save file | `Ctrl+S` |
| Quit | `Ctrl+Q` |

Ready to dive in? Start with [Getting Started](getting-started.md).
