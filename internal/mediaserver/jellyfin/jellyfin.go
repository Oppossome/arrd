// Package jellyfin tells a Jellyfin server to rescan its libraries.
package jellyfin

import (
	"context"
	"net/http"
	"net/url"

	"generic-arr-project/internal/formspec"
	"generic-arr-project/internal/mediaserver"
)

// Definition describes Jellyfin.
func Definition() mediaserver.Definition {
	return mediaserver.Definition{
		ID:          "jellyfin",
		Name:        "Jellyfin",
		Description: "Rescans every Jellyfin library after an import.",
		Fields: formspec.Fields{
			{Key: "url", Label: "Server URL", Kind: formspec.KindText, Required: true, Placeholder: "http://localhost:8096"},
			{Key: "api_key", Label: "API key", Kind: formspec.KindPassword, Required: true, Secret: true,
				Help: "Create one in Jellyfin under Dashboard → API Keys."},
		},
		Parse: parse,
	}
}

func parse(v formspec.Values, client *http.Client) (mediaserver.Server, error) {
	base, err := mediaserver.ParseBaseURL("url", v["url"])
	if err != nil {
		return nil, err
	}
	if v["api_key"] == "" {
		return nil, formspec.Fieldf("api_key", "is required")
	}
	return &server{base: base, apiKey: v["api_key"], client: client}, nil
}

type server struct {
	base   *url.URL
	apiKey string
	client *http.Client
}

func (s *server) send(ctx context.Context, method, path string) error {
	req, err := http.NewRequestWithContext(ctx, method, s.base.JoinPath(path).String(), nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", `MediaBrowser Token="`+s.apiKey+`"`)
	return mediaserver.Do(s.client, req, nil)
}

// Test reads the server info, which needs a valid key.
func (s *server) Test(ctx context.Context) error {
	return s.send(ctx, http.MethodGet, "/System/Info")
}

func (s *server) RefreshLibrary(ctx context.Context) error {
	return s.send(ctx, http.MethodPost, "/Library/Refresh")
}
