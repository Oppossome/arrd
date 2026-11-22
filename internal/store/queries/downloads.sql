-- name: ListDownloads :many
SELECT * FROM downloads ORDER BY added_at DESC, id DESC;

-- name: ListDownloadsByStatus :many
SELECT * FROM downloads WHERE status = @status ORDER BY id;

-- name: ListDownloadsForMovie :many
SELECT * FROM downloads WHERE movie_id = @movie_id ORDER BY id DESC;

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
