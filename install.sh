#!/bin/sh
set -e

# thicc installer
# Usage: curl -fsSL https://raw.githubusercontent.com/elleryfamilia/thicc/main/install.sh | sh

REPO="elleryfamilia/thicc"
INSTALL_DIR="/usr/local/bin"

echo "Installing thicc..."

# Check for Go
if ! command -v go >/dev/null 2>&1; then
  echo "Error: Go is required to build thicc."
  echo "Install it from https://go.dev"
  exit 1
fi

# Check for git
if ! command -v git >/dev/null 2>&1; then
  echo "Error: git is required to install thicc."
  exit 1
fi

# Create temp directory and build
TMPDIR=$(mktemp -d)
trap "rm -rf $TMPDIR" EXIT

cd "$TMPDIR"
git clone --depth 1 "https://github.com/$REPO.git"
cd thicc
make build

# Install
if [ -w "$INSTALL_DIR" ]; then
  mv thicc "$INSTALL_DIR/"
else
  echo "Moving thicc to $INSTALL_DIR (requires sudo)..."
  sudo mv thicc "$INSTALL_DIR/"
fi

echo ""
echo "thicc installed successfully!"
echo "Run 'thicc' to get started."
