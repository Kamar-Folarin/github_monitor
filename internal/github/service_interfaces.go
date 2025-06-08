package github

import (
	"context"
	"time"

	"github.com/Kamar-Folarin/github-monitor/internal/models"
)

// CommitService defines the interface for commit operations
type CommitService interface {
	// GetCommits retrieves commits for a repository with pagination
	GetCommits(ctx context.Context, repoURL string, page, perPage int) ([]*models.Commit, error)

	// GetCommitsWithPagination retrieves commits for a repository with pagination and date filtering
	GetCommitsWithPagination(ctx context.Context, repoURL string, limit, offset int, since, until *time.Time) ([]*models.Commit, int64, error)

	// GetTopCommitAuthors gets the top commit authors for a repository with optional date filtering
	GetTopCommitAuthors(ctx context.Context, repoURL string, limit int, since, until *time.Time) ([]*models.AuthorStats, error)

	// GetLastCommitDate gets the date of the last commit for a repository
	GetLastCommitDate(ctx context.Context, repoURL string) (*time.Time, error)

	// SyncCommits synchronizes commits for a repository from a given date with a specific batch size
	SyncCommits(ctx context.Context, repoURL string, since time.Time, batchSize int) error

	// GetCommitProgress gets the current progress of commit synchronization
	GetCommitProgress() <-chan *models.BatchProgress
}

// SyncService defines the interface for sync operations
type SyncService interface {
	// StartSync starts the background sync process
	StartSync(ctx context.Context, repoURL string) error

	// StopSync stops the background sync process
	StopSync(ctx context.Context, repoURL string) error

	// GetSyncStatus gets the current sync status for a repository
	GetSyncStatus(ctx context.Context, repoURL string) (*models.SyncStatus, error)

	// ResetSync resets the sync process for a repository from a given date
	ResetSync(ctx context.Context, repoURL string, since time.Time) error

	// ForceClearSyncStatus forcefully clears a stuck sync status
	ForceClearSyncStatus(ctx context.Context, repoURL string) error
}

// StatusManager defines the interface for status management
type StatusManager interface {
	// GetStatus gets the current sync status for a repository
	GetStatus(ctx context.Context, repoURL string) (*models.SyncStatus, error)

	// UpdateStatus updates the sync status for a repository
	UpdateStatus(ctx context.Context, status *models.SyncStatus) error

	// DeleteStatus removes the sync status for a repository
	DeleteStatus(ctx context.Context, repoURL string) error

	// ListStatuses lists all sync statuses for repositories
	ListStatuses(ctx context.Context) ([]*models.SyncStatus, error)

	// ClearStatuses clears all sync statuses
	ClearStatuses(ctx context.Context) error

	// GetAllStatuses gets the sync status for all repositories
	GetAllStatuses(ctx context.Context) ([]*models.SyncStatus, error)
}

// BatchProcessor defines the interface for batch processing
type BatchProcessor interface {
	// ProcessItems processes items in batches
	ProcessItems(ctx context.Context, items []interface{}, processFn func(context.Context, []interface{}) error) error

	// GetProgress gets the current progress of batch processing
	GetProgress() <-chan *models.BatchProgress
} 