package library

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"generic-arr-project/internal/metadata"
	"generic-arr-project/internal/store/db"
)

// MovieStatus summarises where a movie is in its lifecycle.
type MovieStatus string

const (
	MovieMissing     MovieStatus = "missing"     // monitored, no file, nothing downloading
	MovieDownloading MovieStatus = "downloading" // a download is in progress or awaiting import
	MovieDownloaded  MovieStatus = "downloaded"
	MovieUnmonitored MovieStatus = "unmonitored"
)

// Movie is a movie in the library.
type Movie struct {
	ID            int64       `json:"id"`
	TMDBID        int64       `json:"tmdb_id"`
	IMDBID        *string     `json:"imdb_id,omitempty"`
	Title         string      `json:"title"`
	OriginalTitle string      `json:"original_title"`
	Year          int         `json:"year"`
	Overview      string      `json:"overview"`
	PosterPath    *string     `json:"poster_path,omitempty"`
	BackdropPath  *string     `json:"backdrop_path,omitempty"`
	RootFolderID  int64       `json:"root_folder_id"`
	Monitored     bool        `json:"monitored"`
	FilePath      *string     `json:"file_path,omitempty"`
	FileQuality   *string     `json:"file_quality,omitempty"`
	AddedAt       string      `json:"added_at"`
	Status        MovieStatus `json:"status"`
}

func movieFromRow(r db.Movie, activeDownload bool) Movie {
	m := Movie{
		ID: r.ID, TMDBID: r.TMDBID, IMDBID: r.IMDBID, Title: r.Title, OriginalTitle: r.OriginalTitle,
		Year: int(r.Year), Overview: r.Overview, PosterPath: r.PosterPath, BackdropPath: optString(r.BackdropPath), RootFolderID: r.RootFolderID, Monitored: r.Monitored,
		FilePath: r.FilePath, FileQuality: r.FileQuality, AddedAt: r.AddedAt,
	}
	switch {
	case r.FilePath != nil:
		m.Status = MovieDownloaded
	case activeDownload:
		m.Status = MovieDownloading
	case !r.Monitored:
		m.Status = MovieUnmonitored
	default:
		m.Status = MovieMissing
	}
	return m
}

// LookupMovies searches TMDB, marking movies that are in the library.
func (s *Service) LookupMovies(ctx context.Context, query string, year int) ([]DiscoverMovie, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, invalid("q", "is required")
	}
	tm, err := s.tmdb(ctx)
	if err != nil {
		return nil, err
	}
	movies, err := tm.SearchMovies(ctx, query, year)
	if err != nil {
		return nil, err
	}
	return s.annotateMovies(ctx, movies)
}

// AddMovieInput adds a movie by TMDB ID.
type AddMovieInput struct {
	TMDBID int64 `json:"tmdb_id"`
	// RootFolderID defaults to the default movie root folder.
	RootFolderID *int64 `json:"root_folder_id"`
	// Monitored defaults to true.
	Monitored *bool `json:"monitored"`
	// Search starts an automatic search right after adding.
	Search bool `json:"search"`
}

// AddMovie fetches the movie from TMDB and adds it to the library.
func (s *Service) AddMovie(ctx context.Context, in AddMovieInput) (Movie, error) {
	if in.TMDBID <= 0 {
		return Movie{}, invalid("tmdb_id", "is required")
	}
	if _, err := s.store.GetMovieByTMDBID(ctx, in.TMDBID); err == nil {
		return Movie{}, conflict("movie with TMDB id %d is already in the library", in.TMDBID)
	} else if !errors.Is(err, sql.ErrNoRows) {
		return Movie{}, fmt.Errorf("check existing movie: %w", err)
	}
	root, err := s.rootFolderFor(ctx, MediaMovie, in.RootFolderID)
	if err != nil {
		return Movie{}, err
	}
	tm, err := s.tmdb(ctx)
	if err != nil {
		return Movie{}, err
	}
	meta, err := tm.GetMovie(ctx, in.TMDBID)
	if err != nil {
		if errors.Is(err, metadata.ErrNotFound) {
			return Movie{}, invalid("tmdb_id", "no movie with TMDB id %d", in.TMDBID)
		}
		return Movie{}, err
	}
	monitored := in.Monitored == nil || *in.Monitored
	var imdb *string
	if meta.IMDBID != "" {
		imdb = &meta.IMDBID
	}
	row, err := s.store.CreateMovie(ctx, db.CreateMovieParams{
		TMDBID: meta.TMDBID, IMDBID: imdb, Title: meta.Title, OriginalTitle: meta.OriginalTitle,
		Year: int64(meta.Year), Overview: meta.Overview, PosterPath: optString(meta.PosterPath), BackdropPath: meta.BackdropPath, RootFolderID: root.ID, Monitored: monitored,
	})
	if err != nil {
		if isUniqueViolation(err) {
			return Movie{}, conflict("movie with TMDB id %d is already in the library", in.TMDBID)
		}
		return Movie{}, fmt.Errorf("add movie: %w", err)
	}
	s.log.Info("movie added", "movie", row.ID, "title", row.Title, "year", row.Year)
	if in.Search && monitored {
		// The search outcome is visible in the queue and logs; adding has
		// succeeded either way.
		go func() {
			if _, err := s.AutoSearchMovie(context.WithoutCancel(ctx), row.ID); err != nil {
				s.log.Warn("search after add", "movie", row.ID, "err", err)
			}
		}()
	}
	return movieFromRow(row, false), nil
}

// rootFolderFor returns root folder id, or the default for mt when id is nil,
// checking that it holds mt.
func (s *Service) rootFolderFor(ctx context.Context, mt MediaType, id *int64) (db.RootFolder, error) {
	if id == nil {
		return s.defaultRootFolder(ctx, mt)
	}
	root, err := s.store.GetRootFolder(ctx, *id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return db.RootFolder{}, invalid("root_folder_id", "root folder %d does not exist", *id)
		}
		return db.RootFolder{}, fmt.Errorf("load root folder: %w", err)
	}
	if MediaType(root.MediaType) != mt {
		return db.RootFolder{}, invalid("root_folder_id", "root folder %d is for %s, not %s", *id, root.MediaType, mt)
	}
	return root, nil
}

// Movies lists the library.
func (s *Service) Movies(ctx context.Context) ([]Movie, error) {
	rows, err := s.store.ListMovies(ctx)
	if err != nil {
		return nil, fmt.Errorf("list movies: %w", err)
	}
	active, err := s.moviesWithActiveDownloads(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]Movie, len(rows))
	for i, r := range rows {
		out[i] = movieFromRow(r, active[r.ID])
	}
	return out, nil
}

// Movie returns one movie.
func (s *Service) Movie(ctx context.Context, id int64) (Movie, error) {
	row, err := s.store.GetMovie(ctx, id)
	if err != nil {
		return Movie{}, notFound(err, "movie", id)
	}
	dls, err := s.store.ListDownloadsForMovie(ctx, &id)
	if err != nil {
		return Movie{}, fmt.Errorf("list downloads: %w", err)
	}
	return movieFromRow(row, hasActive(dls)), nil
}

func (s *Service) moviesWithActiveDownloads(ctx context.Context) (map[int64]bool, error) {
	out := map[int64]bool{}
	for _, status := range []string{statusDownloading, statusCompleted} {
		rows, err := s.store.ListDownloadsByStatus(ctx, status)
		if err != nil {
			return nil, fmt.Errorf("list downloads: %w", err)
		}
		for _, d := range rows {
			if d.MovieID != nil {
				out[*d.MovieID] = true
			}
		}
	}
	return out, nil
}

func hasActive(dls []db.Download) bool {
	for _, d := range dls {
		if d.Status == statusDownloading || d.Status == statusCompleted {
			return true
		}
	}
	return false
}

// UpdateMovieInput changes a movie's settings; nil fields are unchanged.
type UpdateMovieInput struct {
	Monitored    *bool  `json:"monitored"`
	RootFolderID *int64 `json:"root_folder_id"`
}

// UpdateMovie applies in. Changing the root folder affects future imports
// only; existing files are not moved.
func (s *Service) UpdateMovie(ctx context.Context, id int64, in UpdateMovieInput) (Movie, error) {
	row, err := s.store.GetMovie(ctx, id)
	if err != nil {
		return Movie{}, notFound(err, "movie", id)
	}
	params := db.UpdateMovieSettingsParams{ID: id, Monitored: row.Monitored, RootFolderID: row.RootFolderID}
	if in.Monitored != nil {
		params.Monitored = *in.Monitored
	}
	if in.RootFolderID != nil {
		root, err := s.rootFolderFor(ctx, MediaMovie, in.RootFolderID)
		if err != nil {
			return Movie{}, err
		}
		params.RootFolderID = root.ID
	}
	if _, err := s.store.UpdateMovieSettings(ctx, params); err != nil {
		return Movie{}, fmt.Errorf("update movie: %w", err)
	}
	return s.Movie(ctx, id)
}

// DeleteMovie removes a movie from the library. Files on disk and downloads
// in progress are left alone.
func (s *Service) DeleteMovie(ctx context.Context, id int64) error {
	n, err := s.store.DeleteMovie(ctx, id)
	if err != nil {
		return fmt.Errorf("delete movie: %w", err)
	}
	if n == 0 {
		return fmt.Errorf("movie %d: %w", id, ErrNotFound)
	}
	return nil
}

// optString returns nil for "", else a pointer to s.
func optString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
