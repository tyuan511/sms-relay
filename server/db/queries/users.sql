-- name: CreateUser :one
INSERT INTO users (id, password_hash, password_fingerprint)
VALUES (?, ?, ?)
RETURNING *;

-- name: GetUserByFingerprint :one
SELECT * FROM users WHERE password_fingerprint = ? LIMIT 1;

-- name: GetUserByID :one
SELECT * FROM users WHERE id = ? LIMIT 1;

-- name: UpdateUserPasswordFingerprint :exec
UPDATE users SET password_fingerprint = ? WHERE id = ?;
