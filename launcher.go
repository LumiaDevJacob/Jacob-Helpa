package main

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// openWindow shows the UI. A Chromium-based browser in "app mode" gives a
// frameless window with no tabs or address bar, which is what makes this feel
// like a native app rather than a web page. If none is installed we fall back
// to the default browser.
func openWindow(target string) error {
	profile, err := os.MkdirTemp("", "jacob-helpa-window")
	if err != nil {
		profile = "" // not fatal; the browser will use its normal profile
	}

	args := []string{
		"--app=" + target,
		"--window-size=1280,860",
		"--no-first-run",
		"--no-default-browser-check",
	}
	if profile != "" {
		// A throwaway profile keeps the app window out of the user's normal
		// browser session and stops it from reusing an existing process.
		args = append(args, "--user-data-dir="+profile)
	}

	for _, browser := range chromiumCandidates() {
		path, err := exec.LookPath(browser)
		if err != nil {
			if _, statErr := os.Stat(browser); statErr != nil {
				continue
			}
			path = browser
		}
		cmd := exec.Command(path, args...)
		if err := cmd.Start(); err != nil {
			log.Printf("could not launch %s: %v", path, err)
			continue
		}
		log.Printf("opened app window with %s", filepath.Base(path))
		go func() { _ = cmd.Wait() }()
		return nil
	}

	log.Printf("no chromium-based browser found, falling back to the default browser")
	return openExternal(target)
}

// chromiumCandidates lists browsers that support --app=, best first.
func chromiumCandidates() []string {
	switch runtime.GOOS {
	case "windows":
		programFiles := os.Getenv("ProgramFiles")
		programFilesX86 := os.Getenv("ProgramFiles(x86)")
		localAppData := os.Getenv("LOCALAPPDATA")
		var out []string
		add := func(base, rest string) {
			if base != "" {
				out = append(out, filepath.Join(base, rest))
			}
		}
		add(programFilesX86, `Microsoft\Edge\Application\msedge.exe`)
		add(programFiles, `Microsoft\Edge\Application\msedge.exe`)
		add(programFiles, `Google\Chrome\Application\chrome.exe`)
		add(programFilesX86, `Google\Chrome\Application\chrome.exe`)
		add(localAppData, `Google\Chrome\Application\chrome.exe`)
		add(programFiles, `BraveSoftware\Brave-Browser\Application\brave.exe`)
		return append(out, "msedge.exe", "chrome.exe")
	case "darwin":
		return []string{
			"/Applications/Google Chrome.app/Contents/MacOS/Google Chrome",
			"/Applications/Microsoft Edge.app/Contents/MacOS/Microsoft Edge",
			"/Applications/Brave Browser.app/Contents/MacOS/Brave Browser",
			"/Applications/Chromium.app/Contents/MacOS/Chromium",
		}
	default:
		return []string{
			"google-chrome", "google-chrome-stable", "chromium", "chromium-browser",
			"microsoft-edge", "brave-browser",
		}
	}
}

// openExternal hands a URL or path to the operating system's default handler.
func openExternal(target string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		// rundll32 takes the target as a single argument, so there is no shell
		// involved and nothing to quote wrongly.
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", target)
	case "darwin":
		cmd = exec.Command("open", target)
	default:
		cmd = exec.Command("xdg-open", target)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("could not open %q: %w", target, err)
	}
	go func() { _ = cmd.Wait() }()
	return nil
}

// handleOpen backs the quick-launch panel. Only web links and paths that
// actually exist on disk are accepted, and the target is always passed as a
// single argument so there is no shell to inject into.
func (a *app) handleOpen(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Target string `json:"target"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	target := strings.TrimSpace(req.Target)
	if target == "" {
		writeErr(w, http.StatusBadRequest, "nothing to open")
		return
	}

	if parsed, err := url.Parse(target); err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") {
		if parsed.Host == "" {
			writeErr(w, http.StatusBadRequest, "that link has no address")
			return
		}
		if err := openExternal(parsed.String()); err != nil {
			writeErr(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"opened": parsed.String()})
		return
	}

	// Anything else has to be a real file or folder.
	if strings.Contains(target, "://") {
		writeErr(w, http.StatusBadRequest, "only http and https links, files and folders can be opened")
		return
	}
	if _, err := os.Stat(target); err != nil {
		writeErr(w, http.StatusBadRequest, "that file or folder doesn't exist")
		return
	}
	if err := openExternal(target); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"opened": target})
}
