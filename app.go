package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the object Wails binds to the frontend. Every exported method on it
// becomes callable from JavaScript as window.go.main.App.<Name>().
type App struct {
	ctx       context.Context
	startedAt time.Time

	mu     sync.Mutex
	config *Config
	vault  *vault
}

// Config is the small bit of state that survives a restart.
type Config struct {
	Theme      string `json:"theme"`      // "dark" or "light"
	Background string `json:"background"` // welcome animation, or "off"
	Accent     string `json:"accent"`     // hex colour for the UI accent
	Greeting   string `json:"greeting"`   // name used on the welcome screen
	SkipIntro  bool   `json:"skipIntro"`  // jump straight past the welcome animation
	Links      []Link `json:"links"`      // quick-launch shortcuts
	Updated    string `json:"updated"`
}

// Link is one quick-launch shortcut.
type Link struct {
	Label  string `json:"label"`
	Target string `json:"target"`
}

// Meta describes the running app.
type Meta struct {
	App     string `json:"app"`
	Version string `json:"version"`
	Config  Config `json:"config"`
}

func defaultConfig() *Config {
	return &Config{
		Theme:      "dark",
		Background: "net",
		Accent:     "#5b8cff",
		Links: []Link{
			{Label: "GitHub", Target: "https://github.com"},
			{Label: "Go docs", Target: "https://go.dev/doc/"},
		},
	}
}

func newApp() (*App, error) {
	dir, err := dataDir()
	if err != nil {
		return nil, err
	}
	a := &App{
		startedAt: time.Now(),
		config:    defaultConfig(),
		vault:     newVault(filepath.Join(dir, "vault.dat")),
	}
	a.loadConfig()
	return a, nil
}

// startup is handed the context Wails needs for window operations.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// --- bound methods -------------------------------------------------------

// GetMeta returns the app name, version and saved settings.
func (a *App) GetMeta() Meta {
	return Meta{App: appName, Version: version, Config: a.snapshotConfig()}
}

// GetConfig returns the saved settings.
func (a *App) GetConfig() Config {
	return a.snapshotConfig()
}

// SaveConfig validates and stores the settings, returning what was written.
func (a *App) SaveConfig(incoming Config) (Config, error) {
	if err := validateConfig(&incoming); err != nil {
		return Config{}, err
	}
	if err := a.saveConfig(&incoming); err != nil {
		return Config{}, fmt.Errorf("could not save settings: %w", err)
	}
	return incoming, nil
}

// OpenTarget opens a web link, file or folder with whatever the system uses
// for it. Only http and https links and paths that exist are accepted.
func (a *App) OpenTarget(target string) error {
	target = strings.TrimSpace(target)
	if target == "" {
		return fmt.Errorf("nothing to open")
	}

	if parsed, err := url.Parse(target); err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") {
		if parsed.Host == "" {
			return fmt.Errorf("that link has no address")
		}
		wailsruntime.BrowserOpenURL(a.ctx, parsed.String())
		return nil
	}

	if strings.Contains(target, "://") {
		return fmt.Errorf("only http and https links, files and folders can be opened")
	}
	if _, err := os.Stat(target); err != nil {
		return fmt.Errorf("that file or folder doesn't exist")
	}
	return openPath(target)
}

// Quit closes the window and ends the program.
func (a *App) Quit() {
	wailsruntime.Quit(a.ctx)
}

// --- config on disk ------------------------------------------------------

func validateConfig(cfg *Config) error {
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

// dataDir is where config, the vault and the log live. It is created if needed.
func dataDir() (string, error) {
	var base string
	switch runtime.GOOS {
	case "windows":
		base = os.Getenv("APPDATA")
		if base == "" {
			base = os.Getenv("LOCALAPPDATA")
		}
	case "darwin":
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, "Library", "Application Support")
	default:
		base = os.Getenv("XDG_CONFIG_HOME")
		if base == "" {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", err
			}
			base = filepath.Join(home, ".config")
		}
	}
	if base == "" {
		var err error
		base, err = os.UserConfigDir()
		if err != nil {
			return "", err
		}
	}
	dir := filepath.Join(base, "JacobHelpa")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("could not create %s: %w", dir, err)
	}
	return dir, nil
}

func (a *App) configPath() (string, error) {
	dir, err := dataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// loadConfig merges the saved config over the defaults, so a config written by
// an older version still works after an upgrade.
func (a *App) loadConfig() {
	path, err := a.configPath()
	if err != nil {
		return
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	cfg := defaultConfig()
	if err := json.Unmarshal(data, cfg); err != nil {
		log.Printf("config at %s is unreadable, using defaults: %v", path, err)
		return
	}
	a.mu.Lock()
	a.config = cfg
	a.mu.Unlock()
}

func (a *App) saveConfig(cfg *Config) error {
	path, err := a.configPath()
	if err != nil {
		return err
	}
	cfg.Updated = time.Now().Format(time.RFC3339)

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	// Write to a temp file then rename, so a crash can't leave a half-written
	// config behind.
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return err
	}

	a.mu.Lock()
	a.config = cfg
	a.mu.Unlock()
	return nil
}

func (a *App) snapshotConfig() Config {
	a.mu.Lock()
	defer a.mu.Unlock()
	return *a.config
}

// setupLogging writes a rolling log next to the app data. A Wails window has no
// console attached, so this is the only place errors are recorded.
func setupLogging() *os.File {
	log.SetFlags(log.Ldate | log.Ltime)
	dir, err := dataDir()
	if err != nil {
		return nil
	}
	path := filepath.Join(dir, "jacob-helpa.log")
	if info, err := os.Stat(path); err == nil && info.Size() > 1<<20 {
		_ = os.Remove(path)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil
	}
	log.SetOutput(f)
	return f
}
