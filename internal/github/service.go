package github

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/Kamar-Folarin/github-monitor/internal/db"
	"github.com/Kamar-Folarin/github-monitor/internal/models"
	"github.com/Kamar-Folarin/github-monitor/internal/utils"
)

// SyncConfig holds configuration for repository synchronization
type SyncConfig struct {
	MaxConcurrentSyncs int
	SyncInterval       time.Duration
	BatchSize         int
	// Add persistence config
	StatusPersistenceInterval time.Duration
}

// DefaultSyncConfig returns the default sync configuration
func DefaultSyncConfig() SyncConfig {
	return SyncConfig{
		MaxConcurrentSyncs:        3,
		SyncInterval:              time.Minute * 5,
		BatchSize:                 100,
		StatusPersistenceInterval: time.Minute,
	}
}

// BatchConfig holds configuration for batch processing
type BatchConfig struct {
	Size int
	Workers int
	MaxRetries int
	BatchDelay time.Duration
	MaxCommits int
}

// DefaultBatchConfig returns default batch processing configuration
func DefaultBatchConfig() *BatchConfig {
	return &BatchConfig{
		Size:       1000,           // Process 1000 commits per batch
		Workers:    3,             // Use 3 parallel workers
		MaxRetries: 3,             // Retry failed batches up to 3 times
		BatchDelay: time.Second,   // Wait 1 second between batches
		MaxCommits: 0,             // No limit on total commits
	}
}

// Service represents the GitHub service layer
type Service struct {
	client *GitHubClient
	store  db.Store
	logger *logrus.Logger
	config SyncConfig
	syncMutex    sync.RWMutex
	syncStatus   map[string]*models.SyncStatus
	statusPersistenceTicker *time.Ticker
	statusPersistenceDone   chan struct{}
	batchConfig *BatchConfig
	progress    map[string]*models.BatchProgress
	progressMu  sync.RWMutex
	isRunning bool
	mu          sync.RWMutex
}

// NewService creates a new GitHub service
func NewService(token string, store db.Store, logger *logrus.Logger, config SyncConfig) *Service {
	logger.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: time.RFC3339,
	})
	logger.SetLevel(logrus.InfoLevel)

	service := &Service{
		client:     NewGitHubClient(token, logger, WithRetryConfig(3, time.Second, time.Minute)),
		store:      store,
		logger:     logger,
		config:     config,
		syncStatus: make(map[string]*models.SyncStatus),
		batchConfig: DefaultBatchConfig(),
		progress:    make(map[string]*models.BatchProgress),
	}

	if err := service.loadPersistedStatus(); err != nil {
		logger.WithError(err).Warn("Failed to load persisted sync status")
	}

	return service
}

// loadPersistedStatus loads the persisted sync status from the store
func (s *Service) loadPersistedStatus() error {
	statuses, err := s.store.ListSyncStatuses(context.Background())
	if err != nil {
		return fmt.Errorf("failed to load sync status: %w", err)
	}

	s.syncMutex.Lock()
	defer s.syncMutex.Unlock()

	for _, status := range statuses {
		s.syncStatus[status.RepositoryURL] = status
	}

	return nil
}

// persistStatus saves the current sync status to the store
func (s *Service) persistStatus() error {
	s.syncMutex.RLock()
	defer s.syncMutex.RUnlock()

	for _, status := range s.syncStatus {
		if err := s.store.UpdateSyncStatus(context.Background(), status); err != nil {
			s.logger.WithError(err).WithField("repo", status.RepositoryURL).Warn("Failed to update sync status")
		}
	}

	return nil
}

// StartSync starts the background sync process for all monitored repositories
func (s *Service) StartSync(ctx context.Context) {
	s.logger.Info("Starting background sync process")
	ticker := time.NewTicker(s.config.SyncInterval)
	defer ticker.Stop()
	s.logger.Info("Performing initial sync of all repositories")
	if err := s.SyncAllRepositories(ctx); err != nil {
		s.logger.Errorf("Error during initial sync: %v", err)
	}

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("Stopping background sync process")
			return
		case <-ticker.C:
			s.logger.Info("Starting scheduled sync of all repositories")
			if err := s.SyncAllRepositories(ctx); err != nil {
				s.logger.Errorf("Error during scheduled sync: %v", err)
			}
		}
	}
}

// SyncAllRepositories syncs all monitored repositories
func (s *Service) SyncAllRepositories(ctx context.Context) error {
	repos, err := s.store.ListRepositories(ctx)
	if err != nil {
		return fmt.Errorf("failed to list repositories: %w", err)
	}

	s.logger.Infof("Found %d repositories to sync", len(repos))

	for _, repo := range repos {
		s.logger.Infof("Starting sync for repository: %s", repo.URL)
		owner, name, err := utils.ParseRepoURL(repo.URL)
		if err != nil {
			s.logger.Errorf("Invalid repository URL %s: %v", repo.URL, err)
			continue
		}

		if err := s.SyncRepository(ctx, owner, name); err != nil {
			s.logger.Errorf("Failed to sync repository %s: %v", repo.URL, err)
		} else {
			s.logger.Infof("Successfully completed sync for repository: %s", repo.URL)
		}
	}

	return nil
}

// SyncRepository syncs a single repository
func (s *Service) SyncRepository(ctx context.Context, owner, name string) error {
	s.logger.Infof("Fetching repository info for %s/%s", owner, name)
	repo, err := s.client.GetRepository(ctx, owner, name)
	if err != nil {
		return fmt.Errorf("failed to get repository: %w", err)
	}

	s.logger.Infof("Saving repository info for %s", repo.URL)
	if err := s.store.SaveRepository(ctx, repo); err != nil {
		return fmt.Errorf("failed to save repository: %w", err)
	}

	s.logger.Infof("Getting last commit date for %s", repo.URL)
	lastCommitDate, err := s.store.GetLastCommitDate(ctx, int64(repo.ID))
	if err != nil {
		return fmt.Errorf("failed to get last commit date: %w", err)
	}

	if lastCommitDate != nil {
		s.logger.Infof("Last commit date for %s: %s", repo.URL, lastCommitDate.Format(time.RFC3339))
	} else {
		s.logger.Infof("No previous commits found for %s, will fetch all commits", repo.URL)
	}

	status := &models.SyncStatus{
		LastSyncAt:    time.Now(),
		IsSyncing:     true,
		LastError:     "",
		TotalCommits:  0,
		BatchProgress: &models.BatchProgress{
			StartTime:      time.Now(),
			LastUpdateTime: time.Now(),
		},
	}
	if err := s.SaveSyncStatus(ctx, repo.URL, status); err != nil {
		return fmt.Errorf("failed to save initial sync status: %w", err)
	}

	batchSize := s.batchConfig.Size
	totalCommits := 0

	since := time.Time{}
	if lastCommitDate != nil {
		since = *lastCommitDate
	}

	err = s.client.GetCommits(ctx, owner, name, since, batchSize, func(batch []*models.Commit) error {
		s.logger.Infof("Processing batch of %d commits for %s", len(batch), repo.URL)
		
		if err := s.store.SaveCommits(ctx, int64(repo.ID), batch); err != nil {
			return fmt.Errorf("failed to save batch: %w", err)
		}
		
		totalCommits += len(batch)
		status.TotalCommits = totalCommits
		status.LastCommitSHA = batch[len(batch)-1].SHA
		status.BatchProgress.ProcessedCommits = totalCommits
		status.BatchProgress.LastUpdateTime = time.Now()
		status.BatchProgress.LastProcessedSHA = batch[len(batch)-1].SHA
		status.BatchProgress.ProcessedBatches++
		
		if err := s.SaveSyncStatus(ctx, repo.URL, status); err != nil {
			return fmt.Errorf("failed to update sync status: %w", err)
		}
		s.logger.Infof("Successfully processed batch. Total commits so far: %d", totalCommits)
		return nil
	})

	if err != nil {
		
		status.IsSyncing = false
		status.LastError = err.Error()
		if saveErr := s.SaveSyncStatus(ctx, repo.URL, status); saveErr != nil {
			s.logger.Errorf("Failed to save error status: %v", saveErr)
		}
		return fmt.Errorf("failed to process commits: %w", err)
	}

	
	status.IsSyncing = false
	status.LastError = ""
	status.LastSyncAt = time.Now()
	if err := s.SaveSyncStatus(ctx, repo.URL, status); err != nil {
		return fmt.Errorf("failed to save final sync status: %w", err)
	}

	s.logger.Infof("Successfully completed sync for %s. Total commits: %d", repo.URL, totalCommits)
	return nil
}

// AddRepository adds a new repository to monitor
func (s *Service) AddRepository(ctx context.Context, repoURL string, initialSyncDate *time.Time) error {
	owner, name, err := utils.ParseRepoURL(repoURL)
	if err != nil {
		return fmt.Errorf("invalid repository URL: %w", err)
	}

	
	repo, err := s.client.GetRepository(ctx, owner, name)
	if err != nil {
		return fmt.Errorf("failed to get repository: %w", err)
	}

	
	if err := s.store.SaveRepository(ctx, repo); err != nil {
		return fmt.Errorf("failed to save repository: %w", err)
	}

	
	if err := s.store.AddMonitoredRepository(ctx, repoURL, initialSyncDate); err != nil {
		return fmt.Errorf("failed to add monitored repository: %w", err)
	}

	return nil
}

// GetSyncStatus returns the current sync status for a repository
func (s *Service) GetSyncStatus(ctx context.Context, repoURL string) (*models.SyncStatus, error) {
	return s.store.GetSyncStatus(ctx, repoURL)
}

// SaveSyncStatus saves the sync status for a repository
func (s *Service) SaveSyncStatus(ctx context.Context, repoURL string, status *models.SyncStatus) error {
	return s.store.UpdateSyncStatus(ctx, status)
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
		BaseModel: models.BaseModel{
			CreatedAt: ghRepo.CreatedAt,
			UpdatedAt: ghRepo.UpdatedAt,
		},
		Name:            ghRepo.Name,
		Description:     ghRepo.Description,
		URL:             ghRepo.URL,
		Language:        ghRepo.Language,
		ForksCount:      ghRepo.ForksCount,
		StarsCount:      ghRepo.StarsCount,
		OpenIssuesCount: ghRepo.OpenIssuesCount,
		WatchersCount:   ghRepo.WatchersCount,
		LastSyncedAt:    time.Now(),
	}

	if err := s.store.SaveRepository(ctx, repo); err != nil {
		return nil, err
	}

	return repo, nil
}

func (s *Service) GetTopCommitAuthors(ctx context.Context, repoURL string, limit int) ([]*models.AuthorStats, error) {
	_, _, err := utils.ParseRepoURL(repoURL)
	if err != nil {
		return nil, err
	}

	repo, err := s.store.GetRepositoryByURL(ctx, repoURL)
	if err != nil {
		return nil, fmt.Errorf("repository not found: %w", err)
	}

	return s.store.GetTopCommitAuthors(ctx, int64(repo.ID), limit)
}

func (s *Service) GetCommits(ctx context.Context, repoURL string, limit, offset int) ([]*models.Commit, error) {
	_, _, err := utils.ParseRepoURL(repoURL)
	if err != nil {
		return nil, err
	}

	repo, err := s.store.GetRepositoryByURL(ctx, repoURL)
	if err != nil {
		return nil, fmt.Errorf("repository not found: %w", err)
	}

	return s.store.GetCommits(ctx, int64(repo.ID), limit, offset)
}

func (s *Service) ResetRepository(ctx context.Context, repoURL string, since time.Time) error {
	_, _, err := utils.ParseRepoURL(repoURL)
	if err != nil {
		return err
	}

	repo, err := s.store.GetRepositoryByURL(ctx, repoURL)
	if err != nil {
		return fmt.Errorf("repository not found: %w", err)
	}

	return s.store.ResetRepository(ctx, int64(repo.ID), since)
}

// WithBatchConfig sets the batch processing configuration
func (s *Service) WithBatchConfig(config *BatchConfig) *Service {
	if config != nil {
		s.batchConfig = config
	}
	return s
}

// GetBatchProgress returns the current progress of batch processing for a repository
func (s *Service) GetBatchProgress(repoURL string) *models.BatchProgress {
	s.progressMu.RLock()
	defer s.progressMu.RUnlock()
	return s.progress[repoURL]
}

// processBatch processes a single batch of commits
func (s *Service) processBatch(ctx context.Context, repoURL string, commits []*models.Commit, progress *models.BatchProgress) error {
	progress.ProcessedCommits += len(commits)
	progress.LastUpdateTime = time.Now()
	if len(commits) > 0 {
		progress.LastProcessedSHA = commits[len(commits)-1].SHA
	}

	tx, err := s.store.BeginTx(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	if err := s.store.SaveCommitsTx(ctx, tx, commits); err != nil {
		return fmt.Errorf("failed to save commits: %w", err)
	}

	status := &models.SyncStatus{
		LastSyncAt:     time.Now(),
		LastCommitSHA:  progress.LastProcessedSHA,
		TotalCommits:   progress.ProcessedCommits,
		IsSyncing:      true,
		BatchProgress:  progress,
	}
	if err := s.store.SaveSyncStatusTx(ctx, tx, map[string]string{repoURL: status.String()}); err != nil {
		return fmt.Errorf("failed to save sync status: %w", err)
	}

	return tx.Commit()
}

// processBatches processes commits in batches with parallel workers
func (s *Service) processBatches(ctx context.Context, repoURL string, commitChan <-chan []*models.Commit) error {
	progress := &models.BatchProgress{
		StartTime:      time.Now(),
		LastUpdateTime: time.Now(),
	}
	s.progressMu.Lock()
	s.progress[repoURL] = progress
	s.progressMu.Unlock()

	defer func() {
		s.progressMu.Lock()
		delete(s.progress, repoURL)
		s.progressMu.Unlock()
	}()

	workerChan := make(chan struct{}, s.batchConfig.Workers)
	errChan := make(chan error, s.batchConfig.Workers)
	var wg sync.WaitGroup

	for batch := range commitChan {
		if s.batchConfig.MaxCommits > 0 && progress.ProcessedCommits >= s.batchConfig.MaxCommits {
			break
		}

		select {
		case workerChan <- struct{}{}:
		case err := <-errChan:
			return err
		case <-ctx.Done():
			return ctx.Err()
		}

		wg.Add(1)
		go func(batch []*models.Commit) {
			defer wg.Done()
			defer func() { <-workerChan }()

			var err error
			for retry := 0; retry < s.batchConfig.MaxRetries; retry++ {
				if err = s.processBatch(ctx, repoURL, batch, progress); err == nil {
					break
				}
				if IsRateLimitError(err) {
					if rle, ok := err.(*RateLimitError); ok {
						time.Sleep(time.Until(rle.ResetTime))
					}
				} else {
					time.Sleep(time.Duration(retry+1) * time.Second)
				}
			}

			if err != nil {
				progress.Errors = append(progress.Errors, err)
				select {
				case errChan <- err:
				default:
				}
			}

			progress.ProcessedBatches++
		}(batch)

		time.Sleep(s.batchConfig.BatchDelay)
	}

	wg.Wait()
	close(errChan)

	select {
	case err := <-errChan:
		return err
	default:
		return nil
	}
}

// GetRepository gets repository information from GitHub
func (s *Service) GetRepository(ctx context.Context, owner, name string) (*models.Repository, error) {
	return s.client.GetRepository(ctx, owner, name)
}

// GetSyncStatuses retrieves sync statuses for all repositories
func (s *Service) GetSyncStatuses(ctx context.Context) (map[string]*models.SyncStatus, error) {
	statuses, err := s.store.ListSyncStatuses(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get sync statuses: %w", err)
	}

	statusMap := make(map[string]*models.SyncStatus)
	for _, status := range statuses {
		statusMap[status.RepositoryURL] = status
	}

	return statusMap, nil
}

// UpdateSyncStatus updates the sync status for a repository
func (s *Service) UpdateSyncStatus(ctx context.Context, status *models.SyncStatus) error {
	return s.store.UpdateSyncStatus(ctx, status)
}