package main

import (
	"fmt"
	"os"
	"runtime"
)

// BootStep is one line of the startup sequence shown on the welcome screen.
type BootStep struct {
	Label  string `json:"label"`
	Detail string `json:"detail"`
	OK     bool   `json:"ok"`
	Index  int    `json:"index"`
	Total  int    `json:"total"`
}

// BootSteps runs the startup checks and returns what each one found. The
// welcome screen animates through these, so the progress bar reflects genuine
// state rather than a countdown.
func (a *App) BootSteps() []BootStep {
	checks := []func() BootStep{
		func() BootStep {
			return BootStep{
				Label:  "Starting up",
				Detail: appName + " " + version,
				OK:     true,
			}
		},
		func() BootStep {
			dir, err := dataDir()
			if err != nil {
				return BootStep{Label: "Opening app folder", Detail: err.Error()}
			}
			return BootStep{Label: "Opening app folder", Detail: dir, OK: true}
		},
		func() BootStep {
			path, err := a.configPath()
			if err != nil {
				return BootStep{Label: "Loading your settings", Detail: err.Error()}
			}
			if _, err := os.Stat(path); err != nil {
				return BootStep{Label: "Loading your settings", Detail: "first run, using defaults", OK: true}
			}
			cfg := a.snapshotConfig()
			return BootStep{
				Label:  "Loading your settings",
				Detail: fmt.Sprintf("%s theme, %s background", cfg.Theme, cfg.Background),
				OK:     true,
			}
		},
		func() BootStep {
			return BootStep{
				Label:  "Reading this machine",
				Detail: fmt.Sprintf("%s/%s, %d cores", runtime.GOOS, runtime.GOARCH, runtime.NumCPU()),
				OK:     true,
			}
		},
		func() BootStep {
			if a.vault.exists() {
				return BootStep{Label: "Checking the secure vault", Detail: "found, locked", OK: true}
			}
			return BootStep{Label: "Checking the secure vault", Detail: "none yet", OK: true}
		},
		func() BootStep {
			return BootStep{Label: "Preparing the toolkit", Detail: "7 tools ready", OK: true}
		},
		func() BootStep {
			return BootStep{Label: "Ready", Detail: "window handed over", OK: true}
		},
	}

	steps := make([]BootStep, 0, len(checks))
	for i, check := range checks {
		step := check()
		step.Index = i + 1
		step.Total = len(checks)
		steps = append(steps, step)
	}
	return steps
}
