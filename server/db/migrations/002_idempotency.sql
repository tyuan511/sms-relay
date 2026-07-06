-- +goose Up
ALTER TABLE inbound_messages ADD COLUMN client_message_id TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS idx_messages_idempotent
ON inbound_messages(user_id, device_id, client_message_id)
WHERE client_message_id IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_messages_idempotent;
