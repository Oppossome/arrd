package media

import (
	"context"
	"fmt"
)

// ActiveItems are the movies and episodes in a download that is in
// progress or awaiting import, by library id.
type ActiveItems struct {
	Movies   map[int64]bool
	Episodes map[int64]bool
}

// ActiveItems lists what running downloads are expected to bring in.
func (s *Service) ActiveItems(ctx context.Context) (ActiveItems, error) {
	rows, err := s.store.ListActiveItemIDs(ctx)
	if err != nil {
		return ActiveItems{}, fmt.Errorf("list active download items: %w", err)
	}
	out := ActiveItems{Movies: map[int64]bool{}, Episodes: map[int64]bool{}}
	for _, r := range rows {
		if r.MovieID != nil {
			out.Movies[*r.MovieID] = true
		}
		if r.EpisodeID != nil {
			out.Episodes[*r.EpisodeID] = true
		}
	}
	return out, nil
}
