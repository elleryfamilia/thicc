# Changelog

## Unreleased

### Added

- **Update channel support**: Install script now defaults to stable releases instead of nightly
  - Use `CHANNEL=nightly` env var to install nightly builds
  - New `updatechannel` config option (`stable` or `nightly`) for auto-updates
  - Nightly versions compared by build timestamp for proper update detection

- **`--update` flag**: Manually check for and install updates from the command line
  ```sh
  thicc --update
  ```

- **`--uninstall` flag**: Self-removal like `rustup self uninstall`
  ```sh
  thicc --uninstall        # or sudo thicc --uninstall
  ```
  - Prompts for confirmation
  - Optionally removes config directory (`~/.config/thicc/`)

- **Improved `--help` output**: Clean, thicc-specific usage information
  - Documents pane toggle shortcuts (Alt+1 through Alt+5)
  - Removed all micro-inherited help text

### Changed

- Install script fetches from GitHub's `/releases/latest` API for stable releases
- All user-facing "micro" references replaced with "thicc"
- Profile output changed from `micro.prof` to `thicc.prof`
- Backup recovery messages updated to reference thicc

### Removed

- Removed `uninstall.sh` script (security concern with `curl | sudo sh`)
- Removed micro-style verbose help output

### Security

- Install script no longer requires piping to `sudo sh`
- Uninstall handled via `--uninstall` flag instead of remote script

---

## File Reference

Key files modified for these changes:

| File | Purpose |
|------|---------|
| `install.sh` | Install script with channel support |
| `cmd/thicc/micro.go` | CLI flags (`--update`, `--uninstall`, `--help`) |
| `internal/update/update.go` | Update checking with channel support |
| `internal/update/github.go` | GitHub API for fetching releases |
| `internal/update/prompt.go` | Update prompting logic |
| `internal/config/settings.go` | `updatechannel` config option |
