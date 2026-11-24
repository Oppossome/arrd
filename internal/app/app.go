// Package app wires arrd's subsystems together and runs them until the
// context is cancelled.
package app

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"generic-arr-project/internal/httpapi"
	"generic-arr-project/internal/library"
	"generic-arr-project/internal/store"
	"generic-arr-project/internal/tray"
	"generic-arr-project/web"
)

const shutdownTimeout = 10 * time.Second

// Options are the few settings needed before the database is open. All
// other configuration lives in the database and is set through onboarding
// and the settings API.
type Options struct {
	DataDir string // database, torrent state and suggested default paths
	Listen  string // HTTP listen address
	// DefinitionsDir holds Cardigann indexer definitions (*.yml), e.g. a
	// checkout of Prowlarr's definitions/v11. Missing means none.
	DefinitionsDir string
}

// openStore creates the data directory (private: the database holds API
// keys, the VPN private key or PIA credentials) and opens the database.
func openStore(ctx context.Context, dataDir string) (*store.Store, error) {
	if dataDir == "" {
		return nil, fmt.Errorf("data directory is required")
	}
	if err := os.MkdirAll(dataDir, 0o700); err != nil {
		return nil, fmt.Errorf("create data dir %q: %w", dataDir, err)
	}
	path := filepath.Join(dataDir, "arrd.db")
	st, err := store.Open(ctx, path)
	if err != nil {
		return nil, err
	}
	if err := os.Chmod(path, 0o600); err != nil {
		st.Close()
		return nil, fmt.Errorf("restrict database permissions: %w", err)
	}
	return st, nil
}

// Run starts arrd and blocks until ctx is cancelled or the HTTP server fails.
func Run(ctx context.Context, opts Options, log *slog.Logger) error {
	release, err := lockDataDir(opts.DataDir)
	if err != nil {
		return err
	}
	defer release()

	st, err := openStore(ctx, opts.DataDir)
	if err != nil {
		return err
	}
	defer st.Close()

	svc, err := library.New(library.Deps{
		Store:          st,
		NewRuntime:     newRuntimeFactory(filepath.Join(opts.DataDir, "torrent-state"), log),
		DataDir:        opts.DataDir,
		DefinitionsDir: opts.DefinitionsDir,
		Log:            log,
	})
	if err != nil {
		return err
	}
	if err := svc.Start(ctx); err != nil {
		return err
	}
	defer svc.Stop()

	setupToken := ""
	openURL := tray.BrowseURL(opts.Listen)
	if !svc.Auth.SetupComplete() {
		if setupToken, err = newSetupToken(); err != nil {
			return err
		}
		openURL = fmt.Sprintf("%s/setup?token=%s", openURL, setupToken)
		log.Warn("setup required: complete onboarding with this one-time token",
			"url", openURL,
			"api", "POST /api/v1/setup with header X-Setup-Token")
	}

	api, err := httpapi.New(svc, setupToken, log, web.Dist())
	if err != nil {
		return err
	}
	srv := &http.Server{Handler: api, ReadHeaderTimeout: 10 * time.Second}
	listener := newExposer(srv, opts.Listen, startMDNSResponder(log), log)
	defer listener.Close()
	if err := startListener(ctx, st, listener, log); err != nil {
		return err
	}
	svc.Settings.SetNetworkApplier(listener)

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		svc.RunScheduler(runCtx, library.DefaultSchedule)
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		tray.Run(runCtx, tray.Options{URL: openURL, Quit: cancel, Log: log})
	}()

	var runErr error
	select {
	case err := <-listener.Failed():
		runErr = fmt.Errorf("http server: %w", err)
	case <-runCtx.Done(): // signal, or Quit from the tray
		log.Info("shutting down")
	}
	cancel()
	shutdownCtx, stop := context.WithTimeout(context.Background(), shutdownTimeout)
	defer stop()
	if err := srv.Shutdown(shutdownCtx); err != nil && runErr == nil {
		runErr = fmt.Errorf("http shutdown: %w", err)
	}
	wg.Wait()
	return runErr
}

// startListener binds the HTTP listener per the saved LAN setting. If LAN
// access cannot start (port taken, no multicast) arrd still starts on
// -listen, so a network problem cannot lock the user out.
func startListener(ctx context.Context, st *store.Store, listener *exposer, log *slog.Logger) error {
	exposeLAN, err := savedExposeLAN(ctx, st)
	if err != nil {
		return err
	}
	if exposeLAN {
		err := listener.ApplyLAN(true)
		if err == nil {
			return nil
		}
		log.Warn("LAN access unavailable; serving on the listen address only", "err", err)
	}
	return listener.ApplyLAN(false)
}

// savedExposeLAN is the saved LAN setting; off before onboarding.
func savedExposeLAN(ctx context.Context, st *store.Store) (bool, error) {
	app, err := st.GetAppSettings(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("load settings: %w", err)
	}
	return app.ExposeLAN, nil
}

func newSetupToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate setup token: %w", err)
	}
	return hex.EncodeToString(b), nil
}
