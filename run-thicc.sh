#!/bin/bash
# THICC launcher script
cd "$(dirname "$0")"
export MICRO_CONFIG_HOME=~/.config/thicc

if [[ "$1" == "mcp" ]]; then
    # MCP server mode - skip log cleanup, use exec for cleaner process
    exec ./thicc "$@"
fi

# Interactive mode - capture stderr to log file
rm -f log.txt  # Clear old log
./thicc "$@" 2>log.txt
