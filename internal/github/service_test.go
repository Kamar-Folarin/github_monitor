package github

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/Kamar-Folarin/github-monitor/internal/db"
	"github.com/Kamar-Folarin/github-monitor/internal/models"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// MockStore implements db.Store for testing
type MockStore struct {
	mock.Mock
	repositories map[string]*models.Repository
	commits      map[string][]*models.Commit
	syncStatus   map[string]string
	syncProgress map[string]*models.SyncProgress
	err          error
}

func (m *MockStore) GetRepositoryByURL(ctx context.Context, url string) (*models.Repository, error) {
	args := m.Called(ctx, url)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Repository), args.Error(1)
}

func (m *MockStore) SaveRepository(ctx context.Context, repo *models.Repository) error {
	args := m.Called(ctx, repo)
	return args.Error(0)
}

func (m *MockStore) GetLastCommitDate(ctx context.Context, repoID int64) (*time.Time, error) {
	args := m.Called(ctx, repoID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*time.Time), args.Error(1)
}

func (m *MockStore) SaveCommits(ctx context.Context, repoID int64, commits []*models.Commit) error {
	args := m.Called(ctx, repoID, commits)
	return args.Error(0)
}

func (m *MockStore) GetTopCommitAuthors(ctx context.Context, repoID int64, limit int) ([]*models.AuthorStats, error) {
	args := m.Called(ctx, repoID, limit)
	return args.Get(0).([]*models.AuthorStats), args.Error(1)
}

func (m *MockStore) GetCommits(ctx context.Context, repoID int64, limit, offset int) ([]*models.Commit, error) {
	if m.err != nil {
		return nil, m.err
	}
	// For testing, just return all commits for the first repo in the map
	for _, commits := range m.commits {
		return commits, nil
	}
	return nil, nil
}

func (m *MockStore) ResetRepository(ctx context.Context, repoID int64, since time.Time) error {
	args := m.Called(ctx, repoID, since)
	return args.Error(0)
}

func (m *MockStore) ListRepositories(ctx context.Context) ([]*models.Repository, error) {
	if m.err != nil {
		return nil, m.err
	}
	repos := make([]*models.Repository, 0, len(m.repositories))
	for _, repo := range m.repositories {
		repos = append(repos, repo)
	}
	return repos, nil
}

func (m *MockStore) GetRepository(ctx context.Context, url string) (*models.Repository, error) {
	if m.err != nil {
		return nil, m.err
	}
	repo, ok := m.repositories[url]
	if !ok {
		return nil, errors.New("repository not found")
	}
	return repo, nil
}

func (m *MockStore) DeleteRepository(ctx context.Context, repoID int64) error {
	if m.err != nil {
		return m.err
	}
	for url, repo := range m.repositories {
		if int64(repo.ID) == repoID {
			delete(m.repositories, url)
			return nil
		}
	}
	return nil
}

func (m *MockStore) CreateCommit(ctx context.Context, commit *models.Commit) error {
	if m.err != nil {
		return m.err
	}
	repo, ok := m.repositories[fmt.Sprintf("https://github.com/owner/repo-%d", commit.RepositoryID)]
	if !ok {
		return errors.New("repository not found")
	}
	m.commits[repo.URL] = append(m.commits[repo.URL], commit)
	return nil
}

func (m *MockStore) DeleteCommits(ctx context.Context, repoID int64) error {
	if m.err != nil {
		return m.err
	}
	for url, repo := range m.repositories {
		if repo.ID == repoID {
			delete(m.commits, url)
			return nil
		}
	}
	return nil
}

func (m *MockStore) GetSyncStatus(ctx context.Context, repoURL string) (string, error) {
	if m.err != nil {
		return "", m.err
	}
	status, ok := m.syncStatus[repoURL]
	if !ok {
		return "", nil
	}
	return status, nil
}

func (m *MockStore) UpdateSyncStatus(ctx context.Context, repoURL string, status string) error {
	if m.err != nil {
		return m.err
	}
	m.syncStatus[repoURL] = status
	return nil
}

func (m *MockStore) GetAllSyncStatuses(ctx context.Context) (map[string]string, error) {
	if m.err != nil {
		return nil, m.err
	}
	status := make(map[string]string)
	for k, v := range m.syncStatus {
		status[k] = v
	}
	return status, nil
}

func (m *MockStore) ClearSyncStatus(ctx context.Context) error {
	if m.err != nil {
		return m.err
	}
	m.syncStatus = make(map[string]string)
	return nil
}

func (m *MockStore) GetSyncProgress(ctx context.Context, repoURL string) (*models.SyncProgress, error) {
	if m.err != nil {
		return nil, m.err
	}
	progress, ok := m.syncProgress[repoURL]
	if !ok {
		return nil, nil
	}
	return progress, nil
}

func (m *MockStore) SaveSyncProgress(ctx context.Context, repoURL string, progress *models.SyncProgress) error {
	if m.err != nil {
		return m.err
	}
	m.syncProgress[repoURL] = progress
	return nil
}

func (m *MockStore) AddMonitoredRepository(ctx context.Context, repo *models.Repository) error {
	args := m.Called(ctx, repo)
	return args.Error(0)
}

type MockGitHubClient struct {
	repositories map[string]*models.Repository
	commits      map[string][]*models.Commit
	err          error
}

func NewMockGitHubClient() *MockGitHubClient {
	return &MockGitHubClient{
		repositories: make(map[string]*models.Repository),
		commits:      make(map[string][]*models.Commit),
	}
}

type RepositoryNotFoundError struct {
	URL string
}

func (e *RepositoryNotFoundError) Error() string {
	return fmt.Sprintf("repository not found: %s", e.URL)
}

func (m *MockGitHubClient) GetRepository(ctx context.Context, owner, repo string) (*models.Repository, error) {
	if m.err != nil {
		return nil, m.err
	}
	key := "https://github.com/" + owner + "/" + repo
	r, ok := m.repositories[key]
	if !ok {
		return nil, &RepositoryNotFoundError{URL: key}
	}
	return r, nil
}

func (m *MockGitHubClient) GetCommitsPage(ctx context.Context, owner, repo string, since *time.Time, page int) ([]*models.Commit, error) {
	if m.err != nil {
		return nil, m.err
	}
	key := "https://github.com/" + owner + "/" + repo
	commits, ok := m.commits[key]
	if !ok {
		return nil, nil
	}
	return commits, nil
}

func setupTestService(t *testing.T) (*Service, *MockStore, *MockGitHubClient, func()) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	store := &MockStore{
		repositories: make(map[string]*models.Repository),
		commits:      make(map[string][]*models.Commit),
		syncStatus:   make(map[string]string),
		syncProgress: make(map[string]*models.SyncProgress),
	}
	client := NewMockGitHubClient()

	config := DefaultSyncConfig()
	config.SyncInterval = time.Millisecond * 100 // Short interval for testing
	config.MaxConcurrentSyncs = 2
	config.BatchSize = 10

	service := NewService("test-token", store, logger, config)

	cleanup := func() {
		service.StopSync()
	}

	return service, store, client, cleanup
}

func TestService_SyncRepository(t *testing.T) {
	service, store, client, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("successful sync", func(t *testing.T) {
		// Setup test data
		repo := &models.Repository{
			URL:      "https://github.com/test-owner/test-repo",
			Name:     "test-repo",
			Language: "Go",
		}
		client.repositories[repo.URL] = repo

		commits := []*models.Commit{
			{
				RepositoryURL: repo.URL,
				SHA:          "abc123",
				Message:      "Test commit",
				AuthorName:   "Test Author",
				AuthorEmail:  "test@example.com",
				CommittedAt:  time.Now(),
				CommitURL:    "https://github.com/test-owner/test-repo/commit/abc123",
			},
		}
		client.commits[repo.URL] = commits

		// Sync repository
		err := service.SyncRepository(ctx, repo.URL)
		require.NoError(t, err)

		// Verify repository was saved
		saved, err := store.GetRepository(ctx, repo.URL)
		require.NoError(t, err)
		assert.Equal(t, repo.URL, saved.URL)
		assert.Equal(t, repo.Name, saved.Name)

		// Verify commits were saved
		savedCommits, err := store.GetCommits(ctx, repo.ID, 10, 0)
		require.NoError(t, err)
		assert.Len(t, savedCommits, 1)
		assert.Equal(t, commits[0].SHA, savedCommits[0].SHA)
	})

	t.Run("invalid repository URL", func(t *testing.T) {
		err := service.SyncRepository(ctx, "invalid-url")
		assert.Error(t, err)
		assert.IsType(t, &ValidationError{}, err)
	})

	t.Run("repository not found", func(t *testing.T) {
		err := service.SyncRepository(ctx, "https://github.com/test-owner/non-existent")
		assert.Error(t, err)
		assert.IsType(t, &RepositoryNotFoundError{}, err)
	})

	t.Run("store error", func(t *testing.T) {
		store.err = errors.New("store error")
		defer func() { store.err = nil }()

		err := service.SyncRepository(ctx, "https://github.com/test-owner/test-repo")
		assert.Error(t, err)
		assert.Equal(t, "store error", err.Error())
	})

	t.Run("client error", func(t *testing.T) {
		client.err = errors.New("client error")
		defer func() { client.err = nil }()

		err := service.SyncRepository(ctx, "https://github.com/test-owner/test-repo")
		assert.Error(t, err)
		assert.Equal(t, "client error", err.Error())
	})
}

func TestService_GetSyncStatus(t *testing.T) {
	service, store, _, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("get existing status", func(t *testing.T) {
		// Setup test data
		status := map[string]string{
			"https://github.com/test-owner/repo1": `{"last_sync_at":"2020-01-01T00:00:00Z","is_syncing":true}`,
		}
		store.syncStatus = status

		// Get status
		saved, err := service.GetSyncStatus(ctx, "https://github.com/test-owner/repo1")
		require.NoError(t, err)
		assert.True(t, saved.IsSyncing)
		assert.Equal(t, time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC), saved.LastSyncAt)
	})

	t.Run("get non-existent status", func(t *testing.T) {
		status, err := service.GetSyncStatus(ctx, "https://github.com/test-owner/non-existent")
		require.NoError(t, err)
		assert.Nil(t, status)
	})

	t.Run("store error", func(t *testing.T) {
		store.err = errors.New("store error")
		defer func() { store.err = nil }()

		_, err := service.GetSyncStatus(ctx, "https://github.com/test-owner/repo1")
		assert.Error(t, err)
		assert.Equal(t, "store error", err.Error())
	})
}

func TestService_ConcurrentSync(t *testing.T) {
	service, store, client, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("sync multiple repositories", func(t *testing.T) {
		// Setup test repositories
		repos := []*models.Repository{
			{
				URL:      "https://github.com/test-owner/repo1",
				Name:     "repo1",
				Language: "Go",
			},
			{
				URL:      "https://github.com/test-owner/repo2",
				Name:     "repo2",
				Language: "Python",
			},
			{
				URL:      "https://github.com/test-owner/repo3",
				Name:     "repo3",
				Language: "JavaScript",
			},
		}

		for _, repo := range repos {
			client.repositories[repo.URL] = repo
			client.commits[repo.URL] = []*models.Commit{
				{
					RepositoryURL: repo.URL,
					SHA:          "abc123",
					Message:      "Test commit",
					AuthorName:   "Test Author",
					AuthorEmail:  "test@example.com",
					CommittedAt:  time.Now(),
					CommitURL:    "https://github.com/test-owner/" + repo.Name + "/commit/abc123",
				},
			}
		}

		// Start sync
		err := service.StartSync(ctx)
		require.NoError(t, err)

		// Add repositories to sync
		for _, repo := range repos {
			err := service.SyncRepository(ctx, repo.URL)
			require.NoError(t, err)
		}

		// Wait for sync to complete
		time.Sleep(time.Second)

		// Verify all repositories were synced
		for _, repo := range repos {
			saved, err := store.GetRepository(ctx, repo.URL)
			require.NoError(t, err)
			assert.Equal(t, repo.URL, saved.URL)

			commits, err := store.GetCommits(ctx, repo.ID, 10, 0)
			require.NoError(t, err)
			assert.Len(t, commits, 1)
		}
	})

	t.Run("handle sync errors", func(t *testing.T) {
		// Setup test repository with error
		repo := &models.Repository{
			URL:      "https://github.com/test-owner/error-repo",
			Name:     "error-repo",
			Language: "Go",
		}
		client.repositories[repo.URL] = repo
		client.err = errors.New("sync error")
		defer func() { client.err = nil }()

		// Start sync
		err := service.StartSync(ctx)
		require.NoError(t, err)

		// Add repository to sync
		err = service.SyncRepository(ctx, repo.URL)
		require.NoError(t, err)

		// Wait for sync to complete
		time.Sleep(time.Second)

		// Verify error was recorded
		status, err := service.GetSyncStatus(ctx, repo.URL)
		require.NoError(t, err)
		assert.NotNil(t, status)
		assert.NotNil(t, status.LastError)
		assert.Contains(t, status.LastError.Error(), "sync error")
	})

	t.Run("respect max concurrent syncs", func(t *testing.T) {
		// Setup test repositories
		repos := make([]*models.Repository, 5)
		for i := range repos {
			repo := &models.Repository{
				URL:      fmt.Sprintf("https://github.com/test-owner/concurrent-repo-%d", i),
				Name:     fmt.Sprintf("concurrent-repo-%d", i),
				Language: "Go",
			}
			repos[i] = repo
			client.repositories[repo.URL] = repo
		}

		// Start sync
		err := service.StartSync(ctx)
		require.NoError(t, err)

		// Add repositories to sync
		for _, repo := range repos {
			err := service.SyncRepository(ctx, repo.URL)
			require.NoError(t, err)
		}

		// Wait for initial syncs to start
		time.Sleep(time.Millisecond * 100)

		// Verify only max concurrent syncs are running
		activeSyncs := 0
		for _, repo := range repos {
			status, err := service.GetSyncStatus(ctx, repo.URL)
			require.NoError(t, err)
			if status != "" && status != "false" {
				activeSyncs++
			}
		}
		assert.LessOrEqual(t, activeSyncs, service.config.MaxConcurrentSyncs)
	})
}

func TestService_EdgeCases(t *testing.T) {
	service, store, client, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("sync interrupted repository", func(t *testing.T) {
		// Setup test repository
		repo := &models.Repository{
			URL:      "https://github.com/test-owner/interrupted-repo",
			Name:     "interrupted-repo",
			Language: "Go",
		}
		client.repositories[repo.URL] = repo

		// Start sync
		err := service.StartSync(ctx)
		require.NoError(t, err)

		// Add repository to sync
		err = service.SyncRepository(ctx, repo.URL)
		require.NoError(t, err)

		// Stop sync immediately
		service.StopSync()

		// Verify sync status
		status, err := service.GetSyncStatus(ctx, repo.URL)
		require.NoError(t, err)
		assert.NotNil(t, status)
		assert.False(t, status.IsSyncing)
	})

	t.Run("duplicate sync attempts", func(t *testing.T) {
		// Setup test repository
		repo := &models.Repository{
			URL:      "https://github.com/test-owner/duplicate-repo",
			Name:     "duplicate-repo",
			Language: "Go",
		}
		client.repositories[repo.URL] = repo

		// Start sync
		err := service.StartSync(ctx)
		require.NoError(t, err)

		// Add repository to sync multiple times
		for i := 0; i < 3; i++ {
			err := service.SyncRepository(ctx, repo.URL)
			require.NoError(t, err)
		}

		// Wait for sync to complete
		time.Sleep(time.Second)

		// Verify repository was only synced once
		commits, err := store.GetCommits(ctx, repo.ID, 10, 0)
		require.NoError(t, err)
		assert.Len(t, commits, 0) // No commits in mock client
	})

	t.Run("sync deleted repository", func(t *testing.T) {
		// Setup test repository
		repo := &models.Repository{
			URL:      "https://github.com/test-owner/deleted-repo",
			Name:     "deleted-repo",
			Language: "Go",
		}
		client.repositories[repo.URL] = repo

		// Start sync
		err := service.StartSync(ctx)
		require.NoError(t, err)

		// Add repository to sync
		err = service.SyncRepository(ctx, repo.URL)
		require.NoError(t, err)

		// Delete repository from client
		delete(client.repositories, repo.URL)

		// Wait for sync to complete
		time.Sleep(time.Second)

		// Verify error was recorded
		status, err := service.GetSyncStatus(ctx, repo.URL)
		require.NoError(t, err)
		assert.NotNil(t, status)
		assert.NotNil(t, status.LastError)
		assert.IsType(t, &RepositoryNotFoundError{}, status.LastError)
	})

	t.Run("sync with rate limit", func(t *testing.T) {
		// Setup test repository
		repo := &models.Repository{
			URL:      "https://github.com/test-owner/rate-limit-repo",
			Name:     "rate-limit-repo",
			Language: "Go",
		}
		client.repositories[repo.URL] = repo

		// Simulate rate limit error
		client.err = &RateLimitError{
			ResetTime: time.Now().Add(time.Second),
		}
		defer func() { client.err = nil }()

		// Start sync
		err := service.StartSync(ctx)
		require.NoError(t, err)

		// Add repository to sync
		err = service.SyncRepository(ctx, repo.URL)
		require.NoError(t, err)

		// Wait for sync to complete
		time.Sleep(time.Second)

		// Verify error was recorded
		status, err := service.GetSyncStatus(ctx, repo.URL)
		require.NoError(t, err)
		assert.NotNil(t, status)
		assert.NotNil(t, status.LastError)
		assert.IsType(t, &RateLimitError{}, status.LastError)
	})
}

// MockDB is a mock implementation of db.Repository
type MockDB struct {
	mock.Mock
}

func (m *MockDB) CreateRepository(ctx context.Context, repo *models.Repository) error {
	args := m.Called(ctx, repo)
	return args.Error(0)
}

func (m *MockDB) GetRepositoryByID(ctx context.Context, id uint) (*models.Repository, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Repository), args.Error(1)
}

func (m *MockDB) GetRepositoryByURL(ctx context.Context, url string) (*models.Repository, error) {
	args := m.Called(ctx, url)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Repository), args.Error(1)
}

func (m *MockDB) ListRepositories(ctx context.Context) ([]models.Repository, error) {
	args := m.Called(ctx)
	return args.Get(0).([]models.Repository), args.Error(1)
}

func (m *MockDB) DeleteRepository(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockDB) UpdateRepositorySyncConfig(ctx context.Context, id uint, config *models.RepositorySyncConfig) error {
	args := m.Called(ctx, id, config)
	return args.Error(0)
}

func (m *MockDB) CreateCommit(ctx context.Context, commit *models.Commit) error {
	args := m.Called(ctx, commit)
	return args.Error(0)
}

func (m *MockDB) GetCommitsByRepositoryID(ctx context.Context, repoID uint) ([]models.Commit, error) {
	args := m.Called(ctx, repoID)
	return args.Get(0).([]models.Commit), args.Error(1)
}

func (m *MockDB) GetCommitsByRepositoryURL(ctx context.Context, repoURL string) ([]models.Commit, error) {
	args := m.Called(ctx, repoURL)
	return args.Get(0).([]models.Commit), args.Error(1)
}

func (m *MockDB) GetAuthorsByRepositoryID(ctx context.Context, repoID uint) ([]models.Author, error) {
	args := m.Called(ctx, repoID)
	return args.Get(0).([]models.Author), args.Error(1)
}

func (m *MockDB) GetAuthorsByRepositoryURL(ctx context.Context, repoURL string) ([]models.Author, error) {
	args := m.Called(ctx, repoURL)
	return args.Get(0).([]models.Author), args.Error(1)
}

func (m *MockDB) UpdateSyncStatus(ctx context.Context, repoURL string, status string) error {
	args := m.Called(ctx, repoURL, status)
	return args.Error(0)
}

func (m *MockDB) GetSyncStatus(ctx context.Context, repoURL string) (string, error) {
	args := m.Called(ctx, repoURL)
	return args.Get(0).(string), args.Error(1)
}

func (m *MockDB) GetAllSyncStatuses(ctx context.Context) (map[string]string, error) {
	args := m.Called(ctx)
	return args.Get(0).(map[string]string), args.Error(1)
}

func (m *MockDB) ClearSyncStatus(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func setupTestService() (*RepositoryService, *MockDB) {
	mockDB := new(MockDB)
	service := NewRepositoryService(mockDB)
	return service, mockDB
}

func TestAddRepository(t *testing.T) {
	service, mockDB := setupTestService()
	ctx := context.Background()

	tests := []struct {
		name           string
		request        AddRepositoryRequest
		mockRepo       *models.Repository
		mockError      error
		expectedError  bool
		expectedRepo   *models.Repository
	}{
		{
			name: "successful repository addition",
			request: AddRepositoryRequest{
				URL: "https://github.com/owner/repo",
				SyncConfig: &models.RepositorySyncConfig{
					Enabled: true,
					SyncInterval: models.FromDuration(3600 * time.Second),
				},
			},
			mockRepo: &models.Repository{
				BaseModel: models.BaseModel{ID: 1},
				URL: "https://github.com/owner/repo",
				SyncConfig: models.RepositorySyncConfig{
					Enabled: true,
					SyncInterval: models.FromDuration(3600 * time.Second),
				},
			},
			mockError:     nil,
			expectedError: false,
			expectedRepo: &models.Repository{
				BaseModel: models.BaseModel{ID: 1},
				URL: "https://github.com/owner/repo",
				SyncConfig: models.RepositorySyncConfig{
					Enabled: true,
					SyncInterval: models.FromDuration(3600 * time.Second),
				},
			},
		},
		{
			name: "invalid repository URL",
			request: AddRepositoryRequest{
				URL: "invalid-url",
			},
			mockRepo:      nil,
			mockError:     nil,
			expectedError: true,
			expectedRepo:  nil,
		},
		{
			name: "repository already exists",
			request: AddRepositoryRequest{
				URL: "https://github.com/owner/repo",
			},
			mockRepo: &models.Repository{
				ID:  1,
				URL: "https://github.com/owner/repo",
			},
			mockError:     nil,
			expectedError: true,
			expectedRepo:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.mockRepo != nil {
				mockDB.On("GetRepositoryByURL", ctx, tt.request.URL).Return(tt.mockRepo, tt.mockError)
				if !tt.expectedError {
					mockDB.On("CreateRepository", ctx, mock.AnythingOfType("*models.Repository")).Return(nil)
				}
			} else {
				mockDB.On("GetRepositoryByURL", ctx, tt.request.URL).Return(nil, db.ErrNotFound)
				if !tt.expectedError {
					mockDB.On("CreateRepository", ctx, mock.AnythingOfType("*models.Repository")).Return(nil)
				}
			}

			repo, err := service.AddRepository(ctx, tt.request)

			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, repo)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, repo)
				assert.Equal(t, tt.expectedRepo.URL, repo.URL)
				assert.Equal(t, tt.expectedRepo.SyncConfig, repo.SyncConfig)
			}
		})
	}
	mockDB.AssertExpectations(t)
}

func TestGetRepositoryByID(t *testing.T) {
	service, mockDB := setupTestService()
	ctx := context.Background()

	tests := []struct {
		name          string
		repoID        uint
		mockRepo      *models.Repository
		mockError     error
		expectedError bool
		expectedRepo  *models.Repository
	}{
		{
			name:   "successful repository retrieval",
			repoID: 1,
			mockRepo: &models.Repository{
				ID:  1,
				URL: "https://github.com/owner/repo",
			},
			mockError:     nil,
			expectedError: false,
			expectedRepo: &models.Repository{
				ID:  1,
				URL: "https://github.com/owner/repo",
			},
		},
		{
			name:          "repository not found",
			repoID:        999,
			mockRepo:      nil,
			mockError:     db.ErrNotFound,
			expectedError: true,
			expectedRepo:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDB.On("GetRepositoryByID", ctx, tt.repoID).Return(tt.mockRepo, tt.mockError)

			repo, err := service.GetRepositoryByID(ctx, tt.repoID)

			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, repo)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, repo)
				assert.Equal(t, tt.expectedRepo.ID, repo.ID)
				assert.Equal(t, tt.expectedRepo.URL, repo.URL)
			}
		})
	}
	mockDB.AssertExpectations(t)
}

func TestListRepositories(t *testing.T) {
	service, mockDB := setupTestService()
	ctx := context.Background()

	tests := []struct {
		name           string
		mockRepos      []models.Repository
		mockError      error
		expectedError  bool
		expectedRepos  []models.Repository
	}{
		{
			name: "successful repository list",
			mockRepos: []models.Repository{
				{
					ID:  1,
					URL: "https://github.com/owner1/repo1",
				},
				{
					ID:  2,
					URL: "https://github.com/owner2/repo2",
				},
			},
			mockError:     nil,
			expectedError: false,
			expectedRepos: []models.Repository{
				{
					ID:  1,
					URL: "https://github.com/owner1/repo1",
				},
				{
					ID:  2,
					URL: "https://github.com/owner2/repo2",
				},
			},
		},
		{
			name:           "empty repository list",
			mockRepos:      []models.Repository{},
			mockError:      nil,
			expectedError:  false,
			expectedRepos:  []models.Repository{},
		},
		{
			name:           "database error",
			mockRepos:      nil,
			mockError:      assert.AnError,
			expectedError:  true,
			expectedRepos:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDB.On("ListRepositories", ctx).Return(tt.mockRepos, tt.mockError)

			repos, err := service.ListRepositories(ctx)

			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, repos)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, repos)
				assert.Equal(t, len(tt.expectedRepos), len(repos))
				for i, repo := range repos {
					assert.Equal(t, tt.expectedRepos[i].ID, repo.ID)
					assert.Equal(t, tt.expectedRepos[i].URL, repo.URL)
				}
			}
		})
	}
	mockDB.AssertExpectations(t)
}

func TestDeleteRepository(t *testing.T) {
	service, mockDB := setupTestService()
	ctx := context.Background()

	tests := []struct {
		name          string
		repoID        uint
		mockError     error
		expectedError bool
	}{
		{
			name:          "successful repository deletion",
			repoID:        1,
			mockError:     nil,
			expectedError: false,
		},
		{
			name:          "repository not found",
			repoID:        999,
			mockError:     db.ErrNotFound,
			expectedError: true,
		},
		{
			name:          "database error",
			repoID:        1,
			mockError:     assert.AnError,
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDB.On("DeleteRepository", ctx, tt.repoID).Return(tt.mockError)

			err := service.DeleteRepositoryByID(ctx, tt.repoID)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
	mockDB.AssertExpectations(t)
} 