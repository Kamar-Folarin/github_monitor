-- +goose Up
-- SQL in this section is executed when the migration is applied

BEGIN;

CREATE TABLE IF NOT EXISTS monitored_repositories (
    id SERIAL PRIMARY KEY,
    repository_id INTEGER NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    initial_sync_date TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    UNIQUE(repository_id)
);

-- Add index on repository_id for faster lookups
CREATE INDEX IF NOT EXISTS idx_monitored_repositories_repository_id ON monitored_repositories(repository_id);

-- Add index on deleted_at for soft delete queries
CREATE INDEX IF NOT EXISTS idx_monitored_repositories_deleted_at ON monitored_repositories(deleted_at);

-- Add unique partial index to ensure only one active record per repository
CREATE UNIQUE INDEX IF NOT EXISTS idx_monitored_repositories_unique_active 
    ON monitored_repositories(repository_id) 
    WHERE deleted_at IS NULL;

COMMIT;

-- +goose Down
-- SQL in this section is executed when the migration is rolled back

BEGIN;

DROP INDEX IF EXISTS idx_monitored_repositories_unique_active;
DROP INDEX IF EXISTS idx_monitored_repositories_deleted_at;
DROP INDEX IF EXISTS idx_monitored_repositories_repository_id;
DROP TABLE IF EXISTS monitored_repositories;

COMMIT; 