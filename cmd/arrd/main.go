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
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"generic-arr-project/internal/app"
	"generic-arr-project/internal/library/vpnconfig"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "arrd:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
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
		definitionsDir = fs.String("definitions-dir", os.Getenv("ARRD_DEFINITIONS_DIR"), "directory of Cardigann indexer definitions (*.yml), e.g. a checkout of Prowlarr's definitions/v11 (default DATA-DIR/definitions; env ARRD_DEFINITIONS_DIR)")
	}
	if err := fs.Parse(args); err != nil {
		return err
	}

	var level slog.Level
	if err := level.UnmarshalText([]byte(*logLevel)); err != nil {
		return fmt.Errorf("-log-level: %w", err)
	}
	log := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level}))
	absData, err := filepath.Abs(*dataDir)
	if err != nil {
		return fmt.Errorf("-data-dir: %w", err)
	}

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
