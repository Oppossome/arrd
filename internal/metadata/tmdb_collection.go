package metadata

import (
	"context"
	"slices"
	"strconv"
)

// Collection is a TMDB collection: a franchise and the movies in it.
type Collection struct {
	ID   int64  `json:"id"`
	Name string `json:"name"`
	// Parts are the collection's movies, oldest first; unreleased movies
	// (Year 0) come last.
	Parts []Movie `json:"parts"`
}

// GetCollection fetches a collection and its movies.
func (c *TMDB) GetCollection(ctx context.Context, id int64) (*Collection, error) {
	var resp struct {
		ID    int64       `json:"id"`
		Name  string      `json:"name"`
		Parts []tmdbMovie `json:"parts"`
	}
	if err := c.get(ctx, "/collection/"+strconv.FormatInt(id, 10), nil, &resp); err != nil {
		return nil, err
	}
	out := &Collection{ID: resp.ID, Name: resp.Name, Parts: make([]Movie, 0, len(resp.Parts))}
	for _, p := range resp.Parts {
		out.Parts = append(out.Parts, p.toMovie())
	}
	slices.SortStableFunc(out.Parts, func(a, b Movie) int {
		return releaseOrder(a.Year) - releaseOrder(b.Year)
	})
	return out, nil
}

// releaseOrder sorts unknown years (0) after every known one.
func releaseOrder(year int) int {
	if year == 0 {
		return 1 << 30
	}
	return year
}
