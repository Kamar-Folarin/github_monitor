-- +goose Up
-- SQL in this section is executed when the migration is applied

CREATE TABLE IF NOT EXISTS sync_progress (
    id SERIAL PRIMARY KEY,
    repo_url VARCHAR(255) NOT NULL UNIQUE,
    progress_json JSONB NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE
);

CREATE INDEX IF NOT EXISTS idx_sync_progress_repo_url ON sync_progress(repo_url);
CREATE INDEX IF NOT EXISTS idx_sync_progress_deleted_at ON sync_progress(deleted_at);

-- +goose Down
-- SQL in this section is executed when the migration is rolled back
DROP TABLE IF EXISTS sync_progress; 