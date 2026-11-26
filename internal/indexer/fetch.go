package indexer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
)

const maxTorrentBytes = 10 << 20

// TorrentSource is what a release resolves to: a magnet link or the bytes
// of a .torrent file.
type TorrentSource struct {
	Magnet   string
	MetaInfo []byte
}

// errMagnetRedirect stops the HTTP client at a redirect to a magnet link,
// which some download links answer with for magnet-only trackers.
type errMagnetRedirect struct{ uri string }

func (e *errMagnetRedirect) Error() string { return "redirect to magnet" }

// Resolve turns a release into a TorrentSource, downloading the .torrent
// with client when the release has no magnet link.
func Resolve(ctx context.Context, client *http.Client, r Release) (TorrentSource, error) {
	if r.MagnetURI != "" {
		return TorrentSource{Magnet: r.MagnetURI}, nil
	}
	if r.DownloadURL == "" {
		return TorrentSource{}, errors.New("release has neither a magnet nor a download link")
	}

	c := *client
	c.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if req.URL.Scheme == "magnet" {
			return &errMagnetRedirect{uri: req.URL.String()}
		}
		if len(via) >= 10 {
			return errors.New("too many redirects")
		}
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, r.DownloadURL, nil)
	if err != nil {
		return TorrentSource{}, fmt.Errorf("torrent link: %w", err)
	}
	resp, err := c.Do(req)
	if err != nil {
		var mr *errMagnetRedirect
		if errors.As(err, &mr) {
			return TorrentSource{Magnet: mr.uri}, nil
		}
		var ue *url.Error
		if errors.As(err, &ue) {
			return TorrentSource{}, fmt.Errorf("download torrent: %s", ue.Err)
		}
		return TorrentSource{}, fmt.Errorf("download torrent: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return TorrentSource{}, fmt.Errorf("download torrent: indexer returned %s", resp.Status)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxTorrentBytes+1))
	if err != nil {
		return TorrentSource{}, fmt.Errorf("download torrent: %w", err)
	}
	if len(body) > maxTorrentBytes {
		return TorrentSource{}, errors.New("download torrent: file larger than 10 MiB")
	}
	if len(body) == 0 || body[0] != 'd' { // bencoded dictionary
		return TorrentSource{}, errors.New("download torrent: response is not a .torrent file")
	}
	return TorrentSource{MetaInfo: body}, nil
}
