package torrent

import (
	"bytes"
	"context"
	"crypto/rand"
	"net/netip"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	atorrent "github.com/anacrolix/torrent"
	"github.com/anacrolix/torrent/bencode"
	"github.com/anacrolix/torrent/metainfo"
	"github.com/rs/zerolog"

	"generic-arr-project/internal/vpn/vpntest"
)

func quietLog() *zerolog.Logger { return new(zerolog.Nop()) }

// makeTorrent writes a random payload into dir and returns its metainfo.
func makeTorrent(t *testing.T, dir string, size int) (*metainfo.MetaInfo, []byte) {
	t.Helper()
	payload := make([]byte, size)
	if _, err := rand.Read(payload); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "payload.bin")
	if err := os.WriteFile(path, payload, 0o600); err != nil {
		t.Fatal(err)
	}
	info := metainfo.Info{PieceLength: 32 << 10}
	if err := info.BuildFromFilePath(path); err != nil {
		t.Fatal(err)
	}
	infoBytes, err := bencode.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	return &metainfo.MetaInfo{InfoBytes: infoBytes}, payload
}

// A full download between two engines whose only network is a WireGuard
// tunnel. The leecher learns about the seeder solely by its tunnel address.
func TestEngineDownloadsThroughTunnel(t *testing.T) {
	tunA, tunB := vpntest.Pair(t)

	seedDir, leechDir := t.TempDir(), t.TempDir()
	mi, payload := makeTorrent(t, seedDir, 1<<20)

	seeder, err := New(Config{StateDir: t.TempDir(), Network: tunB, Seed: true}, quietLog())
	if err != nil {
		t.Fatalf("seeder: %v", err)
	}
	defer seeder.Close()
	seedDL, err := seeder.AddMetaInfo(mi, seedDir)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := seedDL.WaitComplete(ctx); err != nil {
		t.Fatalf("seeder verifying data: %v", err)
	}

	leecher, err := New(Config{StateDir: t.TempDir(), Network: tunA}, quietLog())
	if err != nil {
		t.Fatalf("leecher: %v", err)
	}
	defer leecher.Close()
	dl, err := leecher.AddMetaInfo(mi, leechDir)
	if err != nil {
		t.Fatal(err)
	}
	seederAddr := netip.AddrPortFrom(vpntest.AddrB, uint16(seeder.ListenPort()))
	dl.t.AddPeers([]atorrent.PeerInfo{{Addr: atorrent.StringAddr(seederAddr.String())}})

	if err := dl.WaitComplete(ctx); err != nil {
		t.Fatalf("download: %v (progress %+v)", err, dl.Progress())
	}
	got, err := os.ReadFile(filepath.Join(leechDir, "payload.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatal("downloaded payload differs from the original")
	}
	if p := dl.Progress(); !p.Complete || p.Fraction() != 1 {
		t.Errorf("progress = %+v", p)
	}

	// The seeder must only have seen the leecher's tunnel address.
	for _, p := range seedDL.Peers() {
		if p.Addr != vpntest.AddrA {
			t.Errorf("seeder connected to %v, want only tunnel address %v", p.Addr, vpntest.AddrA)
		}
	}
}

// A paused download transfers nothing until resumed, and lists its peers
// once data flows.
func TestEnginePauseResume(t *testing.T) {
	tunA, tunB := vpntest.Pair(t)

	seedDir, leechDir := t.TempDir(), t.TempDir()
	mi, _ := makeTorrent(t, seedDir, 16<<20)

	seeder, err := New(Config{StateDir: t.TempDir(), Network: tunB, Seed: true}, quietLog())
	if err != nil {
		t.Fatalf("seeder: %v", err)
	}
	defer seeder.Close()
	seedDL, err := seeder.AddMetaInfo(mi, seedDir)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	if err := seedDL.WaitComplete(ctx); err != nil {
		t.Fatalf("seeder verifying data: %v", err)
	}

	leecher, err := New(Config{StateDir: t.TempDir(), Network: tunA}, quietLog())
	if err != nil {
		t.Fatalf("leecher: %v", err)
	}
	defer leecher.Close()
	dl, err := leecher.AddMetaInfo(mi, leechDir)
	if err != nil {
		t.Fatal(err)
	}
	dl.Pause()
	if !dl.Paused() || !dl.Progress().Paused {
		t.Fatal("download not reported as paused")
	}
	seederAddr := netip.AddrPortFrom(vpntest.AddrB, uint16(seeder.ListenPort()))
	dl.t.AddPeers([]atorrent.PeerInfo{{Addr: atorrent.StringAddr(seederAddr.String())}})

	time.Sleep(time.Second)
	if got := dl.Progress().BytesCompleted; got != 0 {
		t.Fatalf("paused download completed %d bytes", got)
	}

	dl.Resume()
	if dl.Paused() {
		t.Fatal("download still reported as paused")
	}

	// Once resumed, the peers connect and are listed on both sides while
	// data flows.
	var leechPeers, seedPeers []Peer
	for !dl.Complete() && ctx.Err() == nil {
		if lp, sp := dl.Peers(), seedDL.Peers(); len(lp) > 0 && lp[0].Progress == 1 && len(sp) > 0 {
			leechPeers, seedPeers = lp, sp
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if len(leechPeers) != 1 || leechPeers[0].Addr != vpntest.AddrB || leechPeers[0].Port != seederAddr.Port() {
		t.Fatalf("leecher peers = %+v, want the seeder at %v with progress 1", leechPeers, seederAddr)
	}
	if len(seedPeers) != 1 || seedPeers[0].Addr != vpntest.AddrA || seedPeers[0].Source != "incoming" {
		t.Fatalf("seeder peers = %+v, want one incoming peer at %v", seedPeers, vpntest.AddrA)
	}

	if err := dl.WaitComplete(ctx); err != nil {
		t.Fatalf("download after resume: %v (progress %+v)", err, dl.Progress())
	}
}

// The engine must not be able to reach peers outside its network.
func TestEngineCannotReachPeerOutsideTunnel(t *testing.T) {
	tunA, _ := vpntest.Pair(t)

	// A seeder on the plain host network.
	seedDir := t.TempDir()
	mi, _ := makeTorrent(t, seedDir, 64<<10)
	hostSeeder, err := New(Config{StateDir: t.TempDir(), Network: directNoV6{}, Seed: true}, quietLog())
	if err != nil {
		t.Fatal(err)
	}
	defer hostSeeder.Close()
	if _, err := hostSeeder.AddMetaInfo(mi, seedDir); err != nil {
		t.Fatal(err)
	}

	leecher, err := New(Config{StateDir: t.TempDir(), Network: tunA}, quietLog())
	if err != nil {
		t.Fatal(err)
	}
	defer leecher.Close()
	dl, err := leecher.AddMetaInfo(mi, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	dl.t.AddPeers([]atorrent.PeerInfo{{Addr: atorrent.StringAddr("127.0.0.1:" + strconv.Itoa(hostSeeder.ListenPort()))}})

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := dl.WaitComplete(ctx); err == nil {
		t.Fatal("tunnel-bound engine downloaded from a host-network peer")
	}
	if p := dl.Progress(); p.BytesCompleted != 0 || p.ActivePeers != 0 {
		t.Errorf("progress = %+v, want no data and no peers", p)
	}
}

// Multi-file torrent: file paths include the top-level folder, and after a
// remove + move + re-add the data is recognised as complete from the shared
// piece-completion store.
func TestEngineMoveAndReAdd(t *testing.T) {
	srcDir := t.TempDir()
	root := filepath.Join(srcDir, "Movie.2020.1080p")
	for name, size := range map[string]int{"movie.mkv": 300 << 10, "extras/a.nfo": 100} {
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		buf := make([]byte, size)
		_, _ = rand.Read(buf)
		if err := os.WriteFile(p, buf, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	info := metainfo.Info{PieceLength: 32 << 10}
	if err := info.BuildFromFilePath(root); err != nil {
		t.Fatal(err)
	}
	infoBytes, err := bencode.Marshal(info)
	if err != nil {
		t.Fatal(err)
	}
	mi := &metainfo.MetaInfo{InfoBytes: infoBytes}

	e, err := New(Config{StateDir: t.TempDir(), Network: directNoV6{}}, quietLog())
	if err != nil {
		t.Fatal(err)
	}
	defer e.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	dl, err := e.AddMetaInfo(mi, srcDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := dl.WaitComplete(ctx); err != nil {
		t.Fatal(err)
	}
	for _, f := range dl.Files() {
		if _, err := os.Stat(filepath.Join(dl.Dir(), f.Path)); err != nil {
			t.Errorf("file %q not under Dir: %v", f.Path, err)
		}
	}
	if _, err := e.AddMetaInfo(mi, srcDir); err == nil {
		t.Error("adding a duplicate torrent succeeded")
	}
	raw, err := dl.MetaInfo()
	if err != nil {
		t.Fatal(err)
	}

	if err := dl.Remove(ctx); err != nil {
		t.Fatal(err)
	}
	dstDir := t.TempDir()
	if err := os.Rename(root, filepath.Join(dstDir, dl.Name())); err != nil {
		t.Fatal(err)
	}
	moved, err := e.AddTorrentBytes(raw, dstDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := moved.WaitComplete(ctx); err != nil {
		t.Fatalf("re-added torrent not complete: %v", err)
	}
	got, ok := e.Get(moved.InfoHash())
	if !ok || got.Dir() != dstDir {
		t.Errorf("Get = %v, %v; want dir %s", got, ok, dstDir)
	}
}
