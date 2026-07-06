-- +goose Up
ALTER TABLE devices ADD COLUMN client_id TEXT;

CREATE UNIQUE INDEX IF NOT EXISTS idx_devices_user_client_id
ON devices(user_id, client_id)
WHERE client_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS idx_forward_logs_message_destination
ON forward_logs(message_id, destination_id);

-- +goose Down
DROP INDEX IF EXISTS idx_forward_logs_message_destination;
DROP INDEX IF EXISTS idx_devices_user_client_id;
