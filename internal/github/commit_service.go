package github

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/Kamar-Folarin/github-monitor/internal/batch"
	"github.com/Kamar-Folarin/github-monitor/internal/config"
	"github.com/Kamar-Folarin/github-monitor/internal/db"
	"github.com/Kamar-Folarin/github-monitor/internal/errors"
	"github.com/Kamar-Folarin/github-monitor/internal/models"
)

// CommitServiceClient defines the GitHub API client interface for commit operations
type CommitServiceClient interface {
	GetCommits(ctx context.Context, owner, name string, since time.Time, batchSize int, callback func([]*models.Commit) error) error
	GetCommitAuthors(ctx context.Context, owner, name string) ([]*models.CommitAuthor, error)
}

// CommitServiceImpl implements the CommitService interface
type CommitServiceImpl struct {
	client    CommitServiceClient
	store     db.Store
	config    *config.SyncConfig
	processor *batch.Processor
}

// NewCommitService creates a new commit service
func NewCommitService(client CommitServiceClient, store db.Store, cfg *config.SyncConfig) CommitService {
	return &CommitServiceImpl{
		client:    client,
		store:     store,
		config:    cfg,
		processor: batch.NewProcessor(&cfg.BatchConfig),
	}
}

// GetCommits retrieves commits for a repository
func (s *CommitServiceImpl) GetCommits(ctx context.Context, repoURL string, page, perPage int) ([]*models.Commit, error) {
	repo, err := s.store.GetRepositoryByURL(ctx, repoURL)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, errors.NewNotFoundError(fmt.Sprintf("repository not found: %s", repoURL), err)
		}
		return nil, err
	}

	// Get commits from database
	commits, err := s.store.GetCommits(ctx, int64(repo.ID), page, perPage)
	if err != nil {
		return nil, fmt.Errorf("failed to get commits: %w", err)
	}

	return commits, nil
}

// GetCommitsWithPagination retrieves commits for a repository with pagination and date filtering
func (s *CommitServiceImpl) GetCommitsWithPagination(ctx context.Context, repoURL string, limit, offset int, since, until *time.Time) ([]*models.Commit, int64, error) {
	repo, err := s.store.GetRepositoryByURL(ctx, repoURL)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, 0, errors.NewNotFoundError(fmt.Sprintf("repository not found: %s", repoURL), err)
		}
		return nil, 0, err
	}

	commits, total, err := s.store.GetCommitsWithPagination(ctx, int64(repo.ID), limit, offset, since, until)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to get commits: %w", err)
	}

	return commits, total, nil
}

// GetTopCommitAuthors gets the top commit authors for a repository with optional date filtering
func (s *CommitServiceImpl) GetTopCommitAuthors(ctx context.Context, repoURL string, limit int, since, until *time.Time) ([]*models.AuthorStats, error) {
	repo, err := s.store.GetRepositoryByURL(ctx, repoURL)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, errors.NewNotFoundError(fmt.Sprintf("repository not found: %s", repoURL), err)
		}
		return nil, err
	}

	authors, err := s.store.GetTopCommitAuthorsWithDateFilter(ctx, int64(repo.ID), limit, since, until)
	if err != nil {
		return nil, fmt.Errorf("failed to get commit authors: %w", err)
	}

	stats := make([]*models.AuthorStats, len(authors))
	for i, author := range authors {
		stats[i] = &models.AuthorStats{
			Name:          author.Name,
			Email:         author.Email,
			CommitCount:   author.CommitCount,
			FirstCommitAt: author.FirstCommitAt,
			LastCommitAt:  author.LastCommitAt,
			RepositoryID:  author.RepositoryID,
		}
	}

	sort.Slice(stats, func(i, j int) bool {
		return stats[i].CommitCount > stats[j].CommitCount
	})

	return stats, nil
}

// GetLastCommitDate retrieves the date of the last commit for a repository
func (s *CommitServiceImpl) GetLastCommitDate(ctx context.Context, repoURL string) (*time.Time, error) {
	repo, err := s.store.GetRepositoryByURL(ctx, repoURL)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, errors.NewNotFoundError(fmt.Sprintf("repository not found: %s", repoURL), err)
		}
		return nil, err
	}

	lastDate, err := s.store.GetLastCommitDate(ctx, int64(repo.ID))
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get last commit date: %w", err)
	}

	return lastDate, nil
}

// SyncCommits synchronizes commits for a repository
func (s *CommitServiceImpl) SyncCommits(ctx context.Context, repoURL string, since time.Time, batchSize int) error {
	repo, err := s.store.GetRepositoryByURL(ctx, repoURL)
	if err != nil {
		if errors.IsNotFound(err) {
			return errors.NewNotFoundError(fmt.Sprintf("repository not found: %s", repoURL), err)
		}
		return err
	}

	owner, name, err := parseRepoURL(repoURL)
	if err != nil {
		return errors.NewValidationError("invalid repository URL format", err)
	}

	if batchSize <= 0 {
		batchSize = s.config.BatchSize
	}

	err = s.client.GetCommits(ctx, owner, name, since, batchSize, func(commits []*models.Commit) error {
		return s.processor.ProcessItems(ctx, convertToInterfaceSlice(commits), func(ctx context.Context, batch []interface{}) error {
			commits := make([]*models.Commit, len(batch))
			for i, item := range batch {
				commits[i] = item.(*models.Commit)
			}
			return s.store.SaveCommits(ctx, int64(repo.ID), commits)
		})
	})

	if err != nil {
		return fmt.Errorf("failed to process commits: %w", err)
	}

	return nil
}

// convertToInterfaceSlice converts a slice of commits to a slice of interfaces
func convertToInterfaceSlice(commits []*models.Commit) []interface{} {
	result := make([]interface{}, len(commits))
	for i, commit := range commits {
		result[i] = commit
	}
	return result
}

// GetCommitProgress returns the current commit sync progress
func (s *CommitServiceImpl) GetCommitProgress() <-chan *models.BatchProgress {
	return s.processor.GetProgress()
} 