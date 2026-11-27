-- name: ListMovies :many
SELECT * FROM movies ORDER BY title, year;

-- name: ListWantedMovies :many
SELECT * FROM movies
WHERE monitored = 1 AND file_path IS NULL
  AND NOT EXISTS (
      SELECT 1 FROM download_items di
      JOIN downloads d ON d.id = di.download_id
      WHERE di.movie_id = movies.id AND d.status IN ('downloading', 'completed')
  )
ORDER BY added_at;

-- name: GetMovie :one
SELECT * FROM movies WHERE id = @id;

-- name: GetMovieByTMDBID :one
SELECT * FROM movies WHERE tmdb_id = @tmdb_id;

-- name: CreateMovie :one
INSERT INTO movies (tmdb_id, imdb_id, title, original_title, year, overview, poster_path, backdrop_path, root_folder_id, monitored)
VALUES (@tmdb_id, @imdb_id, @title, @original_title, @year, @overview, @poster_path, @backdrop_path, @root_folder_id, @monitored)
RETURNING *;

-- name: UpdateMovieSettings :one
UPDATE movies SET monitored = @monitored, root_folder_id = @root_folder_id
WHERE id = @id
RETURNING *;

-- name: SetMovieFile :exec
UPDATE movies SET file_path = @file_path, file_quality = @file_quality WHERE id = @id;

-- name: DeleteMovie :execrows
DELETE FROM movies WHERE id = @id;

-- name: CountMoviesInRootFolder :one
SELECT COUNT(*) FROM movies WHERE root_folder_id = @root_folder_id;

-- name: ListMoviesMissingBackdrop :many
SELECT * FROM movies WHERE backdrop_path = '' ORDER BY id;

-- name: SetMovieBackdrop :exec
UPDATE movies SET backdrop_path = @backdrop_path WHERE id = @id;
