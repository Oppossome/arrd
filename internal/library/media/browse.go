package media

import (
	"context"
	"errors"
	"fmt"
	"slices"

	"generic-arr-project/internal/library/apperr"
	"generic-arr-project/internal/metadata"
)

// Browsing TMDB's catalogue by genre and language, and titles like a given one. Results
// carry the library id of titles already in the library.

// BrowsePage is one page of a browse.
type BrowsePage[T any] struct {
	Items      []T
	Page       int
	TotalPages int
}

// TitleFilter narrows titles by genre and original language. Its zero value
// lets every title through.
type TitleFilter struct {
	// GenreIDs keeps titles with any of these TMDB genres; empty for every genre.
	GenreIDs []int64
	// OriginalLanguage keeps titles first made in this ISO 639-1 language; empty for every language.
	OriginalLanguage string
}

// Matches reports whether a title with these genres and original language
// passes the filter.
func (f TitleFilter) Matches(genreIDs []int64, originalLanguage string) bool {
	if f.OriginalLanguage != "" && f.OriginalLanguage != originalLanguage {
		return false
	}
	return len(f.GenreIDs) == 0 || slices.ContainsFunc(genreIDs, func(id int64) bool { return slices.Contains(f.GenreIDs, id) })
}

// BrowseQuery picks the filter, sort order and page of a browse.
type BrowseQuery struct {
	TitleFilter
	Sort metadata.DiscoverSort
	Page int
}

// MovieGenres lists TMDB's movie genres.
func (s *Service) MovieGenres(ctx context.Context) ([]metadata.Genre, error) {
	tm, err := s.tmdb.Client(ctx)
	if err != nil {
		return nil, err
	}
	return tm.MovieGenres(ctx)
}

// SeriesGenres lists TMDB's TV genres.
func (s *Service) SeriesGenres(ctx context.Context) ([]metadata.Genre, error) {
	tm, err := s.tmdb.Client(ctx)
	if err != nil {
		return nil, err
	}
	return tm.SeriesGenres(ctx)
}

// Languages lists the original languages TMDB knows, by English name.
func (s *Service) Languages(ctx context.Context) ([]metadata.Language, error) {
	tm, err := s.tmdb.Client(ctx)
	if err != nil {
		return nil, err
	}
	return tm.Languages(ctx)
}

// BrowseMovies returns a page of TMDB movies by genre and sort order.
func (s *Service) BrowseMovies(ctx context.Context, q BrowseQuery) (BrowsePage[DiscoverMovie], error) {
	tm, err := s.tmdb.Client(ctx)
	if err != nil {
		return BrowsePage[DiscoverMovie]{}, err
	}
	p, err := tm.DiscoverMovies(ctx, s.discoverQuery(q))
	if err != nil {
		return BrowsePage[DiscoverMovie]{}, err
	}
	items, err := s.annotateMovies(ctx, p.Results)
	return BrowsePage[DiscoverMovie]{Items: items, Page: p.Page, TotalPages: p.TotalPages}, err
}

// BrowseSeries returns a page of TMDB series by genre and sort order.
func (s *Service) BrowseSeries(ctx context.Context, q BrowseQuery) (BrowsePage[DiscoverSeries], error) {
	tm, err := s.tmdb.Client(ctx)
	if err != nil {
		return BrowsePage[DiscoverSeries]{}, err
	}
	p, err := tm.DiscoverSeries(ctx, s.discoverQuery(q))
	if err != nil {
		return BrowsePage[DiscoverSeries]{}, err
	}
	items, err := s.annotateSeries(ctx, p.Results)
	return BrowsePage[DiscoverSeries]{Items: items, Page: p.Page, TotalPages: p.TotalPages}, err
}

func (s *Service) discoverQuery(q BrowseQuery) metadata.DiscoverQuery {
	return metadata.DiscoverQuery{
		GenreIDs: q.GenreIDs, OriginalLanguage: q.OriginalLanguage,
		Sort: q.Sort, Page: q.Page, Today: s.now(),
	}
}

// MoviesLike returns the movies TMDB recommends to people who liked a movie.
func (s *Service) MoviesLike(ctx context.Context, tmdbID int64) ([]DiscoverMovie, error) {
	tm, err := s.tmdb.Client(ctx)
	if err != nil {
		return nil, err
	}
	movies, err := tm.MovieRecommendations(ctx, tmdbID)
	if errors.Is(err, metadata.ErrNotFound) {
		return nil, fmt.Errorf("movie with TMDB id %d: %w", tmdbID, apperr.ErrNotFound)
	}
	if err != nil {
		return nil, err
	}
	return s.annotateMovies(ctx, movies)
}

// SeriesLike returns the series TMDB recommends to people who liked a series.
func (s *Service) SeriesLike(ctx context.Context, tmdbID int64) ([]DiscoverSeries, error) {
	tm, err := s.tmdb.Client(ctx)
	if err != nil {
		return nil, err
	}
	series, err := tm.SeriesRecommendations(ctx, tmdbID)
	if errors.Is(err, metadata.ErrNotFound) {
		return nil, fmt.Errorf("series with TMDB id %d: %w", tmdbID, apperr.ErrNotFound)
	}
	if err != nil {
		return nil, err
	}
	return s.annotateSeries(ctx, series)
}
