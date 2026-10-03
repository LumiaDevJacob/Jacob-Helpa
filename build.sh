#!/usr/bin/env sh
# Build Jacob Helpa. Pass a target, or nothing for all of them.
#
#   ./build.sh            -> every target below
#   ./build.sh windows    -> just the Windows .exe
set -e

OUT=dist
mkdir -p "$OUT"

build_windows() {
  # -H=windowsgui stops a console window opening behind the app.
  CGO_ENABLED=0 GOOS=windows GOARCH=amd64 \
    go build -ldflags="-s -w -H=windowsgui" -o "$OUT/Jacob Helpa.exe" .
  echo "built $OUT/Jacob Helpa.exe"
}

build_linux() {
  CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -ldflags="-s -w" -o "$OUT/jacob-helpa-linux" .
  echo "built $OUT/jacob-helpa-linux"
}

build_macos() {
  CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 \
    go build -ldflags="-s -w" -o "$OUT/jacob-helpa-macos" .
  echo "built $OUT/jacob-helpa-macos"
}

case "${1:-all}" in
  windows) build_windows ;;
  linux)   build_linux ;;
  macos)   build_macos ;;
  all)     build_windows; build_linux; build_macos ;;
  *)       echo "unknown target: $1 (use windows, linux, macos or all)"; exit 1 ;;
esac
