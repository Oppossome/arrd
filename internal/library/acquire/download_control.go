package acquire

import (
	"context"
	"fmt"
	"net/netip"
	"slices"

	"generic-arr-project/internal/library/apperr"
	"generic-arr-project/internal/library/engine"
	"generic-arr-project/internal/library/media"
	"generic-arr-project/internal/store/db"
	"generic-arr-project/internal/torrent"
)

// PauseDownload stops a download's transfers. The pause is saved, so it
// survives restarts. Only downloads that are still downloading can be
// paused.
func (s *Service) PauseDownload(ctx context.Context, id int64) error {
	return s.setPaused(ctx, id, true)
}

// ResumeDownload restarts a paused download.
func (s *Service) ResumeDownload(ctx context.Context, id int64) error {
	return s.setPaused(ctx, id, false)
}

func (s *Service) setPaused(ctx context.Context, id int64, paused bool) error {
	row, err := s.store.GetDownload(ctx, id)
	if err != nil {
		return apperr.NotFound(err, "download", id)
	}
	if row.Status != media.DownloadDownloading {
		return apperr.Conflict("download %d is %s; only downloading items can be paused or resumed", id, row.Status)
	}
	// Hold the runtime so the engine is not swapped between saving the
	// flag and applying it; a later engine applies it on restore.
	s.engine.Inspect(func(rt *engine.Runtime, _ error) { err = s.applyPaused(ctx, rt, row, paused) })
	return err
}

func (s *Service) applyPaused(ctx context.Context, rt *engine.Runtime, row db.Download, paused bool) error {
	if err := s.store.SetDownloadPaused(ctx, db.SetDownloadPausedParams{ID: row.ID, Paused: paused}); err != nil {
		return fmt.Errorf("save paused state: %w", err)
	}
	if rt == nil {
		return nil
	}
	if dl, ok := rt.Engine.Get(row.InfoHash); ok {
		if paused {
			dl.Pause()
		} else {
			dl.Resume()
		}
	}
	return nil
}

// DownloadPeers lists the peers a download is connected to, fastest first.
// It is empty when the download is not running (finished and not
// seeding, or paused).
func (s *Service) DownloadPeers(ctx context.Context, id int64) ([]Peer, error) {
	row, err := s.store.GetDownload(ctx, id)
	if err != nil {
		return nil, apperr.NotFound(err, "download", id)
	}
	var peers []Peer
	err = s.engine.WithEngine(func(eng *torrent.Engine) error {
		if dl, ok := eng.Get(row.InfoHash); ok {
			peers = s.locatePeers(dl.Peers())
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	slices.SortFunc(peers, func(a, b Peer) int {
		if a.DownloadRate != b.DownloadRate {
			if a.DownloadRate > b.DownloadRate {
				return -1
			}
			return 1
		}
		return netip.AddrPortFrom(a.Addr, a.Port).Compare(netip.AddrPortFrom(b.Addr, b.Port))
	})
	return peers, nil
}

// Throughput is the engine-wide transfer rate with its recent history.
type Throughput struct {
	torrent.Throughput
	// Active counts downloads still downloading, paused ones included.
	Active int
}

// Throughput returns current transfer rates; zero while the engine is not
// running.
func (s *Service) Throughput(ctx context.Context) (Throughput, error) {
	n, err := s.store.CountActiveDownloads(ctx)
	if err != nil {
		return Throughput{}, fmt.Errorf("count active downloads: %w", err)
	}
	t := Throughput{Active: int(n)}
	if eng := s.engine.CurrentEngine(); eng != nil {
		t.Throughput = eng.Throughput()
	}
	return t, nil
}
