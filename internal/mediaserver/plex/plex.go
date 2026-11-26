// Package plex tells a Plex Media Server to rescan its libraries.
package plex

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"generic-arr-project/internal/formspec"
	"generic-arr-project/internal/mediaserver"
)

// Definition describes Plex.
func Definition() mediaserver.Definition {
	return mediaserver.Definition{
		ID:          "plex",
		Name:        "Plex",
		Description: "Rescans every Plex library after an import.",
		Fields: formspec.Fields{
			{Key: "url", Label: "Server URL", Kind: formspec.KindText, Required: true, Placeholder: "http://localhost:32400"},
			{Key: "token", Label: "Plex token", Kind: formspec.KindPassword, Required: true, Secret: true,
				Help: "The X-Plex-Token of an account that manages the server."},
		},
		Parse: parse,
	}
}

func parse(v formspec.Values, client *http.Client) (mediaserver.Server, error) {
	base, err := mediaserver.ParseBaseURL("url", v["url"])
	if err != nil {
		return nil, err
	}
	if v["token"] == "" {
		return nil, formspec.Fieldf("token", "is required")
	}
	return &server{base: base, token: v["token"], client: client}, nil
}

type server struct {
	base   *url.URL
	token  string
	client *http.Client
}

func (s *server) request(ctx context.Context, path string) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.base.JoinPath(path).String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Plex-Token", s.token)
	req.Header.Set("Accept", "application/json")
	return req, nil
}

func (s *server) Test(ctx context.Context) error {
	_, err := s.sections(ctx) // /identity needs no token, so it proves nothing
	return err
}

type sectionsResponse struct {
	MediaContainer struct {
		Directory []struct {
			Key   string `json:"key"`
			Title string `json:"title"`
		} `json:"Directory"`
	} `json:"MediaContainer"`
}

func (s *server) sections(ctx context.Context) ([]string, error) {
	req, err := s.request(ctx, "/library/sections")
	if err != nil {
		return nil, err
	}
	var body sectionsResponse
	err = mediaserver.Do(s.client, req, func(r io.Reader) error {
		if err := json.NewDecoder(r).Decode(&body); err != nil {
			return fmt.Errorf("decode library sections: %w", err)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	keys := make([]string, 0, len(body.MediaContainer.Directory))
	for _, d := range body.MediaContainer.Directory {
		keys = append(keys, d.Key)
	}
	return keys, nil
}

func (s *server) RefreshLibrary(ctx context.Context) error {
	keys, err := s.sections(ctx)
	if err != nil {
		return err
	}
	for _, key := range keys {
		req, err := s.request(ctx, "/library/sections/"+url.PathEscape(key)+"/refresh")
		if err != nil {
			return err
		}
		if err := mediaserver.Do(s.client, req, nil); err != nil {
			return err
		}
	}
	return nil
}
