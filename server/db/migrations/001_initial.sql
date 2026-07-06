-- +goose Up
CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    password_hash TEXT NOT NULL,
    password_fingerprint TEXT NOT NULL UNIQUE,
    created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS devices (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    last_seen_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS destinations (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    platform TEXT NOT NULL DEFAULT 'telegram',
    config_nonce BLOB NOT NULL,
    config_enc BLOB NOT NULL,
    key_version INTEGER NOT NULL DEFAULT 1,
    enabled INTEGER NOT NULL DEFAULT 1,
    created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS inbound_messages (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    device_id TEXT REFERENCES devices(id) ON DELETE SET NULL,
    sender_nonce BLOB NOT NULL,
    sender_enc BLOB NOT NULL,
    body_nonce BLOB NOT NULL,
    body_enc BLOB NOT NULL,
    key_version INTEGER NOT NULL DEFAULT 1,
    received_at DATETIME NOT NULL,
    created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE IF NOT EXISTS forward_logs (
    id TEXT PRIMARY KEY,
    message_id TEXT NOT NULL REFERENCES inbound_messages(id) ON DELETE CASCADE,
    destination_id TEXT NOT NULL REFERENCES destinations(id) ON DELETE CASCADE,
    status TEXT NOT NULL,
    error TEXT,
    created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX IF NOT EXISTS idx_messages_user_received ON inbound_messages(user_id, received_at DESC);
CREATE INDEX IF NOT EXISTS idx_messages_device ON inbound_messages(device_id);
CREATE INDEX IF NOT EXISTS idx_forward_logs_message ON forward_logs(message_id);
CREATE INDEX IF NOT EXISTS idx_destinations_user ON destinations(user_id);

-- +goose Down
DROP TABLE IF EXISTS forward_logs;
DROP TABLE IF EXISTS inbound_messages;
DROP TABLE IF EXISTS destinations;
DROP TABLE IF EXISTS devices;
DROP TABLE IF EXISTS users;
