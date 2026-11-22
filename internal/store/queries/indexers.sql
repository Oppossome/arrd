-- name: ListIndexers :many
SELECT * FROM indexers ORDER BY priority, name;

-- name: ListSearchIndexers :many
SELECT * FROM indexers WHERE enabled = 1 AND enable_search = 1 ORDER BY priority, name;

-- name: ListRSSIndexers :many
SELECT * FROM indexers WHERE enabled = 1 AND enable_rss = 1 ORDER BY priority, name;

-- name: GetIndexer :one
SELECT * FROM indexers WHERE id = @id;

-- name: GetIndexerByDefinition :one
SELECT * FROM indexers WHERE definition = @definition;

-- name: CountIndexers :one
SELECT COUNT(*) FROM indexers;

-- name: CreateIndexer :one
INSERT INTO indexers (
    name, definition, settings, movie_categories, tv_categories,
    enabled, enable_rss, enable_search, priority, min_seeders
) VALUES (
    @name, @definition, @settings, @movie_categories, @tv_categories,
    @enabled, @enable_rss, @enable_search, @priority, @min_seeders
)
RETURNING *;

-- name: UpdateIndexer :one
-- Changing the definition invalidates the cached capabilities.
UPDATE indexers SET
    name             = @name,
    definition       = @definition,
    settings         = @settings,
    movie_categories = @movie_categories,
    tv_categories    = @tv_categories,
    enabled          = @enabled,
    enable_rss       = @enable_rss,
    enable_search    = @enable_search,
    priority         = @priority,
    min_seeders      = @min_seeders,
    caps_json        = CASE WHEN definition = @definition THEN caps_json ELSE NULL END
WHERE id = @id
RETURNING *;

-- name: DeleteIndexer :execrows
DELETE FROM indexers WHERE id = @id;

-- name: SetIndexerCaps :exec
UPDATE indexers SET caps_json = @caps_json, last_error = NULL, last_error_at = NULL WHERE id = @id;

-- name: RecordIndexerError :exec
UPDATE indexers
SET last_error = @last_error, last_error_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now')
WHERE id = @id;

-- name: ClearIndexerError :exec
UPDATE indexers SET last_error = NULL, last_error_at = NULL WHERE id = @id;
