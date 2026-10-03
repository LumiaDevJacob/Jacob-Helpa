#!/usr/bin/env sh
# Build the Windows .exe for Jacob Helpa.
set -e
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o "Jacob Helpa.exe" .
echo "Built: Jacob Helpa.exe"
