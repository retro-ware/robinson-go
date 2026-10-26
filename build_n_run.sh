#!/bin/sh
# Builds the native port and starts it with the game data in orig/cd.
# Extra arguments go to the game (e.g. -scale 4, -mute).
set -e
cd "$(dirname "$0")"
mkdir -p bin
go build -o bin/rrq ./cmd/rrq
exec ./bin/rrq -data orig/cd "$@"
