-- name: GetDownloadSettings :one
SELECT * FROM download_settings WHERE id = 1;

-- name: UpsertDownloadSettings :one
INSERT INTO download_settings (id, incomplete_dir, complete_dir, import_mode)
VALUES (1, @incomplete_dir, @complete_dir, @import_mode)
ON CONFLICT (id) DO UPDATE SET
    incomplete_dir = excluded.incomplete_dir,
    complete_dir   = excluded.complete_dir,
    import_mode    = excluded.import_mode
RETURNING *;

-- name: GetQualitySettings :one
SELECT * FROM quality_settings WHERE id = 1;

-- name: UpdateQualitySettings :one
UPDATE quality_settings
SET min_resolution = @min_resolution, max_resolution = @max_resolution, max_size_mb = @max_size_mb
WHERE id = 1
RETURNING *;
