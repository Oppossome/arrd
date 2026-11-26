-- name: ListMediaServers :many
SELECT * FROM media_servers ORDER BY name;

-- name: ListEnabledMediaServers :many
SELECT * FROM media_servers WHERE enabled = 1 ORDER BY name;

-- name: GetMediaServer :one
SELECT * FROM media_servers WHERE id = @id;

-- name: CreateMediaServer :one
INSERT INTO media_servers (name, definition, settings, enabled)
VALUES (@name, @definition, @settings, @enabled)
RETURNING *;

-- name: UpdateMediaServer :one
UPDATE media_servers SET
    name       = @name,
    definition = @definition,
    settings   = @settings,
    enabled    = @enabled
WHERE id = @id
RETURNING *;

-- name: DeleteMediaServer :execrows
DELETE FROM media_servers WHERE id = @id;

-- name: RecordMediaServerError :exec
UPDATE media_servers
SET last_error = @last_error, last_error_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
WHERE id = @id;

-- name: ClearMediaServerError :exec
UPDATE media_servers SET last_error = NULL, last_error_at = NULL WHERE id = @id;
