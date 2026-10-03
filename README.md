# Jacob Helpa

A simple, friendly command-line helper utility, written in Go.

## What it does

Run it with no arguments for an interactive menu, or pass a command directly.

Available tools:
- `time`  — show the current date & time
- `calc`  — quick arithmetic, e.g. `calc 12 * 8`
- `flip`  — flip a coin
- `roll`  — roll a dice (1-6)
- `info`  — show system info
- `about` — about this program
- `help`  — list commands

## Usage

Interactive:

```
Jacob Helpa.exe
```

Direct command:

```
Jacob Helpa.exe time
Jacob Helpa.exe calc 12 * 8
Jacob Helpa.exe flip
```

## Building

Requires [Go](https://go.dev/dl/) 1.21+.

Build a Windows `.exe` (works from any OS):

```sh
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags="-s -w" -o "Jacob Helpa.exe" .
```

Or just build for your current platform:

```sh
go build -o "Jacob Helpa" .
```

The produced `.exe` is a single, self-contained file with no external
dependencies.
