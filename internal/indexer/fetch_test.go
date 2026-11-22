package indexer

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestResolve(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("/torrent", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("d4:infod4:name1:xee")) })
	mux.HandleFunc("/magnet", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "magnet:?xt=urn:btih:0123456789abcdef0123456789abcdef01234567", http.StatusFound)
	})
	mux.HandleFunc("/html", func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("<html>login</html>")) })
	srv := httptest.NewServer(mux)
	defer srv.Close()
	ctx := context.Background()

	src, err := Resolve(ctx, srv.Client(), Release{DownloadURL: srv.URL + "/torrent"})
	if err != nil || len(src.MetaInfo) == 0 || src.Magnet != "" {
		t.Errorf("torrent: %+v, %v", src, err)
	}
	src, err = Resolve(ctx, srv.Client(), Release{DownloadURL: srv.URL + "/magnet"})
	if err != nil || !strings.HasPrefix(src.Magnet, "magnet:?xt=") {
		t.Errorf("magnet redirect: %+v, %v", src, err)
	}
	if _, err = Resolve(ctx, srv.Client(), Release{DownloadURL: srv.URL + "/html"}); err == nil {
		t.Error("html response accepted as torrent")
	}
	src, err = Resolve(ctx, srv.Client(), Release{MagnetURI: "magnet:?x", DownloadURL: srv.URL + "/torrent"})
	if err != nil || src.Magnet != "magnet:?x" {
		t.Errorf("magnet preferred: %+v, %v", src, err)
	}
}
