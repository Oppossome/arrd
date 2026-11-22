package app

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"generic-arr-project/internal/library"
	"generic-arr-project/internal/netx"
	"generic-arr-project/internal/store"
	"generic-arr-project/internal/torrent"
	"generic-arr-project/internal/vpn/provider"
)

// FetchOptions configures a one-off download.
type FetchOptions struct {
	DataDir string
	Magnet  string
	// OutDir overrides the saved complete directory.
	OutDir string
	// IPEchoURL is queried through the download network to show the public
	// address peers will see. Empty skips the check.
	IPEchoURL string
}

const progressInterval = 2 * time.Second

// fetchTunnel connects the saved VPN for a one-off download, or returns nil
// when none is configured. It connects without a
// forwarded port: a running "arrd serve" may own it, and a short fetch
// does not need incoming peers. Providers that renew credentials register
// a separate key for this tunnel.
func fetchTunnel(ctx context.Context, st *store.Store, log *slog.Logger) (*provider.Connection, error) {
	reg, err := library.DefaultVPNProviders(netx.Direct{})
	if err != nil {
		return nil, err
	}
	p, err := library.LoadVPNProvider(ctx, st, reg)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, nil
	}
	return p.Connect(ctx, provider.ConnectOptions{}, log)
}

// Fetch downloads a single magnet link through the saved VPN (never falling
// back to direct when one is configured), or direct without one, printing the
// network's public IP and progress to out. Onboarding must be complete.
func Fetch(ctx context.Context, log *slog.Logger, opts FetchOptions, out io.Writer) error {
	st, err := openStore(ctx, opts.DataDir)
	if err != nil {
		return err
	}
	defer st.Close()
	_, err = st.GetAppSettings(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return errors.New("setup is not complete: run arrd serve and finish onboarding first")
	}
	if err != nil {
		return fmt.Errorf("load settings: %w", err)
	}
	settings, err := st.GetDownloadSettings(ctx)
	if err != nil {
		return fmt.Errorf("load download settings: %w", err)
	}

	outDir := opts.OutDir
	if outDir == "" {
		outDir = settings.CompleteDir
	}
	if err := os.MkdirAll(outDir, 0o750); err != nil {
		return fmt.Errorf("create output dir %q: %w", outDir, err)
	}

	var n netx.Network = netx.Direct{}
	conn, err := fetchTunnel(ctx, st, log)
	if err != nil {
		return err
	}
	if conn != nil {
		defer conn.Close()
		n = conn.Tunnel
	}

	fmt.Fprintf(out, "network: %s\n", n.Name())
	if opts.IPEchoURL != "" {
		ip, err := netx.PublicIP(ctx, n, opts.IPEchoURL)
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "public ip: %s\n", ip)
	}

	// A separate state dir: a running "arrd serve" holds the main one.
	engine, err := torrent.New(torrent.Config{StateDir: filepath.Join(opts.DataDir, "torrent-state-fetch"), Network: n}, log)
	if err != nil {
		return err
	}
	defer engine.Close()

	dl, err := engine.AddMagnet(opts.Magnet, outDir)
	if err != nil {
		return err
	}
	fmt.Fprintf(out, "added %s, saving to %s\n", dl.InfoHash(), outDir)

	done := make(chan error, 1)
	go func() { done <- dl.WaitComplete(ctx) }()
	ticker := time.NewTicker(progressInterval)
	defer ticker.Stop()
	for {
		select {
		case err := <-done:
			if err != nil {
				return err
			}
			p := dl.Progress()
			fmt.Fprintf(out, "complete: %s (%s)\n", p.Name, formatBytes(p.BytesTotal))
			return nil
		case <-ticker.C:
			printProgress(out, dl.Progress())
		}
	}
}

func printProgress(out io.Writer, p torrent.Progress) {
	if !p.HaveInfo {
		fmt.Fprintf(out, "fetching metadata... peers %d/%d\n", p.ActivePeers, p.TotalPeers)
		return
	}
	fmt.Fprintf(out, "%5.1f%%  %s / %s  peers %d (seeders %d)  %s\n",
		100*p.Fraction(), formatBytes(p.BytesCompleted), formatBytes(p.BytesTotal),
		p.ActivePeers, p.Seeders, p.Name)
}

func formatBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}
