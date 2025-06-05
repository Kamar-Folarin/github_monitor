package db

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github-monitor/internal/models"

	_ "github.com/lib/pq"
	"github.com/pressly/goose/v3"
)

type PostgresStore struct {
	db *sql.DB
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

func (s *PostgresStore) GetRepositoryByURL(ctx context.Context, url string) (*models.Repository, error) {
	query := `
		SELECT id, name, description, url, language, forks_count, stars_count, 
			open_issues_count, watchers_count, created_at, updated_at, last_synced_at
		FROM repositories WHERE url = $1`

	var repo models.Repository
	var lastSyncedAt sql.NullTime
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
	)
	if err != nil {
		return nil, err
	}
	if lastSyncedAt.Valid {
		repo.LastSyncedAt = lastSyncedAt.Time
	} else {
		repo.LastSyncedAt = time.Time{}
	}

	return &repo, nil
}

func (s *PostgresStore) SaveRepository(ctx context.Context, repo *models.Repository) error {
	query := `
		INSERT INTO repositories (
			name, description, url, language, forks_count, stars_count, 
			open_issues_count, watchers_count, created_at, updated_at, last_synced_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		ON CONFLICT (url) DO UPDATE SET
			name = EXCLUDED.name,
			description = EXCLUDED.description,
			language = EXCLUDED.language,
			forks_count = EXCLUDED.forks_count,
			stars_count = EXCLUDED.stars_count,
			open_issues_count = EXCLUDED.open_issues_count,
			watchers_count = EXCLUDED.watchers_count,
			updated_at = EXCLUDED.updated_at,
			last_synced_at = EXCLUDED.last_synced_at
		RETURNING id`

	err := s.db.QueryRowContext(ctx, query,
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
		time.Now()).Scan(&repo.ID)

	return err
}

func (s *PostgresStore) GetLastCommitDate(ctx context.Context, repoID int) (*time.Time, error) {
	var lastDate time.Time
	err := s.db.QueryRowContext(ctx,
		"SELECT author_date FROM commits WHERE repository_id = $1 ORDER BY author_date DESC LIMIT 1",
		repoID).Scan(&lastDate)

	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, err
	}

	return &lastDate, nil
}

func (s *PostgresStore) SaveCommits(ctx context.Context, repoID int, commits []*models.Commit) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.PrepareContext(ctx, `
		INSERT INTO commits (repository_id, sha, message, author_name, author_email, author_date, commit_url)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		ON CONFLICT (sha) DO NOTHING`)
	if err != nil {
		return err
	}
	defer stmt.Close()

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
			return err
		}
	}

	return tx.Commit()
}

func (s *PostgresStore) GetTopCommitAuthors(ctx context.Context, repoID int, limit int) ([]*models.AuthorStats, error) {
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

func (s *PostgresStore) GetCommits(ctx context.Context, repoID int, limit, offset int) ([]*models.Commit, error) {
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

func (s *PostgresStore) ResetRepository(ctx context.Context, repoID int, since time.Time) error {
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