// Package geoip maps IP addresses to countries using the free DB-IP
// "IP to Country Lite" database (CC BY 4.0, https://db-ip.com), which it
// downloads and refreshes itself.
package geoip

import (
	"compress/gzip"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/netip"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"generic-arr-project/internal/closer"

	"github.com/oschwald/maxminddb-golang/v2"
	"github.com/rs/zerolog"
)

const (
	// DefaultBaseURL serves the monthly DB-IP Lite builds.
	DefaultBaseURL = "https://download.db-ip.com/free"
	// MaxAge is how old the local database may get before it is replaced;
	// DB-IP publishes a new build every month.
	MaxAge = 35 * 24 * time.Hour

	fileName = "dbip-country-lite.mmdb"
	// maxDBBytes bounds the decompressed download (the real file is ~8 MiB).
	maxDBBytes = 256 << 20
)

// DB answers country lookups. It is safe for concurrent use; lookups made
// before a database is loaded return "".
type DB struct {
	dir     string
	baseURL string
	client  *http.Client
	log     *zerolog.Logger
	now     func() time.Time

	reader   atomic.Pointer[maxminddb.Reader]
	updateMu sync.Mutex
}

// New returns a DB that keeps its file in dir and downloads from baseURL
// (DefaultBaseURL if empty) with client. Call Open and Update to load it.
func New(dir, baseURL string, client *http.Client, log *zerolog.Logger) *DB {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	logger := log.With().Str("component", "geoip").Logger()
	return &DB{
		dir: dir, baseURL: baseURL, client: client, now: time.Now,
		log: &logger,
	}
}

func (d *DB) path() string { return filepath.Join(d.dir, fileName) }

// Open loads the database already on disk, if any, without downloading.
func (d *DB) Open() error {
	if _, err := os.Stat(d.path()); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return d.load()
}

// Update downloads a new database when there is none or it is older than
// MaxAge, then loads it.
func (d *DB) Update(ctx context.Context) error {
	d.updateMu.Lock()
	defer d.updateMu.Unlock()
	if fi, err := os.Stat(d.path()); err == nil && d.now().Sub(fi.ModTime()) < MaxAge {
		if d.reader.Load() != nil {
			return nil
		}
		err := d.load()
		if err == nil {
			return nil
		}
		// Fresh but not loadable (e.g. truncated): download it again.
		d.log.Warn().Err(err).Msg("replacing unreadable database")
	}
	if err := d.download(ctx); err != nil {
		return err
	}
	if err := d.load(); err != nil {
		return err
	}
	d.log.Info().Msg("ip geolocation database updated")
	return nil
}

// Country returns the ISO 3166-1 alpha-2 code for addr, or "" when it is
// not a public address, is not in the database, or no database is loaded.
func (d *DB) Country(addr netip.Addr) string {
	r := d.reader.Load()
	addr = addr.Unmap()
	if r == nil || !addr.IsValid() || addr.IsPrivate() || addr.IsLoopback() ||
		addr.IsLinkLocalUnicast() || addr.IsUnspecified() {
		return ""
	}
	var rec struct {
		Country struct {
			ISOCode string `maxminddb:"iso_code"`
		} `maxminddb:"country"`
	}
	if err := r.Lookup(addr).Decode(&rec); err != nil {
		d.log.Debug().Stringer("addr", addr).Err(err).Msg("lookup")
		return ""
	}
	return rec.Country.ISOCode
}

// Close releases the loaded database.
func (d *DB) Close() error {
	if r := d.reader.Swap(nil); r != nil {
		return r.Close()
	}
	return nil
}

func (d *DB) load() error {
	r, err := maxminddb.Open(d.path())
	if err != nil {
		return fmt.Errorf("open ip geolocation database: %w", err)
	}
	// The previous reader is not closed: in-flight lookups may still use
	// its memory map. It is replaced at most monthly.
	d.reader.Store(r)
	return nil
}

// download fetches this month's build, falling back to last month's when
// this month's is not published yet.
func (d *DB) download(ctx context.Context) error {
	now := d.now().UTC()
	months := []time.Time{now, now.AddDate(0, -1, 0)}
	var errs []error
	for _, m := range months {
		url := fmt.Sprintf("%s/dbip-country-lite-%s.mmdb.gz", d.baseURL, m.Format("2006-01"))
		err := d.fetch(ctx, url)
		if err == nil {
			return nil
		}
		errs = append(errs, err)
		if !errors.Is(err, errNotPublished) {
			break
		}
	}
	return fmt.Errorf("download ip geolocation database: %w", errors.Join(errs...))
}

var errNotPublished = errors.New("not published")

func (d *DB) fetch(ctx context.Context, url string) (err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := d.client.Do(req)
	if err != nil {
		return err
	}
	defer closer.Join(&err, resp.Body)
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("%s: %w", url, errNotPublished)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("%s: %s", url, resp.Status)
	}
	gz, err := gzip.NewReader(resp.Body)
	if err != nil {
		return fmt.Errorf("%s: %w", url, err)
	}
	defer closer.Join(&err, gz)

	if err := os.MkdirAll(d.dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(d.dir, ".dbip-*")
	if err != nil {
		return err
	}
	n, err := io.Copy(tmp, io.LimitReader(gz, maxDBBytes+1))
	closeErr := tmp.Close()
	switch {
	case err != nil:
		err = fmt.Errorf("%s: %w", url, err)
	case closeErr != nil:
		err = closeErr
	case n > maxDBBytes:
		err = fmt.Errorf("%s: database is larger than %d MiB", url, maxDBBytes>>20)
	}
	if err == nil {
		// Reject a corrupt file before it replaces a working one.
		var r *maxminddb.Reader
		if r, err = maxminddb.Open(tmp.Name()); err == nil {
			err = r.Close()
		} else {
			err = fmt.Errorf("%s: %w", url, err)
		}
	}
	if err != nil {
		return errors.Join(err, os.Remove(tmp.Name()))
	}
	if err := os.Rename(tmp.Name(), d.path()); err != nil {
		return errors.Join(err, os.Remove(tmp.Name()))
	}
	return nil
}
