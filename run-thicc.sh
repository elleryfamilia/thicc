#!/bin/bash
# THICC launcher script

# Save the original directory
ORIG_DIR="$(pwd)"

# Convert any relative path arguments to absolute paths BEFORE changing directory
ARGS=()
for arg in "$@"; do
    if [[ -e "$arg" ]]; then
        # If it's an existing path, convert to absolute
        ARGS+=("$(cd "$ORIG_DIR" && realpath "$arg")")
    else
        ARGS+=("$arg")
    fi
done

cd "$(dirname "$0")"
export MICRO_CONFIG_HOME=~/.config/thicc
# rm -f log.txt  # Clear old log - disabled for debugging
./thicc "${ARGS[@]}"
