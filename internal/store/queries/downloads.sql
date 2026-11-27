-- name: ListDownloads :many
SELECT * FROM downloads ORDER BY added_at DESC, id DESC;

-- name: ListDownloadsByStatus :many
SELECT * FROM downloads WHERE status = @status ORDER BY id;

-- Downloads for a movie: grabbed for it, or a pack expected to contain it.
-- name: ListDownloadsForMovie :many
SELECT d.* FROM downloads d
WHERE d.movie_id = @movie_id
   OR d.id IN (SELECT di.download_id FROM download_items di WHERE di.movie_id = @movie_id)
ORDER BY d.id DESC;

-- name: GetDownload :one
SELECT * FROM downloads WHERE id = @id;

-- name: GetDownloadByInfoHash :one
SELECT * FROM downloads WHERE info_hash = @info_hash;

-- name: CreateDownload :one
INSERT INTO downloads (
    movie_id, series_id, indexer_id, info_hash, release_title, quality, size,
    magnet, metainfo, incomplete_dir, complete_dir
) VALUES (
    @movie_id, @series_id, @indexer_id, @info_hash, @release_title, @quality, @size,
    @magnet, @metainfo, @incomplete_dir, @complete_dir
)
RETURNING *;

-- name: SetDownloadMetainfo :exec
UPDATE downloads SET metainfo = @metainfo WHERE id = @id AND metainfo IS NULL;

-- name: MarkDownloadCompleted :exec
UPDATE downloads
SET status = 'completed', completed_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now'), error = NULL
WHERE id = @id;

-- name: MarkDownloadImported :exec
UPDATE downloads
SET status = 'imported', imported_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now'), error = NULL
WHERE id = @id;

-- name: MarkDownloadFailed :exec
UPDATE downloads SET status = 'failed', error = @error WHERE id = @id;

-- Records a non-fatal problem (e.g. an import that will be retried).
-- name: SetDownloadError :exec
UPDATE downloads SET error = @error WHERE id = @id;

-- name: DeleteDownload :execrows
DELETE FROM downloads WHERE id = @id;

-- name: ListDownloadsForSeries :many
SELECT * FROM downloads WHERE series_id = @series_id ORDER BY id DESC;

-- name: SetDownloadPaused :exec
UPDATE downloads SET paused = @paused WHERE id = @id;

-- name: CountActiveDownloads :one
SELECT COUNT(*) FROM downloads WHERE status = 'downloading';

-- name: AddDownloadItem :exec
INSERT INTO download_items (download_id, movie_id, episode_id) VALUES (@download_id, @movie_id, @episode_id);

-- Movies a download is expected to contain.
-- name: ListDownloadItemMovies :many
SELECT m.* FROM movies m
JOIN download_items di ON di.movie_id = m.id
WHERE di.download_id = @download_id
ORDER BY m.year, m.title;

-- Episodes a download is expected to contain.
-- name: ListDownloadItemEpisodes :many
SELECT e.* FROM episodes e
JOIN download_items di ON di.episode_id = e.id
WHERE di.download_id = @download_id
ORDER BY e.season, e.episode;

-- Movies and episodes in a download that is downloading or awaiting import.
-- name: ListActiveItemIDs :many
SELECT DISTINCT di.movie_id, di.episode_id FROM download_items di
JOIN downloads d ON d.id = di.download_id
WHERE d.status IN ('downloading', 'completed');
