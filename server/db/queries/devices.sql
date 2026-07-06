-- name: CreateDevice :one
INSERT INTO devices (id, user_id, name, client_id)
VALUES (?, ?, ?, ?)
RETURNING *;

-- name: GetDeviceByID :one
SELECT * FROM devices WHERE id = ? LIMIT 1;

-- name: GetDeviceByIDAndUser :one
SELECT * FROM devices WHERE id = ? AND user_id = ? LIMIT 1;

-- name: GetDeviceByUserAndClientID :one
SELECT * FROM devices WHERE user_id = ? AND client_id = ? LIMIT 1;

-- name: GetDeviceByUserAndName :one
SELECT * FROM devices WHERE user_id = ? AND name = ? LIMIT 1;

-- name: UpdateDeviceLastSeen :exec
UPDATE devices SET last_seen_at = ? WHERE id = ?;

-- name: UpdateDeviceClientID :exec
UPDATE devices SET client_id = ? WHERE id = ? AND user_id = ?;

-- name: ListDevicesByUser :many
SELECT * FROM devices WHERE user_id = ? ORDER BY created_at DESC;
