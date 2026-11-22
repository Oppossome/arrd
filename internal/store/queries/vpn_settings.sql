-- name: GetVPNSettings :one
SELECT * FROM vpn_settings WHERE id = 1;

-- name: UpsertVPNSettings :exec
INSERT INTO vpn_settings (id, provider, settings)
VALUES (1, @provider, @settings)
ON CONFLICT (id) DO UPDATE SET
    provider = excluded.provider,
    settings = excluded.settings;

-- name: DeleteVPNSettings :exec
DELETE FROM vpn_settings WHERE id = 1;
