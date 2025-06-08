package db

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Kamar-Folarin/github-monitor/internal/models"
)

func setupTestRepository(t *testing.T) *PostgresStore {
	// Use an in-memory SQLite database for testing
	db, err := NewPostgresStore(":memory:")
	require.NoError(t, err)
	require.NoError(t, db.Migrate())
	return db
}

func TestCreateAndGetRepository(t *testing.T) {
	db := setupTestRepository(t)
	ctx := context.Background()

	// Test creating a repository
	repo := &models.Repository{
		URL: "https://github.com/owner/repo",
		SyncConfig: models.RepositorySyncConfig{
			Enabled: true,
			SyncInterval: models.FromDuration(3600 * time.Second),
		},
	}

	err := db.CreateRepository(ctx, repo)
	require.NoError(t, err)
	assert.NotZero(t, repo.ID)

	// Test getting the repository by ID
	retrievedRepo, err := db.GetRepositoryByID(ctx, uint(repo.ID))
	require.NoError(t, err)
	assert.Equal(t, repo.URL, retrievedRepo.URL)
	assert.Equal(t, repo.SyncConfig.Enabled, retrievedRepo.SyncConfig.Enabled)
	assert.Equal(t, repo.SyncConfig.SyncInterval, retrievedRepo.SyncConfig.SyncInterval)

	// Test getting the repository by URL
	retrievedRepo, err = db.GetRepositoryByURL(ctx, repo.URL)
	require.NoError(t, err)
	assert.Equal(t, repo.ID, retrievedRepo.ID)
	assert.Equal(t, repo.URL, retrievedRepo.URL)
}

func TestListRepositories(t *testing.T) {
	db := setupTestRepository(t)
	ctx := context.Background()

	// Create test repositories
	repos := []*models.Repository{
		{
			URL: "https://github.com/owner1/repo1",
			SyncConfig: models.RepositorySyncConfig{
				Enabled: true,
				SyncInterval: models.FromDuration(3600 * time.Second),
			},
		},
		{
			URL: "https://github.com/owner2/repo2",
			SyncConfig: models.RepositorySyncConfig{
				Enabled: false,
				SyncInterval: models.FromDuration(7200 * time.Second),
			},
		},
	}

	for _, repo := range repos {
		err := db.CreateRepository(ctx, repo)
		require.NoError(t, err)
	}

	// Test listing repositories
	retrievedRepos, err := db.ListRepositories(ctx)
	require.NoError(t, err)
	assert.Len(t, retrievedRepos, 2)

	// Verify repository details
	for i, repo := range retrievedRepos {
		assert.Equal(t, repos[i].URL, repo.URL)
		assert.Equal(t, repos[i].SyncConfig.Enabled, repo.SyncConfig.Enabled)
		assert.Equal(t, repos[i].SyncConfig.SyncInterval, repo.SyncConfig.SyncInterval)
	}
}

func TestDeleteRepository(t *testing.T) {
	db := setupTestRepository(t)
	ctx := context.Background()

	// Create a test repository
	repo := &models.Repository{
		URL: "https://github.com/owner/repo",
		SyncConfig: models.RepositorySyncConfig{
			Enabled: true,
			SyncInterval: models.FromDuration(3600 * time.Second),
		},
	}

	err := db.CreateRepository(ctx, repo)
	require.NoError(t, err)

	// Test deleting the repository
	err = db.DeleteRepository(ctx, int64(repo.ID))
	require.NoError(t, err)

	// Verify repository is deleted
	_, err = db.GetRepositoryByID(ctx, uint(repo.ID))
	assert.Error(t, err)
	assert.Equal(t, ErrNotFound, err)
}

func TestUpdateRepositorySyncConfig(t *testing.T) {
	db := setupTestRepository(t)
	ctx := context.Background()

	// Create a test repository
	repo := &models.Repository{
		URL: "https://github.com/owner/repo",
		SyncConfig: models.RepositorySyncConfig{
			Enabled: true,
			SyncInterval: models.FromDuration(3600 * time.Second),
		},
	}

	err := db.CreateRepository(ctx, repo)
	require.NoError(t, err)

	// Update sync config
	newConfig := &models.RepositorySyncConfig{
		Enabled: false,
		SyncInterval: models.FromDuration(7200 * time.Second),
	}

	err = db.UpdateRepositorySyncConfig(ctx, uint(repo.ID), newConfig)
	require.NoError(t, err)

	// Verify config is updated
	retrievedRepo, err := db.GetRepositoryByID(ctx, uint(repo.ID))
	require.NoError(t, err)
	assert.Equal(t, newConfig.Enabled, retrievedRepo.SyncConfig.Enabled)
	assert.Equal(t, newConfig.SyncInterval, retrievedRepo.SyncConfig.SyncInterval)
}

func TestCreateAndGetCommits(t *testing.T) {
	db := setupTestRepository(t)
	ctx := context.Background()

	// Create a test repository
	repo := &models.Repository{
		URL: "https://github.com/owner/repo",
		SyncConfig: models.RepositorySyncConfig{
			Enabled: true,
			SyncInterval: models.FromDuration(3600 * time.Second),
		},
	}

	err := db.CreateRepository(ctx, repo)
	require.NoError(t, err)

	// Create test commits
	commits := []*models.Commit{
		{
			RepositoryID: repo.ID,
			SHA:         "abc123",
			AuthorName:  "John Doe",
			AuthorEmail: "john@example.com",
			Message:     "Initial commit",
			AuthorDate:  time.Now(),
		},
		{
			RepositoryID: repo.ID,
			SHA:         "def456",
			AuthorName:  "Jane Smith",
			AuthorEmail: "jane@example.com",
			Message:     "Update README",
			AuthorDate:  time.Now(),
		},
	}

	for _, commit := range commits {
		err := db.CreateCommit(ctx, commit)
		require.NoError(t, err)
	}

	// Test getting commits by repository ID
	retrievedCommits, err := db.GetCommitsByRepositoryID(ctx, repo.ID)
	require.NoError(t, err)
	assert.Len(t, retrievedCommits, 2)

	// Verify commit details
	for i, commit := range retrievedCommits {
		assert.Equal(t, commits[i].SHA, commit.SHA)
		assert.Equal(t, commits[i].Message, commit.Message)
		assert.Equal(t, commits[i].AuthorName, commit.AuthorName)
		assert.Equal(t, commits[i].AuthorEmail, commit.AuthorEmail)
	}

	// Test getting commits by repository URL
	retrievedCommits, err = db.GetCommitsByRepositoryURL(ctx, repo.URL)
	require.NoError(t, err)
	assert.Len(t, retrievedCommits, 2)
}

func TestGetAuthors(t *testing.T) {
	db := setupTestRepository(t)
	ctx := context.Background()

	// Create a test repository
	repo := &models.Repository{
		URL: "https://github.com/owner/repo",
		SyncConfig: models.RepositorySyncConfig{
			Enabled: true,
			SyncInterval: models.FromDuration(3600 * time.Second),
		},
	}

	err := db.CreateRepository(ctx, repo)
	require.NoError(t, err)

	// Create test commits with different authors
	commits := []*models.Commit{
		{
			RepositoryID: repo.ID,
			SHA:         "abc123",
			AuthorName:  "John Doe",
			AuthorEmail: "john@example.com",
			Message:     "Initial commit",
			AuthorDate:  time.Now(),
		},
		{
			RepositoryID: repo.ID,
			SHA:         "def456",
			AuthorName:  "John Doe",
			AuthorEmail: "john@example.com",
			Message:     "Update README",
			AuthorDate:  time.Now(),
		},
		{
			RepositoryID: repo.ID,
			SHA:         "ghi789",
			AuthorName:  "Jane Smith",
			AuthorEmail: "jane@example.com",
			Message:     "Add tests",
			AuthorDate:  time.Now(),
		},
	}

	for _, commit := range commits {
		err := db.CreateCommit(ctx, commit)
		require.NoError(t, err)
	}

	// Test getting authors by repository ID
	authors, err := db.GetAuthorsByRepositoryID(ctx, repo.ID)
	require.NoError(t, err)
	assert.Len(t, authors, 2) // Should have 2 unique authors

	// Verify author details and commit counts
	authorMap := make(map[string]int)
	for _, author := range authors {
		authorMap[author.Email] = author.CommitCount
	}
	assert.Equal(t, 2, authorMap["john@example.com"]) // John has 2 commits
	assert.Equal(t, 1, authorMap["jane@example.com"]) // Jane has 1 commit

	// Test getting authors by repository URL
	authors, err = db.GetAuthorsByRepositoryURL(ctx, repo.URL)
	require.NoError(t, err)
	assert.Len(t, authors, 2)
}

func TestSyncStatus(t *testing.T) {
	db := setupTestRepository(t)
	ctx := context.Background()

	// Test updating and getting sync status
	repoURL := "https://github.com/owner/repo"
	status := "syncing"

	err := db.UpdateSyncStatus(ctx, repoURL, status)
	require.NoError(t, err)

	retrievedStatus, err := db.GetSyncStatus(ctx, repoURL)
	require.NoError(t, err)
	assert.Equal(t, status, retrievedStatus)

	// Test getting all sync statuses
	repoURL2 := "https://github.com/owner/repo2"
	status2 := "completed"

	err = db.UpdateSyncStatus(ctx, repoURL2, status2)
	require.NoError(t, err)

	allStatuses, err := db.GetAllSyncStatuses(ctx)
	require.NoError(t, err)
	assert.Len(t, allStatuses, 2)
	assert.Equal(t, status, allStatuses[repoURL])
	assert.Equal(t, status2, allStatuses[repoURL2])

	// Test clearing sync status
	err = db.ClearSyncStatus(ctx)
	require.NoError(t, err)

	allStatuses, err = db.GetAllSyncStatuses(ctx)
	require.NoError(t, err)
	assert.Empty(t, allStatuses)
} 