package main

import (
	"fmt"
	"os/exec"
	"runtime"
)

// openPath hands a file or folder to the system's default handler. The target
// is always passed as a single argument, so there is no shell to inject into.
func openPath(target string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
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
