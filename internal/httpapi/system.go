package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"generic-arr-project/internal/httpapi/apigen"
)

func (h *handlers) Health(context.Context, apigen.HealthRequestObject) (apigen.HealthResponseObject, error) {
	return apigen.Health200JSONResponse{Status: "ok"}, nil
}

// Status reports whether downloads are running, and why not.
func (h *handlers) Status(context.Context, apigen.StatusRequestObject) (apigen.StatusResponseObject, error) {
	out := apigen.Status200JSONResponse{DownloadsAvailable: true}
	if err := h.svc.RuntimeError(); err != nil {
		out.DownloadsAvailable = false
		msg := err.Error()
		out.DownloadsError = &msg
	}
	return out, nil
}

// ListDirs lists the subdirectories of an absolute path for the folder
// picker. Only directory names are returned, never files or contents;
// hidden directories are left out.
func (h *handlers) ListDirs(_ context.Context, req apigen.ListDirsRequestObject) (apigen.ListDirsResponseObject, error) {
	raw := deref(req.Params.Path)
	if raw == "" {
		raw = "/"
	}
	if !filepath.IsAbs(raw) {
		return nil, fmt.Errorf("%w: path must be absolute", errBadRequest)
	}
	path := filepath.Clean(raw)
	entries, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", errBadRequest, err)
	}
	out := apigen.ListDirs200JSONResponse{Path: path, Dirs: []string{}}
	if parent := filepath.Dir(path); parent != path {
		out.Parent = &parent
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".") {
			continue
		}
		isDir := e.IsDir()
		if e.Type()&os.ModeSymlink != 0 {
			if info, err := os.Stat(filepath.Join(path, e.Name())); err == nil {
				isDir = info.IsDir()
			}
		}
		if isDir {
			out.Dirs = append(out.Dirs, e.Name())
		}
	}
	sort.Strings(out.Dirs)
	return out, nil
}

// poster serves a cached TMDB poster.
func (h *handlers) poster(w http.ResponseWriter, r *http.Request) {
	path, err := h.svc.Poster(r.Context(), r.PathValue("size"), r.PathValue("file"))
	if err != nil {
		h.writeErr(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "private, max-age=604800")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeFile(w, r, path)
}
