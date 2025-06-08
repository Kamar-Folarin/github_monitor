package db

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"


	"github.com/Kamar-Folarin/github-monitor/internal/models"
	"github.com/Kamar-Folarin/github-monitor/internal/utils"

	_ "github.com/lib/pq"
	"github.com/pressly/goose/v3"
)

type PostgresStore struct {
	db *sql.DB
}

// Store defines the interface for database operations
type Store interface {
	// Repository operations
	GetRepository(ctx context.Context, id int64) (*models.Repository, error)
	GetRepositoryByURL(ctx context.Context, url string) (*models.Repository, error)
	ListRepositories(ctx context.Context) ([]*models.Repository, error)
	SaveRepository(ctx context.Context, repo *models.Repository) error
	UpdateRepository(ctx context.Context, repo *models.Repository) error
	DeleteRepository(ctx context.Context, id int64) error
	AddMonitoredRepository(ctx context.Context, repoURL string, initialSyncDate *time.Time) error

	// Commit operations
	GetCommits(ctx context.Context, repoID int64, limit, offset int) ([]*models.Commit, error)
	GetCommitsWithPagination(ctx context.Context, repoID int64, limit, offset int, since, until *time.Time) ([]*models.Commit, int64, error)
	SaveCommits(ctx context.Context, repoID int64, commits []*models.Commit) error
	SaveCommitsTx(ctx context.Context, tx *sql.Tx, commits []*models.Commit) error
	GetLastCommitDate(ctx context.Context, repoID int64) (*time.Time, error)
	DeleteCommits(ctx context.Context, repoID int64) error
	GetTopCommitAuthors(ctx context.Context, repoID int64, limit int) ([]*models.AuthorStats, error)
	GetTopCommitAuthorsWithDateFilter(ctx context.Context, repoID int64, limit int, since, until *time.Time) ([]*models.AuthorStats, error)
	ResetRepository(ctx context.Context, repoID int64, since time.Time) error

	// Sync operations
	GetSyncStatus(ctx context.Context, repoURL string) (*models.SyncStatus, error)
	UpdateSyncStatus(ctx context.Context, status *models.SyncStatus) error
	ListSyncStatuses(ctx context.Context) ([]*models.SyncStatus, error)
	ClearSyncStatuses(ctx context.Context) error
	SaveSyncStatus(ctx context.Context, status map[string]string) error
	DeleteSyncStatus(ctx context.Context, repoURL string) error
	GetSyncProgress(ctx context.Context, repoURL string) (*models.SyncProgress, error)
	SaveSyncProgress(ctx context.Context, repoURL string, progress *models.SyncProgress) error

	// Transaction operations
	BeginTx(ctx context.Context) (*sql.Tx, error)
	SaveSyncStatusTx(ctx context.Context, tx *sql.Tx, status map[string]string) error
}

func NewPostgresStore(connectionString string) (*PostgresStore, error) {
	db, err := sql.Open("postgres", connectionString)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &PostgresStore{db: db}, nil
}

func (s *PostgresStore) Migrate() error {
	if err := goose.SetDialect("postgres"); err != nil {
		return err
	}

	if err := goose.Up(s.db, "internal/db/migrations"); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

func (s *PostgresStore) SaveCommits(ctx context.Context, repoID int64, commits []*models.Commit) error {
	log.Printf("[DB] Starting to save %d commits for repository ID %d", len(commits), repoID)
	
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		log.Printf("[DB] Failed to begin transaction: %v", err)
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO commits (repository_id, sha, message, author_name, author_email, author_date, commit_url)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (repository_id, sha) DO UPDATE SET
			message = EXCLUDED.message,
			author_name = EXCLUDED.author_name,
			author_email = EXCLUDED.author_email,
			author_date = EXCLUDED.author_date,
			commit_url = EXCLUDED.commit_url,
			updated_at = NOW()`)
	if err != nil {
		log.Printf("[DB] Failed to prepare statement: %v", err)
		return err
	}
	defer stmt.Close()

	savedCount := 0
	for _, commit := range commits {
		_, err := stmt.ExecContext(ctx,
			repoID,
			commit.SHA,
			commit.Message,
			commit.AuthorName,
			commit.AuthorEmail,
			commit.AuthorDate,
			commit.CommitURL)
		if err != nil {
			log.Printf("[DB] Failed to save commit %s: %v", commit.SHA, err)
			return err
		}
		savedCount++
		if savedCount%1000 == 0 {
			log.Printf("[DB] Saved %d commits so far...", savedCount)
		}
	}

	log.Printf("[DB] Committing transaction for %d commits", savedCount)
	if err := tx.Commit(); err != nil {
		log.Printf("[DB] Failed to commit transaction: %v", err)
		return err
	}
	
	log.Printf("[DB] Successfully saved %d commits for repository ID %d", savedCount, repoID)
	return nil
}

func (s *PostgresStore) GetTopCommitAuthors(ctx context.Context, repoID int64, limit int) ([]*models.AuthorStats, error) {
	query := `
		SELECT 
			author_name, 
			author_email, 
			COUNT(*) as commit_count
		FROM 
			commits
		WHERE 
			repository_id = $1
		GROUP BY 
			author_name, author_email
		ORDER BY 
			commit_count DESC
		LIMIT $2`

	rows, err := s.db.QueryContext(ctx, query, repoID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var authors []*models.AuthorStats
	for rows.Next() {
		var a models.AuthorStats
		if err := rows.Scan(&a.Name, &a.Email, &a.CommitCount); err != nil {
			return nil, err
		}
		authors = append(authors, &a)
	}

	return authors, nil
}

func (s *PostgresStore) GetCommits(ctx context.Context, repoID int64, limit, offset int) ([]*models.Commit, error) {
	query := `
		SELECT 
			sha, message, author_name, author_email, author_date, commit_url
		FROM 
			commits
		WHERE 
			repository_id = $1
		ORDER BY 
			author_date DESC
		LIMIT $2 OFFSET $3`

	rows, err := s.db.QueryContext(ctx, query, repoID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var commits []*models.Commit
	for rows.Next() {
		var c models.Commit
		if err := rows.Scan(&c.SHA, &c.Message, &c.AuthorName, &c.AuthorEmail, &c.AuthorDate, &c.CommitURL); err != nil {
			return nil, err
		}
		commits = append(commits, &c)
	}

	return commits, nil
}

func (s *PostgresStore) ResetRepository(ctx context.Context, repoID int64, since time.Time) error {
	_, err := s.db.ExecContext(ctx, 
		"DELETE FROM commits WHERE repository_id = $1 AND author_date >= $2", 
		repoID, since)
	if err != nil {
		return err
	}

	_, err = s.db.ExecContext(ctx,
		"UPDATE repositories SET last_synced_at = NULL WHERE id = $1",
		repoID)
	
	return err
}

// DeleteCommits deletes all commits for a repository
func (s *PostgresStore) DeleteCommits(ctx context.Context, repoID int64) error {
	_, err := s.db.ExecContext(ctx, 
		"DELETE FROM commits WHERE repository_id = $1", 
		repoID)
	return err
}

// DeleteRepository deletes a repository and all its associated data
func (s *PostgresStore) DeleteRepository(ctx context.Context, id int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	_, err = tx.ExecContext(ctx, "DELETE FROM commits WHERE repository_id = $1", id)
	if err != nil {
		return fmt.Errorf("failed to delete commits: %w", err)
	}

	_, err = tx.ExecContext(ctx, "DELETE FROM repositories WHERE id = $1", id)
	if err != nil {
		return fmt.Errorf("failed to delete repository: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// GetRepository retrieves a repository by its ID
// func (s *PostgresStore) GetRepository(ctx context.Context, id int64) (*models.Repository, error) {
//     ... implementation removed ...
// }

// ListRepositories retrieves all repositories
// func (s *PostgresStore) ListRepositories(ctx context.Context) ([]*models.Repository, error) {
//     ... implementation removed ...
// }

// AddMonitoredRepository adds a repository to the monitored repositories list
func (s *PostgresStore) AddMonitoredRepository(ctx context.Context, repoURL string, initialSyncDate *time.Time) error {
	repo, err := s.GetRepositoryByURL(ctx, repoURL)
	if err != nil && err != sql.ErrNoRows {
		return fmt.Errorf("failed to check repository existence: %w", err)
	}

	if repo == nil {

		if _, _, err := utils.ParseRepoURL(repoURL); err != nil {
			return fmt.Errorf("invalid repository URL: %w", err)
		}


		return fmt.Errorf("repository not found: %s", repoURL)
	}

	query := `
		INSERT INTO monitored_repositories (repository_id, initial_sync_date, created_at, updated_at)
		VALUES ($1, $2, NOW(), NOW())
		ON CONFLICT (repository_id) DO UPDATE SET
			initial_sync_date = COALESCE(EXCLUDED.initial_sync_date, monitored_repositories.initial_sync_date),
			updated_at = NOW()
		WHERE monitored_repositories.deleted_at IS NULL`

	var syncDate interface{}
	if initialSyncDate != nil {
		syncDate = *initialSyncDate
	}

	_, err = s.db.ExecContext(ctx, query, repo.ID, syncDate)
	if err != nil {
		return fmt.Errorf("failed to add monitored repository: %w", err)
	}

	return nil
}

// GetCommitsWithPagination retrieves commits with pagination and date filtering
func (s *PostgresStore) GetCommitsWithPagination(ctx context.Context, repoID int64, limit, offset int, since, until *time.Time) ([]*models.Commit, int64, error) {
	
	baseQuery := `
		SELECT 
			id, repository_id, sha, message, author_name, author_email, 
			author_date, commit_url, created_at, updated_at
		FROM commits 
		WHERE repository_id = $1`

	
	args := []interface{}{repoID}
	argCount := 1

	if since != nil {
		argCount++
		baseQuery += fmt.Sprintf(" AND author_date >= $%d", argCount)
		args = append(args, *since)
	}

	if until != nil {
		argCount++
		baseQuery += fmt.Sprintf(" AND author_date <= $%d", argCount)
		args = append(args, *until)
	}

	
	countQuery := fmt.Sprintf("SELECT COUNT(*) FROM (%s) as count_query", baseQuery)
	var total int64
	err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get total count: %w", err)
	}

	
	argCount++
	baseQuery += fmt.Sprintf(" ORDER BY author_date DESC LIMIT $%d OFFSET $%d", argCount, argCount+1)
	args = append(args, limit, offset)

	
	rows, err := s.db.QueryContext(ctx, baseQuery, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to query commits: %w", err)
	}
	defer rows.Close()

	var commits []*models.Commit
	for rows.Next() {
		var c models.Commit
		if err := rows.Scan(
			&c.ID,
			&c.RepositoryID,
			&c.SHA,
			&c.Message,
			&c.AuthorName,
			&c.AuthorEmail,
			&c.AuthorDate,
			&c.CommitURL,
			&c.CreatedAt,
			&c.UpdatedAt,
		); err != nil {
			return nil, 0, fmt.Errorf("failed to scan commit: %w", err)
		}
		commits = append(commits, &c)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("error iterating commits: %w", err)
	}

	return commits, total, nil
}

// GetTopCommitAuthorsWithDateFilter retrieves top commit authors with date filtering
func (s *PostgresStore) GetTopCommitAuthorsWithDateFilter(ctx context.Context, repoID int64, limit int, since, until *time.Time) ([]*models.AuthorStats, error) {
	
	baseQuery := `
		SELECT 
			author_name, 
			author_email, 
			COUNT(*) as commit_count,
			MIN(author_date) as first_commit_at,
			MAX(author_date) as last_commit_at
		FROM 
			commits
		WHERE 
			repository_id = $1`

	
	args := []interface{}{repoID}
	argCount := 1

	if since != nil {
		argCount++
		baseQuery += fmt.Sprintf(" AND author_date >= $%d", argCount)
		args = append(args, *since)
	}

	if until != nil {
		argCount++
		baseQuery += fmt.Sprintf(" AND author_date <= $%d", argCount)
		args = append(args, *until)
	}

	
	argCount++
	baseQuery += `
		GROUP BY 
			author_name, author_email
		ORDER BY 
			commit_count DESC
		LIMIT $` + fmt.Sprintf("%d", argCount)
	args = append(args, limit)

	
	rows, err := s.db.QueryContext(ctx, baseQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query commit authors: %w", err)
	}
	defer rows.Close()

	var authors []*models.AuthorStats
	for rows.Next() {
		var a models.AuthorStats
		var firstCommitAt, lastCommitAt sql.NullTime
		if err := rows.Scan(
			&a.Name,
			&a.Email,
			&a.CommitCount,
			&firstCommitAt,
			&lastCommitAt,
		); err != nil {
			return nil, fmt.Errorf("failed to scan author stats: %w", err)
		}
		if firstCommitAt.Valid {
			a.FirstCommitAt = firstCommitAt.Time
		}
		if lastCommitAt.Valid {
			a.LastCommitAt = lastCommitAt.Time
		}
		a.RepositoryID = repoID
		authors = append(authors, &a)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating author stats: %w", err)
	}

	return authors, nil
}