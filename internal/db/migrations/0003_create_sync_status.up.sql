-- +goose Up
-- SQL in this section is executed when the migration is applied

CREATE TABLE IF NOT EXISTS sync_status (
    id SERIAL PRIMARY KEY,
    repo_url VARCHAR(255) NOT NULL UNIQUE,
    status_json JSONB NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS idx_sync_status_repo_url ON sync_status(repo_url);
CREATE INDEX IF NOT EXISTS idx_sync_status_deleted_at ON sync_status(deleted_at);

-- +goose Down
-- SQL in this section is executed when the migration is rolled back
DROP TABLE IF EXISTS sync_status; 