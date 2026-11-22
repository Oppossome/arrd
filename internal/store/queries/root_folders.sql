-- name: ListRootFolders :many
SELECT * FROM root_folders WHERE media_type = @media_type ORDER BY is_default DESC, path;

-- name: GetRootFolder :one
SELECT * FROM root_folders WHERE id = @id;

-- name: CreateRootFolder :one
INSERT INTO root_folders (media_type, path, is_default)
VALUES (@media_type, @path, @is_default)
RETURNING *;

-- name: ClearDefaultRootFolder :exec
UPDATE root_folders SET is_default = 0 WHERE media_type = @media_type AND is_default = 1;

-- name: SetDefaultRootFolder :execrows
UPDATE root_folders SET is_default = 1 WHERE id = @id;

-- name: DeleteRootFolder :execrows
DELETE FROM root_folders WHERE id = @id;
