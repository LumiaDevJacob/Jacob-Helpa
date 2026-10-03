package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

// app holds everything the HTTP handlers need. One instance per process.
type app struct {
	token     string // session token, minted fresh on every launch
	host      string // 127.0.0.1:port, filled in once the listener is up
	startedAt time.Time

	done     chan struct{} // closed when the UI asks us to quit
	doneOnce sync.Once

	mu     sync.Mutex
	config *config
	vault  *vault
}

// config is the small bit of state that survives a restart.
type config struct {
	Theme      string `json:"theme"`      // "dark" or "light"
	Background string `json:"background"` // vanta effect name, or "off"
	Accent     string `json:"accent"`     // hex colour for the UI accent
	Greeting   string `json:"greeting"`   // name used on the welcome screen
	SkipIntro  bool   `json:"skipIntro"`  // jump straight past the welcome animation
	Links      []link `json:"links"`      // quick-launch shortcuts
	Updated    string `json:"updated"`
}

type link struct {
	Label  string `json:"label"`
	Target string `json:"target"`
}

func defaultConfig() *config {
	return &config{
		Theme:      "dark",
		Background: "net",
		Accent:     "#5b8cff",
		Greeting:   "",
		SkipIntro:  false,
		Links: []link{
			{Label: "GitHub", Target: "https://github.com"},
			{Label: "Go docs", Target: "https://go.dev/doc/"},
		},
	}
}

func newApp() (*app, error) {
	tok, err := newToken()
	if err != nil {
		return nil, err
	}
	dir, err := dataDir()
	if err != nil {
		return nil, err
	}

	a := &app{
		token:     tok,
		startedAt: time.Now(),
		done:      make(chan struct{}),
		config:    defaultConfig(),
		vault:     newVault(filepath.Join(dir, "vault.dat")),
	}
	a.loadConfig()
	return a, nil
}

func (a *app) shutdown() {
	a.doneOnce.Do(func() { close(a.done) })
}

// newToken returns a 256-bit random hex string.
func newToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("no secure randomness available: %w", err)
	}
	return hex.EncodeToString(buf), nil
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

func (a *app) configPath() (string, error) {
	dir, err := dataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// loadConfig merges the saved config over the defaults, so a config written by
// an older version still works after an upgrade.
func (a *app) loadConfig() {
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

func (a *app) saveConfig(cfg *config) error {
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

func (a *app) snapshotConfig() config {
	a.mu.Lock()
	defer a.mu.Unlock()
	return *a.config
}
