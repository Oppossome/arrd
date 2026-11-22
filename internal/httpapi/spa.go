package httpapi

import (
	"bytes"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"io/fs"
	"net/http"
	"path"
	"regexp"
	"strings"
	"time"
)

const notBuiltPage = `<!doctype html>
<html><head><meta charset="utf-8"><title>arrd</title></head>
<body style="font-family:sans-serif;max-width:40em;margin:4em auto">
<h1>arrd</h1>
<p>The web interface was not built into this binary. Run
<code>go generate ./web</code> and rebuild. The API under <code>/api/v1</code> works.</p>
</body></html>`

// spaHandler serves the embedded single-page app. Unknown paths fall back
// to index.html so client-side routes work on reload.
func spaHandler(ui fs.FS) http.Handler {
	var index []byte
	if ui != nil {
		index, _ = fs.ReadFile(ui, "index.html")
	}
	csp := contentSecurityPolicy(index)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		setSecurityHeaders(w, csp)
		if index == nil {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(notBuiltPage))
			return
		}

		name := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if name != "" && name != "index.html" {
			if f, err := ui.Open(name); err == nil {
				info, statErr := f.Stat()
				f.Close()
				if statErr == nil && !info.IsDir() {
					switch {
					case strings.HasPrefix(name, "_app/immutable/"):
						w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
					case name == "_app/version.json":
						w.Header().Set("Cache-Control", "no-cache")
					default:
						w.Header().Set("Cache-Control", "public, max-age=3600")
					}
					http.ServeFileFS(w, r, ui, name)
					return
				}
			} else if !errors.Is(err, fs.ErrNotExist) {
				http.Error(w, "internal error", http.StatusInternalServerError)
				return
			}
			// A missing asset is a real 404; anything else is an app route.
			if strings.HasPrefix(name, "_app/") || path.Ext(name) != "" {
				http.NotFound(w, r)
				return
			}
		}
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		http.ServeContent(w, r, "index.html", time.Time{}, bytes.NewReader(index))
	})
}

func setSecurityHeaders(w http.ResponseWriter, csp string) {
	h := w.Header()
	h.Set("Content-Security-Policy", csp)
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Referrer-Policy", "same-origin")
}

// inlineScript matches <script> elements without a src attribute.
var inlineScript = regexp.MustCompile(`(?s)<script(?:\s[^>]*)?>(.*?)</script>`)

// contentSecurityPolicy allows only same-origin resources, plus the exact
// inline bootstrap script(s) SvelteKit writes into index.html, by hash.
func contentSecurityPolicy(index []byte) string {
	scriptSrc := "'self'"
	for _, m := range inlineScript.FindAllSubmatch(index, -1) {
		if bytes.Contains(m[0][:bytes.IndexByte(m[0], '>')], []byte("src=")) {
			continue
		}
		sum := sha256.Sum256(m[1])
		scriptSrc += " 'sha256-" + base64.StdEncoding.EncodeToString(sum[:]) + "'"
	}
	return "default-src 'self'; script-src " + scriptSrc +
		"; img-src 'self' data:; style-src 'self' 'unsafe-inline'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'"
}
