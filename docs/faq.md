# FAQ

Common questions and troubleshooting for thicc.

## Display Issues

### Icons aren't showing (or look like boxes/question marks)

You need a **Nerd Font** installed and configured in your terminal.

**macOS:**
```sh
brew install --cask font-jetbrains-mono-nerd-font
```

Then set "JetBrainsMono Nerd Font" as your terminal font in iTerm2/Terminal preferences.

**Linux:**
```sh
mkdir -p ~/.local/share/fonts
cd ~/.local/share/fonts
curl -fLO https://github.com/ryanoasis/nerd-fonts/releases/latest/download/JetBrainsMono.zip
unzip JetBrainsMono.zip && fc-cache -fv
```

Then configure your terminal emulator to use the font.

### Colors look wrong or washed out

thicc requires a terminal with **true color (24-bit) support**. Recommended terminals:

- **macOS**: iTerm2, Kitty, Alacritty, WezTerm
- **Linux**: Kitty, Alacritty, WezTerm, GNOME Terminal (newer versions)
- **Windows**: Windows Terminal, WezTerm

If you're using tmux, make sure true color is enabled:
```sh
# Add to ~/.tmux.conf
set -g default-terminal "tmux-256color"
set -ag terminal-overrides ",xterm-256color:RGB"
```

### Text is too small/large

thicc uses your terminal's font settings. Adjust the font size in your terminal emulator preferences.

## Navigation

### How do I exit the terminal pane?

Press `Ctrl+Space` to cycle focus to another panel. The terminal stays running in the background.

To actually quit thicc, press `Ctrl+Q`.

### How do I get back to the editor from the terminal?

Press `Ctrl+Space` to cycle panels, or press `Ctrl+Space` repeatedly until the editor is focused.

### I pressed a key and nothing happened

Make sure the correct panel has focus. Look for the highlighted border around the active panel. Press `Ctrl+Space` to cycle focus.

## Customization

### Can I customize the colors?

No. thicc is opinionated—one colorscheme, one layout. This is by design: less time configuring, more time shipping.

### Can I change the keybindings?

Yes, but for editor keybindings only (inherited from micro). Create or edit:
```
~/.config/thicc/bindings.json
```

See the [keybindings documentation](keybindings.md) for format.

### Where are config files stored?

Configuration lives in:
```
~/.config/thicc/
```

This includes:
- `settings.json` — Editor settings
- `bindings.json` — Custom keybindings
- `recent.json` — Recent projects list
- `colorschemes/` — Custom colorschemes (not recommended)
- `plug/` — Installed plugins

### What settings can I change?

Common settings you might want to adjust (edit `~/.config/thicc/settings.json`):

```json
{
  "tabsize": 4,
  "tabstospaces": true,
  "autoindent": true,
  "ruler": true,
  "cursorline": true,
  "hlsearch": true,
  "ignorecase": true,
  "syntax": true,
  "mouse": true,
  "scrollspeed": 2
}
```

Run `thicc -options` to see all available settings with descriptions.

### Can I have different settings for different file types?

Yes! Prefix settings with `ft:` followed by the filetype:

```json
{
  "ft:python": {
    "tabsize": 4,
    "tabstospaces": true
  },
  "ft:go": {
    "tabsize": 8,
    "tabstospaces": false
  }
}
```

## Terminal

### The terminal prompt looks wrong

thicc sets up a custom Powerline prompt automatically. If it looks wrong:

1. Make sure you have a Nerd Font installed
2. Check that your shell is zsh or bash (the custom prompt supports these)

### Commands aren't found in the terminal

The terminal inherits your shell's environment. Make sure commands are in your `PATH` and that your shell profile (`.zshrc`, `.bashrc`) is properly configured.

### AI tool says "API key not found"

Most AI tools require an API key. Check the tool's documentation:

- **Claude Code**: Set `ANTHROPIC_API_KEY`
- **GitHub Copilot**: Run `copilot auth login`
- **Aider**: Set `OPENAI_API_KEY` or configure for other providers

Add environment variables to your shell profile to persist them.

## Performance

### thicc is slow to start

The file browser scans your project directory on startup. For very large projects, this can take a moment. thicc uses background scanning, so you should be able to start working while it loads.

### The file browser is laggy

Large directories with many files can slow down rendering. thicc automatically skips common heavy directories like:
- `node_modules`
- `.git`
- `vendor`
- `__pycache__`

If you have other large directories, they may still be scanned.

## Updates

### How do I update thicc?

```sh
thicc -update
```

This checks for and installs the latest version.

### How do I check my version?

```sh
thicc -version
```

### How do I switch to nightly builds?

```sh
curl -fsSL https://raw.githubusercontent.com/elleryfamilia/thicc/main/install.sh | CHANNEL=nightly sh
```

Then inside thicc, run:
```
:set updatechannel nightly
```

### How do I uninstall?

```sh
thicc -uninstall
```

Or manually delete the binary from `~/.local/bin/thicc` or `/usr/local/bin/thicc`.

## Troubleshooting

### thicc crashed

If thicc crashes, check the log file:
```
~/.config/thicc/log.txt
```

Please report crashes at: https://github.com/elleryfamilia/thicc/issues

### Something isn't working

1. Check you're running the latest version (`thicc -update`)
2. Check the log file for errors
3. Try running `thicc -clean` to reset configuration
4. Run with debug mode: `thicc -debug myproject/` and check `./log.txt`
5. Open an issue with:
   - Your OS and terminal
   - Steps to reproduce
   - Any error messages

## Still stuck?

Open an issue: https://github.com/elleryfamilia/thicc/issues

---

[Back to Documentation Home](index.md)
