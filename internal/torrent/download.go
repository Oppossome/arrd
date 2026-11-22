package torrent

import (
	"bytes"
	"context"
	"fmt"
	"math"
	"net/netip"
	"path/filepath"
	"time"

	atorrent "github.com/anacrolix/torrent"
)

// Download is one torrent in the engine.
type Download struct {
	t   *atorrent.Torrent
	e   *Engine
	dir string
}

// Progress is a point-in-time snapshot of a Download.
type Progress struct {
	InfoHash       string `json:"info_hash"`
	Name           string `json:"name"`
	HaveInfo       bool   `json:"have_info"`
	BytesCompleted int64  `json:"bytes_completed"`
	BytesTotal     int64  `json:"bytes_total"` // 0 until metadata is known
	ActivePeers    int    `json:"active_peers"`
	TotalPeers     int    `json:"total_peers"`
	Seeders        int    `json:"seeders"`
	Complete       bool   `json:"complete"`
	Paused         bool   `json:"paused"`
	// DownloadRate and UploadRate are smoothed payload rates in bytes/s.
	DownloadRate float64 `json:"download_rate"`
	UploadRate   float64 `json:"upload_rate"`
	// ETA is the estimated time left; 0 when unknown or complete.
	ETA time.Duration `json:"eta"`
}

// Peer is one connected peer of a download.
type Peer struct {
	Addr netip.Addr
	Port uint16
	// Client is the peer's self-reported client name, or its peer-ID
	// prefix; empty if neither is known.
	Client string
	// Source says how the peer was found: tracker, dht, pex, incoming,
	// lpd, holepunch, direct, or "" if unknown.
	Source       string
	DownloadRate float64 // bytes/s we receive from the peer
	UploadRate   float64 // bytes/s we send to the peer
	// Progress is the fraction of pieces the peer has, in [0,1]; 0 before
	// metadata is known.
	Progress float64
}

// Fraction returns completion in [0,1], or 0 before metadata is known.
func (p Progress) Fraction() float64 {
	if p.BytesTotal == 0 {
		return 0
	}
	return float64(p.BytesCompleted) / float64(p.BytesTotal)
}

// File is one file of a download.
type File struct {
	// Path is relative to Dir and includes the torrent's top-level folder
	// for multi-file torrents.
	Path string
	Size int64
}

// InfoHash returns the hex info hash.
func (d *Download) InfoHash() string { return d.t.InfoHash().HexString() }

// Dir is the directory the download's data is stored under.
func (d *Download) Dir() string { return d.dir }

// HaveInfo reports whether metadata is known.
func (d *Download) HaveInfo() bool {
	select {
	case <-d.t.GotInfo():
		return true
	default:
		return false
	}
}

// Name is the torrent's top-level file or folder name; it may be a
// display name until metadata arrives.
func (d *Download) Name() string { return d.t.Name() }

// Files lists the download's files. It returns nil before metadata is known.
func (d *Download) Files() []File {
	if !d.HaveInfo() {
		return nil
	}
	tf := d.t.Files()
	out := make([]File, len(tf))
	for i, f := range tf {
		out[i] = File{Path: filepath.FromSlash(f.Path()), Size: f.Length()}
	}
	return out
}

// MetaInfo returns the bencoded .torrent once metadata is known.
func (d *Download) MetaInfo() ([]byte, error) {
	if !d.HaveInfo() {
		return nil, fmt.Errorf("torrent %s: metadata not yet known", d.InfoHash())
	}
	mi := d.t.Metainfo()
	var buf bytes.Buffer
	if err := mi.Write(&buf); err != nil {
		return nil, fmt.Errorf("encode metainfo: %w", err)
	}
	return buf.Bytes(), nil
}

// Progress returns the current state.
func (d *Download) Progress() Progress {
	p := Progress{InfoHash: d.InfoHash(), Name: d.t.Name()}
	if d.HaveInfo() {
		p.HaveInfo = true
		p.BytesTotal = d.t.Length()
		p.BytesCompleted = d.t.BytesCompleted()
		p.Complete = d.t.Complete().Bool()
	}
	s := d.t.Stats()
	p.ActivePeers, p.TotalPeers, p.Seeders = s.ActivePeers, s.TotalPeers, s.ConnectedSeeders
	p.Paused = d.Paused()
	r := d.e.rates.rate(p.InfoHash)
	p.DownloadRate, p.UploadRate = r.Down, r.Up
	p.ETA = eta(p)
	return p
}

// minETARate is the slowest download rate (bytes/s) an ETA is shown for.
const minETARate = 1

// eta estimates the time left from the remaining bytes and current rate.
func eta(p Progress) time.Duration {
	if !p.HaveInfo || p.Complete || p.Paused || p.DownloadRate < minETARate {
		return 0
	}
	secs := float64(p.BytesTotal-p.BytesCompleted) / p.DownloadRate
	return time.Duration(math.Ceil(secs)) * time.Second
}

// Peers lists the currently connected peers.
func (d *Download) Peers() []Peer {
	var pieces int
	if d.HaveInfo() {
		pieces = d.t.NumPieces()
	}
	conns := d.t.PeerConns()
	out := make([]Peer, 0, len(conns))
	for _, c := range conns {
		ap, err := netip.ParseAddrPort(c.RemoteAddr.String())
		if err != nil {
			// Non-IP transports (e.g. WebRTC) have no address to show.
			continue
		}
		s := c.Stats()
		p := Peer{
			Addr:         ap.Addr().Unmap(),
			Port:         ap.Port(),
			Client:       peerClient(c),
			Source:       peerSource(c.Discovery),
			DownloadRate: s.DownloadRate,
			UploadRate:   s.LastWriteUploadRate,
		}
		if pieces > 0 {
			p.Progress = min(1, float64(s.RemotePieceCount)/float64(pieces))
		}
		out = append(out, p)
	}
	return out
}

// peerClient returns the extended-handshake client name, falling back to
// the Azureus-style peer-ID prefix ("-qB4650-" -> "qB4650").
func peerClient(c *atorrent.PeerConn) string {
	if v, ok := c.PeerClientName.Load().(string); ok && v != "" {
		return v
	}
	id := c.PeerID
	if id[0] == '-' && id[7] == '-' {
		return string(id[1:7])
	}
	return ""
}

func peerSource(s atorrent.PeerSource) string {
	switch s {
	case atorrent.PeerSourceTracker:
		return "tracker"
	case atorrent.PeerSourceIncoming:
		return "incoming"
	case atorrent.PeerSourceDhtGetPeers, atorrent.PeerSourceDhtAnnouncePeer:
		return "dht"
	case atorrent.PeerSourcePex:
		return "pex"
	case atorrent.PeerSourceLPD:
		return "lpd"
	case atorrent.PeerSourceUtHolepunch:
		return "holepunch"
	case atorrent.PeerSourceDirect:
		return "direct"
	default:
		return ""
	}
}

// Pause stops all data transfer for the download. A paused download
// neither needs data nor seeds, so the engine also stops opening and
// accepting peer connections for it.
func (d *Download) Pause() {
	d.t.DisallowDataDownload()
	d.t.DisallowDataUpload()
	d.e.mu.Lock()
	d.e.paused[d.InfoHash()] = true
	d.e.mu.Unlock()
}

// Resume restarts data transfer after Pause.
func (d *Download) Resume() {
	d.e.mu.Lock()
	delete(d.e.paused, d.InfoHash())
	d.e.mu.Unlock()
	d.t.AllowDataDownload()
	d.t.AllowDataUpload()
}

// Paused reports whether Pause was called without a later Resume.
func (d *Download) Paused() bool {
	d.e.mu.Lock()
	defer d.e.mu.Unlock()
	return d.e.paused[d.InfoHash()]
}

// Complete reports whether all data is downloaded and verified.
func (d *Download) Complete() bool { return d.HaveInfo() && d.t.Complete().Bool() }

// WaitComplete blocks until all data is downloaded and verified.
func (d *Download) WaitComplete(ctx context.Context) error {
	select {
	case <-d.t.GotInfo():
	case <-ctx.Done():
		return fmt.Errorf("waiting for metadata: %w", context.Cause(ctx))
	}
	select {
	case <-d.t.Complete().On():
		return nil
	case <-d.t.Closed():
		return fmt.Errorf("torrent %s was removed", d.InfoHash())
	case <-ctx.Done():
		return fmt.Errorf("waiting for download: %w", context.Cause(ctx))
	}
}

// Remove drops the torrent from the engine and waits until its files are
// closed. Downloaded data is kept on disk.
func (d *Download) Remove(ctx context.Context) error {
	d.t.Drop()
	d.e.mu.Lock()
	delete(d.e.dirs, d.InfoHash())
	delete(d.e.paused, d.InfoHash())
	d.e.mu.Unlock()
	select {
	case <-d.t.Closed():
		return nil
	case <-ctx.Done():
		return fmt.Errorf("waiting for torrent %s to close: %w", d.InfoHash(), context.Cause(ctx))
	}
}
