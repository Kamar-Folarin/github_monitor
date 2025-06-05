-- +goose Up
-- SQL in this section is executed when the migration is applied

BEGIN;

CREATE TABLE repositories (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    url VARCHAR(255) NOT NULL,
    language VARCHAR(100),
    forks_count INTEGER NOT NULL DEFAULT 0,
    stars_count INTEGER NOT NULL DEFAULT 0,
    open_issues_count INTEGER NOT NULL DEFAULT 0,
    watchers_count INTEGER NOT NULL DEFAULT 0,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL,
    last_synced_at TIMESTAMP WITH TIME ZONE,
    CONSTRAINT unique_repo_url UNIQUE (url)
);

CREATE TABLE commits (
    id SERIAL PRIMARY KEY,
    repository_id INTEGER NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,
    sha VARCHAR(255) NOT NULL,
    message TEXT NOT NULL,
    author_name VARCHAR(255) NOT NULL,
    author_email VARCHAR(255) NOT NULL,
    author_date TIMESTAMP WITH TIME ZONE NOT NULL,
    commit_url VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT unique_commit_sha UNIQUE (sha)
);

CREATE INDEX idx_commits_repository_id ON commits(repository_id);
CREATE INDEX idx_commits_author_date ON commits(author_date);
CREATE INDEX idx_commits_author_name ON commits(author_name);

COMMIT;

-- +goose Down
-- SQL in this section is executed when the migration is rolled back
BEGIN;
DROP INDEX idx_commits_author_name;
DROP INDEX idx_commits_author_date;
DROP INDEX idx_commits_repository_id;
DROP TABLE commits;
DROP TABLE repositories;
COMMIT;