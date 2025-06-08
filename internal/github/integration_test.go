package github

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	"github.com/Kamar-Folarin/github-monitor/internal/db"
	"github.com/Kamar-Folarin/github-monitor/internal/models"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	testDB     *sql.DB
	testStore  db.Store
	testLogger *logrus.Logger
)

func TestMain(m *testing.M) {
	// Setup test database
	connStr := os.Getenv("TEST_DATABASE_URL")
	if connStr == "" {
		connStr = "postgres://postgres:postgres@localhost:5432/github_monitor_test?sslmode=disable"
	}

	var err error
	testDB, err = sql.Open("postgres", connStr)
	if err != nil {
		panic(err)
	}
	defer testDB.Close()

	// Create test store
	testStore, err = db.NewPostgresStore(connStr)
	if err != nil {
		panic(err)
	}

	// Setup test logger
	testLogger = logrus.New()
	testLogger.SetOutput(os.Stdout)
	testLogger.SetLevel(logrus.DebugLevel)

	// Run migrations
	if err := testStore.(*db.PostgresStore).Migrate(); err != nil {
		panic(err)
	}

	// Run tests
	code := m.Run()

	// Cleanup
	testDB.Exec("DROP SCHEMA public CASCADE")
	testDB.Exec("CREATE SCHEMA public")

	os.Exit(code)
}

func setupTestService(t *testing.T) (*Service, func()) {
	// Create test service
	service := NewService(
		os.Getenv("GITHUB_TOKEN"),
		testStore,
		testLogger,
		DefaultSyncConfig(),
	)

	// Return cleanup function
	cleanup := func() {
		// Clean test data
		testDB.Exec("DELETE FROM commits")
		testDB.Exec("DELETE FROM repositories")
		testDB.Exec("DELETE FROM monitored_repositories")
		testDB.Exec("DELETE FROM sync_status")
	}

	return service, cleanup
}

func TestService_Integration_SyncRepository(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()
	repoURL := "https://github.com/golang/go" // Using a well-known repository

	t.Run("successful sync", func(t *testing.T) {
		err := service.SyncRepository(ctx, repoURL, "")
		require.NoError(t, err)

		// Verify repository was created
		repo, err := testStore.GetRepositoryByURL(ctx, repoURL)
		require.NoError(t, err)
		assert.NotEmpty(t, repo.ID)
		assert.Equal(t, "go", repo.Name)

		// Verify commits were saved
		commits, err := testStore.GetCommits(ctx, int64(repo.ID), 10, 0)
		require.NoError(t, err)
		assert.NotEmpty(t, commits)

		// Verify sync status
		status, err := service.GetSyncStatus(ctx, repoURL)
		require.NoError(t, err)
		assert.False(t, status.IsSyncing)
		assert.Empty(t, status.LastError)
		assert.NotZero(t, status.TotalCommits)
	})

	t.Run("sync with start date", func(t *testing.T) {
		startDate := time.Now().AddDate(0, -1, 0) // 1 month ago
		err := service.SyncRepository(ctx, repoURL, &startDate)
		require.NoError(t, err)

		// Verify only recent commits were fetched
		repo, err := testStore.GetRepositoryByURL(ctx, repoURL)
		require.NoError(t, err)
		commits, err := testStore.GetCommits(ctx, int64(repo.ID), 100, 0)
		require.NoError(t, err)

		for _, commit := range commits {
			assert.True(t, commit.AuthorDate.After(startDate))
		}
	})

	t.Run("invalid repository", func(t *testing.T) {
		err := service.SyncRepository(ctx, "https://github.com/invalid/repo", "")
		assert.Error(t, err)
		assert.IsType(t, &RepositoryNotFoundError{}, err)
	})
}

func TestService_Integration_ConcurrentSync(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()
	repos := []string{
		"https://github.com/golang/go",
		"https://github.com/kubernetes/kubernetes",
		"https://github.com/docker/docker",
	}

	// Add repositories to monitoring
	for _, repoURL := range repos {
		err := testStore.(*db.PostgresStore).AddMonitoredRepository(ctx, repoURL, nil)
		require.NoError(t, err)
	}

	t.Run("concurrent sync", func(t *testing.T) {
		err := service.syncAllMonitoredRepositories(ctx)
		require.NoError(t, err)

		// Verify all repositories were synced
		for _, repoURL := range repos {
			status, err := service.GetSyncStatus(ctx, repoURL)
			require.NoError(t, err)
			assert.False(t, status.IsSyncing)
			assert.Empty(t, status.LastError)
			assert.NotZero(t, status.TotalCommits)
		}
	})
}

func TestService_Integration_RateLimitHandling(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()
	repoURL := "https://github.com/golang/go"

	t.Run("rate limit recovery", func(t *testing.T) {
		// Force rate limit by making many requests
		for i := 0; i < 10; i++ {
			err := service.SyncRepository(ctx, repoURL, "")
			if err != nil {
				// Should handle rate limit gracefully
				assert.True(t, IsRateLimitError(err))
				break
			}
		}

		// Verify service recovered
		status, err := service.GetSyncStatus(ctx, repoURL)
		require.NoError(t, err)
		assert.False(t, status.IsSyncing)
	})
}

func TestService_Integration_ErrorHandling(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("invalid URL", func(t *testing.T) {
		err := service.SyncRepository(ctx, "invalid-url", "")
		assert.Error(t, err)
		assert.IsType(t, &ValidationError{}, err)
	})

	t.Run("non-existent repository", func(t *testing.T) {
		err := service.SyncRepository(ctx, "https://github.com/nonexistent/repo", "")
		assert.Error(t, err)
		assert.IsType(t, &RepositoryNotFoundError{}, err)
	})

	t.Run("database error", func(t *testing.T) {
		// Force database error by closing connection
		testDB.Close()
		defer func() {
			var err error
			testDB, err = sql.Open("postgres", os.Getenv("TEST_DATABASE_URL"))
			require.NoError(t, err)
		}()

		err := service.SyncRepository(ctx, "https://github.com/golang/go", "")
		assert.Error(t, err)
	})
}

func TestService_Integration_SyncStatusPersistence(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()
	repoURL := "https://github.com/golang/go"

	t.Run("status persistence", func(t *testing.T) {
		// Initial sync
		err := service.SyncRepository(ctx, repoURL, "")
		require.NoError(t, err)

		// Get initial status
		initialStatus, err := service.GetSyncStatus(ctx, repoURL)
		require.NoError(t, err)

		// Create new service instance to test persistence
		newService := NewService(
			os.Getenv("GITHUB_TOKEN"),
			testStore,
			testLogger,
			DefaultSyncConfig(),
		)

		// Verify status was persisted
		persistedStatus, err := newService.GetSyncStatus(ctx, repoURL)
		require.NoError(t, err)
		assert.Equal(t, initialStatus.TotalCommits, persistedStatus.TotalCommits)
		assert.Equal(t, initialStatus.LastCommitSHA, persistedStatus.LastCommitSHA)
	})

	t.Run("status cleanup", func(t *testing.T) {
		// Delete repository
		repo, err := testStore.GetRepositoryByURL(ctx, repoURL)
		require.NoError(t, err)
		err = testStore.DeleteRepository(ctx, repo.ID)
		require.NoError(t, err)

		// Verify sync status is cleaned up
		err = testStore.DeleteSyncStatus(ctx, repoURL)
		require.NoError(t, err)

		_, err = service.GetSyncStatus(ctx, repoURL)
		assert.Error(t, err)
	})
}

func TestService_Integration_EdgeCases(t *testing.T) {
	service, cleanup := setupTestService(t)
	defer cleanup()

	ctx := context.Background()
	repoURL := "https://github.com/golang/go"

	t.Run("sync interruption", func(t *testing.T) {
		// Start sync in background
		errChan := make(chan error, 1)
		go func() {
			errChan <- service.SyncRepository(ctx, repoURL, "")
		}()

		// Interrupt sync
		time.Sleep(100 * time.Millisecond)
		cancelCtx, cancel := context.WithCancel(ctx)
		cancel()

		// Start new sync with cancelled context
		err := service.SyncRepository(cancelCtx, repoURL, "")
		assert.Error(t, err)
		assert.Equal(t, context.Canceled, err)

		// Wait for original sync to complete
		err = <-errChan
		assert.NoError(t, err)
	})

	t.Run("duplicate sync", func(t *testing.T) {
		// Start two syncs for the same repository
		errChan1 := make(chan error, 1)
		errChan2 := make(chan error, 1)

		go func() {
			errChan1 <- service.SyncRepository(ctx, repoURL, "")
		}()

		// Wait a bit to ensure first sync has started
		time.Sleep(100 * time.Millisecond)

		go func() {
			errChan2 <- service.SyncRepository(ctx, repoURL, "")
		}()

		// First sync should succeed
		err1 := <-errChan1
		assert.NoError(t, err1)

		// Second sync should fail with "already syncing" error
		err2 := <-errChan2
		assert.Error(t, err2)
		assert.Contains(t, err2.Error(), "already syncing")
	})

	t.Run("malformed response", func(t *testing.T) {
		// Create service with invalid token to force API errors
		invalidService := NewService(
			"invalid-token",
			testStore,
			testLogger,
			DefaultSyncConfig(),
		)

		err := invalidService.SyncRepository(ctx, repoURL, "")
		assert.Error(t, err)
		assert.IsType(t, &GitHubError{}, err)
	})
} 