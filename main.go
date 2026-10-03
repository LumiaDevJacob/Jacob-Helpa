// Jacob Helpa - a desktop helper app.
//
// The program is a single self-contained executable. On launch it starts a
// local-only HTTP server on a random high port, then opens the bundled UI in a
// chromeless browser window so it looks and behaves like a native app.
//
// Nothing is exposed to the network: the listener binds to 127.0.0.1 and every
// request must carry the session token minted at startup.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"
)

const (
	appName = "Jacob Helpa"
	version = "2.0.0"
)

func main() {
	logFile := setupLogging()
	if logFile != nil {
		defer logFile.Close()
	}

	log.Printf("%s %s starting", appName, version)

	app, err := newApp()
	if err != nil {
		fatal(fmt.Errorf("could not start: %w", err))
	}

	// Port 0 lets the OS hand us a free port, so two copies never collide.
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fatal(fmt.Errorf("could not open a local port: %w", err))
	}
	addr := listener.Addr().(*net.TCPAddr)
	app.host = fmt.Sprintf("127.0.0.1:%d", addr.Port)

	srv := &http.Server{
		Handler:           app.routes(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		if err := srv.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("server stopped: %v", err)
			app.shutdown()
		}
	}()

	url := fmt.Sprintf("http://%s/?t=%s", app.host, app.token)
	log.Printf("ui available at http://%s", app.host)

	if err := openWindow(url); err != nil {
		log.Printf("could not open an app window: %v", err)
		fmt.Printf("Open this in your browser:\n\n  %s\n\n", url)
	}

	// Quit on a window close (the UI tells us), Ctrl+C, or a kill signal.
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, os.Interrupt, syscall.SIGTERM)
	select {
	case <-app.done:
		log.Printf("ui closed, shutting down")
	case s := <-signals:
		log.Printf("got signal %v, shutting down", s)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
	log.Printf("goodbye")
}

// setupLogging writes a rolling log next to the app data. Builds for Windows
// are linked with -H=windowsgui and so have no console to print to.
func setupLogging() *os.File {
	log.SetFlags(log.Ldate | log.Ltime)
	dir, err := dataDir()
	if err != nil {
		return nil
	}
	path := filepath.Join(dir, "jacob-helpa.log")
	// Keep the log from growing without bound across runs.
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

func fatal(err error) {
	log.Printf("fatal: %v", err)
	fmt.Fprintf(os.Stderr, "%s could not start: %v\n", appName, err)
	os.Exit(1)
}
