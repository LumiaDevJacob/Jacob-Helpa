package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"strings"
	"time"
)

//go:embed all:web
var webFiles embed.FS

const sessionCookie = "helpa_session"

func (a *app) routes() http.Handler {
	mux := http.NewServeMux()

	assets, err := fs.Sub(webFiles, "web")
	if err != nil {
		fatal(fmt.Errorf("bundled assets are missing: %w", err))
	}
	fileServer := http.FileServer(http.FS(assets))

	// The entry point. It takes the token from the query string, hands back a
	// cookie, and serves the shell. Every later request uses the cookie.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			a.guard(fileServer.ServeHTTP)(w, r)
			return
		}
		if r.URL.Query().Get("t") != a.token && !a.hasSession(r) {
			http.Error(w, "This window is not authorised. Relaunch Jacob Helpa.", http.StatusForbidden)
			return
		}
		http.SetCookie(w, &http.Cookie{
			Name:     sessionCookie,
			Value:    a.token,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteStrictMode,
		})
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		page, err := fs.ReadFile(assets, "index.html")
		if err != nil {
			http.Error(w, "interface missing", http.StatusInternalServerError)
			return
		}
		_, _ = w.Write(page)
	})

	api := map[string]http.HandlerFunc{
		"/api/meta":         a.handleMeta,
		"/api/boot":         a.handleBootStream,
		"/api/password":     a.handlePassword,
		"/api/username":     a.handleUsername,
		"/api/hash":         a.handleHash,
		"/api/tokens":       a.handleTokens,
		"/api/text":         a.handleText,
		"/api/sysinfo":      a.handleSysinfo,
		"/api/config":       a.handleConfig,
		"/api/open":         a.handleOpen,
		"/api/vault/status": a.handleVaultStatus,
		"/api/vault/unlock": a.handleVaultUnlock,
		"/api/vault/lock":   a.handleVaultLock,
		"/api/vault/list":   a.handleVaultList,
		"/api/vault/save":   a.handleVaultSave,
		"/api/vault/delete": a.handleVaultDelete,
		"/api/quit":         a.handleQuit,
	}
	for path, handler := range api {
		mux.HandleFunc(path, a.guard(handler))
	}

	return mux
}

// guard rejects anything that isn't a same-origin request from our own window.
func (a *app) guard(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Refuse a Host we didn't hand out, which blocks DNS rebinding.
		if r.Host != a.host {
			http.Error(w, "bad host", http.StatusForbidden)
			return
		}
		if origin := r.Header.Get("Origin"); origin != "" && origin != "http://"+a.host {
			http.Error(w, "bad origin", http.StatusForbidden)
			return
		}
		if !a.hasSession(r) && r.URL.Query().Get("t") != a.token {
			http.Error(w, "not authorised", http.StatusForbidden)
			return
		}
		w.Header().Set("Cache-Control", "no-store")
		next(w, r)
	}
}

func (a *app) hasSession(r *http.Request) bool {
	c, err := r.Cookie(sessionCookie)
	return err == nil && c.Value == a.token
}

// --- small helpers -------------------------------------------------------

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		log.Printf("could not write response: %v", err)
	}
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// readJSON decodes a request body, refusing anything oversized or malformed.
func readJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "use POST")
		return false
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeErr(w, http.StatusBadRequest, "could not read the request")
		return false
	}
	if len(body) == 0 {
		return true // an empty body means "all defaults"
	}
	if err := json.Unmarshal(body, dst); err != nil {
		writeErr(w, http.StatusBadRequest, "that request wasn't valid JSON")
		return false
	}
	return true
}

// --- handlers ------------------------------------------------------------

func (a *app) handleMeta(w http.ResponseWriter, r *http.Request) {
	cfg := a.snapshotConfig()
	writeJSON(w, http.StatusOK, map[string]any{
		"app":     appName,
		"version": version,
		"config":  cfg,
	})
}

// bootStep is one line of the startup sequence shown on the welcome screen.
type bootStep struct {
	Label  string `json:"label"`
	Detail string `json:"detail"`
	OK     bool   `json:"ok"`
	Done   bool   `json:"done"`
	Index  int    `json:"index"`
	Total  int    `json:"total"`
}

// handleBootStream streams the real startup checks to the welcome screen as
// server-sent events. The progress bar in the UI tracks actual work, not a
// timer; the short pause between steps is only so the animation reads well.
func (a *app) handleBootStream(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, http.StatusInternalServerError, "streaming unsupported")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	steps := a.bootChecks()
	for i, check := range steps {
		step := check()
		step.Index = i + 1
		step.Total = len(steps)
		step.Done = i == len(steps)-1
		payload, err := json.Marshal(step)
		if err != nil {
			continue
		}
		fmt.Fprintf(w, "data: %s\n\n", payload)
		flusher.Flush()

		select {
		case <-r.Context().Done():
			return
		case <-time.After(260 * time.Millisecond):
		}
	}
}

func (a *app) handleQuit(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "closing"})
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	go func() {
		time.Sleep(150 * time.Millisecond)
		a.shutdown()
	}()
}

func (a *app) handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		cfg := a.snapshotConfig()
		writeJSON(w, http.StatusOK, cfg)
		return
	}
	incoming := a.snapshotConfig()
	if !readJSON(w, r, &incoming) {
		return
	}
	if err := validateConfig(&incoming); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := a.saveConfig(&incoming); err != nil {
		writeErr(w, http.StatusInternalServerError, "could not save settings: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, incoming)
}

func validateConfig(cfg *config) error {
	switch cfg.Theme {
	case "dark", "light":
	default:
		return fmt.Errorf("theme must be dark or light")
	}
	switch cfg.Background {
	case "net", "halo", "waves", "globe", "off":
	default:
		return fmt.Errorf("unknown background %q", cfg.Background)
	}
	if !isHexColour(cfg.Accent) {
		return fmt.Errorf("accent must be a hex colour like #5b8cff")
	}
	if len(cfg.Greeting) > 40 {
		return fmt.Errorf("that name is too long")
	}
	if len(cfg.Links) > 40 {
		return fmt.Errorf("40 shortcuts is the limit")
	}
	for i := range cfg.Links {
		cfg.Links[i].Label = strings.TrimSpace(cfg.Links[i].Label)
		cfg.Links[i].Target = strings.TrimSpace(cfg.Links[i].Target)
		if cfg.Links[i].Label == "" || cfg.Links[i].Target == "" {
			return fmt.Errorf("every shortcut needs a name and a target")
		}
		if len(cfg.Links[i].Label) > 60 || len(cfg.Links[i].Target) > 2000 {
			return fmt.Errorf("that shortcut is too long")
		}
	}
	return nil
}

func isHexColour(s string) bool {
	if len(s) != 7 || s[0] != '#' {
		return false
	}
	for _, c := range s[1:] {
		isDigit := c >= '0' && c <= '9'
		isHex := (c >= 'a' && c <= 'f') || (c >= 'A' && c <= 'F')
		if !isDigit && !isHex {
			return false
		}
	}
	return true
}
