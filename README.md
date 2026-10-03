# Jacob Helpa

A desktop helper app for Windows. One `.exe`, nothing to install, works with no
internet connection.

It opens with an animated welcome screen, then gives you a set of small tools
behind a glass interface: password and username generators, random keys and
UUIDs, hashes, text conversion, an encrypted note vault, system info and your
own shortcuts.

## Getting it

Grab `Jacob Helpa.exe` from the [Releases page](../../releases), or build it
yourself (see below). Double-click it. That's the whole setup.

The first launch creates a small folder for your settings and notes:

| System  | Folder |
|---------|--------|
| Windows | `%APPDATA%\JacobHelpa` |
| macOS   | `~/Library/Application Support/JacobHelpa` |
| Linux   | `~/.config/JacobHelpa` |

## What's inside

| Tool | What it does |
|------|--------------|
| **Passwords** | Random passwords from 6 to 128 characters. Choose which character types to use, optionally drop look-alikes like `O`/`0`, and see the real entropy in bits. |
| **Usernames** | Four styles — clean, word pair, dotted, numbered — built from a word you pick or from a built-in word list. |
| **Keys & UUIDs** | Random values as hex, base64url and version-4 UUIDs, 8 to 128 bytes. |
| **Hashes** | MD5, SHA-1, SHA-256 and SHA-512 of any text. |
| **Text** | Case changes, slugs, trim, reverse, base64, URL encoding, sort/dedupe/shuffle lines, plus a live character and word count. |
| **Vault** | Notes encrypted on your machine with AES-256-GCM. The key comes from your master password, which is never stored. |
| **System** | OS, cores, hostname, local addresses, uptime and where the app keeps its files. |
| **Settings** | Theme, accent colour, which welcome animation to use, your name, and your shortcuts. |

Click any generated value to copy it. `Ctrl`+`1` to `Ctrl`+`9` jump between pages.

## How it works

The `.exe` is a small Go web server with the whole interface built into it. On
launch it binds to a random port on `127.0.0.1` and opens a Chromium-based
browser (Edge, Chrome or Brave) in app mode, which gives a clean window with no
tabs or address bar. If none of those are installed it falls back to your
default browser.

Nothing leaves your machine:

- The listener is bound to `127.0.0.1`, so it is never reachable from the network.
- Every request needs a session token generated fresh on each launch.
- Requests with an unexpected `Host` or `Origin` header are refused, which blocks DNS rebinding.
- There are no external requests at all — every library is bundled into the executable.

## Building it

You need [Go](https://go.dev/dl/) 1.24 or newer. There are no other
dependencies and nothing to download.

```sh
./build.sh windows   # dist/Jacob Helpa.exe
./build.sh           # Windows, Linux and macOS
```

The Windows build cross-compiles from any operating system:

```sh
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 \
  go build -ldflags="-s -w -H=windowsgui" -o "Jacob Helpa.exe" .
```

To run it locally while working on it:

```sh
go run .
```

## Project layout

```
main.go        launcher: logging, the listener, shutdown
app.go         app state, config file handling
server.go      routes, session guard, the boot event stream
boot.go        the startup checks shown on the welcome screen
tools.go       passwords, usernames, keys, hashes, text, system info
vault.go       encrypted notes
launcher.go    opening the app window and external links
web/           the interface (embedded into the executable)
```

## Credits

The interface is built on two open source projects, bundled under `web/vendor`
with their licences:

- [GlassKit](https://github.com/JUNGHERZ/GlassKit) — the glass component library (MIT)
- [Vanta.js](https://github.com/tengbao/vanta) and [three.js](https://github.com/mrdoob/three.js) — the animated welcome backgrounds (MIT)
