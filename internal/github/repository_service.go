package github

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"


	"github.com/Kamar-Folarin/github-monitor/internal/config"
	"github.com/Kamar-Folarin/github-monitor/internal/db"
	"github.com/Kamar-Folarin/github-monitor/internal/errors"
	"github.com/Kamar-Folarin/github-monitor/internal/models"
)

// RepositoryServiceClient defines the GitHub API client interface for repository operations
type RepositoryServiceClient interface {
	GetRepository(ctx context.Context, owner, name string) (*models.Repository, error)
	GetRepositoryByURL(ctx context.Context, repoURL string) (*models.Repository, error)
}

// RepositoryService defines the interface for repository operations
type RepositoryService interface {
	GetRepository(ctx context.Context, repoURL string) (*models.Repository, error)
	GetRepositoryByID(ctx context.Context, id uint) (*models.Repository, error)
	AddRepository(ctx context.Context, req AddRepositoryRequest) (*models.Repository, error)
	ListRepositories(ctx context.Context) ([]*models.Repository, error)
	DeleteRepository(ctx context.Context, repoURL string) error
	DeleteRepositoryByID(ctx context.Context, id uint) error
	UpdateRepositorySyncConfig(ctx context.Context, repoURL string, config models.RepositorySyncConfig) error
}

// RepositoryServiceImpl implements the RepositoryService interface
type RepositoryServiceImpl struct {
	client    RepositoryServiceClient
	store     db.Store
	config    *config.GitHubConfig
	statusMgr StatusManager
}

// NewRepositoryService creates a new repository service
func NewRepositoryService(client RepositoryServiceClient, store db.Store, cfg *config.GitHubConfig, statusMgr StatusManager) RepositoryService {
	return &RepositoryServiceImpl{
		client:    client,
		store:     store,
		config:    cfg,
		statusMgr: statusMgr,
	}
}

// GetRepository retrieves a repository by URL
func (s *RepositoryServiceImpl) GetRepository(ctx context.Context, repoURL string) (*models.Repository, error) {
	repo, err := s.store.GetRepositoryByURL(ctx, repoURL)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, errors.NewNotFoundError(fmt.Sprintf("repository not found: %s", repoURL), err)
		}
		return nil, err
	}
	return repo, nil
}

// GetRepositoryByID retrieves a repository by ID
func (s *RepositoryServiceImpl) GetRepositoryByID(ctx context.Context, id uint) (*models.Repository, error) {
	repo, err := s.store.GetRepository(ctx, int64(id))
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, errors.NewNotFoundError(fmt.Sprintf("repository not found with ID: %d", id), err)
		}
		return nil, err
	}
	return repo, nil
}

// AddRepositoryRequest represents the request to add a new repository
type AddRepositoryRequest struct {
	URL string `json:"url" binding:"required"`
	SyncConfig *models.RepositorySyncConfig `json:"sync_config,omitempty"`
}

// AddRepository adds a new repository to monitor
func (s *RepositoryServiceImpl) AddRepository(ctx context.Context, req AddRepositoryRequest) (*models.Repository, error) {
	normalizedURL, err := s.normalizeRepoURL(req.URL)
	if err != nil {
		return nil, err
	}

	owner, name, err := parseRepoURL(normalizedURL)
	if err != nil {
		return nil, errors.NewValidationError("invalid repository URL format", err)
	}

	existingRepo, err := s.store.GetRepositoryByURL(ctx, normalizedURL)
	if err == nil {
		if req.SyncConfig != nil {
			existingRepo.SyncConfig = *req.SyncConfig
			if err := s.store.UpdateRepository(ctx, existingRepo); err != nil {
				return nil, fmt.Errorf("failed to update repository sync config: %w", err)
			}
		}
		return existingRepo, nil
	}
	
	if err != nil && !strings.Contains(err.Error(), "repository not found with URL") {
		return nil, fmt.Errorf("database error while checking repository: %w", err)
	}

	repoInfo, err := s.client.GetRepository(ctx, owner, name)
	
	fmt.Printf("Repository info: %+v\n", repoInfo)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch repository info: %w", err)
	}

	syncConfig := models.RepositorySyncConfig{
		Enabled: true,
		Priority: 0,
	}
	if req.SyncConfig != nil {
		syncConfig = *req.SyncConfig
	}

	now := time.Now()
	repo := &models.Repository{
		BaseModel: models.BaseModel{
			CreatedAt: now,
			UpdatedAt: now,
		},
		Name:            repoInfo.Name,
		Description:     repoInfo.Description,
		URL:            normalizedURL,
		Language:       repoInfo.Language,
		ForksCount:     repoInfo.ForksCount,
		StarsCount:     repoInfo.StarsCount,
		OpenIssuesCount: repoInfo.OpenIssuesCount,
		WatchersCount:  repoInfo.WatchersCount,
		LastSyncedAt:   *req.SyncConfig.InitialSyncDate,
		SyncConfig:     syncConfig,
	}

	if err := s.store.SaveRepository(ctx, repo); err != nil {
		return nil, fmt.Errorf("failed to save repository: %w", err)
	}

	status := &models.SyncStatus{
		RepositoryID:  int64(repo.ID),
		RepositoryURL: repo.URL,
		Status:        "pending",
		IsSyncing:     false,
		StartTime:     now,
		LastSyncAt:    repo.LastSyncedAt,
	}

	if err := s.statusMgr.UpdateStatus(ctx, status); err != nil {
		fmt.Printf("Warning: failed to create initial sync status: %v\n", err)
	}

	return repo, nil
}

// ListRepositories lists all monitored repositories
func (s *RepositoryServiceImpl) ListRepositories(ctx context.Context) ([]*models.Repository, error) {
	return s.store.ListRepositories(ctx)
}

// DeleteRepository removes a repository from monitoring
func (s *RepositoryServiceImpl) DeleteRepository(ctx context.Context, repoURL string) error {
	normalizedURL, err := s.normalizeRepoURL(repoURL)
	if err != nil {
		return err
	}

	repo, err := s.store.GetRepositoryByURL(ctx, normalizedURL)
	if err != nil {
		if errors.IsNotFound(err) {
			return errors.NewNotFoundError(fmt.Sprintf("repository not found: %s", normalizedURL), err)
		}
		return err
	}

	if err := s.store.DeleteRepository(ctx, int64(repo.ID)); err != nil {
		return fmt.Errorf("failed to delete repository: %w", err)
	}

	return nil
}

// DeleteRepositoryByID removes a repository from monitoring by ID
func (s *RepositoryServiceImpl) DeleteRepositoryByID(ctx context.Context, id uint) error {
	_, err := s.store.GetRepository(ctx, int64(id))
	if err != nil {
		if errors.IsNotFound(err) {
			return errors.NewNotFoundError(fmt.Sprintf("repository not found with ID: %d", id), err)
		}
		return err
	}

	if err := s.store.DeleteRepository(ctx, int64(id)); err != nil {
		return fmt.Errorf("failed to delete repository: %w", err)
	}

	return nil
}

// normalizeRepoURL normalizes a repository URL
func (s *RepositoryServiceImpl) normalizeRepoURL(repoURL string) (string, error) {
	if strings.HasPrefix(repoURL, "git@") {
		parts := strings.Split(repoURL, ":")
		if len(parts) != 2 {
			return "", errors.NewValidationError("invalid SSH URL format", nil)
		}
		repoURL = fmt.Sprintf("https://%s/%s", parts[0], parts[1])
	}

	u, err := url.Parse(repoURL)
	if err != nil {
		return "", errors.NewValidationError(fmt.Sprintf("invalid URL: %v", err), err)
	}

	if !strings.HasSuffix(u.Host, "github.com") {
		return "", errors.NewValidationError("only GitHub repositories are supported", nil)
	}

	path := strings.TrimSuffix(u.Path, ".git")
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	normalizedURL := fmt.Sprintf("https://github.com%s", path)
	return normalizedURL, nil
}

// parseRepoURL extracts owner and name from a repository URL
func parseRepoURL(repoURL string) (owner, name string, err error) {
	u, err := url.Parse(repoURL)
	if err != nil {
		return "", "", errors.NewValidationError("invalid URL format", err)
	}

	parts := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(parts) != 2 {
		return "", "", errors.NewValidationError("invalid repository path format", nil)
	}

	return parts[0], parts[1], nil
}

// UpdateRepositorySyncConfig updates the sync configuration for a repository
func (s *RepositoryServiceImpl) UpdateRepositorySyncConfig(ctx context.Context, repoURL string, config models.RepositorySyncConfig) error {
	repo, err := s.store.GetRepositoryByURL(ctx, repoURL)
	if err != nil {
		return fmt.Errorf("repository not found: %w", err)
	}

	
	if repo.SyncConfig.SyncInterval > 0 {
		config.SyncInterval = models.FromDuration(repo.SyncConfig.SyncInterval.ToDuration())
	}

	repo.SyncConfig = config
	return s.store.UpdateRepository(ctx, repo)
} 