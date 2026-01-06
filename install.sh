#!/bin/sh
set -e

# thicc installer
# Usage: curl -fsSL https://raw.githubusercontent.com/elleryfamilia/thicc/main/install.sh | sh
#
# Options (via environment variables):
#   CHANNEL=nightly  - Install nightly build instead of stable release
#   CHANNEL=stable   - Install latest stable release (default)
#   GLOBAL=1         - Install to /usr/local/bin (requires sudo)
#
# Examples:
#   curl -fsSL ... | sh                      # Install to ~/.local/bin (no sudo)
#   curl -fsSL ... | GLOBAL=1 sh             # Install to /usr/local/bin (sudo)
#   curl -fsSL ... | CHANNEL=nightly sh      # Install nightly

REPO="elleryfamilia/thicc"
CHANNEL="${CHANNEL:-stable}"

# Default to ~/.local/bin (user-writable, no sudo needed)
# Use GLOBAL=1 for /usr/local/bin
if [ "${GLOBAL:-0}" = "1" ]; then
  INSTALL_DIR="/usr/local/bin"
else
  INSTALL_DIR="$HOME/.local/bin"
fi

echo "Installing thicc (channel: $CHANNEL) to $INSTALL_DIR..."

# Detect OS
OS=$(uname -s | tr '[:upper:]' '[:lower:]')
ARCH=$(uname -m)

# Map to release artifact names
case "$OS" in
  darwin)
    case "$ARCH" in
      x86_64) PLATFORM="osx" ;;
      arm64)  PLATFORM="macos-arm64" ;;
      *)      echo "Unsupported architecture: $ARCH"; exit 1 ;;
    esac
    ;;
  linux)
    case "$ARCH" in
      x86_64)  PLATFORM="linux64" ;;
      aarch64) PLATFORM="linux-arm64" ;;
      armv6l|armv7l) PLATFORM="linux-arm" ;;
      i686|i386) PLATFORM="linux32" ;;
      *)       echo "Unsupported architecture: $ARCH"; exit 1 ;;
    esac
    ;;
  freebsd)
    case "$ARCH" in
      x86_64|amd64) PLATFORM="freebsd64" ;;
      i686|i386)    PLATFORM="freebsd32" ;;
      *)            echo "Unsupported architecture: $ARCH"; exit 1 ;;
    esac
    ;;
  openbsd)
    case "$ARCH" in
      x86_64|amd64) PLATFORM="openbsd64" ;;
      i686|i386)    PLATFORM="openbsd32" ;;
      *)            echo "Unsupported architecture: $ARCH"; exit 1 ;;
    esac
    ;;
  netbsd)
    case "$ARCH" in
      x86_64|amd64) PLATFORM="netbsd64" ;;
      i686|i386)    PLATFORM="netbsd32" ;;
      *)            echo "Unsupported architecture: $ARCH"; exit 1 ;;
    esac
    ;;
  *)
    echo "Unsupported OS: $OS"
    echo "Please build from source: https://github.com/$REPO"
    exit 1
    ;;
esac

echo "Detected: $OS/$ARCH -> $PLATFORM"

# Determine release tag based on channel
if [ "$CHANNEL" = "nightly" ]; then
  RELEASE_TAG="nightly"
else
  # Get latest stable release tag
  LATEST_URL="https://api.github.com/repos/$REPO/releases/latest"
  if command -v curl >/dev/null 2>&1; then
    RELEASE_TAG=$(curl -sL "$LATEST_URL" | grep -o '"tag_name": *"[^"]*"' | head -1 | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')
  elif command -v wget >/dev/null 2>&1; then
    RELEASE_TAG=$(wget -qO- "$LATEST_URL" | grep -o '"tag_name": *"[^"]*"' | head -1 | sed 's/.*"tag_name": *"\([^"]*\)".*/\1/')
  fi

  # Fall back to nightly if no stable release found
  if [ -z "$RELEASE_TAG" ]; then
    echo "No stable release found, using nightly..."
    RELEASE_TAG="nightly"
  fi
fi

RELEASE_URL="https://github.com/$REPO/releases/download/$RELEASE_TAG"

# Find the asset URL - nightly uses version in filename
echo "Fetching from $RELEASE_TAG release..."

# Create temp directory
TMPDIR=$(mktemp -d)
trap "rm -rf $TMPDIR" EXIT
cd "$TMPDIR"

# Download the asset list and find matching file
ASSETS_URL="https://api.github.com/repos/$REPO/releases/tags/$RELEASE_TAG"
if command -v curl >/dev/null 2>&1; then
  ASSET_NAME=$(curl -sL "$ASSETS_URL" | grep -o "thicc-[^\"]*-${PLATFORM}.tar.gz" | head -1)
  if [ -z "$ASSET_NAME" ]; then
    echo "Error: Could not find binary for $PLATFORM"
    echo "Available at: https://github.com/$REPO/releases/tag/$RELEASE_TAG"
    exit 1
  fi
  DOWNLOAD_URL="$RELEASE_URL/$ASSET_NAME"
  echo "Downloading $ASSET_NAME..."
  curl -sL -o thicc.tar.gz "$DOWNLOAD_URL"
elif command -v wget >/dev/null 2>&1; then
  ASSET_NAME=$(wget -qO- "$ASSETS_URL" | grep -o "thicc-[^\"]*-${PLATFORM}.tar.gz" | head -1)
  if [ -z "$ASSET_NAME" ]; then
    echo "Error: Could not find binary for $PLATFORM"
    echo "Available at: https://github.com/$REPO/releases/tag/$RELEASE_TAG"
    exit 1
  fi
  DOWNLOAD_URL="$RELEASE_URL/$ASSET_NAME"
  echo "Downloading $ASSET_NAME..."
  wget -q -O thicc.tar.gz "$DOWNLOAD_URL"
else
  echo "Error: curl or wget required"
  exit 1
fi

# Extract
echo "Extracting..."
tar -xzf thicc.tar.gz

# Find the binary (it's in a versioned directory)
BINARY=$(find . -name "thicc" -type f | head -1)
if [ -z "$BINARY" ]; then
  echo "Error: Could not find thicc binary in archive"
  exit 1
fi

# Make executable
chmod +x "$BINARY"

# Create install directory if it doesn't exist
if [ ! -d "$INSTALL_DIR" ]; then
  echo "Creating $INSTALL_DIR..."
  mkdir -p "$INSTALL_DIR"
fi

# Install
if [ -w "$INSTALL_DIR" ]; then
  mv "$BINARY" "$INSTALL_DIR/"
else
  echo "Installing to $INSTALL_DIR (requires sudo)..."
  sudo mv "$BINARY" "$INSTALL_DIR/"
fi

echo ""
echo "thicc installed successfully to $INSTALL_DIR/thicc"

# Check if INSTALL_DIR is in PATH
case ":$PATH:" in
  *":$INSTALL_DIR:"*)
    echo "Run 'thicc' to get started."
    ;;
  *)
    echo ""
    echo "NOTE: $INSTALL_DIR is not in your PATH."
    echo "Add it by running one of:"
    echo ""
    echo "  # For bash (add to ~/.bashrc):"
    echo "  export PATH=\"\$HOME/.local/bin:\$PATH\""
    echo ""
    echo "  # For zsh (add to ~/.zshrc):"
    echo "  export PATH=\"\$HOME/.local/bin:\$PATH\""
    echo ""
    echo "  # For fish (add to ~/.config/fish/config.fish):"
    echo "  set -gx PATH \$HOME/.local/bin \$PATH"
    echo ""
    echo "Then restart your shell or run: source ~/.bashrc (or ~/.zshrc)"
    ;;
esac
