#!/bin/bash
# This script creates pre-releases (RC, beta, alpha) on GitHub for thicc
# Requires: GitHub CLI (gh) authenticated
#
# Usage: ./pre-release.sh VERSION [DESCRIPTION]
# Example: ./pre-release.sh 1.2.3-rc1 "Release candidate for v1.2.3"

set -e

if [ -z "$1" ]; then
    echo "Usage: $0 VERSION [DESCRIPTION]"
    echo "Example: $0 1.2.3-rc1 \"Release candidate for v1.2.3\""
    exit 1
fi

VERSION="$1"
DESCRIPTION="${2:-Pre-release v$VERSION}"
TAG="v$VERSION"

# Ensure we're in the repo root
cd "$(git rev-parse --show-toplevel)"

# Check for uncommitted changes
if ! git diff-index --quiet HEAD --; then
    echo "Error: You have uncommitted changes. Please commit or stash them first."
    exit 1
fi

# Check if tag already exists
if git rev-parse "$TAG" >/dev/null 2>&1; then
    echo "Error: Tag $TAG already exists"
    exit 1
fi

echo "Creating tag $TAG..."
git tag -a "$TAG" -m "Pre-release $TAG"

echo "Pushing tag to origin..."
git push origin "$TAG"

echo "Cross compiling binaries..."
./tools/cross-compile.sh "$VERSION"

echo "Creating GitHub pre-release..."
gh release create "$TAG" \
    --title "$TAG" \
    --notes "$DESCRIPTION" \
    --prerelease \
    binaries/thicc-"$VERSION"-*.tar.gz \
    binaries/thicc-"$VERSION"-*.zip \
    binaries/thicc-"$VERSION"-*.deb \
    binaries/thicc-"$VERSION"-*.sha 2>/dev/null || \
gh release create "$TAG" \
    --title "$TAG" \
    --notes "$DESCRIPTION" \
    --prerelease \
    binaries/thicc-"$VERSION"-*

echo "Cleaning up..."
rm -rf binaries

echo ""
echo "Pre-release $TAG created successfully!"
echo "View at: https://github.com/elleryfamilia/thicc/releases/tag/$TAG"
