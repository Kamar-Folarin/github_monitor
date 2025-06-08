package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	

	"github.com/Kamar-Folarin/github-monitor/internal/models"
)

// GetSyncStatus retrieves the sync status for a repository
func (s *PostgresStore) GetSyncStatus(ctx context.Context, repoURL string) (*models.SyncStatus, error) {
	var status models.SyncStatus
	var statusJSON []byte

	err := s.db.QueryRowContext(ctx, `
		SELECT status_json FROM sync_status 
		WHERE repo_url = $1 AND deleted_at IS NULL
	`, repoURL).Scan(&statusJSON)

	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to get sync status: %w", err)
	}

	if err := json.Unmarshal(statusJSON, &status); err != nil {
		return nil, fmt.Errorf("failed to unmarshal sync status: %w", err)
	}

	return &status, nil
}

// UpdateSyncStatus updates the sync status for a repository
func (s *PostgresStore) UpdateSyncStatus(ctx context.Context, status *models.SyncStatus) error {
	if status == nil {
		return fmt.Errorf("status cannot be nil")
	}

	statusJSON, err := json.Marshal(status)
	if err != nil {
		return fmt.Errorf("failed to marshal sync status: %w", err)
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO sync_status (repo_url, status_json, created_at, updated_at)
		VALUES ($1, $2, NOW(), NOW())
		ON CONFLICT (repo_url) DO UPDATE SET
			status_json = EXCLUDED.status_json,
			updated_at = NOW()
		WHERE sync_status.deleted_at IS NULL
	`, status.RepositoryURL, statusJSON)

	if err != nil {
		return fmt.Errorf("failed to update sync status: %w", err)
	}

	return nil
}

// ListSyncStatuses retrieves all sync statuses
func (s *PostgresStore) ListSyncStatuses(ctx context.Context) ([]*models.SyncStatus, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT status_json FROM sync_status 
		WHERE deleted_at IS NULL
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query sync statuses: %w", err)
	}
	defer rows.Close()

	var statuses []*models.SyncStatus
	for rows.Next() {
		var status models.SyncStatus
		var statusJSON []byte
		if err := rows.Scan(&statusJSON); err != nil {
			return nil, fmt.Errorf("failed to scan sync status row: %w", err)
		}

		if err := json.Unmarshal(statusJSON, &status); err != nil {
			return nil, fmt.Errorf("failed to unmarshal sync status: %w", err)
		}

		statuses = append(statuses, &status)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating sync status rows: %w", err)
	}

	return statuses, nil
}

// ClearSyncStatuses clears all sync statuses
func (s *PostgresStore) ClearSyncStatuses(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE sync_status 
		SET deleted_at = NOW()
		WHERE deleted_at IS NULL
	`)
	if err != nil {
		return fmt.Errorf("failed to clear sync statuses: %w", err)
	}
	return nil
}

// SaveSyncStatus saves the sync status for repositories
func (s *PostgresStore) SaveSyncStatus(ctx context.Context, status map[string]string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	query := `
		INSERT INTO sync_status (repo_url, status_json, created_at, updated_at)
		VALUES ($1, $2, NOW(), NOW())
		ON CONFLICT (repo_url) DO UPDATE
		SET status_json = $2,
			updated_at = NOW()
		WHERE sync_status.deleted_at IS NULL
	`

	stmt, err := tx.PrepareContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to prepare sync status statement: %w", err)
	}
	defer stmt.Close()

	for repoURL, statusJSON := range status {
		if _, err := stmt.ExecContext(ctx, repoURL, statusJSON); err != nil {
			return fmt.Errorf("failed to save sync status for %s: %w", repoURL, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit sync status transaction: %w", err)
	}

	return nil
}

// DeleteSyncStatus soft deletes the sync status for a repository
func (s *PostgresStore) DeleteSyncStatus(ctx context.Context, repoURL string) error {
	query := `
		UPDATE sync_status
		SET deleted_at = NOW()
		WHERE repo_url = $1 AND deleted_at IS NULL
	`

	result, err := s.db.ExecContext(ctx, query, repoURL)
	if err != nil {
		return fmt.Errorf("failed to delete sync status: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return fmt.Errorf("no sync status found for repository %s", repoURL)
	}

	return nil
}

// BeginTx starts a new transaction
func (s *PostgresStore) BeginTx(ctx context.Context) (*sql.Tx, error) {
	return s.db.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelReadCommitted,
		ReadOnly:  false,
	})
}

// SaveCommitsTx saves a batch of commits within a transaction
func (s *PostgresStore) SaveCommitsTx(ctx context.Context, tx *sql.Tx, commits []*models.Commit) error {
	if len(commits) == 0 {
		return nil
	}

	// Prepare the batch insert statement
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO commits (
			repository_id, sha, message, author_name, author_email, 
			author_date, commit_url
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7
		) ON CONFLICT (repository_id, sha) DO UPDATE SET
			message = EXCLUDED.message,
			author_name = EXCLUDED.author_name,
			author_email = EXCLUDED.author_email,
			author_date = EXCLUDED.author_date,
			commit_url = EXCLUDED.commit_url
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare commit insert statement: %w", err)
	}
	defer stmt.Close()

	// Execute batch insert
	for _, commit := range commits {
		_, err := stmt.ExecContext(ctx,
			commit.RepositoryID,
			commit.SHA,
			commit.Message,
			commit.AuthorName,
			commit.AuthorEmail,
			commit.AuthorDate,
			commit.CommitURL,
		)
		if err != nil {
			return fmt.Errorf("failed to insert commit %s: %w", commit.SHA, err)
		}
	}

	return nil
}

// SaveSyncStatusTx saves sync status within a transaction
func (s *PostgresStore) SaveSyncStatusTx(ctx context.Context, tx *sql.Tx, status map[string]string) error {
	if len(status) == 0 {
		return nil
	}

	// Prepare the batch upsert statement
	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO sync_status (
			repo_url, status_json, created_at, updated_at
		) VALUES (
			$1, $2, NOW(), NOW()
		) ON CONFLICT (repo_url) DO UPDATE SET
			status_json = EXCLUDED.status_json,
			updated_at = NOW()
		WHERE sync_status.deleted_at IS NULL
	`)
	if err != nil {
		return fmt.Errorf("failed to prepare sync status upsert statement: %w", err)
	}
	defer stmt.Close()

	// Execute batch upsert
	for repoURL, statusJSON := range status {
		_, err := stmt.ExecContext(ctx, repoURL, statusJSON)
		if err != nil {
			return fmt.Errorf("failed to upsert sync status for %s: %w", repoURL, err)
		}
	}

	return nil
}

// GetSyncProgress retrieves the sync progress for a repository
func (s *PostgresStore) GetSyncProgress(ctx context.Context, repoURL string) (*models.SyncProgress, error) {
	var progress models.SyncProgress
	var progressJSON []byte

	err := s.db.QueryRowContext(ctx, `
		SELECT progress_json FROM sync_progress 
		WHERE repo_url = $1 AND deleted_at IS NULL
	`, repoURL).Scan(&progressJSON)

	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("failed to get sync progress: %w", err)
	}

	if err := json.Unmarshal(progressJSON, &progress); err != nil {
		return nil, fmt.Errorf("failed to unmarshal sync progress: %w", err)
	}

	return &progress, nil
}

// SaveSyncProgress saves the sync progress for a repository
func (s *PostgresStore) SaveSyncProgress(ctx context.Context, repoURL string, progress *models.SyncProgress) error {
	progressJSON, err := json.Marshal(progress)
	if err != nil {
		return fmt.Errorf("failed to marshal sync progress: %w", err)
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO sync_progress (repo_url, progress_json, created_at, updated_at)
		VALUES ($1, $2, NOW(), NOW())
		ON CONFLICT (repo_url) DO UPDATE SET
			progress_json = EXCLUDED.progress_json,
			updated_at = EXCLUDED.updated_at
	`, repoURL, progressJSON)

	if err != nil {
		return fmt.Errorf("failed to save sync progress: %w", err)
	}

	return nil
}

// UpdateRepository updates an existing repository
func (s *PostgresStore) UpdateRepository(ctx context.Context, repo *models.Repository) error {
	if repo == nil {
		return fmt.Errorf("repository cannot be nil")
	}

	query := `
		UPDATE repositories 
		SET name = $1,
			description = $2,
			language = $3,
			forks_count = $4,
			stars_count = $5,
			open_issues_count = $6,
			watchers_count = $7,
			updated_at = NOW(),
			sync_config = $8
		WHERE id = $9
		RETURNING id`

	var syncConfigJSON []byte
	var err error
	if (repo.SyncConfig != models.RepositorySyncConfig{}) {
		syncConfigJSON, err = json.Marshal(repo.SyncConfig)
		if err != nil {
			return fmt.Errorf("failed to marshal sync config: %w", err)
		}
	}

	err = s.db.QueryRowContext(ctx, query,
		repo.Name,
		repo.Description,
		repo.Language,
		repo.ForksCount,
		repo.StarsCount,
		repo.OpenIssuesCount,
		repo.WatchersCount,
		syncConfigJSON,
		repo.ID,
	).Scan(&repo.ID)

	if err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("repository not found with id %d", repo.ID)
		}
		return fmt.Errorf("failed to update repository: %w", err)
	}

	return nil
}

func (s *PostgresStore) GetRepository(ctx context.Context, id int64) (*models.Repository, error) {
	query := `
		SELECT id, name, description, url, language, forks_count, stars_count, 
			open_issues_count, watchers_count, created_at, updated_at, last_synced_at,
			sync_config
		FROM repositories WHERE id = $1`

	var repo models.Repository
	var lastSyncedAt sql.NullTime
	var syncConfigJSON []byte
	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&repo.ID,
		&repo.Name,
		&repo.Description,
		&repo.URL,
		&repo.Language,
		&repo.ForksCount,
		&repo.StarsCount,
		&repo.OpenIssuesCount,
		&repo.WatchersCount,
		&repo.CreatedAt,
		&repo.UpdatedAt,
		&lastSyncedAt,
		&syncConfigJSON,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("repository not found with id %d", id)
		}
		return nil, fmt.Errorf("failed to get repository: %w", err)
	}

	if lastSyncedAt.Valid {
		repo.LastSyncedAt = lastSyncedAt.Time
	} else {
		repo.LastSyncedAt = time.Time{}
	}

	// Unmarshal sync config if present
	if syncConfigJSON != nil {
		if err := json.Unmarshal(syncConfigJSON, &repo.SyncConfig); err != nil {
			return nil, fmt.Errorf("failed to unmarshal sync config: %w", err)
		}
	}

	return &repo, nil
}

// ListRepositories retrieves all repositories
func (s *PostgresStore) ListRepositories(ctx context.Context) ([]*models.Repository, error) {
	query := `
		SELECT id, name, description, url, language, forks_count, stars_count, 
			open_issues_count, watchers_count, created_at, updated_at, last_synced_at,
			sync_config
		FROM repositories
		ORDER BY updated_at DESC`

	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to query repositories: %w", err)
	}
	defer rows.Close()

	var repos []*models.Repository
	for rows.Next() {
		var repo models.Repository
		var lastSyncedAt sql.NullTime
		var syncConfigJSON []byte
		if err := rows.Scan(
			&repo.ID,
			&repo.Name,
			&repo.Description,
			&repo.URL,
			&repo.Language,
			&repo.ForksCount,
			&repo.StarsCount,
			&repo.OpenIssuesCount,
			&repo.WatchersCount,
			&repo.CreatedAt,
			&repo.UpdatedAt,
			&lastSyncedAt,
			&syncConfigJSON,
		); err != nil {
			return nil, fmt.Errorf("failed to scan repository row: %w", err)
		}

		if lastSyncedAt.Valid {
			repo.LastSyncedAt = lastSyncedAt.Time
		} else {
			repo.LastSyncedAt = time.Time{}
		}

		// Unmarshal sync config if present
		if syncConfigJSON != nil {
			if err := json.Unmarshal(syncConfigJSON, &repo.SyncConfig); err != nil {
				return nil, fmt.Errorf("failed to unmarshal sync config: %w", err)
			}
		}

		repos = append(repos, &repo)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating repository rows: %w", err)
	}

	return repos, nil
}

func (s *PostgresStore) GetRepositoryByURL(ctx context.Context, url string) (*models.Repository, error) {
	query := `
		SELECT id, name, description, url, language, forks_count, stars_count, 
			open_issues_count, watchers_count, created_at, updated_at, last_synced_at,
			sync_config
		FROM repositories WHERE url = $1`

	var repo models.Repository
	var lastSyncedAt sql.NullTime
	var syncConfigJSON []byte
	err := s.db.QueryRowContext(ctx, query, url).Scan(
		&repo.ID,
		&repo.Name,
		&repo.Description,
		&repo.URL,
		&repo.Language,
		&repo.ForksCount,
		&repo.StarsCount,
		&repo.OpenIssuesCount,
		&repo.WatchersCount,
		&repo.CreatedAt,
		&repo.UpdatedAt,
		&lastSyncedAt,
		&syncConfigJSON,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("repository not found with URL %s", url)
		}
		return nil, fmt.Errorf("failed to get repository: %w", err)
	}

	if lastSyncedAt.Valid {
		repo.LastSyncedAt = lastSyncedAt.Time
	} else {
		repo.LastSyncedAt = time.Time{}
	}

	// Unmarshal sync config if present
	if syncConfigJSON != nil {
		if err := json.Unmarshal(syncConfigJSON, &repo.SyncConfig); err != nil {
			return nil, fmt.Errorf("failed to unmarshal sync config: %w", err)
		}
	}

	return &repo, nil
}

func (s *PostgresStore) GetLastCommitDate(ctx context.Context, repoID int64) (*time.Time, error) {
	// Use a more efficient query with proper indexing
	query := `
		SELECT author_date 
		FROM commits 
		WHERE repository_id = $1 
		ORDER BY author_date DESC 
		LIMIT 1 
		FOR UPDATE SKIP LOCKED`

	var lastDate sql.NullTime
	err := s.db.QueryRowContext(ctx, query, repoID).Scan(&lastDate)

	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		if ctx.Err() != nil {
			return nil, fmt.Errorf("context canceled while getting last commit date: %w", ctx.Err())
		}
		return nil, fmt.Errorf("failed to get last commit date: %w", err)
	}

	if !lastDate.Valid {
		return nil, nil
	}

	return &lastDate.Time, nil
}

func (s *PostgresStore) SaveRepository(ctx context.Context, repo *models.Repository) error {
	syncConfigJSON, err := json.Marshal(repo.SyncConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal sync config: %w", err)
	}

	query := `
		INSERT INTO repositories (
			name, description, url, language, forks_count, stars_count, 
			open_issues_count, watchers_count, created_at, updated_at, last_synced_at,
			sync_config
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)
		ON CONFLICT (url) DO UPDATE SET
			name = EXCLUDED.name,
			description = EXCLUDED.description,
			language = EXCLUDED.language,
			forks_count = EXCLUDED.forks_count,
			stars_count = EXCLUDED.stars_count,
			open_issues_count = EXCLUDED.open_issues_count,
			watchers_count = EXCLUDED.watchers_count,
			updated_at = EXCLUDED.updated_at,
			last_synced_at = EXCLUDED.last_synced_at,
			sync_config = EXCLUDED.sync_config
		RETURNING id`

	err = s.db.QueryRowContext(ctx, query,
		repo.Name,
		repo.Description,
		repo.URL,
		repo.Language,
		repo.ForksCount,
		repo.StarsCount,
		repo.OpenIssuesCount,
		repo.WatchersCount,
		repo.CreatedAt,
		repo.UpdatedAt,
		repo.LastSyncedAt,
		syncConfigJSON).Scan(&repo.ID)

	return err
} 