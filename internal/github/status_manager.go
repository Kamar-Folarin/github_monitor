package github

import (
	"context"
	"fmt"
	"sync"
	


	"github.com/Kamar-Folarin/github-monitor/internal/db"
	"github.com/Kamar-Folarin/github-monitor/internal/errors"
	"github.com/Kamar-Folarin/github-monitor/internal/models"
)

// StatusManagerImpl implements the StatusManager interface
type StatusManagerImpl struct {
	store db.Store
	mu    sync.RWMutex
	cache map[string]*models.SyncStatus
}

// NewStatusManager creates a new status manager
func NewStatusManager(store db.Store) StatusManager {
	return &StatusManagerImpl{
		store: store,
		cache: make(map[string]*models.SyncStatus),
	}
}

// GetStatus retrieves the sync status for a repository
func (m *StatusManagerImpl) GetStatus(ctx context.Context, repoURL string) (*models.SyncStatus, error) {
	m.mu.RLock()
	if status, exists := m.cache[repoURL]; exists {
		m.mu.RUnlock()
		return status, nil
	}
	m.mu.RUnlock()

	status, err := m.store.GetSyncStatus(ctx, repoURL)
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, errors.NewNotFoundError(fmt.Sprintf("sync status not found for repository: %s", repoURL), err)
		}
		return nil, fmt.Errorf("failed to get sync status: %w", err)
	}

	if status == nil {
		return nil, errors.NewNotFoundError(fmt.Sprintf("sync status not found for repository: %s", repoURL), nil)
	}

	m.mu.Lock()
	m.cache[repoURL] = status
	m.mu.Unlock()

	return status, nil
}

// UpdateStatus updates the sync status for a repository
func (m *StatusManagerImpl) UpdateStatus(ctx context.Context, status *models.SyncStatus) error {
	if status == nil {
		return errors.NewValidationError("status cannot be nil", nil)
	}
	if status.RepositoryURL == "" {
		return errors.NewValidationError("repository URL cannot be empty", nil)
	}

	if err := m.store.UpdateSyncStatus(ctx, status); err != nil {
		return fmt.Errorf("failed to update sync status: %w", err)
	}

	m.mu.Lock()
	m.cache[status.RepositoryURL] = status
	m.mu.Unlock()

	return nil
}

// DeleteStatus deletes the sync status for a repository
func (m *StatusManagerImpl) DeleteStatus(ctx context.Context, repoURL string) error {
	if err := m.store.DeleteSyncStatus(ctx, repoURL); err != nil {
		return fmt.Errorf("failed to delete sync status: %w", err)
	}

	m.mu.Lock()
	delete(m.cache, repoURL)
	m.mu.Unlock()

	return nil
}

// ListStatuses retrieves all sync statuses
func (m *StatusManagerImpl) ListStatuses(ctx context.Context) ([]*models.SyncStatus, error) {
	statuses, err := m.store.ListSyncStatuses(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to list sync statuses: %w", err)
	}

	m.mu.Lock()
	for _, status := range statuses {
		m.cache[status.RepositoryURL] = status
	}
	m.mu.Unlock()

	return statuses, nil
}

// ClearStatuses clears all sync statuses
func (m *StatusManagerImpl) ClearStatuses(ctx context.Context) error {
	if err := m.store.ClearSyncStatuses(ctx); err != nil {
		return fmt.Errorf("failed to clear sync statuses: %w", err)
	}

	m.mu.Lock()
	m.cache = make(map[string]*models.SyncStatus)
	m.mu.Unlock()

	return nil
}

// GetAllStatuses retrieves all sync statuses (alias for ListStatuses)
func (m *StatusManagerImpl) GetAllStatuses(ctx context.Context) ([]*models.SyncStatus, error) {
	return m.ListStatuses(ctx)
} 