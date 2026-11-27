-- name: ListSeries :many
SELECT * FROM series ORDER BY title, year;

-- name: ListMonitoredSeries :many
SELECT * FROM series WHERE monitored = 1 ORDER BY title;

-- name: GetSeries :one
SELECT * FROM series WHERE id = @id;

-- name: GetSeriesByTMDBID :one
SELECT * FROM series WHERE tmdb_id = @tmdb_id;

-- name: CreateSeries :one
INSERT INTO series (tmdb_id, tvdb_id, imdb_id, title, original_title, year, overview, status, poster_path, backdrop_path, root_folder_id, monitored)
VALUES (@tmdb_id, @tvdb_id, @imdb_id, @title, @original_title, @year, @overview, @status, @poster_path, @backdrop_path, @root_folder_id, @monitored)
RETURNING *;

-- name: UpdateSeriesMetadata :exec
UPDATE series SET
    tvdb_id = @tvdb_id, imdb_id = @imdb_id, title = @title, original_title = @original_title,
    year = @year, overview = @overview, status = @status, poster_path = @poster_path, backdrop_path = @backdrop_path,
    last_refreshed_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
WHERE id = @id;

-- name: UpdateSeriesSettings :one
UPDATE series SET monitored = @monitored, root_folder_id = @root_folder_id
WHERE id = @id
RETURNING *;

-- name: DeleteSeries :execrows
DELETE FROM series WHERE id = @id;

-- name: CountSeriesInRootFolder :one
SELECT COUNT(*) FROM series WHERE root_folder_id = @root_folder_id;

-- name: ListEpisodes :many
SELECT * FROM episodes WHERE series_id = @series_id ORDER BY season, episode;

-- name: ListSeasonEpisodes :many
SELECT * FROM episodes WHERE series_id = @series_id AND season = @season ORDER BY episode;

-- name: GetEpisode :one
SELECT * FROM episodes WHERE id = @id;

-- name: UpsertEpisode :one
INSERT INTO episodes (series_id, season, episode, title, air_date, monitored)
VALUES (@series_id, @season, @episode, @title, @air_date, @monitored)
ON CONFLICT (series_id, season, episode) DO UPDATE SET
    title = excluded.title,
    air_date = excluded.air_date
RETURNING *;

-- name: SetEpisodeMonitored :execrows
UPDATE episodes SET monitored = @monitored WHERE id = @id;

-- name: SetSeasonMonitored :execrows
UPDATE episodes SET monitored = @monitored WHERE series_id = @series_id AND season = @season;

-- name: SetEpisodeFile :exec
UPDATE episodes SET file_path = @file_path, file_quality = @file_quality WHERE id = @id;

-- Monitored episodes without a file that have aired by @today and are not
-- part of an active download.
-- name: ListWantedEpisodes :many
SELECT e.* FROM episodes e
JOIN series s ON s.id = e.series_id
WHERE s.monitored = 1 AND e.monitored = 1 AND e.file_path IS NULL
  AND e.air_date IS NOT NULL AND e.air_date <= CAST(@today AS TEXT)
  AND NOT EXISTS (
      SELECT 1 FROM download_items di
      JOIN downloads d ON d.id = di.download_id
      WHERE di.episode_id = e.id AND d.status IN ('downloading', 'completed')
  )
ORDER BY e.series_id, e.season, e.episode;

-- name: ListSeriesMissingBackdrop :many
SELECT * FROM series WHERE backdrop_path = '' ORDER BY id;

-- name: SetSeriesBackdrop :exec
UPDATE series SET backdrop_path = @backdrop_path WHERE id = @id;
