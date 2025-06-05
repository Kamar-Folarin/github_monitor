package github

import (
	"context"
	"fmt"
	"time"

	"github-monitor/internal/db"
	"github-monitor/internal/models"

	"github.com/sirupsen/logrus"
)

type Service struct {
	client *GitHubClient
	store  *db.PostgresStore
	logger *logrus.Logger
}

func NewService(token string, store *db.PostgresStore, logger *logrus.Logger) *Service {
	return &Service{
		client: NewGitHubClient(token),
		store:  store,
		logger: logger,
	}
}

func (s *Service) StartSync(ctx context.Context, interval time.Duration, repoURL string) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	if err := s.SyncRepository(ctx, repoURL); err != nil {
		s.logger.Errorf("Initial sync failed: %v", err)
	}

	for {
		select {
		case <-ticker.C:
			if err := s.SyncRepository(ctx, repoURL); err != nil {
				s.logger.Errorf("Sync failed: %v", err)
			}
		case <-ctx.Done():
			s.logger.Info("Stopping sync service")
			return
		}
	}
}

func (s *Service) SyncRepository(ctx context.Context, repoURL string) error {
	owner, name, err := ParseRepoURL(repoURL)
	if err != nil {
		return fmt.Errorf("invalid repository URL: %w", err)
	}

	repo, err := s.getOrCreateRepository(ctx, owner, name)
	if err != nil {
		return fmt.Errorf("failed to fetch repository: %w", err)
	}

	lastCommitDate, err := s.store.GetLastCommitDate(ctx, repo.ID)
	if err != nil {
		return fmt.Errorf("failed to get last commit date: %w", err)
	}

	ghCommits, err := s.client.GetCommits(ctx, owner, name, lastCommitDate)
	if err != nil {
		return fmt.Errorf("failed to fetch commits: %w", err)
	}

	commits := make([]*models.Commit, 0, len(ghCommits))
	for _, ghCommit := range ghCommits {
		commits = append(commits, &models.Commit{
			SHA:         ghCommit.SHA,
			Message:     ghCommit.Message,
			AuthorName:  ghCommit.AuthorName,
			AuthorEmail: ghCommit.AuthorEmail,
			AuthorDate:  ghCommit.AuthorDate,
			CommitURL:   ghCommit.CommitURL,
		})
	}

	if err := s.store.SaveCommits(ctx, repo.ID, commits); err != nil {
		return fmt.Errorf("failed to save commits: %w", err)
	}

	repo.LastSyncedAt = time.Now()
	if err := s.store.SaveRepository(ctx, repo); err != nil {
		return fmt.Errorf("failed to update sync time: %w", err)
	}

	s.logger.Infof("Synced %s/%s: %d new commits", owner, name, len(commits))
	return nil
}

func (s *Service) getOrCreateRepository(ctx context.Context, owner, name string) (*models.Repository, error) {
	repoURL := fmt.Sprintf("https://github.com/%s/%s", owner, name)
	
	repo, err := s.store.GetRepositoryByURL(ctx, repoURL)
	if err == nil {
		return repo, nil
	}

	ghRepo, err := s.client.GetRepository(ctx, owner, name)
	if err != nil {
		return nil, err
	}

	repo = &models.Repository{
		Name:            ghRepo.Name,
		Description:     ghRepo.Description,
		URL:             ghRepo.URL,
		Language:        ghRepo.Language,
		ForksCount:      ghRepo.ForksCount,
		StarsCount:      ghRepo.StarsCount,
		OpenIssuesCount: ghRepo.OpenIssuesCount,
		WatchersCount:   ghRepo.WatchersCount,
		CreatedAt:       ghRepo.CreatedAt,
		UpdatedAt:       ghRepo.UpdatedAt,
		LastSyncedAt:    time.Now(),
	}

	if err := s.store.SaveRepository(ctx, repo); err != nil {
		return nil, err
	}

	return repo, nil
}

func (s *Service) GetTopCommitAuthors(ctx context.Context, repoURL string, limit int) ([]*models.AuthorStats, error) {
	_, _, err := ParseRepoURL(repoURL)
	if err != nil {
		return nil, err
	}

	repo, err := s.store.GetRepositoryByURL(ctx, repoURL)
	if err != nil {
		return nil, fmt.Errorf("repository not found: %w", err)
	}

	return s.store.GetTopCommitAuthors(ctx, repo.ID, limit)
}

func (s *Service) GetCommits(ctx context.Context, repoURL string, limit, offset int) ([]*models.Commit, error) {
	_, _, err := ParseRepoURL(repoURL)
	if err != nil {
		return nil, err
	}

	repo, err := s.store.GetRepositoryByURL(ctx, repoURL)
	if err != nil {
		return nil, fmt.Errorf("repository not found: %w", err)
	}

	return s.store.GetCommits(ctx, repo.ID, limit, offset)
}

func (s *Service) ResetRepository(ctx context.Context, repoURL string, since time.Time) error {
	_, _, err := ParseRepoURL(repoURL)
	if err != nil {
		return err
	}

	repo, err := s.store.GetRepositoryByURL(ctx, repoURL)
	if err != nil {
		return fmt.Errorf("repository not found: %w", err)
	}

	return s.store.ResetRepository(ctx, repo.ID, since)
}