package db

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/Kamar-Folarin/github-monitor/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) (*PostgresStore, func()) {
	// Connect to test database
	db, err := sql.Open("postgres", "postgres://postgres:postgres@localhost:5432/github_monitor_test?sslmode=disable")
	require.NoError(t, err)

	// Create store
	store, err := NewPostgresStore(db)
	require.NoError(t, err)

	// Run migrations
	err = store.Migrate()
	require.NoError(t, err)

	// Return cleanup function
	cleanup := func() {
		// Drop all tables
		_, err := db.Exec(`
			DROP TABLE IF EXISTS commits;
			DROP TABLE IF EXISTS repositories;
			DROP TABLE IF EXISTS sync_status;
		`)
		require.NoError(t, err)
		db.Close()
	}

	return store, cleanup
}

func TestPostgresStore_RepositoryOperations(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("save and get repository", func(t *testing.T) {
		repo := &models.Repository{
			URL:             "https://github.com/test-owner/test-repo",
			Name:            "test-repo",
			Description:     "Test repository",
			Language:        "Go",
			ForksCount:      100,
			StarsCount:      200,
			OpenIssuesCount: 10,
			WatchersCount:   300,
			LastSyncedAt:    time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
			SyncConfig: models.RepositorySyncConfig{
				Enabled: true,
				Priority: 0,
			},
		}

		// Save repository
		err := store.SaveRepository(ctx, repo)
		require.NoError(t, err)

		// Get repository
		saved, err := store.GetRepositoryByURL(ctx, repo.URL)
		require.NoError(t, err)
		assert.Equal(t, repo.URL, saved.URL)
		assert.Equal(t, repo.Name, saved.Name)
		assert.Equal(t, repo.Description, saved.Description)
		assert.Equal(t, repo.Language, saved.Language)
		assert.Equal(t, repo.ForksCount, saved.ForksCount)
		assert.Equal(t, repo.StarsCount, saved.StarsCount)
		assert.Equal(t, repo.OpenIssuesCount, saved.OpenIssuesCount)
		assert.Equal(t, repo.WatchersCount, saved.WatchersCount)
		assert.Equal(t, repo.LastSyncedAt, saved.LastSyncedAt)
	})

	t.Run("list repositories", func(t *testing.T) {
		// Create test repositories
		repos := []*models.Repository{
			{
				URL:      "https://github.com/test-owner/repo1",
				Name:     "repo1",
				Language: "Go",
				SyncConfig: models.RepositorySyncConfig{
					Enabled: true,
					Priority: 0,
				},
			},
			{
				URL:      "https://github.com/test-owner/repo2",
				Name:     "repo2",
				Language: "Python",
				SyncConfig: models.RepositorySyncConfig{
					Enabled: true,
					Priority: 0,
				},
			},
		}

		for _, repo := range repos {
			err := store.SaveRepository(ctx, repo)
			require.NoError(t, err)
		}

		// List repositories
		saved, err := store.ListRepositories(ctx)
		require.NoError(t, err)
		assert.Len(t, saved, len(repos))

		// Verify repositories are returned in correct order
		for i, repo := range repos {
			assert.Equal(t, repo.URL, saved[i].URL)
			assert.Equal(t, repo.Name, saved[i].Name)
			assert.Equal(t, repo.Language, saved[i].Language)
		}
	})

	t.Run("update repository", func(t *testing.T) {
		repo := &models.Repository{
			URL:      "https://github.com/test-owner/update-test",
			Name:     "update-test",
			Language: "Go",
			SyncConfig: models.RepositorySyncConfig{
				Enabled: true,
				Priority: 0,
			},
		}

		// Save initial repository
		err := store.SaveRepository(ctx, repo)
		require.NoError(t, err)

		// Update repository
		repo.Language = "Python"
		repo.StarsCount = 1000
		err = store.UpdateRepository(ctx, repo)
		require.NoError(t, err)

		// Get updated repository
		saved, err := store.GetRepositoryByURL(ctx, repo.URL)
		require.NoError(t, err)
		assert.Equal(t, "Python", saved.Language)
		assert.Equal(t, 1000, saved.StarsCount)
	})

	t.Run("delete repository", func(t *testing.T) {
		repo := &models.Repository{
			URL:      "https://github.com/test-owner/delete-test",
			Name:     "delete-test",
			Language: "Go",
			SyncConfig: models.RepositorySyncConfig{
				Enabled: true,
				Priority: 0,
			},
		}

		// Save repository
		err := store.SaveRepository(ctx, repo)
		require.NoError(t, err)

		// Delete repository
		err = store.DeleteRepository(ctx, int64(repo.ID))
		require.NoError(t, err)

		// Verify repository is deleted
		_, err = store.GetRepositoryByURL(ctx, repo.URL)
		assert.Error(t, err)
		assert.Equal(t, sql.ErrNoRows, err)
	})
}

func TestPostgresStore_CommitOperations(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	// Create test repository
	repo := &models.Repository{
		URL:      "https://github.com/test-owner/commit-test",
		Name:     "commit-test",
		Language: "Go",
		SyncConfig: models.RepositorySyncConfig{
			Enabled: true,
			Priority: 0,
		},
	}
	err := store.SaveRepository(ctx, repo)
	require.NoError(t, err)

	t.Run("save and get commits", func(t *testing.T) {
		commits := []*models.Commit{
			{
				RepositoryID:  int64(repo.ID),
				SHA:          "abc123",
				Message:      "First commit",
				AuthorName:   "Test Author",
				AuthorEmail:  "test@example.com",
				AuthorDate:   time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
				CommitURL:    "https://github.com/test-owner/commit-test/commit/abc123",
			},
			{
				RepositoryID:  int64(repo.ID),
				SHA:          "def456",
				Message:      "Second commit",
				AuthorName:   "Test Author",
				AuthorEmail:  "test@example.com",
				AuthorDate:   time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC),
				CommitURL:    "https://github.com/test-owner/commit-test/commit/def456",
			},
		}

		// Save commits
		tx, err := store.BeginTx(ctx)
		require.NoError(t, err)
		err = store.SaveCommitsTx(ctx, tx, commits)
		require.NoError(t, err)
		err = tx.Commit()
		require.NoError(t, err)

		// Get commits
		saved, err := store.GetCommits(ctx, int64(repo.ID), 10, 0)
		require.NoError(t, err)
		assert.Len(t, saved, len(commits))

		// Verify commits are returned in correct order (newest first)
		for i, commit := range commits {
			idx := len(commits) - 1 - i
			assert.Equal(t, commit.SHA, saved[idx].SHA)
			assert.Equal(t, commit.Message, saved[idx].Message)
			assert.Equal(t, commit.AuthorName, saved[idx].AuthorName)
			assert.Equal(t, commit.AuthorEmail, saved[idx].AuthorEmail)
			assert.Equal(t, commit.AuthorDate, saved[idx].AuthorDate)
			assert.Equal(t, commit.CommitURL, saved[idx].CommitURL)
		}
	})

	t.Run("get commits with pagination", func(t *testing.T) {
		// Create more commits
		for i := 0; i < 15; i++ {
			commit := &models.Commit{
				RepositoryID:  int64(repo.ID),
				SHA:          fmt.Sprintf("sha%d", i),
				Message:      fmt.Sprintf("Commit %d", i),
				AuthorName:   "Test Author",
				AuthorEmail:  "test@example.com",
				AuthorDate:   time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).Add(time.Duration(i) * time.Hour),
				CommitURL:    fmt.Sprintf("https://github.com/test-owner/commit-test/commit/sha%d", i),
			}
			err := store.SaveCommit(ctx, commit)
			require.NoError(t, err)
		}

		// Test pagination
		commits, err := store.GetCommits(ctx, int64(repo.ID), 17, 0)
		require.NoError(t, err)
		assert.Len(t, commits, 17) // 2 from previous test + 15 new ones

		// Verify commits are ordered by committed_at DESC
		for i := 0; i < len(commits)-1; i++ {
			assert.True(t, commits[i].AuthorDate.After(commits[i+1].AuthorDate))
		}
	})

	t.Run("delete commits", func(t *testing.T) {
		// Delete all commits for repository
		err := store.DeleteCommits(ctx, int64(repo.ID))
		require.NoError(t, err)

		// Verify commits are deleted
		commits, err := store.GetCommits(ctx, int64(repo.ID), 10, 0)
		require.NoError(t, err)
		assert.Empty(t, commits)
	})
}

func TestPostgresStore_SyncStatusOperations(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("save and get sync status", func(t *testing.T) {
		status := &models.SyncStatus{
			RepositoryURL: "https://github.com/test-owner/repo1",
			IsSyncing:     true,
			Status:        "in_progress",
			StartTime:     time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
			LastSyncAt:    time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
		}

		err := store.UpdateSyncStatus(ctx, status)
		require.NoError(t, err)

		saved, err := store.GetSyncStatus(ctx, status.RepositoryURL)
		require.NoError(t, err)
		assert.NotNil(t, saved)
		assert.Equal(t, status.RepositoryURL, saved.RepositoryURL)
		assert.Equal(t, status.IsSyncing, saved.IsSyncing)
		assert.Equal(t, status.Status, saved.Status)
		assert.Equal(t, status.LastSyncAt, saved.LastSyncAt)
	})

	t.Run("update sync status", func(t *testing.T) {
		status := &models.SyncStatus{
			RepositoryURL: "https://github.com/test-owner/repo1",
			IsSyncing:     false,
			Status:        "completed",
			StartTime:     time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC),
			LastSyncAt:    time.Date(2020, 1, 2, 0, 0, 0, 0, time.UTC),
		}

		err := store.UpdateSyncStatus(ctx, status)
		require.NoError(t, err)

		saved, err := store.GetSyncStatus(ctx, status.RepositoryURL)
		require.NoError(t, err)
		assert.NotNil(t, saved)
		assert.Equal(t, status.IsSyncing, saved.IsSyncing)
		assert.Equal(t, status.Status, saved.Status)
		assert.Equal(t, status.LastSyncAt, saved.LastSyncAt)
	})

	t.Run("delete sync status", func(t *testing.T) {
		status := &models.SyncStatus{
			RepositoryURL: "https://github.com/test-owner/repo2",
			IsSyncing:     false,
			Status:        "completed",
			StartTime:     time.Date(2020, 1, 3, 0, 0, 0, 0, time.UTC),
			LastSyncAt:    time.Date(2020, 1, 3, 0, 0, 0, 0, time.UTC),
		}
		err := store.UpdateSyncStatus(ctx, status)
		require.NoError(t, err)

		err = store.DeleteSyncStatus(ctx, status.RepositoryURL)
		require.NoError(t, err)

		saved, err := store.GetSyncStatus(ctx, status.RepositoryURL)
		require.NoError(t, err)
		assert.Nil(t, saved)
	})
}

func TestPostgresStore_SyncProgressOperations(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("save and get sync progress", func(t *testing.T) {
		progress := &models.SyncProgress{
			RepoURL:          "https://github.com/test-owner/repo1",
			CurrentPage:      2,
			TotalPages:       10,
			ProcessedCommits: 100,
			TotalCommits:     1000,
			LastProcessedSHA: "abc123",
			StartDate:        time.Now(),
			LastUpdated:      time.Now(),
			Status:          "in_progress",
			BatchSize:       100,
			CurrentBatch:    1,
			TotalBatches:    10,
		}

		// Save progress
		err := store.SaveSyncProgress(ctx, progress.RepoURL, progress)
		require.NoError(t, err)

		// Get progress
		saved, err := store.GetSyncProgress(ctx, progress.RepoURL)
		require.NoError(t, err)
		assert.Equal(t, progress.RepoURL, saved.RepoURL)
		assert.Equal(t, progress.CurrentPage, saved.CurrentPage)
		assert.Equal(t, progress.TotalPages, saved.TotalPages)
		assert.Equal(t, progress.ProcessedCommits, saved.ProcessedCommits)
		assert.Equal(t, progress.TotalCommits, saved.TotalCommits)
		assert.Equal(t, progress.LastProcessedSHA, saved.LastProcessedSHA)
		assert.Equal(t, progress.Status, saved.Status)
		assert.Equal(t, progress.BatchSize, saved.BatchSize)
		assert.Equal(t, progress.CurrentBatch, saved.CurrentBatch)
		assert.Equal(t, progress.TotalBatches, saved.TotalBatches)
	})

	t.Run("update sync progress", func(t *testing.T) {
		// Initial progress
		progress := &models.SyncProgress{
			RepoURL:          "https://github.com/test-owner/repo2",
			CurrentPage:      1,
			TotalPages:       5,
			ProcessedCommits: 50,
			TotalCommits:     500,
			Status:          "in_progress",
		}

		// Save initial progress
		err := store.SaveSyncProgress(ctx, progress.RepoURL, progress)
		require.NoError(t, err)

		// Update progress
		progress.CurrentPage = 2
		progress.ProcessedCommits = 100
		progress.Status = "completed"

		err = store.SaveSyncProgress(ctx, progress.RepoURL, progress)
		require.NoError(t, err)

		// Get updated progress
		saved, err := store.GetSyncProgress(ctx, progress.RepoURL)
		require.NoError(t, err)
		assert.Equal(t, 2, saved.CurrentPage)
		assert.Equal(t, 100, saved.ProcessedCommits)
		assert.Equal(t, "completed", saved.Status)
	})

	t.Run("get non-existent progress", func(t *testing.T) {
		progress, err := store.GetSyncProgress(ctx, "https://github.com/test-owner/non-existent")
		require.NoError(t, err)
		assert.Nil(t, progress)
	})

	t.Run("concurrent sync progress operations", func(t *testing.T) {
		const numGoroutines = 10
		done := make(chan bool)

		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				progress := &models.SyncProgress{
					RepoURL:          fmt.Sprintf("https://github.com/test-owner/repo-%d", id),
					CurrentPage:      id,
					TotalPages:       10,
					ProcessedCommits: id * 100,
					TotalCommits:     1000,
					Status:          "in_progress",
					LastUpdated:      time.Now(),
				}

				// Save progress
				err := store.SaveSyncProgress(ctx, progress.RepoURL, progress)
				require.NoError(t, err)

				// Get progress
				saved, err := store.GetSyncProgress(ctx, progress.RepoURL)
				require.NoError(t, err)
				assert.Equal(t, progress.RepoURL, saved.RepoURL)
				assert.Equal(t, progress.CurrentPage, saved.CurrentPage)

				done <- true
			}(i)
		}

		// Wait for all goroutines to complete
		for i := 0; i < numGoroutines; i++ {
			<-done
		}
	})
}

func TestPostgresStore_ConcurrentOperations(t *testing.T) {
	store, cleanup := setupTestDB(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("concurrent repository operations", func(t *testing.T) {
		const numGoroutines = 10
		done := make(chan bool)

		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				repo := &models.Repository{
					URL:      fmt.Sprintf("https://github.com/test-owner/concurrent-repo-%d", id),
					Name:     fmt.Sprintf("concurrent-repo-%d", id),
					Language: "Go",
					SyncConfig: models.RepositorySyncConfig{
						Enabled: true,
						Priority: 0,
					},
				}

				// Save repository
				err := store.SaveRepository(ctx, repo)
				require.NoError(t, err)

				// Get repository
				saved, err := store.GetRepositoryByURL(ctx, repo.URL)
				require.NoError(t, err)
				assert.Equal(t, repo.URL, saved.URL)

				done <- true
			}(i)
		}

		// Wait for all goroutines to complete
		for i := 0; i < numGoroutines; i++ {
			<-done
		}

		// Verify all repositories were saved
		repos, err := store.ListRepositories(ctx)
		require.NoError(t, err)
		assert.Len(t, repos, numGoroutines)
	})

	t.Run("concurrent commit operations", func(t *testing.T) {
		// Create test repository
		repo := &models.Repository{
			URL:      "https://github.com/test-owner/concurrent-commits",
			Name:     "concurrent-commits",
			Language: "Go",
			SyncConfig: models.RepositorySyncConfig{
				Enabled: true,
				Priority: 0,
			},
		}
		err := store.SaveRepository(ctx, repo)
		require.NoError(t, err)

		const numGoroutines = 10
		done := make(chan bool)

		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				commit := &models.Commit{
					RepositoryID:  int64(repo.ID),
					SHA:          fmt.Sprintf("sha%d", id),
					Message:      fmt.Sprintf("Concurrent commit %d", id),
					AuthorName:   "Test Author",
					AuthorEmail:  "test@example.com",
					AuthorDate:   time.Now(),
					CommitURL:    fmt.Sprintf("https://github.com/test-owner/concurrent-commits/commit/sha%d", id),
				}

				// Save commit
				err := store.SaveCommit(ctx, commit)
				require.NoError(t, err)

				done <- true
			}(i)
		}

		// Wait for all goroutines to complete
		for i := 0; i < numGoroutines; i++ {
			<-done
		}

		// Verify all commits were saved
		commits, err := store.GetCommits(ctx, int64(repo.ID), 10, 0)
		require.NoError(t, err)
		assert.Len(t, commits, numGoroutines)
	})

	t.Run("concurrent sync status operations", func(t *testing.T) {
		const numGoroutines = 10
		done := make(chan bool)

		for i := 0; i < numGoroutines; i++ {
			go func(id int) {
				status := &models.SyncStatus{
					RepositoryURL: fmt.Sprintf("https://github.com/test-owner/repo-%d", id),
					IsSyncing:     true,
					Status:        "in_progress",
					StartTime:     time.Now(),
					LastSyncAt:    time.Now(),
				}

				// Save sync status
				err := store.UpdateSyncStatus(ctx, status)
				require.NoError(t, err)

				// Get sync status
				saved, err := store.GetSyncStatus(ctx, status.RepositoryURL)
				require.NoError(t, err)
				assert.NotNil(t, saved)
				assert.Equal(t, status.RepositoryURL, saved.RepositoryURL)
				assert.Equal(t, status.IsSyncing, saved.IsSyncing)
				assert.Equal(t, status.Status, saved.Status)
				assert.Equal(t, status.LastSyncAt, saved.LastSyncAt)

				done <- true
			}(i)
		}

		// Wait for all goroutines to complete
		for i := 0; i < numGoroutines; i++ {
			<-done
		}

		// Verify all statuses were saved
		status, err := store.GetSyncStatus(ctx)
		require.NoError(t, err)
		assert.Len(t, status, numGoroutines)
	})
} 