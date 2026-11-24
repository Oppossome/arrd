package media

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"generic-arr-project/internal/metadata"
)

func TestRecommendSkipsUnknownAndFailsOnErrors(t *testing.T) {
	ctx := context.Background()
	sources := []RecommendationSource{{1, "Gone"}, {2, "Kept"}, {3, "Nothing new"}}
	idOf := func(id int64) int64 { return id }
	fetch := func(_ context.Context, id int64) ([]int64, error) {
		switch id {
		case 1:
			return nil, metadata.ErrNotFound
		case 2:
			return []int64{10, 11}, nil
		default:
			return []int64{11}, nil
		}
	}
	rows, err := recommend(ctx, sources, map[int64]bool{}, fetch, idOf)
	if err != nil || len(rows) != 1 || rows[0].Because.Title != "Kept" || fmt.Sprint(rows[0].Items) != "[10 11]" {
		t.Errorf("rows = %+v, %v", rows, err)
	}

	boom := errors.New("boom")
	failing := func(_ context.Context, id int64) ([]int64, error) {
		if id == 2 {
			return nil, boom
		}
		return nil, nil
	}
	if _, err := recommend(ctx, sources, nil, failing, idOf); !errors.Is(err, boom) {
		t.Errorf("err = %v, want boom", err)
	}
}

func TestNewestSeeds(t *testing.T) {
	seeds := []seed{
		{RecommendationSource{1, "Old"}, "2026-01-01T00:00:00Z", 1},
		{RecommendationSource{2, "Same second, lower id"}, "2026-03-01T00:00:00Z", 2},
		{RecommendationSource{3, "Same second, higher id"}, "2026-03-01T00:00:00Z", 3},
	}
	got := newestSeeds(seeds)
	if len(got) != 3 || got[0].TMDBID != 3 || got[1].TMDBID != 2 || got[2].TMDBID != 1 {
		t.Errorf("order = %+v", got)
	}

	many := make([]seed, recommendationSeeds+3)
	for i := range many {
		many[i] = seed{RecommendationSource{TMDBID: int64(i)}, "2026-01-01T00:00:00Z", int64(i)}
	}
	if got := newestSeeds(many); len(got) != recommendationSeeds {
		t.Errorf("len = %d, want %d", len(got), recommendationSeeds)
	}
}
