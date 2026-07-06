CREATE TABLE users (
    id TEXT PRIMARY KEY,
    password_hash TEXT NOT NULL,
    password_fingerprint TEXT NOT NULL UNIQUE,
    created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE devices (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    client_id TEXT,
    last_seen_at DATETIME,
    created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE TABLE destinations (
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

CREATE TABLE inbound_messages (
    id TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    device_id TEXT REFERENCES devices(id) ON DELETE SET NULL,
    client_message_id TEXT,
    sender_nonce BLOB NOT NULL,
    sender_enc BLOB NOT NULL,
    body_nonce BLOB NOT NULL,
    body_enc BLOB NOT NULL,
    key_version INTEGER NOT NULL DEFAULT 1,
    received_at DATETIME NOT NULL,
    created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE UNIQUE INDEX idx_messages_idempotent ON inbound_messages(user_id, device_id, client_message_id)
WHERE client_message_id IS NOT NULL;

CREATE TABLE forward_logs (
    id TEXT PRIMARY KEY,
    message_id TEXT NOT NULL REFERENCES inbound_messages(id) ON DELETE CASCADE,
    destination_id TEXT NOT NULL REFERENCES destinations(id) ON DELETE CASCADE,
    status TEXT NOT NULL,
    error TEXT,
    created_at DATETIME NOT NULL DEFAULT (datetime('now'))
);

CREATE INDEX idx_messages_user_received ON inbound_messages(user_id, received_at DESC);
CREATE INDEX idx_messages_device ON inbound_messages(device_id);
CREATE INDEX idx_forward_logs_message ON forward_logs(message_id);
CREATE INDEX idx_destinations_user ON destinations(user_id);
CREATE UNIQUE INDEX idx_devices_user_client_id ON devices(user_id, client_id)
WHERE client_id IS NOT NULL;
CREATE UNIQUE INDEX idx_forward_logs_message_destination ON forward_logs(message_id, destination_id);
