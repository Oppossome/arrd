// Command arrd is an all-in-one media manager for TV and movies with a
// built-in torrent engine that can be confined to a WireGuard tunnel.
//
// There is no config file: on first start arrd prints a one-time setup
// token, and everything else is configured through onboarding and the
// settings API. Only these bootstrap options are read from flags or the
// environment:
//
//	arrd [serve] [-data-dir DIR] [-listen ADDR] [-definitions-dir DIR] [-log-level LEVEL]
//	arrd fetch   [-data-dir DIR] [-out DIR] [-ip-echo URL] <magnet>
//
// Environment: ARRD_DATA_DIR, ARRD_LISTEN, ARRD_DEFINITIONS_DIR, ARRD_LOG_LEVEL.
//
// Settings → Network can serve the web UI on all interfaces at the -listen
// port and announce it over mDNS as arrd.local.
//
// Only one server may use a data directory at a time (DATA-DIR/arrd.lock).
// On Linux and Windows desktops serve shows a tray icon to open the web UI or
// quit; build with -tags notray for containers and other headless installs.
// Until onboarding is complete, the tray also opens the setup page in the
// browser at startup.
//
// Windows releases are built with -ldflags -H=windowsgui so no console window
// opens. Started from a terminal, arrd logs to it; otherwise it logs to
// DATA-DIR/arrd.log and shows a fatal error in a dialog.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/rs/zerolog"

	"generic-arr-project/internal/app"
	"generic-arr-project/internal/closer"
	"generic-arr-project/internal/console"
	"generic-arr-project/internal/library/vpnconfig"
)

func main() {
	hasStderr := console.AttachParent()
	if err := run(os.Args[1:], hasStderr); err != nil {
		if hasStderr {
			fmt.Fprintln(os.Stderr, "arrd:", err)
		} else {
			console.ShowError("arrd could not start or stopped unexpectedly:\n\n" + err.Error())
		}
		os.Exit(1)
	}
}

// run executes the command in args. Without a usable stderr (a Windows GUI
// build started outside a terminal) logs go to DATA-DIR/arrd.log.
func run(args []string, hasStderr bool) (err error) {
	cmd := "serve"
	if len(args) > 0 && (args[0] == "serve" || args[0] == "fetch") {
		cmd, args = args[0], args[1:]
	}

	defaultDataDir, err := defaultDataDir()
	if err != nil {
		return err
	}
	fs := flag.NewFlagSet("arrd "+cmd, flag.ContinueOnError)
	dataDir := fs.String("data-dir", envOr("ARRD_DATA_DIR", defaultDataDir), "directory for the database and torrent state (env ARRD_DATA_DIR)")
	logLevel := fs.String("log-level", envOr("ARRD_LOG_LEVEL", "info"), "debug, info, warn or error (env ARRD_LOG_LEVEL)")
	var listen, definitionsDir *string
	var fetchOpts app.FetchOptions
	switch cmd {
	case "fetch":
		fs.StringVar(&fetchOpts.OutDir, "out", "", "download directory (default: the saved complete directory)")
		fs.StringVar(&fetchOpts.IPEchoURL, "ip-echo", vpnconfig.DefaultIPEchoURL, "plain-text IP echo URL queried through the download network; empty to skip")
	default:
		listen = fs.String("listen", envOr("ARRD_LISTEN", "127.0.0.1:8484"), "HTTP listen address (env ARRD_LISTEN)")
		definitionsDir = fs.String("definitions-dir", os.Getenv("ARRD_DEFINITIONS_DIR"), "directory of Cardigann indexer definitions (*.yml) (default DATA-DIR/definitions; env ARRD_DEFINITIONS_DIR)")
	}
	if err := fs.Parse(args); err != nil {
		return err
	}

	level, err := parseLogLevel(*logLevel)
	if err != nil {
		return fmt.Errorf("-log-level: %w", err)
	}
	absData, err := filepath.Abs(*dataDir)
	if err != nil {
		return fmt.Errorf("-data-dir: %w", err)
	}
	logOut := os.Stderr
	if !hasStderr {
		if logOut, err = openLogFile(absData); err != nil {
			return err
		}
		defer closer.Join(&err, logOut)
	}
	log := newLogger(logOut, level)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	switch cmd {
	case "fetch":
		if fs.NArg() != 1 {
			return errors.New("usage: arrd fetch [flags] <magnet>")
		}
		fetchOpts.DataDir, fetchOpts.Magnet = absData, fs.Arg(0)
		return app.Fetch(ctx, log, fetchOpts, os.Stdout)
	default:
		if fs.NArg() != 0 {
			return fmt.Errorf("unexpected arguments: %v", fs.Args())
		}
		defs := *definitionsDir
		if defs == "" {
			defs = filepath.Join(absData, "definitions")
		}
		if defs, err = filepath.Abs(defs); err != nil {
			return fmt.Errorf("-definitions-dir: %w", err)
		}
		return app.Run(ctx, app.Options{DataDir: absData, Listen: *listen, DefinitionsDir: defs}, log)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// defaultDataDir is $XDG_DATA_HOME/arrd, or ~/.local/share/arrd.
func defaultDataDir() (string, error) {
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "arrd"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("find home directory (set -data-dir or ARRD_DATA_DIR): %w", err)
	}
	return filepath.Join(home, ".local", "share", "arrd"), nil
}

// parseLogLevel accepts the levels -log-level documents, in any case.
func parseLogLevel(s string) (zerolog.Level, error) {
	switch strings.ToLower(s) {
	case "debug":
		return zerolog.DebugLevel, nil
	case "info":
		return zerolog.InfoLevel, nil
	case "warn":
		return zerolog.WarnLevel, nil
	case "error":
		return zerolog.ErrorLevel, nil
	}
	return zerolog.NoLevel, fmt.Errorf("unknown level %q: want debug, info, warn or error", s)
}

// openLogFile opens DATA-DIR/arrd.log for appending, creating the (private)
// data directory if needed. Appending keeps the log of a running instance
// intact when a second start fails on the data-directory lock.
func openLogFile(dataDir string) (*os.File, error) {
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, fmt.Errorf("create data dir %q: %w", dataDir, err)
	}
	path := filepath.Join(dataDir, "arrd.log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open log file: %w", err)
	}
	return f, nil
}

// newLogger writes readable console lines to a terminal and JSON lines
// otherwise (systemd, containers, log files).
func newLogger(out *os.File, level zerolog.Level) *zerolog.Logger {
	var w io.Writer = out
	if info, err := out.Stat(); err == nil && info.Mode()&os.ModeCharDevice != 0 {
		w = zerolog.ConsoleWriter{Out: out, TimeFormat: time.DateTime}
	}
	return new(zerolog.New(w).Level(level).With().Timestamp().Logger())
}
