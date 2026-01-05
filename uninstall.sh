#!/bin/sh
set -e

# thicc uninstaller
# Usage: curl -fsSL https://raw.githubusercontent.com/elleryfamilia/thicc/main/uninstall.sh | sh

INSTALL_DIR="/usr/local/bin"
CONFIG_DIR="${HOME}/.config/thicc"

echo "Uninstalling thicc..."

# Remove binary
if [ -f "$INSTALL_DIR/thicc" ]; then
  if [ -w "$INSTALL_DIR/thicc" ]; then
    rm "$INSTALL_DIR/thicc"
  else
    echo "Removing $INSTALL_DIR/thicc (requires sudo)..."
    sudo rm "$INSTALL_DIR/thicc"
  fi
  echo "Removed $INSTALL_DIR/thicc"
else
  echo "Binary not found at $INSTALL_DIR/thicc"
fi

# Ask about config directory
if [ -d "$CONFIG_DIR" ]; then
  printf "Remove configuration directory at %s? [y/N] " "$CONFIG_DIR"
  read -r response
  case "$response" in
    [yY][eE][sS]|[yY])
      rm -rf "$CONFIG_DIR"
      echo "Removed $CONFIG_DIR"
      ;;
    *)
      echo "Kept $CONFIG_DIR"
      ;;
  esac
else
  echo "No configuration directory found at $CONFIG_DIR"
fi

echo ""
echo "thicc uninstalled successfully."
