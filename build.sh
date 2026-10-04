#!/usr/bin/env sh
# Build the Windows .exe for Jacob Helpa.
#
# Wails' Windows backend is pure Go, so this cross-compiles from any operating
# system with nothing installed but Go itself - no Wails CLI, no Node, no gcc.
#
# Linux and macOS builds are a different story: those backends need CGO and the
# system webkit headers, so they only build on the machine they target, with
# `wails build`.
set -e

mkdir -p dist
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 \
  go build -ldflags="-s -w -H=windowsgui" -o "dist/Jacob Helpa.exe" .
echo "built dist/Jacob Helpa.exe"
