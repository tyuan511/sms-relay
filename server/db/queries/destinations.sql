-- name: CreateDestination :one
INSERT INTO destinations (id, user_id, name, platform, config_nonce, config_enc, key_version, enabled)
VALUES (?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetDestinationByID :one
SELECT * FROM destinations WHERE id = ? LIMIT 1;

-- name: ListDestinationsByUser :many
SELECT * FROM destinations WHERE user_id = ? ORDER BY created_at DESC;

-- name: UpdateDestination :exec
UPDATE destinations
SET name = ?, enabled = ?, config_nonce = ?, config_enc = ?, key_version = ?
WHERE id = ? AND user_id = ?;

-- name: DeleteDestination :exec
DELETE FROM destinations WHERE id = ? AND user_id = ?;
