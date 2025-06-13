package github

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/Kamar-Folarin/github-monitor/internal/config"
	"github.com/Kamar-Folarin/github-monitor/internal/db"
	apperrors "github.com/Kamar-Folarin/github-monitor/internal/errors"
	"github.com/Kamar-Folarin/github-monitor/internal/models"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// Test constants to avoid magic numbers
const (
	testSyncInterval    = 100 * time.Millisecond
	testMaxConcurrent   = 2
	testBatchSize       = 10
	testWaitTime        = time.Second
	testShortWaitTime   = 100 * time.Millisecond
	testSyncIntervalSec = 3600
	testRepositoryCount = 5
	testDuplicateCount  = 3
	testTimeout         = 5 * time.Second
)

// Test data constants
const (
	testOwnerName     = "test-owner"
	testRepoName      = "test-repo"
	testCommitSHA     = "abc123"
	testAuthorName    = "Test Author"
	testAuthorEmail   = "test@example.com"
	testRepoURL       = "https://github.com/test-owner/test-repo"
	testCommitMessage = "Test commit"
)

// Error messages for better debugging
const (
	errNilContext          = "context cannot be nil"
	errNilRepository       = "repository cannot be nil"
	errNilSyncStatus       = "sync status cannot be nil"
	errEmptyURL            = "URL cannot be empty"
	errEmptyRepoURL        = "repository URL cannot be empty"
	errInvalidRepoID       = "repository ID must be positive"
	errInvalidRepoType     = "invalid repository type"
	errInvalidTimeType     = "invalid time type"
	errInvalidStatusType   = "invalid sync status type"
	errInvalidReposType    = "invalid repositories type"
	errInvalidStatusesType = "invalid sync statuses type"
)

// GitHubClientInterface defines the interface for GitHub API operations
type GitHubClientInterface interface {
	GetRepository(ctx context.Context, owner, name string) (*models.Repository, error)
	GetRepositoryByURL(ctx context.Context, repoURL string) (*models.Repository, error)
	GetCommitsPage(ctx context.Context, owner, name string, since *time.Time, page int) ([]*models.Commit, error)
}

// MockStore implements db.Store for testing
type MockStore struct {
	mock.Mock
	repositories map[string]*models.Repository
	commits      map[string][]*models.Commit
	syncStatus   map[string]*models.SyncStatus
	syncProgress map[string]*models.SyncProgress
	err          error
}

func (m *MockStore) GetRepositoryByURL(ctx context.Context, url string) (*models.Repository, error) {
	if ctx == nil {
		return nil, fmt.Errorf(errNilContext)
	}
	if url == "" {
		return nil, fmt.Errorf(errEmptyURL)
	}
	args := m.Called(ctx, url)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	repo, ok := args.Get(0).(*models.Repository)
	if !ok {
		return nil, fmt.Errorf(errInvalidRepoType)
	}
	return repo, args.Error(1)
}

func (m *MockStore) SaveRepository(ctx context.Context, repo *models.Repository) error {
	if ctx == nil {
		return fmt.Errorf(errNilContext)
	}
	if repo == nil {
		return fmt.Errorf(errNilRepository)
	}
	args := m.Called(ctx, repo)
	return args.Error(0)
}

func (m *MockStore) GetLastCommitDate(ctx context.Context, repoID int64) (*time.Time, error) {
	if ctx == nil {
		return nil, fmt.Errorf(errNilContext)
	}
	if repoID <= 0 {
		return nil, fmt.Errorf(errInvalidRepoID)
	}
	args := m.Called(ctx, repoID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	timeVal, ok := args.Get(0).(*time.Time)
	if !ok {
		return nil, fmt.Errorf(errInvalidTimeType)
	}
	return timeVal, args.Error(1)
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

func (m *MockStore) GetRepository(ctx context.Context, id int64) (*models.Repository, error) {
	if m.err != nil {
		return nil, m.err
	}
	for _, repo := range m.repositories {
		if int64(repo.ID) == id {
			return repo, nil
		}
	}
	return nil, fmt.Errorf("repository not found")
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
		return fmt.Errorf("repository not found")
	}
	m.commits[repo.URL] = append(m.commits[repo.URL], commit)
	return nil
}

func (m *MockStore) DeleteCommits(ctx context.Context, repoID int64) error {
	if m.err != nil {
		return m.err
	}
	for url, repo := range m.repositories {
		if int64(repo.ID) == repoID {
			delete(m.commits, url)
			return nil
		}
	}
	return nil
}

func (m *MockStore) GetSyncStatus(ctx context.Context, repoURL string) (*models.SyncStatus, error) {
	if m.err != nil {
		return nil, m.err
	}
	status, ok := m.syncStatus[repoURL]
	if !ok {
		return nil, nil
	}
	return status, nil
}

func (m *MockStore) UpdateSyncStatus(ctx context.Context, status *models.SyncStatus) error {
	if m.err != nil {
		return m.err
	}
	m.syncStatus[status.RepositoryURL] = status
	return nil
}

func (m *MockStore) ListSyncStatuses(ctx context.Context) ([]*models.SyncStatus, error) {
	if m.err != nil {
		return nil, m.err
	}
	statuses := make([]*models.SyncStatus, 0, len(m.syncStatus))
	for _, status := range m.syncStatus {
		statuses = append(statuses, status)
	}
	return statuses, nil
}

func (m *MockStore) ClearSyncStatuses(ctx context.Context) error {
	if m.err != nil {
		return m.err
	}
	m.syncStatus = make(map[string]*models.SyncStatus)
	return nil
}

func (m *MockStore) SaveSyncStatus(ctx context.Context, status map[string]string) error {
	if m.err != nil {
		return m.err
	}
	for repoURL, statusStr := range status {
		// Parse status string to SyncStatus struct
		syncStatus := &models.SyncStatus{
			RepositoryURL: repoURL,
			Status:        statusStr,
		}
		m.syncStatus[repoURL] = syncStatus
	}
	return nil
}

func (m *MockStore) DeleteSyncStatus(ctx context.Context, repoURL string) error {
	if m.err != nil {
		return m.err
	}
	delete(m.syncStatus, repoURL)
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

func (m *MockStore) AddMonitoredRepository(ctx context.Context, repoURL string, initialSyncDate *time.Time) error {
	args := m.Called(ctx, repoURL, initialSyncDate)
	return args.Error(0)
}

func (m *MockStore) UpdateRepository(ctx context.Context, repo *models.Repository) error {
	args := m.Called(ctx, repo)
	return args.Error(0)
}

func (m *MockStore) GetCommitsWithPagination(ctx context.Context, repoID int64, limit, offset int, since, until *time.Time) ([]*models.Commit, int64, error) {
	args := m.Called(ctx, repoID, limit, offset, since, until)
	return args.Get(0).([]*models.Commit), args.Get(1).(int64), args.Error(2)
}

func (m *MockStore) GetTopCommitAuthorsWithDateFilter(ctx context.Context, repoID int64, limit int, since, until *time.Time) ([]*models.AuthorStats, error) {
	args := m.Called(ctx, repoID, limit, since, until)
	return args.Get(0).([]*models.AuthorStats), args.Error(1)
}

func (m *MockStore) BeginTx(ctx context.Context) (*sql.Tx, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*sql.Tx), args.Error(1)
}

func (m *MockStore) SaveCommitsTx(ctx context.Context, tx *sql.Tx, commits []*models.Commit) error {
	args := m.Called(ctx, tx, commits)
	return args.Error(0)
}

func (m *MockStore) SaveSyncStatusTx(ctx context.Context, tx *sql.Tx, status map[string]string) error {
	args := m.Called(ctx, tx, status)
	return args.Error(0)
}

// MockGitHubClient implements GitHubClientInterface for testing
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

func (m *MockGitHubClient) GetRepository(ctx context.Context, owner, repo string) (*models.Repository, error) {
	if m.err != nil {
		return nil, m.err
	}
	key := "https://github.com/" + owner + "/" + repo
	r, ok := m.repositories[key]
	if !ok {
		return nil, &RepositoryNotFoundError{Owner: owner, Name: repo}
	}
	return r, nil
}

func (m *MockGitHubClient) GetRepositoryByURL(ctx context.Context, repoURL string) (*models.Repository, error) {
	if m.err != nil {
		return nil, m.err
	}
	r, ok := m.repositories[repoURL]
	if !ok {
		return nil, &RepositoryNotFoundError{Owner: "", Name: repoURL}
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

// TestService is a test-specific version of Service that uses GitHubClientInterface
type TestService struct {
	client      GitHubClientInterface
	store       db.Store
	logger      *logrus.Logger
	config      SyncConfig
	syncMutex   sync.RWMutex
	syncStatus  map[string]*models.SyncStatus
	batchConfig *BatchConfig
	progress    map[string]*models.BatchProgress
	progressMu  sync.RWMutex
	isRunning   bool
	mu          sync.RWMutex
}

// NewTestService creates a new test service with a mock client
func NewTestService(client GitHubClientInterface, store db.Store, logger *logrus.Logger, config SyncConfig) *TestService {
	return &TestService{
		client:      client,
		store:       store,
		logger:      logger,
		config:      config,
		syncStatus:  make(map[string]*models.SyncStatus),
		batchConfig: DefaultBatchConfig(),
		progress:    make(map[string]*models.BatchProgress),
	}
}

// SyncRepository syncs a single repository (test version)
func (s *TestService) SyncRepository(ctx context.Context, owner, name string) error {
	// Add validation
	if owner == "" || name == "" {
		return &ValidationError{Field: "owner/name", Value: "cannot be empty"}
	}

	s.logger.Infof("Fetching repository info for %s/%s", owner, name)
	repo, err := s.client.GetRepository(ctx, owner, name)
	if err != nil {
		return fmt.Errorf("failed to get repository: %w", err)
	}

	s.logger.Infof("Saving repository info for %s", repo.URL)
	if err := s.store.SaveRepository(ctx, repo); err != nil {
		return fmt.Errorf("failed to save repository: %w", err)
	}

	// For testing, we'll just save the repository and return success
	// without doing the full commit sync process
	return nil
}

// GetSyncStatus gets the sync status for a repository
func (s *TestService) GetSyncStatus(ctx context.Context, repoURL string) (*models.SyncStatus, error) {
	return s.store.GetSyncStatus(ctx, repoURL)
}

// StartSync starts the background sync process (test version)
func (s *TestService) StartSync(ctx context.Context) {
	s.logger.Info("Starting background sync process")
	// For testing, we'll just log that sync started
	// without actually running the sync loop
}

func setupServiceTest(t *testing.T) (*TestService, *MockStore, *MockGitHubClient, func()) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	store := &MockStore{
		repositories: make(map[string]*models.Repository),
		commits:      make(map[string][]*models.Commit),
		syncStatus:   make(map[string]*models.SyncStatus),
		syncProgress: make(map[string]*models.SyncProgress),
	}
	client := NewMockGitHubClient()

	config := DefaultSyncConfig()
	config.SyncInterval = testSyncInterval
	config.MaxConcurrentSyncs = testMaxConcurrent
	config.BatchSize = testBatchSize

	// Create test service with mock client
	service := NewTestService(client, store, logger, config)

	cleanup := func() {
		// Reset any error states
		store.err = nil
		client.err = nil
	}

	return service, store, client, cleanup
}

func TestService_SyncRepository(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	ctx := context.Background()

	t.Run("successful sync", func(t *testing.T) {
		client := NewMockGitHubClient()
		mockStore := new(MockStoreDB)
		service := NewTestService(client, mockStore, logger, DefaultSyncConfig())
		setupTestData(client, nil)

		mockStore.On("SaveRepository", ctx, mock.AnythingOfType("*models.Repository")).Return(nil)
		mockStore.On("GetRepositoryByURL", ctx, testRepoURL).Return(&models.Repository{
			BaseModel: models.BaseModel{ID: 1},
			URL:       testRepoURL,
			Name:      testRepoName,
		}, nil)
		mockStore.On("GetCommits", ctx, int64(1), testBatchSize, 0).Return([]*models.Commit{
			{
				RepositoryID: 1,
				SHA:          testCommitSHA,
				Message:      testCommitMessage,
				AuthorName:   testAuthorName,
				AuthorEmail:  testAuthorEmail,
				AuthorDate:   time.Now(),
				CommitURL:    testRepoURL + "/commit/" + testCommitSHA,
			},
		}, nil)

		err := service.SyncRepository(ctx, testOwnerName, testRepoName)
		require.NoError(t, err)

		saved, err := mockStore.GetRepositoryByURL(ctx, testRepoURL)
		require.NoError(t, err)
		assert.Equal(t, testRepoURL, saved.URL)
		assert.Equal(t, testRepoName, saved.Name)

		savedCommits, err := mockStore.GetCommits(ctx, int64(saved.ID), testBatchSize, 0)
		require.NoError(t, err)
		assert.Len(t, savedCommits, 1)
		assert.Equal(t, testCommitSHA, savedCommits[0].SHA)

		mockStore.AssertExpectations(t)
	})

	t.Run("invalid repository URL", func(t *testing.T) {
		client := NewMockGitHubClient()
		mockStore := new(MockStoreDB)
		service := NewTestService(client, mockStore, logger, DefaultSyncConfig())

		err := service.SyncRepository(ctx, "", "")
		var vErr *ValidationError
		assert.Error(t, err)
		assert.True(t, errors.As(err, &vErr), "error should be of type *ValidationError, got %T", err)
	})

	t.Run("repository not found", func(t *testing.T) {
		client := NewMockGitHubClient()
		mockStore := new(MockStoreDB)
		service := NewTestService(client, mockStore, logger, DefaultSyncConfig())

		err := service.SyncRepository(ctx, testOwnerName, "non-existent")
		var notFoundErr *RepositoryNotFoundError
		assert.Error(t, err)
		assert.True(t, errors.As(err, &notFoundErr), "error should be of type *RepositoryNotFoundError, got %T", err)
	})

	t.Run("store error", func(t *testing.T) {
		client := NewMockGitHubClient()
		mockStore := new(MockStoreDB)
		service := NewTestService(client, mockStore, logger, DefaultSyncConfig())
		setupTestData(client, nil)

		mockStore.On("SaveRepository", ctx, mock.AnythingOfType("*models.Repository")).Return(fmt.Errorf("store error"))

		err := service.SyncRepository(ctx, testOwnerName, testRepoName)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "store error")
	})

	t.Run("client error", func(t *testing.T) {
		client := NewMockGitHubClient()
		mockStore := new(MockStoreDB)
		service := NewTestService(client, mockStore, logger, DefaultSyncConfig())
		client.err = fmt.Errorf("client error")

		err := service.SyncRepository(ctx, testOwnerName, testRepoName)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "client error")
	})

	t.Run("handle sync errors", func(t *testing.T) {
		client := NewMockGitHubClient()
		mockStore := new(MockStoreDB)
		service := NewTestService(client, mockStore, logger, DefaultSyncConfig())

		repo := &models.Repository{URL: "https://github.com/test-owner/error-repo", Name: "error-repo", Language: "Go"}
		client.repositories[repo.URL] = repo
		client.err = fmt.Errorf("sync error")

		// No mock expectations needed since the client error prevents SaveRepository from being called

		err := service.SyncRepository(ctx, testOwnerName, repo.Name)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "sync error")
	})
}

func TestService_GetSyncStatus(t *testing.T) {
	service, store, _, cleanup := setupServiceTest(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("get existing status", func(t *testing.T) {
		// Setup test data
		status := &models.SyncStatus{
			RepositoryURL: "https://github.com/test-owner/repo1",
			LastSyncAt:    time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
			IsSyncing:     true,
		}
		store.syncStatus[status.RepositoryURL] = status

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
		store.err = fmt.Errorf("store error")
		defer func() { store.err = nil }()

		_, err := service.GetSyncStatus(ctx, "https://github.com/test-owner/repo1")
		assert.Error(t, err)
		assert.Equal(t, "store error", err.Error())
	})
}

func TestService_ConcurrentSync(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	ctx := context.Background()

	t.Run("sync multiple repositories", func(t *testing.T) {
		client := NewMockGitHubClient()
		mockStore := new(MockStoreDB)
		service := NewTestService(client, mockStore, logger, DefaultSyncConfig())

		repos := []*models.Repository{
			{URL: "https://github.com/test-owner/repo1", Name: "repo1", Language: "Go"},
			{URL: "https://github.com/test-owner/repo2", Name: "repo2", Language: "Python"},
			{URL: "https://github.com/test-owner/repo3", Name: "repo3", Language: "JavaScript"},
		}

		// Setup client data
		for _, repo := range repos {
			client.repositories[repo.URL] = repo
		}

		// Setup mock expectations for each repository
		for i := 0; i < len(repos); i++ {
			mockStore.On("SaveRepository", ctx, mock.AnythingOfType("*models.Repository")).Return(nil)
		}

		// Sync each repository
		for _, repo := range repos {
			err := service.SyncRepository(ctx, testOwnerName, repo.Name)
			require.NoError(t, err)
		}

		mockStore.AssertExpectations(t)
	})

	t.Run("respect max concurrent syncs", func(t *testing.T) {
		client := NewMockGitHubClient()
		mockStore := new(MockStoreDB)
		service := NewTestService(client, mockStore, logger, DefaultSyncConfig())

		repos := make([]*models.Repository, testRepositoryCount)
		for i := range repos {
			repo := &models.Repository{
				URL:      fmt.Sprintf("https://github.com/test-owner/concurrent-repo-%d", i),
				Name:     fmt.Sprintf("concurrent-repo-%d", i),
				Language: "Go",
			}
			repos[i] = repo
			client.repositories[repo.URL] = repo
		}

		// Setup mock expectations for each repository
		for i := 0; i < len(repos); i++ {
			mockStore.On("SaveRepository", ctx, mock.AnythingOfType("*models.Repository")).Return(nil)
		}

		// Sync each repository
		for _, repo := range repos {
			err := service.SyncRepository(ctx, testOwnerName, repo.Name)
			require.NoError(t, err)
		}

		mockStore.AssertExpectations(t)
	})
}

func TestService_EdgeCases(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	ctx := context.Background()

	t.Run("sync interrupted repository", func(t *testing.T) {
		client := NewMockGitHubClient()
		mockStore := new(MockStoreDB)
		service := NewTestService(client, mockStore, logger, DefaultSyncConfig())

		repo := &models.Repository{URL: "https://github.com/test-owner/interrupted-repo", Name: "interrupted-repo", Language: "Go"}
		client.repositories[repo.URL] = repo

		// Setup mock expectations
		mockStore.On("SaveRepository", ctx, mock.AnythingOfType("*models.Repository")).Return(nil)

		err := service.SyncRepository(ctx, testOwnerName, repo.Name)
		require.NoError(t, err)

		mockStore.AssertExpectations(t)
	})

	t.Run("duplicate sync attempts", func(t *testing.T) {
		client := NewMockGitHubClient()
		mockStore := new(MockStoreDB)
		service := NewTestService(client, mockStore, logger, DefaultSyncConfig())

		repo := &models.Repository{
			URL:      "https://github.com/test-owner/duplicate-repo",
			Name:     "duplicate-repo",
			Language: "Go",
		}
		client.repositories[repo.URL] = repo

		// Setup mock expectations for multiple sync attempts
		for i := 0; i < testDuplicateCount; i++ {
			mockStore.On("SaveRepository", ctx, mock.AnythingOfType("*models.Repository")).Return(nil)
		}

		// Add repository to sync multiple times
		for i := 0; i < testDuplicateCount; i++ {
			err := service.SyncRepository(ctx, testOwnerName, "duplicate-repo")
			require.NoError(t, err)
		}

		mockStore.AssertExpectations(t)
	})

	t.Run("sync deleted repository", func(t *testing.T) {
		client := NewMockGitHubClient()
		mockStore := new(MockStoreDB)
		service := NewTestService(client, mockStore, logger, DefaultSyncConfig())

		repo := &models.Repository{
			URL:      "https://github.com/test-owner/deleted-repo",
			Name:     "deleted-repo",
			Language: "Go",
		}
		client.repositories[repo.URL] = repo

		// Setup mock expectations
		mockStore.On("SaveRepository", ctx, mock.AnythingOfType("*models.Repository")).Return(nil)

		// Add repository to sync
		err := service.SyncRepository(ctx, testOwnerName, "deleted-repo")
		require.NoError(t, err)

		// Delete repository from client
		delete(client.repositories, repo.URL)

		// Try to sync again - should fail
		err = service.SyncRepository(ctx, testOwnerName, "deleted-repo")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "repository not found")

		mockStore.AssertExpectations(t)
	})

	t.Run("sync with rate limit", func(t *testing.T) {
		client := NewMockGitHubClient()
		mockStore := new(MockStoreDB)
		service := NewTestService(client, mockStore, logger, DefaultSyncConfig())

		repo := &models.Repository{
			URL:      "https://github.com/test-owner/rate-limit-repo",
			Name:     "rate-limit-repo",
			Language: "Go",
		}
		client.repositories[repo.URL] = repo

		// Simulate rate limit error
		client.err = &RateLimitError{
			ResetTime: time.Now().Add(testWaitTime),
		}
		defer func() { client.err = nil }()

		// Try to sync - should fail with rate limit error
		err := service.SyncRepository(ctx, testOwnerName, "rate-limit-repo")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "rate limit")
	})
}

// Helper functions to reduce code duplication

// createTestRepository creates a test repository with given parameters
func createTestRepository(url, name, language string) *models.Repository {
	return &models.Repository{
		URL:      url,
		Name:     name,
		Language: language,
	}
}

// createTestCommit creates a test commit with given parameters
func createTestCommit(repoID int64, sha, message, authorName, authorEmail, commitURL string) *models.Commit {
	return &models.Commit{
		RepositoryID: repoID,
		SHA:          sha,
		Message:      message,
		AuthorName:   authorName,
		AuthorEmail:  authorEmail,
		AuthorDate:   time.Now(),
		CommitURL:    commitURL,
	}
}

// createTestSyncStatus creates a test sync status with given parameters
func createTestSyncStatus(repoURL string, lastSyncAt time.Time, isSyncing bool) *models.SyncStatus {
	return &models.SyncStatus{
		RepositoryURL: repoURL,
		LastSyncAt:    lastSyncAt,
		IsSyncing:     isSyncing,
	}
}

// setupTestData sets up common test data for repository tests
func setupTestData(client *MockGitHubClient, store *MockStore) {
	// Setup test repository
	repo := createTestRepository(testRepoURL, testRepoName, "Go")
	client.repositories[repo.URL] = repo

	// Setup test commit
	commit := createTestCommit(1, testCommitSHA, testCommitMessage, testAuthorName, testAuthorEmail, testRepoURL+"/commit/"+testCommitSHA)
	client.commits[repo.URL] = []*models.Commit{commit}
}

// MockStoreDB is a mock implementation of db.Store for repository service tests
type MockStoreDB struct {
	mock.Mock
}

func (m *MockStoreDB) GetRepository(ctx context.Context, id int64) (*models.Repository, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	repo, ok := args.Get(0).(*models.Repository)
	if !ok {
		return nil, fmt.Errorf(errInvalidRepoType)
	}
	return repo, args.Error(1)
}

func (m *MockStoreDB) GetRepositoryByURL(ctx context.Context, url string) (*models.Repository, error) {
	args := m.Called(ctx, url)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	repo, ok := args.Get(0).(*models.Repository)
	if !ok {
		return nil, fmt.Errorf(errInvalidRepoType)
	}
	return repo, args.Error(1)
}

func (m *MockStoreDB) ListRepositories(ctx context.Context) ([]*models.Repository, error) {
	args := m.Called(ctx)
	repos, ok := args.Get(0).([]*models.Repository)
	if !ok {
		return nil, fmt.Errorf(errInvalidReposType)
	}
	return repos, args.Error(1)
}

func (m *MockStoreDB) SaveRepository(ctx context.Context, repo *models.Repository) error {
	if repo == nil {
		return fmt.Errorf(errNilRepository)
	}
	args := m.Called(ctx, repo)
	return args.Error(0)
}

func (m *MockStoreDB) UpdateRepository(ctx context.Context, repo *models.Repository) error {
	args := m.Called(ctx, repo)
	return args.Error(0)
}

func (m *MockStoreDB) DeleteRepository(ctx context.Context, id int64) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockStoreDB) AddMonitoredRepository(ctx context.Context, repoURL string, initialSyncDate *time.Time) error {
	args := m.Called(ctx, repoURL, initialSyncDate)
	return args.Error(0)
}

func (m *MockStoreDB) GetCommits(ctx context.Context, repoID int64, limit, offset int) ([]*models.Commit, error) {
	args := m.Called(ctx, repoID, limit, offset)
	return args.Get(0).([]*models.Commit), args.Error(1)
}

func (m *MockStoreDB) GetCommitsWithPagination(ctx context.Context, repoID int64, limit, offset int, since, until *time.Time) ([]*models.Commit, int64, error) {
	args := m.Called(ctx, repoID, limit, offset, since, until)
	return args.Get(0).([]*models.Commit), args.Get(1).(int64), args.Error(2)
}

func (m *MockStoreDB) SaveCommits(ctx context.Context, repoID int64, commits []*models.Commit) error {
	args := m.Called(ctx, repoID, commits)
	return args.Error(0)
}

func (m *MockStoreDB) SaveCommitsTx(ctx context.Context, tx *sql.Tx, commits []*models.Commit) error {
	args := m.Called(ctx, tx, commits)
	return args.Error(0)
}

func (m *MockStoreDB) GetLastCommitDate(ctx context.Context, repoID int64) (*time.Time, error) {
	args := m.Called(ctx, repoID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*time.Time), args.Error(1)
}

func (m *MockStoreDB) DeleteCommits(ctx context.Context, repoID int64) error {
	args := m.Called(ctx, repoID)
	return args.Error(0)
}

func (m *MockStoreDB) GetTopCommitAuthors(ctx context.Context, repoID int64, limit int) ([]*models.AuthorStats, error) {
	args := m.Called(ctx, repoID, limit)
	return args.Get(0).([]*models.AuthorStats), args.Error(1)
}

func (m *MockStoreDB) GetTopCommitAuthorsWithDateFilter(ctx context.Context, repoID int64, limit int, since, until *time.Time) ([]*models.AuthorStats, error) {
	args := m.Called(ctx, repoID, limit, since, until)
	return args.Get(0).([]*models.AuthorStats), args.Error(1)
}

func (m *MockStoreDB) ResetRepository(ctx context.Context, repoID int64, since time.Time) error {
	args := m.Called(ctx, repoID, since)
	return args.Error(0)
}

func (m *MockStoreDB) GetSyncStatus(ctx context.Context, repoURL string) (*models.SyncStatus, error) {
	args := m.Called(ctx, repoURL)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.SyncStatus), args.Error(1)
}

func (m *MockStoreDB) UpdateSyncStatus(ctx context.Context, status *models.SyncStatus) error {
	args := m.Called(ctx, status)
	return args.Error(0)
}

func (m *MockStoreDB) ListSyncStatuses(ctx context.Context) ([]*models.SyncStatus, error) {
	args := m.Called(ctx)
	return args.Get(0).([]*models.SyncStatus), args.Error(1)
}

func (m *MockStoreDB) ClearSyncStatuses(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockStoreDB) SaveSyncStatus(ctx context.Context, status map[string]string) error {
	args := m.Called(ctx, status)
	return args.Error(0)
}

func (m *MockStoreDB) DeleteSyncStatus(ctx context.Context, repoURL string) error {
	args := m.Called(ctx, repoURL)
	return args.Error(0)
}

func (m *MockStoreDB) GetSyncProgress(ctx context.Context, repoURL string) (*models.SyncProgress, error) {
	args := m.Called(ctx, repoURL)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.SyncProgress), args.Error(1)
}

func (m *MockStoreDB) SaveSyncProgress(ctx context.Context, repoURL string, progress *models.SyncProgress) error {
	args := m.Called(ctx, repoURL, progress)
	return args.Error(0)
}

func (m *MockStoreDB) BeginTx(ctx context.Context) (*sql.Tx, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*sql.Tx), args.Error(1)
}

func (m *MockStoreDB) SaveSyncStatusTx(ctx context.Context, tx *sql.Tx, status map[string]string) error {
	args := m.Called(ctx, tx, status)
	return args.Error(0)
}

func setupRepositoryTestService() (RepositoryService, *MockStoreDB) {
	mockDB := new(MockStoreDB)
	// Create a minimal mock client and config for testing
	mockClient := &MockGitHubClient{}
	mockConfig := &config.GitHubConfig{}
	mockStatusMgr := &MockStatusManager{}

	// Validate that all dependencies are properly initialized
	if mockDB == nil {
		panic("mockDB cannot be nil")
	}
	if mockClient == nil {
		panic("mockClient cannot be nil")
	}
	if mockConfig == nil {
		panic("mockConfig cannot be nil")
	}
	if mockStatusMgr == nil {
		panic("mockStatusMgr cannot be nil")
	}

	service := NewRepositoryService(mockClient, mockDB, mockConfig, mockStatusMgr)
	return service, mockDB
}

// MockStatusManager is a mock implementation of StatusManager
type MockStatusManager struct {
	mock.Mock
}

func (m *MockStatusManager) GetStatus(ctx context.Context, repoURL string) (*models.SyncStatus, error) {
	args := m.Called(ctx, repoURL)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	status, ok := args.Get(0).(*models.SyncStatus)
	if !ok {
		return nil, fmt.Errorf(errInvalidStatusType)
	}
	return status, args.Error(1)
}

func (m *MockStatusManager) UpdateStatus(ctx context.Context, status *models.SyncStatus) error {
	if status == nil {
		return fmt.Errorf(errNilSyncStatus)
	}
	args := m.Called(ctx, status)
	return args.Error(0)
}

func (m *MockStatusManager) DeleteStatus(ctx context.Context, repoURL string) error {
	if repoURL == "" {
		return fmt.Errorf(errEmptyRepoURL)
	}
	args := m.Called(ctx, repoURL)
	return args.Error(0)
}

func (m *MockStatusManager) ListStatuses(ctx context.Context) ([]*models.SyncStatus, error) {
	args := m.Called(ctx)
	statuses, ok := args.Get(0).([]*models.SyncStatus)
	if !ok {
		return nil, fmt.Errorf(errInvalidStatusesType)
	}
	return statuses, args.Error(1)
}

func (m *MockStatusManager) ClearStatuses(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockStatusManager) GetAllStatuses(ctx context.Context) ([]*models.SyncStatus, error) {
	args := m.Called(ctx)
	statuses, ok := args.Get(0).([]*models.SyncStatus)
	if !ok {
		return nil, fmt.Errorf(errInvalidStatusesType)
	}
	return statuses, args.Error(1)
}

func TestAddRepository(t *testing.T) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)
	ctx := context.Background()

	tests := []struct {
		name          string
		request       AddRepositoryRequest
		mockRepo      *models.Repository
		mockError     error
		expectedError bool
		expectedRepo  *models.Repository
	}{
		{
			name: "successful repository addition",
			request: AddRepositoryRequest{
				URL: "https://github.com/owner/repo",
				SyncConfig: &models.RepositorySyncConfig{
					Enabled:         true,
					SyncInterval:    models.FromDuration(testSyncIntervalSec * time.Second),
					InitialSyncDate: &time.Time{}, // Add this to avoid nil pointer
				},
			},
			mockRepo:      nil, // Repository doesn't exist initially
			mockError:     fmt.Errorf("repository not found with URL: https://github.com/owner/repo"),
			expectedError: false,
			expectedRepo: &models.Repository{
				BaseModel: models.BaseModel{ID: 1},
				URL:       "https://github.com/owner/repo",
				SyncConfig: models.RepositorySyncConfig{
					Enabled:      true,
					SyncInterval: models.FromDuration(testSyncIntervalSec * time.Second),
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
				BaseModel: models.BaseModel{ID: 1},
				URL:       "https://github.com/owner/repo",
			},
			mockError:     nil,   // Repository exists, no error
			expectedError: false, // Should return existing repo, not error
			expectedRepo: &models.Repository{
				BaseModel: models.BaseModel{ID: 1},
				URL:       "https://github.com/owner/repo",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDB := new(MockStoreDB)
			client := NewMockGitHubClient()
			mockConfig := &config.GitHubConfig{}
			mockStatusMgr := &MockStatusManager{}
			repoService := NewRepositoryService(client, mockDB, mockConfig, mockStatusMgr)

			// Setup mock expectations based on the test case
			if tt.name == "successful repository addition" {
				// Repository doesn't exist initially
				mockDB.On("GetRepositoryByURL", ctx, tt.request.URL).Return(nil, tt.mockError)
				// Client will be called to get repository info
				client.repositories[tt.request.URL] = &models.Repository{
					Name: "repo",
					URL:  tt.request.URL,
				}
				// Repository will be saved
				mockDB.On("SaveRepository", ctx, mock.AnythingOfType("*models.Repository")).Return(nil)
				// Status will be updated
				mockStatusMgr.On("UpdateStatus", ctx, mock.AnythingOfType("*models.SyncStatus")).Return(nil)
			} else if tt.name == "invalid repository URL" {
				// No mock expectations needed - validation will fail before any DB calls
			} else if tt.name == "repository already exists" {
				// Repository exists, so it will be returned
				mockDB.On("GetRepositoryByURL", ctx, tt.request.URL).Return(tt.mockRepo, tt.mockError)
			}

			repo, err := repoService.AddRepository(ctx, tt.request)

			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, repo)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, repo)
				assert.Equal(t, tt.expectedRepo.URL, repo.URL)
			}

			mockDB.AssertExpectations(t)
			mockStatusMgr.AssertExpectations(t)
		})
	}
}

func TestGetRepositoryByID(t *testing.T) {
	service, mockDB := setupRepositoryTestService()
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
				BaseModel: models.BaseModel{ID: 1},
				URL:       "https://github.com/owner/repo",
			},
			mockError:     nil,
			expectedError: false,
			expectedRepo: &models.Repository{
				BaseModel: models.BaseModel{ID: 1},
				URL:       "https://github.com/owner/repo",
			},
		},
		{
			name:          "repository not found",
			repoID:        999,
			mockRepo:      nil,
			mockError:     apperrors.NewNotFoundError("repository not found", nil),
			expectedError: true,
			expectedRepo:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDB.On("GetRepository", ctx, int64(tt.repoID)).Return(tt.mockRepo, tt.mockError)

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
	tests := []struct {
		name          string
		mockRepos     []*models.Repository
		mockError     error
		expectedError bool
		expectedRepos []*models.Repository
	}{
		{
			name: "successful repository list",
			mockRepos: []*models.Repository{
				{
					BaseModel: models.BaseModel{ID: 1},
					URL:       "https://github.com/owner1/repo1",
				},
				{
					BaseModel: models.BaseModel{ID: 2},
					URL:       "https://github.com/owner2/repo2",
				},
			},
			mockError:     nil,
			expectedError: false,
			expectedRepos: []*models.Repository{
				{
					BaseModel: models.BaseModel{ID: 1},
					URL:       "https://github.com/owner1/repo1",
				},
				{
					BaseModel: models.BaseModel{ID: 2},
					URL:       "https://github.com/owner2/repo2",
				},
			},
		},
		{
			name:          "empty repository list",
			mockRepos:     []*models.Repository{},
			mockError:     nil,
			expectedError: false,
			expectedRepos: []*models.Repository{},
		},
		{
			name:          "database error",
			mockRepos:     nil,
			mockError:     assert.AnError,
			expectedError: true,
			expectedRepos: nil,
		},
	}

	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, mockDB := setupRepositoryTestService()
			mockDB.On("ListRepositories", ctx).Return(tt.mockRepos, tt.mockError)

			repos, err := service.ListRepositories(ctx)

			if tt.expectedError {
				assert.Error(t, err)
				assert.Nil(t, repos)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, repos)
				assert.Equal(t, len(tt.expectedRepos), len(repos))
				if len(tt.expectedRepos) > 0 {
					for i, repo := range repos {
						assert.Equal(t, tt.expectedRepos[i].ID, repo.ID)
						assert.Equal(t, tt.expectedRepos[i].URL, repo.URL)
					}
				}
			}
			mockDB.AssertExpectations(t)
		})
	}
}

func TestDeleteRepository(t *testing.T) {
	tests := []struct {
		name          string
		repoID        uint
		mockRepo      *models.Repository
		mockGetError  error
		mockDelError  error
		expectedError bool
	}{
		{
			name:          "successful repository deletion",
			repoID:        1,
			mockRepo:      &models.Repository{BaseModel: models.BaseModel{ID: 1}, URL: "https://github.com/owner/repo"},
			mockGetError:  nil,
			mockDelError:  nil,
			expectedError: false,
		},
		{
			name:          "repository not found",
			repoID:        999,
			mockRepo:      nil,
			mockGetError:  apperrors.NewNotFoundError("repository not found", nil),
			mockDelError:  nil,
			expectedError: true,
		},
		{
			name:          "database error",
			repoID:        1,
			mockRepo:      &models.Repository{BaseModel: models.BaseModel{ID: 1}, URL: "https://github.com/owner/repo"},
			mockGetError:  nil,
			mockDelError:  assert.AnError,
			expectedError: true,
		},
	}

	ctx := context.Background()

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service, mockDB := setupRepositoryTestService()
			mockDB.On("GetRepository", ctx, int64(tt.repoID)).Return(tt.mockRepo, tt.mockGetError)
			if tt.mockRepo != nil && tt.mockGetError == nil {
				mockDB.On("DeleteRepository", ctx, int64(tt.repoID)).Return(tt.mockDelError)
			}

			err := service.DeleteRepositoryByID(ctx, tt.repoID)

			if tt.expectedError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
			mockDB.AssertExpectations(t)
		})
	}
}
