package github

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/Kamar-Folarin/github-monitor/internal/config"
	"github.com/Kamar-Folarin/github-monitor/internal/errors"
	"github.com/Kamar-Folarin/github-monitor/internal/models"
)

// SyncServiceImpl implements the SyncService interface
type SyncServiceImpl struct {
	repoService  RepositoryService
	commitService CommitService
	statusManager StatusManager
	config       *config.SyncConfig
	mu           sync.RWMutex
	syncStatus   map[string]*models.SyncStatus
	syncTickers  map[string]*time.Ticker
	tickerDone   map[string]chan struct{}
}

// NewSyncService creates a new sync service
func NewSyncService(
	repoService RepositoryService,
	commitService CommitService,
	statusManager StatusManager,
	cfg *config.SyncConfig,
) SyncService {
	return &SyncServiceImpl{
		repoService:   repoService,
		commitService: commitService,
		statusManager: statusManager,
		config:       cfg,
		syncStatus:   make(map[string]*models.SyncStatus),
		syncTickers:  make(map[string]*time.Ticker),
		tickerDone:   make(map[string]chan struct{}),
	}
}

// StartSync starts the background sync process for a repository
func (s *SyncServiceImpl) StartSync(ctx context.Context, repoURL string) error {
	logger := logrus.WithFields(logrus.Fields{
		"repository": repoURL,
		"action":    "start_sync",
	})
	logger.Info("Starting sync process for repository")

	repo, err := s.repoService.GetRepository(ctx, repoURL)
	if err != nil {
		logger.WithError(err).Error("Failed to get repository info")
		return fmt.Errorf("failed to get repository: %w", err)
	}

	status, err := s.statusManager.GetStatus(ctx, repoURL)
	if err != nil {
		logger.WithError(err).Error("Failed to get sync status")
		return fmt.Errorf("failed to get sync status: %w", err)
	}

	if status != nil && status.IsSyncing {
		logger.Warn("Sync already in progress")
		return errors.NewSyncInProgressError(repoURL)
	}

	status = &models.SyncStatus{
		RepositoryID:  int64(repo.ID),
		RepositoryURL: repoURL,
		Status:        "in_progress",
		IsSyncing:     true,
		StartTime:     time.Now(),
		LastSyncAt:    time.Now(),
	}

	logger.Info("Updating sync status to in_progress")
	if err := s.statusManager.UpdateStatus(ctx, status); err != nil {
		logger.WithError(err).Error("Failed to update sync status")
		return fmt.Errorf("failed to update sync status: %w", err)
	}

	go func() {
		logger := logger.WithField("goroutine", "sync_process")
		logger.Info("Starting background sync process")

		syncCtx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()

		var since time.Time
		if repo.SyncConfig.InitialSyncDate != nil {
			since = *repo.SyncConfig.InitialSyncDate
			logger.WithField("initial_sync_date", since).Info("Using initial sync date from config")
		} else {
			lastCommitDate, err := s.commitService.GetLastCommitDate(syncCtx, repoURL)
			if err != nil {
				logger.WithError(err).Error("Failed to get last commit date")
				s.updateErrorStatus(syncCtx, repoURL, fmt.Errorf("failed to get last commit date: %w", err))
				return
			}
			if lastCommitDate != nil {
				since = *lastCommitDate
				logger.WithField("last_commit_date", since).Info("Using last commit date from database")
			} else {
				logger.Info("No previous commits found, will fetch all commits")
			}
		}

		batchSize := s.config.BatchSize
		if repo.SyncConfig.BatchSize > 0 {
			batchSize = repo.SyncConfig.BatchSize
			logger.WithField("batch_size", batchSize).Info("Using repository-specific batch size")
		} else {
			logger.WithField("batch_size", batchSize).Info("Using default batch size")
		}

		logger.WithFields(logrus.Fields{
			"since":      since,
			"batch_size": batchSize,
		}).Info("Starting to fetch commits from GitHub")

		if err := s.commitService.SyncCommits(syncCtx, repoURL, since, batchSize); err != nil {
			logger.WithError(err).Error("Failed to sync commits")
			s.updateErrorStatus(syncCtx, repoURL, fmt.Errorf("failed to sync commits: %w", err))
			return
		}

		logger.Info("Sync completed successfully, updating status")
		status.IsSyncing = false
		status.Status = "completed"
		status.LastSyncAt = time.Now()
		if err := s.statusManager.UpdateStatus(syncCtx, status); err != nil {
			logger.WithError(err).Error("Failed to update final sync status")
			return
		}
		logger.Info("Sync process completed successfully")
	}()

	if repo.SyncConfig.SyncInterval > 0 {
		logger.WithField("sync_interval", repo.SyncConfig.SyncInterval).Info("Starting sync ticker")
		s.startSyncTicker(ctx, repoURL, repo.SyncConfig.SyncInterval.ToDuration())
	}

	return nil
}

// startSyncTicker starts a ticker for repository-specific sync interval
func (s *SyncServiceImpl) startSyncTicker(ctx context.Context, repoURL string, interval time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if ticker, exists := s.syncTickers[repoURL]; exists {
		ticker.Stop()
		if done, exists := s.tickerDone[repoURL]; exists {
			close(done)
		}
	}

	ticker := time.NewTicker(interval)
	done := make(chan struct{})
	s.syncTickers[repoURL] = ticker
	s.tickerDone[repoURL] = done

	go func() {
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				return
			case <-done:
				ticker.Stop()
				return
			case <-ticker.C:
				repo, err := s.repoService.GetRepository(ctx, repoURL)
				if err != nil || !repo.SyncConfig.Enabled {
					ticker.Stop()
					return
				}
				if err := s.StartSync(ctx, repoURL); err != nil {
				}
			}
		}
	}()
}

// StopSync stops the background sync process for a repository
func (s *SyncServiceImpl) StopSync(ctx context.Context, repoURL string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if ticker, exists := s.syncTickers[repoURL]; exists {
		ticker.Stop()
		delete(s.syncTickers, repoURL)
		if done, exists := s.tickerDone[repoURL]; exists {
			close(done)
			delete(s.tickerDone, repoURL)
		}
	}

	status, err := s.statusManager.GetStatus(ctx, repoURL)
	if err != nil && !errors.IsNotFound(err) {
		return fmt.Errorf("failed to get sync status: %w", err)
	}

	if status != nil {
		status.IsSyncing = false
		status.Status = "stopped"
		if err := s.statusManager.UpdateStatus(ctx, status); err != nil {
			return fmt.Errorf("failed to update sync status: %w", err)
		}
	}

	return nil
}

// SyncAllRepositories syncs all enabled repositories in priority order
func (s *SyncServiceImpl) SyncAllRepositories(ctx context.Context) error {
	repos, err := s.repoService.ListRepositories(ctx)
	if err != nil {
		return fmt.Errorf("failed to list repositories: %w", err)
	}

	sort.Slice(repos, func(i, j int) bool {
		return repos[i].SyncConfig.Priority > repos[j].SyncConfig.Priority
	})

	activeSyncs := 0
	maxConcurrent := s.config.MaxConcurrentSyncs

	for _, repo := range repos {
		if !repo.SyncConfig.Enabled {
			continue
		}

		if activeSyncs >= maxConcurrent {
			time.Sleep(time.Second)
			continue
		}

		if err := s.StartSync(ctx, repo.URL); err != nil {
			continue
		}

		activeSyncs++
	}

	return nil
}

// GetSyncStatus gets the current sync status for a repository
func (s *SyncServiceImpl) GetSyncStatus(ctx context.Context, repoURL string) (*models.SyncStatus, error) {
	return s.statusManager.GetStatus(ctx, repoURL)
}

// updateErrorStatus updates the sync status with an error
func (s *SyncServiceImpl) updateErrorStatus(ctx context.Context, repoURL string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if status, exists := s.syncStatus[repoURL]; exists {
		status.Status = "error"
		status.LastError = err.Error()
		status.LastSyncAt = time.Now()
		s.statusManager.UpdateStatus(ctx, status)
	}
}

// ForceClearSyncStatus forcefully clears a stuck sync status
func (s *SyncServiceImpl) ForceClearSyncStatus(ctx context.Context, repoURL string) error {
	logger := logrus.WithFields(logrus.Fields{
		"repository": repoURL,
	})
	logger.Info("Forcefully clearing sync status")

	repo, err := s.repoService.GetRepository(ctx, repoURL)
	if err != nil {
		logger.WithError(err).Error("Failed to get repository")
		return fmt.Errorf("failed to get repository: %w", err)
	}

	status := &models.SyncStatus{
		RepositoryID:  int64(repo.ID),
		RepositoryURL: repoURL,
		Status:        "cleared",
		IsSyncing:     false,
		StartTime:     time.Now(),
		LastSyncAt:    time.Now(),
		LastError:     "Sync status was forcefully cleared",
	}

	if err := s.statusManager.UpdateStatus(ctx, status); err != nil {
		logger.WithError(err).Error("Failed to update sync status")
		return fmt.Errorf("failed to update sync status: %w", err)
	}

	logger.Info("Successfully cleared sync status")
	return nil
}

// ResetSync resets the sync process for a repository from a given date
func (s *SyncServiceImpl) ResetSync(ctx context.Context, repoURL string, since time.Time) error {
	logger := logrus.WithFields(logrus.Fields{
		"repository": repoURL,
		"since":     since,
	})
	logger.Info("Starting repository sync reset")

	repo, err := s.repoService.GetRepository(ctx, repoURL)
	if err != nil {
		logger.WithError(err).Error("Failed to get repository")
		return fmt.Errorf("failed to get repository: %w", err)
	}

	status, err := s.statusManager.GetStatus(ctx, repoURL)
	if err != nil && !errors.IsNotFound(err) {
		logger.WithError(err).Error("Failed to get sync status")
		return fmt.Errorf("failed to get sync status: %w", err)
	}

	if status != nil && status.IsSyncing {
		logger.Warn("Sync appears to be in progress, attempting to force clear status")
		if err := s.ForceClearSyncStatus(ctx, repoURL); err != nil {
			logger.WithError(err).Error("Failed to force clear sync status")
			return fmt.Errorf("sync is in progress and could not be cleared: %w", err)
		}
		status, err = s.statusManager.GetStatus(ctx, repoURL)
		if err != nil && !errors.IsNotFound(err) {
			logger.WithError(err).Error("Failed to get updated sync status")
			return fmt.Errorf("failed to get updated sync status: %w", err)
		}
	}

	status = &models.SyncStatus{
		RepositoryID:  int64(repo.ID),
		RepositoryURL: repoURL,
		Status:        "resetting",
		IsSyncing:     true,
		StartTime:     time.Now(),
		LastSyncAt:    time.Now(),
	}

	if err := s.statusManager.UpdateStatus(ctx, status); err != nil {
		logger.WithError(err).Error("Failed to update sync status")
		return fmt.Errorf("failed to update sync status: %w", err)
	}

	resetCtx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	go func() {
		logger := logger.WithField("goroutine", "reset_sync")
		logger.Info("Starting background sync process")

		batchSize := s.config.BatchSize
		if repo.SyncConfig.BatchSize > 0 {
			batchSize = repo.SyncConfig.BatchSize
			logger = logger.WithField("batch_size", batchSize)
		}

		done := make(chan error, 1)
		go func() {
			err := s.commitService.SyncCommits(resetCtx, repoURL, since, batchSize)
			done <- err
		}()

		var syncErr error
		select {
		case syncErr = <-done:
		case <-resetCtx.Done():
			syncErr = fmt.Errorf("reset operation timed out after 30 minutes")
			logger.WithError(syncErr).Error("Reset operation timed out")
		}

		if syncErr != nil {
			logger.WithError(syncErr).Error("Failed to sync commits during reset")
			s.updateErrorStatus(ctx, repoURL, fmt.Errorf("failed to sync commits: %w", syncErr))
		} else {
			status.IsSyncing = false
			status.Status = "completed"
			status.LastSyncAt = time.Now()
			if err := s.statusManager.UpdateStatus(ctx, status); err != nil {
				logger.WithError(err).Error("Failed to update final sync status")
				return
			}
			logger.Info("Successfully completed repository sync reset")
		}
	}()

	logger.Info("Repository sync reset initiated")
	return nil
} 