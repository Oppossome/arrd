package library

import (
	"context"
	"time"
)

// Schedule controls how often background jobs run.
type Schedule struct {
	ProcessDownloads time.Duration // progress, completion, import
	RetryRuntime     time.Duration // restart a failed engine/VPN
	CheckVPN         time.Duration // keep a PIA tunnel and forwarded port alive
	PurgeSessions    time.Duration // delete expired login sessions
	RSSSync          time.Duration // grab new releases from indexer feeds
	RefreshSeries    time.Duration // pick up new episodes and air dates
	WantedSearch     time.Duration // active search for missing movies and episodes
	UpdateGeoIP      time.Duration // refresh the peer geolocation database; 0 disables
	// FirstRun delays the first RSS, refresh and wanted runs after startup
	// so indexers and the VPN have time to settle.
	FirstRun time.Duration
}

// DefaultSchedule is used by arrd serve.
var DefaultSchedule = Schedule{
	ProcessDownloads: 5 * time.Second,
	RetryRuntime:     time.Minute,
	CheckVPN:         30 * time.Second,
	PurgeSessions:    time.Hour,
	RSSSync:          15 * time.Minute,
	RefreshSeries:    12 * time.Hour,
	WantedSearch:     6 * time.Hour,
	UpdateGeoIP:      24 * time.Hour,
	FirstRun:         2 * time.Minute,
}

// RunScheduler runs background jobs until ctx is cancelled. Jobs run one at
// a time, so a slow search never overlaps with an import.
func (s *Services) RunScheduler(ctx context.Context, sch Schedule) {
	process := time.NewTicker(sch.ProcessDownloads)
	defer process.Stop()
	retry := time.NewTicker(sch.RetryRuntime)
	defer retry.Stop()
	checkVPN := time.NewTicker(sch.CheckVPN)
	defer checkVPN.Stop()
	purge := time.NewTicker(sch.PurgeSessions)
	defer purge.Stop()
	rss := time.NewTimer(sch.FirstRun)
	defer rss.Stop()
	// Refresh before the wanted search so it sees newly aired episodes.
	refresh := time.NewTimer(sch.FirstRun)
	defer refresh.Stop()
	wanted := time.NewTimer(sch.FirstRun + time.Minute)
	defer wanted.Stop()
	if sch.UpdateGeoIP > 0 {
		// Its own loop: the first download must not hold up imports.
		go s.runGeoIPUpdates(ctx, sch.UpdateGeoIP)
	}

	logErr := func(job string, err error) {
		if err != nil && ctx.Err() == nil {
			s.log.Error(job, "err", err)
		}
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-process.C:
			logErr("process downloads", s.Acquire.ProcessDownloads(ctx))
		case <-retry.C:
			s.retryRuntime(ctx)
		case <-checkVPN.C:
			logErr("check vpn", s.VPN.CheckVPN(ctx))
		case <-purge.C:
			logErr("purge sessions", s.Auth.PurgeExpiredSessions(ctx))
		case <-rss.C:
			_, err := s.Acquire.RSSSync(ctx)
			logErr("rss sync", err)
			rss.Reset(sch.RSSSync)
		case <-refresh.C:
			s.log.Info("refreshing series metadata")
			logErr("refresh series", s.Media.RefreshAllSeries(ctx))
			logErr("backfill backdrops", s.Media.BackfillBackdrops(ctx))
			refresh.Reset(sch.RefreshSeries)
		case <-wanted.C:
			s.log.Info("searching for wanted movies and episodes")
			logErr("wanted movie search", s.Acquire.SearchWantedMovies(ctx))
			logErr("wanted episode search", s.Acquire.SearchWantedEpisodes(ctx, nil))
			wanted.Reset(sch.WantedSearch)
		}
	}
}

// runGeoIPUpdates keeps the peer geolocation database current: it is
// fetched right away (peers are listed without countries until it is),
// then checked every interval until ctx is cancelled.
func (s *Services) runGeoIPUpdates(ctx context.Context, interval time.Duration) {
	for {
		if err := s.geo.Update(ctx); err != nil && ctx.Err() == nil {
			s.log.Error("update ip geolocation database", "err", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(interval):
		}
	}
}
