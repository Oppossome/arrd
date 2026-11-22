// Package app wires arrd's subsystems together and runs them until the
// context is cancelled.
package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
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
	defer svc.StopRuntime()

	setupToken := ""
	if !svc.SetupComplete() {
		if setupToken, err = newSetupToken(); err != nil {
			return err
		}
		log.Warn("setup required: complete onboarding with this one-time token",
			"url", fmt.Sprintf("http://%s/setup?token=%s", opts.Listen, setupToken),
			"api", "POST /api/v1/setup with header X-Setup-Token")
	}

	api, err := httpapi.New(svc, setupToken, log, web.Dist())
	if err != nil {
		return err
	}
	srv := &http.Server{Addr: opts.Listen, Handler: api, ReadHeaderTimeout: 10 * time.Second}

	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		svc.RunScheduler(runCtx, library.DefaultSchedule)
	}()

	serveErr := make(chan error, 1)
	go func() {
		log.Info("http server listening", "addr", opts.Listen, "data_dir", opts.DataDir)
		serveErr <- srv.ListenAndServe()
	}()

	var runErr error
	select {
	case err := <-serveErr:
		runErr = fmt.Errorf("http server: %w", err)
	case <-ctx.Done():
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

func newSetupToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("generate setup token: %w", err)
	}
	return hex.EncodeToString(b), nil
}
