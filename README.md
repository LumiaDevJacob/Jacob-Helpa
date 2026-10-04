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

The app is built with [Wails](https://wails.io). The interface is HTML and CSS
embedded in the executable, drawn in a native window by the WebView that ships
with Windows. The frontend calls Go methods directly, so there is no browser, no
local server, no port and no token.

Nothing leaves your machine. Every library is bundled into the executable, and
the app makes no network requests at all.

Windows 11 and up-to-date Windows 10 already have the WebView2 runtime. On an
older machine the app offers to fetch it on first run.

## Building it

You need [Go](https://go.dev/dl/) 1.24 or newer. Nothing else - no Wails CLI,
no Node, no C compiler, because the Wails Windows backend is pure Go.

```sh
./build.sh      # dist/Jacob Helpa.exe
```

That works from Windows, macOS or Linux. Under the hood it is just:

```sh
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 \
  go build -ldflags="-s -w -H=windowsgui" -o "Jacob Helpa.exe" .
```

Builds for Linux and macOS are the exception: those backends need CGO and the
system webkit headers, so they have to be built on the machine they target with
`wails build`.

## Project layout

```
main.go           the Wails window
app.go            app state, config, and the methods bound to the frontend
boot.go           the startup checks shown on the welcome screen
tools.go          passwords, usernames, keys, hashes, text, system info
vault.go          encrypted notes
open.go           opening files and folders
frontend/dist/    the interface (embedded into the executable)
```

## Credits

The app is built with [Wails](https://github.com/wailsapp/wails) (MIT). The
interface uses two more open source projects, bundled under
`frontend/dist/vendor` with their licences:

- [GlassKit](https://github.com/JUNGHERZ/GlassKit) — the glass component library (MIT)
- [Vanta.js](https://github.com/tengbao/vanta) and [three.js](https://github.com/mrdoob/three.js) — the animated welcome backgrounds (MIT)
