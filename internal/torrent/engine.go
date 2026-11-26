// Package torrent wraps anacrolix/torrent so that every socket it uses comes
// from a single netx.Network. When that network is a VPN tunnel, peer
// connections, DHT, uTP, trackers, webseeds and the DNS lookups for them all
// stay inside the tunnel.
package torrent

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/anacrolix/dht/v2"
	atorrent "github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/metainfo"
	"github.com/anacrolix/torrent/storage"
	"github.com/rs/zerolog"

	"generic-arr-project/internal/netx"
)

const dnsTimeout = 15 * time.Second

// Config configures an Engine.
type Config struct {
	// StateDir holds engine state (piece completion). Payloads go to the
	// directory given to each Add call.
	StateDir string
	// Network carries all of the engine's traffic.
	Network netx.Network
	// ListenPort for incoming peers; 0 picks a free port.
	ListenPort int
	// Seed keeps uploading after a download completes.
	Seed bool
}

// Engine is a running torrent client.
type Engine struct {
	cl        *atorrent.Client
	net       netx.Network
	log       *zerolog.Logger
	listeners []net.Listener
	pieces    storage.PieceCompletion

	rates       *rateTracker
	stopSampler chan struct{}
	samplerDone chan struct{}

	mu     sync.Mutex
	dirs   map[string]string // info hash -> download dir
	paused map[string]bool   // info hash -> paused
}

// New starts a torrent client bound to cfg.Network.
func New(cfg Config, log *zerolog.Logger) (*Engine, error) {
	if cfg.Network == nil {
		return nil, errors.New("torrent: Network is required")
	}
	if cfg.StateDir == "" {
		return nil, errors.New("torrent: StateDir is required")
	}
	n := cfg.Network
	logger := log.With().Str("component", "torrent").Str("network", n.Name()).Logger()
	log = &logger
	if err := os.MkdirAll(cfg.StateDir, 0o700); err != nil {
		return nil, fmt.Errorf("torrent state dir: %w", err)
	}
	// One piece-completion store shared by all torrents, so a torrent's
	// files can be moved and re-added without re-hashing.
	pieces, err := storage.NewDefaultPieceCompletionForDir(cfg.StateDir)
	if err != nil {
		return nil, fmt.Errorf("torrent piece completion: %w", err)
	}
	e := &Engine{
		net: n, log: log, pieces: pieces,
		dirs: map[string]string{}, paused: map[string]bool{},
		rates: newRateTracker(),
	}

	cc := atorrent.NewDefaultClientConfig()
	cc.DataDir = cfg.StateDir
	// Every torrent gets its own storage; the default must still share the
	// one piece-completion store (bolt allows a single opener per file).
	cc.DefaultStorage = storage.NewFileOpts(storage.NewFileClientOpts{
		ClientBaseDir:   filepath.Join(cfg.StateDir, "unassigned"),
		PieceCompletion: sharedPieceCompletion{pieces},
	})
	cc.Seed = cfg.Seed
	cc.Slogger = slog.New(zerolog.NewSlogHandler(*log)) // anacrolix logs through slog
	cc.ListenHost = func(string) string { return "" }
	cc.ListenPort = cfg.ListenPort
	cc.DisableIPv4 = !n.HasIPv4()
	cc.DisableIPv6 = !n.HasIPv6()

	// Paths that would open host sockets regardless of n are switched off:
	cc.NoDefaultPortForwarding = true // UPnP/NAT-PMP on the host LAN
	cc.LocalServiceDiscovery = nil    // LAN multicast
	cc.DisableWebtorrent = true       // WebRTC opens its own sockets
	cc.DisableTCP = true              // built-in TCP uses net.Listen; we add our own below

	// Everything else is pointed at n.
	cc.ListenPacket = n.ListenPacket // uTP + DHT share this UDP socket
	cc.TrackerListenPacket = n.ListenPacket
	cc.TrackerDialContext = n.DialContext
	cc.HTTPDialContext = n.DialContext // webseeds, metainfo sources
	cc.HTTPProxy = nil
	cc.DhtStartingNodes = func(network string) dht.StartingNodesGetter {
		return func() ([]dht.Addr, error) { return e.dhtBootstrapAddrs(network) }
	}
	// Metainfo fetched from "xs" sources may carry UDP tracker hostnames,
	// which anacrolix would resolve with the host resolver.
	cc.MetainfoSourcesMerger = func(t *atorrent.Torrent, mi *metainfo.MetaInfo) error {
		spec, err := atorrent.TorrentSpecFromMetaInfoErr(mi)
		if err != nil {
			return err
		}
		spec.Trackers = e.resolveUDPTrackers(spec.Trackers)
		return t.MergeSpec(spec)
	}

	// The UDP socket picks the port; TCP must then bind the same number. With
	// a random port that TCP port may be taken, so retry with a new one.
	attempts := 1
	if cfg.ListenPort == 0 {
		attempts = 10
	}
	for i := 0; ; i++ {
		cl, err := atorrent.NewClient(cc)
		if err != nil {
			return nil, errors.Join(fmt.Errorf("start torrent client: %w", err), pieces.Close())
		}
		e.cl = cl
		err = e.addTCP(cl.LocalPort())
		if err == nil {
			break
		}
		e.closeClient()
		if !errors.Is(err, syscall.EADDRINUSE) || i+1 >= attempts {
			return nil, errors.Join(err, pieces.Close())
		}
		log.Debug().Err(err).Msg("tcp port taken, retrying with another port")
	}
	e.stopSampler = make(chan struct{})
	e.samplerDone = make(chan struct{})
	go e.runSampler()
	log.Info().Int("port", e.cl.LocalPort()).Msg("torrent engine started")
	return e, nil
}

// addTCP registers TCP listeners and dialers from n for each address family.
func (e *Engine) addTCP(port int) error {
	families := []struct {
		network string
		enabled bool
	}{{"tcp4", e.net.HasIPv4()}, {"tcp6", e.net.HasIPv6()}}
	for _, f := range families {
		if !f.enabled {
			continue
		}
		l, err := e.net.Listen(f.network, net.JoinHostPort("", strconv.Itoa(port)))
		if err != nil {
			if f.network == "tcp6" {
				// Hosts without IPv6 are common; IPv4 alone is enough.
				e.log.Warn().Err(err).Msg("no IPv6 TCP listener")
				continue
			}
			return fmt.Errorf("torrent tcp listen: %w", err)
		}
		e.listeners = append(e.listeners, l)
		e.cl.AddListener(l)
		e.cl.AddDialer(atorrent.NetworkDialer{Network: f.network, Dialer: e.net})
	}
	if len(e.listeners) == 0 {
		return errors.New("torrent: no TCP listener could be opened")
	}
	return nil
}

// Close stops the client, its listeners and the piece-completion store.
func (e *Engine) Close() {
	if e.stopSampler != nil {
		close(e.stopSampler)
		<-e.samplerDone
		e.stopSampler = nil
	}
	e.closeClient()
	if err := e.pieces.Close(); err != nil {
		e.log.Warn().Err(err).Msg("torrent piece completion close")
	}
}

// closeClient stops the client and its listeners.
func (e *Engine) closeClient() {
	if e.cl != nil {
		for _, err := range e.cl.Close() {
			e.log.Warn().Err(err).Msg("torrent client close")
		}
		e.cl = nil
	}
	for _, l := range e.listeners {
		_ = l.Close()
	}
	e.listeners = nil
}

// runSampler reads every torrent's payload counters once per sampleInterval
// until Close.
func (e *Engine) runSampler() {
	defer close(e.samplerDone)
	tick := time.NewTicker(sampleInterval)
	defer tick.Stop()
	for {
		select {
		case <-e.stopSampler:
			return
		case now := <-tick.C:
			current := map[string]counters{}
			for _, t := range e.cl.Torrents() {
				s := t.Stats()
				current[t.InfoHash().HexString()] = counters{
					read:    s.BytesReadUsefulData.Int64(),
					written: s.BytesWrittenData.Int64(),
				}
			}
			e.rates.sample(now, current)
		}
	}
}

// Throughput returns the engine-wide transfer rate and its recent history.
func (e *Engine) Throughput() Throughput { return e.rates.throughput() }

// ListenPort is the port peers can reach the engine on (inside n).
func (e *Engine) ListenPort() int { return e.cl.LocalPort() }

// AddMagnet adds a magnet link, saving its data under dir. All files are
// downloaded once metadata arrives.
func (e *Engine) AddMagnet(uri, dir string) (*Download, error) {
	spec, err := atorrent.TorrentSpecFromMagnetUri(uri)
	if err != nil {
		return nil, fmt.Errorf("parse magnet: %w", err)
	}
	return e.addSpec(spec, dir)
}

// AddMetaInfo adds a parsed .torrent file, saving its data under dir.
func (e *Engine) AddMetaInfo(mi *metainfo.MetaInfo, dir string) (*Download, error) {
	spec, err := atorrent.TorrentSpecFromMetaInfoErr(mi)
	if err != nil {
		return nil, fmt.Errorf("torrent metainfo: %w", err)
	}
	return e.addSpec(spec, dir)
}

// AddTorrentBytes adds a bencoded .torrent file, saving its data under dir.
func (e *Engine) AddTorrentBytes(b []byte, dir string) (*Download, error) {
	mi, err := metainfo.Load(bytes.NewReader(b))
	if err != nil {
		return nil, fmt.Errorf("parse .torrent: %w", err)
	}
	return e.AddMetaInfo(mi, dir)
}

func (e *Engine) addSpec(spec *atorrent.TorrentSpec, dir string) (*Download, error) {
	if dir == "" {
		return nil, errors.New("add torrent: download directory is required")
	}
	if existing, ok := e.cl.Torrent(spec.InfoHash); ok {
		return nil, fmt.Errorf("add torrent: %s is already in the engine (%s)", spec.InfoHash.HexString(), existing.Name())
	}
	spec.Trackers = e.resolveUDPTrackers(spec.Trackers)
	spec.Storage = storage.NewFileOpts(storage.NewFileClientOpts{
		ClientBaseDir:   dir,
		PieceCompletion: sharedPieceCompletion{e.pieces},
	})
	t, _, err := e.cl.AddTorrentSpec(spec)
	if err != nil {
		return nil, fmt.Errorf("add torrent: %w", err)
	}
	e.mu.Lock()
	e.dirs[t.InfoHash().HexString()] = dir
	e.mu.Unlock()
	d := &Download{t: t, e: e, dir: dir}
	go func() {
		select {
		case <-t.GotInfo():
			t.DownloadAll()
		case <-t.Closed():
		}
	}()
	return d, nil
}

// Get returns the download with the given hex info hash.
func (e *Engine) Get(infoHash string) (*Download, bool) {
	var h metainfo.Hash
	if err := h.FromHexString(infoHash); err != nil {
		return nil, false
	}
	t, ok := e.cl.Torrent(h)
	if !ok {
		return nil, false
	}
	e.mu.Lock()
	dir := e.dirs[infoHash]
	e.mu.Unlock()
	return &Download{t: t, e: e, dir: dir}, true
}

// sharedPieceCompletion stops per-torrent storage from closing the store
// all torrents share; Engine.Close closes it.
type sharedPieceCompletion struct{ storage.PieceCompletion }

func (sharedPieceCompletion) Close() error { return nil }

// dhtBootstrapAddrs resolves the public DHT routers through n instead of the
// dht package's host resolver.
func (e *Engine) dhtBootstrapAddrs(network string) ([]dht.Addr, error) {
	ctx, cancel := context.WithTimeout(context.Background(), dnsTimeout)
	defer cancel()
	var addrs []dht.Addr
	var errs []error
	for _, hp := range dht.DefaultGlobalBootstrapHostPorts {
		host, port, err := net.SplitHostPort(hp)
		if err != nil {
			return nil, fmt.Errorf("dht bootstrap %q: %w", hp, err)
		}
		ips, err := e.net.LookupHost(ctx, host)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		for _, ip := range ips {
			ua, err := net.ResolveUDPAddr(network, net.JoinHostPort(ip, port)) // IP literal: no DNS
			if err != nil {
				continue // wrong family for this socket
			}
			addrs = append(addrs, dht.NewAddr(ua))
		}
	}
	if len(addrs) == 0 {
		return nil, fmt.Errorf("dht bootstrap: nothing resolved: %w", errors.Join(errs...))
	}
	return addrs, nil
}
