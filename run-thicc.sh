#!/bin/bash
# THICC launcher script
cd "$(dirname "$0")"
export MICRO_CONFIG_HOME=~/.config/thicc
rm -f log.txt  # Clear old log
./thicc "$@"
