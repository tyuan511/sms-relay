-- name: CreateMessage :one
INSERT INTO inbound_messages (
    id, user_id, device_id, client_message_id, sender_nonce, sender_enc, body_nonce, body_enc, key_version, received_at
) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
RETURNING *;

-- name: GetMessageByClientID :one
SELECT * FROM inbound_messages
WHERE user_id = ? AND device_id = ? AND client_message_id = ?
LIMIT 1;

-- name: ListMessagesByUser :many
SELECT * FROM inbound_messages
WHERE user_id = ?
ORDER BY received_at DESC
LIMIT ? OFFSET ?;

-- name: GetMessageByID :one
SELECT * FROM inbound_messages WHERE id = ? LIMIT 1;

-- name: CreateForwardLog :one
INSERT INTO forward_logs (id, message_id, destination_id, status, error)
VALUES (?, ?, ?, ?, ?)
RETURNING *;

-- name: GetForwardLogByMessageDestination :one
SELECT * FROM forward_logs
WHERE message_id = ? AND destination_id = ?
LIMIT 1;

-- name: UpdateForwardLog :exec
UPDATE forward_logs
SET status = ?, error = ?
WHERE id = ?;

-- name: ListForwardLogsByMessage :many
SELECT * FROM forward_logs WHERE message_id = ? ORDER BY created_at ASC;
