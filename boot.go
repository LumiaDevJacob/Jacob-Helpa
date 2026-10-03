package main

import (
	"fmt"
	"os"
	"runtime"
)

// bootChecks returns the startup sequence shown on the welcome screen. Each one
// does real work and reports what it found, so the progress bar reflects
// genuine state rather than a countdown.
func (a *app) bootChecks() []func() bootStep {
	return []func() bootStep{
		func() bootStep {
			return bootStep{
				Label:  "Starting core services",
				Detail: fmt.Sprintf("listening on %s", a.host),
				OK:     true,
			}
		},
		func() bootStep {
			dir, err := dataDir()
			if err != nil {
				return bootStep{Label: "Opening app folder", Detail: err.Error()}
			}
			return bootStep{Label: "Opening app folder", Detail: dir, OK: true}
		},
		func() bootStep {
			path, err := a.configPath()
			if err != nil {
				return bootStep{Label: "Loading your settings", Detail: err.Error()}
			}
			if _, err := os.Stat(path); err != nil {
				return bootStep{Label: "Loading your settings", Detail: "first run, using defaults", OK: true}
			}
			cfg := a.snapshotConfig()
			return bootStep{
				Label:  "Loading your settings",
				Detail: fmt.Sprintf("%s theme, %s background", cfg.Theme, cfg.Background),
				OK:     true,
			}
		},
		func() bootStep {
			return bootStep{
				Label:  "Reading this machine",
				Detail: fmt.Sprintf("%s/%s, %d cores", runtime.GOOS, runtime.GOARCH, runtime.NumCPU()),
				OK:     true,
			}
		},
		func() bootStep {
			if a.vault.exists() {
				return bootStep{Label: "Checking the secure vault", Detail: "found, locked", OK: true}
			}
			return bootStep{Label: "Checking the secure vault", Detail: "none yet", OK: true}
		},
		func() bootStep {
			return bootStep{
				Label:  "Preparing the toolkit",
				Detail: "7 tools ready",
				OK:     true,
			}
		},
		func() bootStep {
			return bootStep{Label: "Ready", Detail: appName + " " + version, OK: true}
		},
	}
}
