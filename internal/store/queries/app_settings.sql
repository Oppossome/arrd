-- name: GetAppSettings :one
SELECT * FROM app_settings WHERE id = 1;

-- name: CreateAppSettings :one
INSERT INTO app_settings (id, api_key, tmdb_api_key, password_hash)
VALUES (1, @api_key, @tmdb_api_key, @password_hash)
RETURNING *;

-- name: SetTMDBAPIKey :exec
UPDATE app_settings SET tmdb_api_key = @tmdb_api_key WHERE id = 1;

-- name: SetAPIKey :exec
UPDATE app_settings SET api_key = @api_key WHERE id = 1;

-- name: SetPasswordHash :exec
UPDATE app_settings SET password_hash = @password_hash WHERE id = 1;
