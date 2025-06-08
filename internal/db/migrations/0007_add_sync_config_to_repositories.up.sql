-- +goose Up
ALTER TABLE repositories ADD COLUMN IF NOT EXISTS sync_config JSONB;

-- +goose Down
ALTER TABLE repositories DROP COLUMN IF EXISTS sync_config; 