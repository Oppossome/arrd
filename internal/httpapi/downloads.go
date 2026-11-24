package httpapi

import (
	"context"

	"generic-arr-project/internal/httpapi/apigen"
	"generic-arr-project/internal/library/acquire"
	"generic-arr-project/internal/library/indexers"
	"generic-arr-project/internal/torrent"
)

func (h *handlers) ListQueue(ctx context.Context, req apigen.ListQueueRequestObject) (apigen.ListQueueResponseObject, error) {
	out, err := h.svc.Acquire.Queue(ctx, acquire.QueueFilter{MovieID: req.Params.MovieID, SeriesID: req.Params.SeriesID})
	if err != nil {
		return nil, err
	}
	return apigen.ListQueue200JSONResponse(mapSlice(out, toAPIDownload)), nil
}

func (h *handlers) RemoveQueueItem(ctx context.Context, req apigen.RemoveQueueItemRequestObject) (apigen.RemoveQueueItemResponseObject, error) {
	if err := h.svc.Acquire.RemoveDownload(ctx, req.ID, deref(req.Params.DeleteData)); err != nil {
		return nil, err
	}
	return apigen.RemoveQueueItem204Response{}, nil
}

func (h *handlers) RSSSync(ctx context.Context, _ apigen.RSSSyncRequestObject) (apigen.RSSSyncResponseObject, error) {
	out, err := h.svc.Acquire.RSSSync(ctx)
	if err != nil {
		return nil, err
	}
	return apigen.RSSSync200JSONResponse(toAPIRSSResult(out)), nil
}

func toAPIDownload(d acquire.Download) apigen.Download {
	return apigen.Download{
		ID:           d.ID,
		MovieID:      d.MovieID,
		SeriesID:     d.SeriesID,
		Title:        optional(d.Title),
		Year:         optional(d.Year),
		PosterPath:   d.PosterPath,
		BackdropPath: d.BackdropPath,
		Episodes:     optSlice(d.Episodes, identity[string]),
		IndexerID:    d.IndexerID,
		InfoHash:     d.InfoHash,
		ReleaseTitle: d.ReleaseTitle,
		Quality:      d.Quality,
		Size:         d.Size,
		Status:       apigen.DownloadStatus(d.Status),
		Paused:       d.Paused,
		Error:        d.Error,
		AddedAt:      d.AddedAt,
		CompletedAt:  d.CompletedAt,
		ImportedAt:   d.ImportedAt,
		Progress:     toAPIProgress(d.Progress, d.PeerCountries),
	}
}

func toAPIProgress(p *torrent.Progress, countries []acquire.CountryCount) *apigen.Progress {
	if p == nil {
		return nil
	}
	return &apigen.Progress{
		InfoHash:       p.InfoHash,
		Name:           p.Name,
		HaveInfo:       p.HaveInfo,
		BytesCompleted: p.BytesCompleted,
		BytesTotal:     p.BytesTotal,
		ActivePeers:    p.ActivePeers,
		TotalPeers:     p.TotalPeers,
		Seeders:        p.Seeders,
		Complete:       p.Complete,
		DownloadRate:   p.DownloadRate,
		UploadRate:     p.UploadRate,
		EtaSeconds:     int64(p.ETA.Seconds()),
		Countries:      mapSlice(countries, toAPICountryCount),
	}
}

func toAPICountryCount(c acquire.CountryCount) apigen.CountryCount {
	return apigen.CountryCount{Country: c.Country, Count: c.Count}
}

func toAPIRelease(r acquire.ReleaseResult) apigen.ReleaseResult {
	return apigen.ReleaseResult{
		Title:       r.Title,
		IndexerID:   r.IndexerID,
		Indexer:     r.Indexer,
		DownloadURL: optional(r.DownloadURL),
		MagnetURI:   optional(r.MagnetURI),
		InfoHash:    optional(r.InfoHash),
		Size:        r.Size,
		Seeders:     r.Seeders,
		Peers:       r.Peers,
		PublishDate: r.PublishDate,
		Quality:     r.Quality,
		Accepted:    r.Accepted,
		Rejections:  optSlice(r.Rejections, identity[string]),
		Score:       r.Score,
		Season:      r.Season,
		Episodes:    optSlice(r.Episodes, identity[int]),
		SeasonPack:  optional(r.SeasonPack),
	}
}

func fromAPIRelease(in apigen.ReleaseInput) acquire.ReleaseResult {
	return acquire.ReleaseResult{
		Title:       in.Title,
		IndexerID:   in.IndexerID,
		Indexer:     deref(in.Indexer),
		DownloadURL: deref(in.DownloadURL),
		MagnetURI:   deref(in.MagnetURI),
		InfoHash:    deref(in.InfoHash),
		Size:        deref(in.Size),
		Seeders:     deref(in.Seeders),
		Peers:       deref(in.Peers),
		PublishDate: deref(in.PublishDate),
		Quality:     deref(in.Quality),
		Accepted:    deref(in.Accepted),
		Rejections:  derefSlice(in.Rejections),
		Score:       deref(in.Score),
		Season:      in.Season,
		Episodes:    derefSlice(in.Episodes),
		SeasonPack:  deref(in.SeasonPack),
	}
}

func toAPIIndexerFailure(f indexers.IndexerFailure) apigen.IndexerFailure {
	return apigen.IndexerFailure{IndexerID: f.IndexerID, Indexer: f.Indexer, Error: f.Error}
}

func toAPISearchResult(r acquire.SearchResult) apigen.SearchResult {
	return apigen.SearchResult{
		Releases:        mapSlice(r.Releases, toAPIRelease),
		IndexerFailures: mapSlice(r.Failures, toAPIIndexerFailure),
	}
}

func toAPIAutoSearchResult(r acquire.AutoSearchResult) apigen.AutoSearchResult {
	out := apigen.AutoSearchResult{
		ReleasesConsidered: r.Searched,
		IndexerFailures:    mapSlice(r.Failures, toAPIIndexerFailure),
	}
	if r.Grabbed != nil {
		d := toAPIDownload(*r.Grabbed)
		out.Grabbed = &d
	}
	return out
}

func toAPIRSSResult(r acquire.RSSResult) apigen.RSSResult {
	return apigen.RSSResult{
		ReleasesSeen:    r.Releases,
		Grabbed:         mapSlice(r.Grabbed, toAPIDownload),
		IndexerFailures: mapSlice(r.Failures, toAPIIndexerFailure),
	}
}

func (h *handlers) PauseDownload(ctx context.Context, req apigen.PauseDownloadRequestObject) (apigen.PauseDownloadResponseObject, error) {
	if err := h.svc.Acquire.PauseDownload(ctx, req.ID); err != nil {
		return nil, err
	}
	return apigen.PauseDownload204Response{}, nil
}

func (h *handlers) ResumeDownload(ctx context.Context, req apigen.ResumeDownloadRequestObject) (apigen.ResumeDownloadResponseObject, error) {
	if err := h.svc.Acquire.ResumeDownload(ctx, req.ID); err != nil {
		return nil, err
	}
	return apigen.ResumeDownload204Response{}, nil
}

func (h *handlers) ListDownloadPeers(ctx context.Context, req apigen.ListDownloadPeersRequestObject) (apigen.ListDownloadPeersResponseObject, error) {
	peers, err := h.svc.Acquire.DownloadPeers(ctx, req.ID)
	if err != nil {
		return nil, err
	}
	return apigen.ListDownloadPeers200JSONResponse(mapSlice(peers, toAPIPeer)), nil
}

func (h *handlers) GetThroughput(ctx context.Context, _ apigen.GetThroughputRequestObject) (apigen.GetThroughputResponseObject, error) {
	t, err := h.svc.Acquire.Throughput(ctx)
	if err != nil {
		return nil, err
	}
	return apigen.GetThroughput200JSONResponse{
		DownloadRate:     t.Down,
		UploadRate:       t.Up,
		PeakDownloadRate: t.PeakDown,
		Active:           t.Active,
		History:          mapSlice(t.History, toAPIThroughputSample),
	}, nil
}

func toAPIThroughputSample(s torrent.Sample) apigen.ThroughputSample {
	return apigen.ThroughputSample{At: s.At.UTC(), DownloadRate: s.Down, UploadRate: s.Up}
}

func toAPIPeer(p acquire.Peer) apigen.Peer {
	return apigen.Peer{
		IP:           p.Addr.String(),
		Port:         int(p.Port),
		Country:      p.Country,
		Client:       p.Client,
		Source:       apigen.PeerSource(p.Source),
		DownloadRate: p.DownloadRate,
		UploadRate:   p.UploadRate,
		Progress:     p.Progress,
	}
}
