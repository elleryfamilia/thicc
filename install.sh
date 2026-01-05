#!/bin/sh
set -e

# thicc installer
# Usage: curl -fsSL https://raw.githubusercontent.com/elleryfamilia/thicc/main/install.sh | sh

REPO="elleryfamilia/thicc"
INSTALL_DIR="/usr/local/bin"

echo "Installing thicc..."

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

# Get latest release tag (or use nightly)
RELEASE_TAG="nightly"
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

# Install
if [ -w "$INSTALL_DIR" ]; then
  mv "$BINARY" "$INSTALL_DIR/"
else
  echo "Installing to $INSTALL_DIR (requires sudo)..."
  sudo mv "$BINARY" "$INSTALL_DIR/"
fi

echo ""
echo "thicc installed successfully!"
echo "Run 'thicc' to get started."
