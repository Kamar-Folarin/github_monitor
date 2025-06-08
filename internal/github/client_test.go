package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestClient(t *testing.T) (*GitHubClient, *httptest.Server, func()) {
	logger := logrus.New()
	logger.SetLevel(logrus.DebugLevel)

	// Create test server
	server := httptest.NewServer(nil)
	client := NewGitHubClient(
		"test-token",
		logger,
		WithRetryConfig(3, time.Millisecond*100, time.Second),
	)

	// Override client's HTTP client to use test server
	client.client = server.Client()

	cleanup := func() {
		server.Close()
	}

	return client, server, cleanup
}

func TestGitHubClient_GetRepository(t *testing.T) {
	client, server, cleanup := setupTestClient(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("successful request", func(t *testing.T) {
		server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.Equal(t, "/repos/test-owner/test-repo", r.URL.Path)
			assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))

			w.Header().Set("X-RateLimit-Limit", "5000")
			w.Header().Set("X-RateLimit-Remaining", "4999")
			w.Header().Set("X-RateLimit-Reset", "1234567890")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{
				"name": "test-repo",
				"description": "Test repository",
				"url": "https://github.com/test-owner/test-repo",
				"language": "Go",
				"forks_count": 100,
				"stars_count": 200,
				"open_issues_count": 10,
				"watchers_count": 300,
				"created_at": "2020-01-01T00:00:00Z",
				"updated_at": "2020-01-02T00:00:00Z"
			}`))
		})

		repo, err := client.GetRepository(ctx, "test-owner", "test-repo")
		require.NoError(t, err)
		assert.Equal(t, "test-repo", repo.Name)
		assert.Equal(t, "Test repository", repo.Description)
		assert.Equal(t, "https://github.com/test-owner/test-repo", repo.URL)
		assert.Equal(t, "Go", repo.Language)
		assert.Equal(t, 100, repo.ForksCount)
		assert.Equal(t, 200, repo.StarsCount)
		assert.Equal(t, 10, repo.OpenIssuesCount)
		assert.Equal(t, 300, repo.WatchersCount)
	})

	t.Run("rate limit handling", func(t *testing.T) {
		server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-RateLimit-Limit", "5000")
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.Header().Set("X-RateLimit-Reset", "1234567890")
			w.Header().Set("Retry-After", "60")
			w.WriteHeader(http.StatusTooManyRequests)
		})

		_, err := client.GetRepository(ctx, "test-owner", "test-repo")
		assert.Error(t, err)
		assert.True(t, IsRateLimitError(err))
	})

	t.Run("secondary rate limit", func(t *testing.T) {
		server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Retry-After", "30")
			w.WriteHeader(http.StatusTooManyRequests)
		})

		_, err := client.GetRepository(ctx, "test-owner", "test-repo")
		assert.Error(t, err)
		assert.True(t, IsRateLimitError(err))
	})

	t.Run("validation error", func(t *testing.T) {
		_, err := client.GetRepository(ctx, "", "test-repo")
		assert.Error(t, err)
		assert.IsType(t, &ValidationError{}, err)

		_, err = client.GetRepository(ctx, "test-owner", "")
		assert.Error(t, err)
		assert.IsType(t, &ValidationError{}, err)
	})

	t.Run("repository not found", func(t *testing.T) {
		server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		})

		_, err := client.GetRepository(ctx, "test-owner", "test-repo")
		assert.Error(t, err)
		assert.IsType(t, &RepositoryNotFoundError{}, err)
	})
}

func TestGitHubClient_GetCommitsPage(t *testing.T) {
	client, server, cleanup := setupTestClient(t)
	defer cleanup()

	ctx := context.Background()
	since := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)

	t.Run("successful request", func(t *testing.T) {
		server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, "GET", r.Method)
			assert.Equal(t, "/repos/test-owner/test-repo/commits", r.URL.Path)
			assert.Equal(t, "Bearer test-token", r.Header.Get("Authorization"))
			assert.Equal(t, "1", r.URL.Query().Get("page"))
			assert.Equal(t, "100", r.URL.Query().Get("per_page"))
			assert.Equal(t, since.Format(time.RFC3339), r.URL.Query().Get("since"))

			w.Header().Set("X-RateLimit-Limit", "5000")
			w.Header().Set("X-RateLimit-Remaining", "4999")
			w.Header().Set("X-RateLimit-Reset", "1234567890")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`[
				{
					"sha": "abc123",
					"commit": {
						"message": "Test commit",
						"author": {
							"name": "Test Author",
							"email": "test@example.com",
							"date": "2020-01-01T00:00:00Z"
						}
					},
					"html_url": "https://github.com/test-owner/test-repo/commit/abc123"
				}
			]`))
		})

		commits, err := client.GetCommitsPage(ctx, "test-owner", "test-repo", &since, 1)
		require.NoError(t, err)
		require.Len(t, commits, 1)
		assert.Equal(t, "abc123", commits[0].SHA)
		assert.Equal(t, "Test commit", commits[0].Message)
		assert.Equal(t, "Test Author", commits[0].AuthorName)
		assert.Equal(t, "test@example.com", commits[0].AuthorEmail)
		assert.Equal(t, "https://github.com/test-owner/test-repo/commit/abc123", commits[0].CommitURL)
	})

	t.Run("empty page", func(t *testing.T) {
		server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-RateLimit-Limit", "5000")
			w.Header().Set("X-RateLimit-Remaining", "4999")
			w.Header().Set("X-RateLimit-Reset", "1234567890")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`[]`))
		})

		commits, err := client.GetCommitsPage(ctx, "test-owner", "test-repo", &since, 1)
		require.NoError(t, err)
		assert.Empty(t, commits)
	})

	t.Run("validation error", func(t *testing.T) {
		_, err := client.GetCommitsPage(ctx, "", "test-repo", &since, 1)
		assert.Error(t, err)
		assert.IsType(t, &ValidationError{}, err)

		_, err = client.GetCommitsPage(ctx, "test-owner", "", &since, 1)
		assert.Error(t, err)
		assert.IsType(t, &ValidationError{}, err)
	})

	t.Run("server error with retry", func(t *testing.T) {
		attempts := 0
		server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempts++
			if attempts < 3 {
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`[]`))
		})

		commits, err := client.GetCommitsPage(ctx, "test-owner", "test-repo", &since, 1)
		require.NoError(t, err)
		assert.Empty(t, commits)
		assert.Equal(t, 3, attempts)
	})

	t.Run("malformed response", func(t *testing.T) {
		server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`invalid json`))
		})

		_, err := client.GetCommitsPage(ctx, "test-owner", "test-repo", &since, 1)
		assert.Error(t, err)
		assert.IsType(t, &GitHubError{}, err)
	})
}

func TestGitHubClient_RateLimitHandling(t *testing.T) {
	client, server, cleanup := setupTestClient(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("primary rate limit", func(t *testing.T) {
		server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-RateLimit-Limit", "5000")
			w.Header().Set("X-RateLimit-Remaining", "0")
			w.Header().Set("X-RateLimit-Reset", "1234567890")
			w.WriteHeader(http.StatusTooManyRequests)
		})

		_, err := client.GetRepository(ctx, "test-owner", "test-repo")
		assert.Error(t, err)
		assert.True(t, IsRateLimitError(err))

		// Verify rate limit info was updated
		assert.Equal(t, 5000, client.rateLimitInfo.Limit)
		assert.Equal(t, 0, client.rateLimitInfo.Remaining)
		assert.Equal(t, time.Unix(1234567890, 0), client.rateLimitInfo.ResetTime)
	})

	t.Run("secondary rate limit", func(t *testing.T) {
		server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Retry-After", "30")
			w.WriteHeader(http.StatusTooManyRequests)
		})

		_, err := client.GetRepository(ctx, "test-owner", "test-repo")
		assert.Error(t, err)
		assert.True(t, IsRateLimitError(err))

		// Verify secondary rate limit was set
		assert.True(t, time.Until(client.rateLimitInfo.SecondaryLimitReset) <= 30*time.Second)
	})

	t.Run("rate limit recovery", func(t *testing.T) {
		attempts := 0
		server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attempts++
			if attempts == 1 {
				w.Header().Set("X-RateLimit-Limit", "5000")
				w.Header().Set("X-RateLimit-Remaining", "0")
				w.Header().Set("X-RateLimit-Reset", "1234567890")
				w.WriteHeader(http.StatusTooManyRequests)
				return
			}
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"name": "test-repo"}`))
		})

		repo, err := client.GetRepository(ctx, "test-owner", "test-repo")
		require.NoError(t, err)
		assert.Equal(t, "test-repo", repo.Name)
		assert.Equal(t, 2, attempts)
	})
}

func TestGitHubClient_ErrorHandling(t *testing.T) {
	client, server, cleanup := setupTestClient(t)
	defer cleanup()

	ctx := context.Background()

	t.Run("network error", func(t *testing.T) {
		server.Close() // Force network error

		_, err := client.GetRepository(ctx, "test-owner", "test-repo")
		assert.Error(t, err)
		assert.IsType(t, &GitHubError{}, err)
	})

	t.Run("invalid response", func(t *testing.T) {
		server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`invalid json`))
		})

		_, err := client.GetRepository(ctx, "test-owner", "test-repo")
		assert.Error(t, err)
		assert.IsType(t, &GitHubError{}, err)
	})

	t.Run("unauthorized", func(t *testing.T) {
		server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusUnauthorized)
		})

		_, err := client.GetRepository(ctx, "test-owner", "test-repo")
		assert.Error(t, err)
		assert.IsType(t, &GitHubError{}, err)
	})

	t.Run("forbidden", func(t *testing.T) {
		server.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
		})

		_, err := client.GetRepository(ctx, "test-owner", "test-repo")
		assert.Error(t, err)
		assert.IsType(t, &GitHubError{}, err)
	})
}

/* Removed duplicate TestParseRepoURL (and helper) (see internal/github/parse_test.go) */

/*
func TestParseRepoURL(t *testing.T) {
	tests := []struct {
		name     string
		url      string
		owner    string
		repo     string
		hasError bool
	}{
		{
			name:     "valid URL",
			url:      "https://github.com/test-owner/test-repo",
			owner:    "test-owner",
			repo:     "test-repo",
			hasError: false,
		},
		{
			name:     "valid URL with trailing slash",
			url:      "https://github.com/test-owner/test-repo/",
			owner:    "test-owner",
			repo:     "test-repo",
			hasError: false,
		},
		{
			name:     "invalid URL",
			url:      "invalid-url",
			owner:    "",
			repo:     "",
			hasError: true,
		},
		{
			name:     "missing repo",
			url:      "https://github.com/test-owner",
			owner:    "",
			repo:     "",
			hasError: true,
		},
		{
			name:     "wrong domain",
			url:      "https://gitlab.com/test-owner/test-repo",
			owner:    "",
			repo:     "",
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, repo, err := ParseRepoURL(tt.url)
			if tt.hasError {
				assert.Error(t, err)
				assert.Empty(t, owner)
				assert.Empty(t, repo)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.owner, owner)
				assert.Equal(t, tt.repo, repo)
			}
		})
	}
}
*/ 