# Command Line Usage

thicc can be launched with various arguments and flags.

## Basic Usage

```sh
thicc              # Open the dashboard
thicc .            # Open current directory
thicc <file>       # Open a specific file
thicc <directory>  # Open a directory
```

## Command Line Flags

| Flag | Description |
|------|-------------|
| `-version` | Show version, commit hash, and compile date |
| `-config-dir <path>` | Use a custom configuration directory (must exist) |
| `-options` | Show all available configuration options |
| `-debug` | Enable debug mode (outputs to `./log.txt`) |
| `-profile` | Enable CPU profiling (outputs to `./thicc.prof`) |
| `-plugin <cmd>` | Execute a plugin command |
| `-clean` | Clean configuration directory and exit |
| `-update` | Check for updates and install if available |
| `-uninstall` | Uninstall thicc from system |
| `-report-bug` | Report a bug or issue |

## Override Settings

You can override any configuration setting from the command line:

```sh
thicc -tabsize 2 myfile.py       # Open with tab size 2
thicc -colorscheme monokai .     # Open with different colorscheme
thicc -ruler false myfile.txt    # Open without line numbers
```

## Examples

### Check Version

```sh
thicc -version
```

Output:
```
Version: 1.0.0
Commit hash: abc1234
Compiled: 2024-01-15
```

### Update thicc

```sh
thicc -update
```

This checks for a new version and installs it if available.

### Debug Mode

```sh
thicc -debug myproject/
```

This runs thicc with debug logging enabled. Logs are written to `./log.txt` in the current directory. Useful for troubleshooting issues.

### Use Custom Config Directory

```sh
thicc -config-dir ~/.config/thicc-work myproject/
```

The directory must already exist. Useful for maintaining separate configurations.

### Show All Options

```sh
thicc -options
```

Lists all available configuration options with their current values and descriptions.

### Uninstall

```sh
thicc -uninstall
```

Removes thicc from your system. If installed with `GLOBAL=1`, you may need:

```sh
sudo thicc -uninstall
```

## Configuration Directory

thicc looks for configuration in these locations (in order):

1. `$THICC_CONFIG_HOME` (if set)
2. `$MICRO_CONFIG_HOME` (for compatibility)
3. `$XDG_CONFIG_HOME/thicc`
4. `~/.config/thicc` (default)

The configuration directory contains:

```
~/.config/thicc/
├── settings.json      # Editor settings
├── bindings.json      # Custom keybindings
├── recent.json        # Recent projects list
├── colorschemes/      # Custom colorschemes
└── plug/              # Installed plugins
```

## Environment Variables

| Variable | Description |
|----------|-------------|
| `THICC_CONFIG_HOME` | Override config directory location |
| `MICRO_CONFIG_HOME` | Fallback config directory (compatibility) |
| `SHELL` | Detected for terminal shell selection |
| `ANTHROPIC_API_KEY` | For Claude Code integration |

---

[Back to Documentation Home](index.md)
