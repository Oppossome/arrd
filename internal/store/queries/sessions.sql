-- name: CreateSession :exec
INSERT INTO sessions (id_hash, created_at, last_seen_at, expires_at)
VALUES (@id_hash, CAST(@now AS TEXT), CAST(@now AS TEXT), @expires_at);

-- name: GetValidSession :one
SELECT * FROM sessions WHERE id_hash = @id_hash AND expires_at > CAST(@now AS TEXT);

-- name: TouchSession :exec
UPDATE sessions SET last_seen_at = CAST(@now AS TEXT), expires_at = @expires_at WHERE id_hash = @id_hash;

-- name: DeleteSession :exec
DELETE FROM sessions WHERE id_hash = @id_hash;

-- name: DeleteOtherSessions :exec
DELETE FROM sessions WHERE id_hash != @keep_id_hash;

-- name: DeleteExpiredSessions :execrows
DELETE FROM sessions WHERE expires_at <= CAST(@now AS TEXT);
