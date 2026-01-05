#!/bin/sh
# Generic npm tool installer for thicc
# Usage: install-tool.sh <package-name>
# Example: curl -fsSL .../install-tool.sh | sh -s -- @google/gemini-cli

set -e

PACKAGE="$1"
if [ -z "$PACKAGE" ]; then
  echo "Usage: $0 <npm-package-name>"
  exit 1
fi

echo "Installing $PACKAGE..."

# Check/install npm if not present
if ! command -v npm >/dev/null 2>&1; then
  echo "Node.js/npm not found. Installing..."
  OS=$(uname -s | tr '[:upper:]' '[:lower:]')

  case "$OS" in
    darwin)
      if command -v brew >/dev/null 2>&1; then
        brew install node
      else
        echo "Error: Homebrew not found. Please install Node.js from https://nodejs.org"
        exit 1
      fi
      ;;
    linux)
      if command -v apt-get >/dev/null 2>&1; then
        sudo apt-get update && sudo apt-get install -y nodejs npm
      elif command -v dnf >/dev/null 2>&1; then
        sudo dnf install -y nodejs npm
      elif command -v pacman >/dev/null 2>&1; then
        sudo pacman -S --noconfirm nodejs npm
      else
        echo "Error: Could not detect package manager. Please install Node.js from https://nodejs.org"
        exit 1
      fi
      ;;
    *)
      echo "Error: Unsupported OS. Please install Node.js from https://nodejs.org"
      exit 1
      ;;
  esac
fi

# Install the tool globally
npm install -g "$PACKAGE"

echo ""
echo "$PACKAGE installed successfully!"
