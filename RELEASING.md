# Releasing thicc

This document describes how to create releases for thicc.

## Release Types

| Type | Tag Format | Trigger | Prerelease? |
|------|------------|---------|-------------|
| Stable | `v1.2.3` | Push tag | No |
| Nightly | `nightly` | Daily cron / manual | Yes |

## Creating a Stable Release

### 1. Prepare the Release

Ensure all changes are merged to `main` and tests pass:

```bash
git checkout main
git pull origin main
make build
make test  # if available
```

### 2. Choose a Version Number

Follow [Semantic Versioning](https://semver.org/):
- **MAJOR** (`v2.0.0`): Breaking changes
- **MINOR** (`v1.1.0`): New features, backward compatible
- **PATCH** (`v1.0.1`): Bug fixes, backward compatible

### 3. Create and Push the Tag

```bash
# Create an annotated tag
git tag -a v1.2.3 -m "Release v1.2.3"

# Push the tag to GitHub
git push origin v1.2.3
```

### 4. Automated Build & Release

Once the tag is pushed, GitHub Actions will automatically:
1. Build binaries for all platforms (macOS, Linux, Windows, BSD)
2. Create SHA256 checksums
3. Create a GitHub Release with auto-generated release notes
4. Attach all binaries to the release

Monitor progress at: https://github.com/elleryfamilia/thicc/actions

### 5. Edit Release Notes (Optional)

After the workflow completes:
1. Go to https://github.com/elleryfamilia/thicc/releases
2. Click on the new release
3. Click "Edit" to add custom release notes, highlights, or breaking changes

## Manual Release (Alternative)

If you need to trigger a release manually:

1. Create and push the tag first (steps above)
2. Go to GitHub Actions → "Release builds"
3. Click "Run workflow"
4. Select the branch (usually `main`)
5. Click "Run workflow"

## Updating thicc

Users can update thicc directly from the command line.

### Check for Updates

```bash
thicc --update                    # Check using your configured channel
thicc --update --channel nightly  # Check nightly channel
thicc --update --channel stable   # Check stable channel
```

### Force Update

Use `--force` to install regardless of version (useful for switching channels):

```bash
thicc --update --channel stable --force  # Switch back to stable from nightly
thicc --update --channel nightly --force # Force reinstall nightly
```

The update channel can also be set permanently in settings via the `updatechannel` option.

## Nightly Builds

Nightly builds run automatically every day at midnight UTC. They can also be triggered manually:

1. Go to GitHub Actions → "Nightly builds"
2. Click "Run workflow"

Nightly builds are marked as prereleases and tagged with `nightly`.

## Version Number in Code

The version is injected at build time via linker flags. The `tools/build-version.go` script determines the version:

- **On a tag**: Uses the exact tag (e.g., `1.2.3`)
- **After a tag**: Uses `{next-patch}-dev.{commits-ahead}+{timestamp}` (e.g., `1.2.4-dev.5+202501021530`)

## Release Checklist

- [ ] All tests pass
- [ ] CHANGELOG updated (if maintained)
- [ ] Version number follows semver
- [ ] Tag is annotated (`git tag -a`, not just `git tag`)
- [ ] Tag is pushed to origin
- [ ] GitHub Actions workflow completed successfully
- [ ] Release notes reviewed/edited
- [ ] Binaries download and run correctly (spot check)

## Supported Platforms

Releases include binaries for:

| Platform | Architecture | File |
|----------|--------------|------|
| macOS | Intel (amd64) | `thicc-VERSION-osx.tar.gz` |
| macOS | Apple Silicon (arm64) | `thicc-VERSION-macos-arm64.tar.gz` |
| Linux | 64-bit (amd64) | `thicc-VERSION-linux64.tar.gz` |
| Linux | 32-bit (386) | `thicc-VERSION-linux32.tar.gz` |
| Linux | ARM 32-bit | `thicc-VERSION-linux-arm.tar.gz` |
| Linux | ARM 64-bit | `thicc-VERSION-linux-arm64.tar.gz` |
| Linux | Debian package | `thicc-VERSION-amd64.deb` |
| Windows | 64-bit (amd64) | `thicc-VERSION-win64.zip` |
| Windows | 32-bit (386) | `thicc-VERSION-win32.zip` |
| Windows | ARM 64-bit | `thicc-VERSION-win-arm64.zip` |
| FreeBSD | 64-bit | `thicc-VERSION-freebsd64.tar.gz` |
| FreeBSD | 32-bit | `thicc-VERSION-freebsd32.tar.gz` |
| OpenBSD | 64-bit | `thicc-VERSION-openbsd64.tar.gz` |
| OpenBSD | 32-bit | `thicc-VERSION-openbsd32.tar.gz` |
| NetBSD | 64-bit | `thicc-VERSION-netbsd64.tar.gz` |
| NetBSD | 32-bit | `thicc-VERSION-netbsd32.tar.gz` |

## Troubleshooting

### Tag already exists
```bash
# Delete local tag
git tag -d v1.2.3

# Delete remote tag (use with caution!)
git push origin :refs/tags/v1.2.3
```

### Workflow failed
1. Check the Actions log for errors
2. Fix the issue
3. Delete the tag and release (if created)
4. Create a new tag and push again

### Wrong version in binary
The version comes from git tags. Ensure:
- You're on the correct commit
- The tag exists and is pushed
- Run `git describe --tags` to see what version will be used
