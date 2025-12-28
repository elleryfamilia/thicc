#!/bin/bash
# THOCK launcher script
cd /Users/ellery/_git/thock
export MICRO_CONFIG_HOME=~/.config/thock
rm -f log.txt  # Clear old log
./thock "$@"
