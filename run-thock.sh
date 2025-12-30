#!/bin/bash
# THOCK launcher script
cd "$(dirname "$0")"
export MICRO_CONFIG_HOME=~/.config/thock
rm -f log.txt  # Clear old log
./thock "$@"
