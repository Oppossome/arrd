package geoip

import (
	"bytes"
	"compress/gzip"
	"context"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/netip"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/maxmind/mmdbwriter"
	"github.com/maxmind/mmdbwriter/mmdbtype"
)

func quietLog() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// fixtureDB builds a gzipped country database mapping documentation
// ranges to countries.
func fixtureDB(t *testing.T, networks map[string]string) []byte {
	t.Helper()
	tree, err := mmdbwriter.New(mmdbwriter.Options{
		DatabaseType:            "DBIP-Country-Lite",
		IncludeReservedNetworks: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	for cidr, iso := range networks {
		_, n, err := net.ParseCIDR(cidr)
		if err != nil {
			t.Fatal(err)
		}
		rec := mmdbtype.Map{"country": mmdbtype.Map{"iso_code": mmdbtype.String(iso)}}
		if err := tree.Insert(n, rec); err != nil {
			t.Fatal(err)
		}
	}
	var raw bytes.Buffer
	if _, err := tree.WriteTo(&raw); err != nil {
		t.Fatal(err)
	}
	var gz bytes.Buffer
	w := gzip.NewWriter(&gz)
	if _, err := w.Write(raw.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return gz.Bytes()
}

var fixtureNetworks = map[string]string{
	"198.51.100.0/24": "DE",
	"203.0.113.0/24":  "NZ",
	"2001:db8::/32":   "FR",
}

// server serves body for the given month and 404 for any other path.
func server(t *testing.T, month string, body []byte) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if r.URL.Path != "/dbip-country-lite-"+month+".mmdb.gz" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

func newDB(t *testing.T, dir, url string, now time.Time) *DB {
	t.Helper()
	d := New(dir, url, http.DefaultClient, quietLog())
	d.now = func() time.Time { return now }
	t.Cleanup(func() { _ = d.Close() })
	return d
}

func TestUpdateDownloadsAndLooksUp(t *testing.T) {
	srv, _ := server(t, "2026-11", fixtureDB(t, fixtureNetworks))
	d := newDB(t, t.TempDir(), srv.URL, time.Date(2026, 11, 22, 0, 0, 0, 0, time.UTC))

	if got := d.Country(netip.MustParseAddr("198.51.100.7")); got != "" {
		t.Errorf("country before load = %q, want empty", got)
	}
	if err := d.Update(context.Background()); err != nil {
		t.Fatal(err)
	}
	for ip, want := range map[string]string{
		"198.51.100.7":       "DE",
		"::ffff:203.0.113.9": "NZ",
		"2001:db8::1":        "FR",
		"192.0.2.1":          "", // not in the database
		"10.0.0.5":           "", // private
		"127.0.0.1":          "", // loopback
		"fe80::1":            "", // link-local
	} {
		if got := d.Country(netip.MustParseAddr(ip)); got != want {
			t.Errorf("Country(%s) = %q, want %q", ip, got, want)
		}
	}
}

func TestUpdateFallsBackToPreviousMonth(t *testing.T) {
	srv, _ := server(t, "2026-10", fixtureDB(t, fixtureNetworks))
	d := newDB(t, t.TempDir(), srv.URL, time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC))
	if err := d.Update(context.Background()); err != nil {
		t.Fatal(err)
	}
	if got := d.Country(netip.MustParseAddr("198.51.100.7")); got != "DE" {
		t.Errorf("country = %q, want DE", got)
	}
}

func TestUpdateSkipsFreshFileAndRefreshesStale(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 11, 22, 0, 0, 0, 0, time.UTC)
	srv, hits := server(t, "2026-11", fixtureDB(t, fixtureNetworks))

	d := newDB(t, dir, srv.URL, now)
	if err := d.Update(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := d.Update(context.Background()); err != nil {
		t.Fatal(err)
	}
	if hits.Load() != 1 {
		t.Fatalf("downloads = %d, want 1 (fresh file reused)", hits.Load())
	}

	// A restarted process opens the file from disk without downloading.
	d2 := newDB(t, dir, srv.URL, now)
	if err := d2.Open(); err != nil {
		t.Fatal(err)
	}
	if got := d2.Country(netip.MustParseAddr("198.51.100.7")); got != "DE" {
		t.Errorf("country after Open = %q, want DE", got)
	}

	old := now.Add(-MaxAge - time.Hour)
	if err := os.Chtimes(filepath.Join(dir, fileName), old, old); err != nil {
		t.Fatal(err)
	}
	if err := d2.Update(context.Background()); err != nil {
		t.Fatal(err)
	}
	if hits.Load() != 2 {
		t.Errorf("downloads = %d, want 2 (stale file replaced)", hits.Load())
	}
}

func TestUpdateRejectsCorruptDownloadAndKeepsOld(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 11, 22, 0, 0, 0, 0, time.UTC)
	good, _ := server(t, "2026-11", fixtureDB(t, fixtureNetworks))
	d := newDB(t, dir, good.URL, now)
	if err := d.Update(context.Background()); err != nil {
		t.Fatal(err)
	}

	var junk bytes.Buffer
	w := gzip.NewWriter(&junk)
	_, _ = w.Write([]byte("not a database"))
	_ = w.Close()
	bad, _ := server(t, "2026-11", junk.Bytes())
	d.baseURL = bad.URL
	old := now.Add(-MaxAge - time.Hour)
	if err := os.Chtimes(filepath.Join(dir, fileName), old, old); err != nil {
		t.Fatal(err)
	}
	if err := d.Update(context.Background()); err == nil {
		t.Fatal("corrupt download accepted")
	}
	if got := d.Country(netip.MustParseAddr("198.51.100.7")); got != "DE" {
		t.Errorf("country after failed update = %q, want DE from the old file", got)
	}
}

func TestOpenWithoutFile(t *testing.T) {
	d := newDB(t, t.TempDir(), "http://unused.invalid", time.Now())
	if err := d.Open(); err != nil {
		t.Fatal(err)
	}
	if got := d.Country(netip.MustParseAddr("198.51.100.7")); got != "" {
		t.Errorf("country = %q, want empty", got)
	}
}
