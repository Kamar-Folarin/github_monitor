package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/oauth2"

	"github.com/Kamar-Folarin/github-monitor/internal/models"
)

// RateLimitInfo holds information about GitHub API rate limits
type RateLimitInfo struct {
	Limit     int
	Remaining int
	ResetTime time.Time
	// Add secondary rate limit info
	SecondaryLimitRemaining int
	SecondaryLimitReset     time.Time
}

// GitHubClient represents a client for interacting with the GitHub API
type GitHubClient struct {
	client *http.Client
	token  string
	logger *logrus.Logger
	rateLimitInfo RateLimitInfo
	// Add backoff configuration
	maxRetries     int
	initialBackoff time.Duration
	maxBackoff     time.Duration
}

// ClientOption allows configuring the GitHub client
type ClientOption func(*GitHubClient)

// WithRetryConfig configures retry behavior
func WithRetryConfig(maxRetries int, initialBackoff, maxBackoff time.Duration) ClientOption {
	return func(c *GitHubClient) {
		c.maxRetries = maxRetries
		c.initialBackoff = initialBackoff
		c.maxBackoff = maxBackoff
	}
}

// NewGitHubClient creates a new GitHub client with the given token and options
func NewGitHubClient(token string, logger *logrus.Logger, opts ...ClientOption) *GitHubClient {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	httpClient := oauth2.NewClient(context.Background(), ts)
	httpClient.Timeout = 120 * time.Second

	client := &GitHubClient{
		client:         httpClient,
		token:          token,
		logger:         logger,
		maxRetries:     3,
		initialBackoff: time.Second,
		maxBackoff:     time.Minute,
	}

	// Apply options
	for _, opt := range opts {
		opt(client)
	}

	return client
}

// updateRateLimitInfo updates the rate limit information from response headers
func (c *GitHubClient) updateRateLimitInfo(resp *http.Response) {
	if limit := resp.Header.Get("X-RateLimit-Limit"); limit != "" {
		c.rateLimitInfo.Limit, _ = strconv.Atoi(limit)
	}
	if remaining := resp.Header.Get("X-RateLimit-Remaining"); remaining != "" {
		c.rateLimitInfo.Remaining, _ = strconv.Atoi(remaining)
	}
	if reset := resp.Header.Get("X-RateLimit-Reset"); reset != "" {
		if resetTime, err := strconv.ParseInt(reset, 10, 64); err == nil {
			c.rateLimitInfo.ResetTime = time.Unix(resetTime, 0)
		}
	}

	// Handle secondary rate limits
	if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
		if retrySeconds, err := strconv.ParseInt(retryAfter, 10, 64); err == nil {
			c.rateLimitInfo.SecondaryLimitReset = time.Now().Add(time.Duration(retrySeconds) * time.Second)
		}
	}
}

// checkRateLimit checks if we're about to hit rate limits and handles accordingly
func (c *GitHubClient) checkRateLimit() error {
	now := time.Now()

	// Check primary rate limit
	if c.rateLimitInfo.Remaining <= 5 { // Buffer of 5 requests
		waitTime := time.Until(c.rateLimitInfo.ResetTime)
		if waitTime > 0 {
			c.logger.Warnf("Primary rate limit nearly exceeded. Waiting %v before next request", waitTime)
			time.Sleep(waitTime)
		}
	}

	// Check secondary rate limit
	if !c.rateLimitInfo.SecondaryLimitReset.IsZero() && now.Before(c.rateLimitInfo.SecondaryLimitReset) {
		waitTime := time.Until(c.rateLimitInfo.SecondaryLimitReset)
		c.logger.Warnf("Secondary rate limit active. Waiting %v before next request", waitTime)
		time.Sleep(waitTime)
	}

	return nil
}

// doRequestWithBackoff performs an HTTP request with exponential backoff
func (c *GitHubClient) doRequestWithBackoff(req *http.Request, result interface{}) error {
	var lastErr error
	backoff := c.initialBackoff

	for attempt := 0; attempt < c.maxRetries; attempt++ {
		if err := c.checkRateLimit(); err != nil {
			return err
		}

		resp, err := c.client.Do(req)
		if err != nil {
			lastErr = NewGitHubError(0, "request failed", err)
			c.logger.Warnf("Request attempt %d failed: %v", attempt+1, err)
			time.Sleep(backoff)
			backoff = time.Duration(math.Min(float64(backoff*2), float64(c.maxBackoff)))
			continue
		}

		c.updateRateLimitInfo(resp)

		// Handle rate limit responses
		if resp.StatusCode == http.StatusTooManyRequests {
			resetTime := c.rateLimitInfo.ResetTime
			if !c.rateLimitInfo.SecondaryLimitReset.IsZero() {
				resetTime = c.rateLimitInfo.SecondaryLimitReset
			}
			waitTime := time.Until(resetTime)
			c.logger.Warnf("Rate limit exceeded. Waiting %v before retry", waitTime)
			time.Sleep(waitTime)
			continue
		}

		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = NewGitHubError(resp.StatusCode, "failed to read response body", err)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			lastErr = NewGitHubError(resp.StatusCode, string(body), nil)
			if resp.StatusCode >= 500 {
				// Retry on server errors
				time.Sleep(backoff)
				backoff = time.Duration(math.Min(float64(backoff*2), float64(c.maxBackoff)))
				continue
			}
			return lastErr
		}

		if result != nil {
			if err := json.Unmarshal(body, result); err != nil {
				return NewGitHubError(resp.StatusCode, "failed to decode response", err)
			}
		}

		return nil
	}

	return fmt.Errorf("max retries exceeded: %w", lastErr)
}

// GetRepository gets repository information from GitHub
func (c *GitHubClient) GetRepository(ctx context.Context, owner, name string) (*models.Repository, error) {
	if owner == "" {
		return nil, NewValidationError("owner", "cannot be empty")
	}
	if name == "" {
		return nil, NewValidationError("name", "cannot be empty")
	}

	url := fmt.Sprintf("https://api.github.com/repos/%s/%s", owner, name)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	var repo models.Repository
	if err := c.doRequestWithBackoff(req, &repo); err != nil {
		return nil, err
	}

	return &repo, nil
}

// GetCommitsPage gets a page of commits from a repository
func (c *GitHubClient) GetCommitsPage(ctx context.Context, owner, name string, since *time.Time, page int) ([]*models.Commit, error) {
	if owner == "" {
		return nil, NewValidationError("owner", "cannot be empty")
	}
	if name == "" {
		return nil, NewValidationError("name", "cannot be empty")
	}

	baseURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/commits", owner, name)
	query := url.Values{}
	if since != nil {
		query.Set("since", since.Format(time.RFC3339))
	}
	query.Set("page", strconv.Itoa(page))
	query.Set("per_page", "100")

	req, err := http.NewRequestWithContext(ctx, "GET", baseURL+"?"+query.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	var commits []struct {
		SHA    string `json:"sha"`
		Commit struct {
			Message string `json:"message"`
			Author  struct {
				Name  string    `json:"name"`
				Email string    `json:"email"`
				Date  time.Time `json:"date"`
			} `json:"author"`
		} `json:"commit"`
		HTMLURL string `json:"html_url"`
	}

	if err := c.doRequestWithBackoff(req, &commits); err != nil {
		return nil, err
	}

	var result []*models.Commit
	for _, c := range commits {
		result = append(result, &models.Commit{
			SHA:         c.SHA,
			Message:     c.Commit.Message,
			AuthorName:  c.Commit.Author.Name,
			AuthorEmail: c.Commit.Author.Email,
			AuthorDate:  c.Commit.Author.Date,
			CommitURL:   c.HTMLURL,
		})
	}

	return result, nil
}

// GetCommits gets commits from a repository in batches
func (c *GitHubClient) GetCommits(ctx context.Context, owner, name string, since time.Time, batchSize int, processBatch func([]*models.Commit) error) error {
	logger := logrus.WithFields(logrus.Fields{
		"owner":     owner,
		"repo":      name,
		"since":     since,
		"batch_size": batchSize,
	})
	logger.Info("Starting to fetch commits from GitHub API")

	if owner == "" || name == "" {
		err := fmt.Errorf("owner and name cannot be empty")
		logger.WithError(err).Error("Invalid repository parameters")
		return err
	}

	baseURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/commits", owner, name)
	query := ""
	if !since.IsZero() {
		query = "since=" + since.Format(time.RFC3339)
	}

	page := 1
	totalCommits := 0

	for {
		pagedURL := baseURL
		if query != "" {
			pagedURL += "?" + query + "&page=" + strconv.Itoa(page)
		} else {
			pagedURL += "?page=" + strconv.Itoa(page)
		}
		pagedURL += "&per_page=100"

		logger.WithFields(logrus.Fields{
			"page": page,
			"url":  pagedURL,
		}).Info("Requesting commits page from GitHub API")

		req, err := http.NewRequestWithContext(ctx, "GET", pagedURL, nil)
		if err != nil {
			logger.WithError(err).Error("Failed to create request")
			return fmt.Errorf("failed to create request: %w", err)
		}

		// Add rate limit info to logs
		if c.rateLimitInfo.Remaining > 0 {
			logger.WithField("rate_limit_remaining", c.rateLimitInfo.Remaining).Debug("Rate limit info")
		}

		var commits []struct {
			SHA    string `json:"sha"`
			Commit struct {
				Message string `json:"message"`
				Author  struct {
					Name  string    `json:"name"`
					Email string    `json:"email"`
					Date  time.Time `json:"date"`
				} `json:"author"`
			} `json:"commit"`
			HTMLURL string `json:"html_url"`
		}

		if err := c.doRequestWithBackoff(req, &commits); err != nil {
			logger.WithError(err).Error("Failed to fetch commits from GitHub API")
			return err
		}

		if len(commits) == 0 {
			logger.WithField("page", page).Info("No more commits to fetch")
			break
		}

		logger.WithFields(logrus.Fields{
			"page":           page,
			"commits_found":  len(commits),
			"total_commits":  totalCommits,
		}).Info("Successfully fetched commits page")

		var result []*models.Commit
		for _, c := range commits {
			result = append(result, &models.Commit{
				SHA:         c.SHA,
				Message:     c.Commit.Message,
				AuthorName:  c.Commit.Author.Name,
				AuthorEmail: c.Commit.Author.Email,
				AuthorDate:  c.Commit.Author.Date,
				CommitURL:   c.HTMLURL,
			})
		}

		// Process the batch
		if err := processBatch(result); err != nil {
			logger.WithError(err).Error("Failed to process commit batch")
			return fmt.Errorf("failed to process batch: %w", err)
		}

		totalCommits += len(result)
		logger.WithFields(logrus.Fields{
			"page":          page,
			"total_commits": totalCommits,
		}).Info("Successfully processed commit batch")

		// Check if we've reached the batch size limit
		if batchSize > 0 && totalCommits >= batchSize {
			logger.WithFields(logrus.Fields{
				"batch_size":    batchSize,
				"total_commits": totalCommits,
			}).Info("Reached batch size limit")
			break
		}

		page++
	}

	logger.WithFields(logrus.Fields{
		"total_pages":   page - 1,
		"total_commits": totalCommits,
	}).Info("Completed fetching commits from GitHub API")

	return nil
}

func (c *GitHubClient) getToken() string {
	return c.token
}

// Add a helper for retrying HTTP requests
func doWithRetry(attempts int, sleep time.Duration, fn func() error) error {
	var err error
	for i := 0; i < attempts; i++ {
		err = fn()
		if err == nil {
			return nil
		}
		time.Sleep(sleep)
	}
	return err
}

// IsRateLimitError checks if an error is a rate limit error
func IsRateLimitError(err error) bool {
	_, ok := err.(*RateLimitError)
	return ok
}

// GetRepositoryByURL retrieves a repository by its URL
func (c *GitHubClient) GetRepositoryByURL(ctx context.Context, repoURL string) (*models.Repository, error) {
	owner, name, err := parseRepoURL(repoURL)
	if err != nil {
		return nil, err
	}
	return c.GetRepository(ctx, owner, name)
}

// GetCommitAuthors retrieves commit authors for a repository
func (c *GitHubClient) GetCommitAuthors(ctx context.Context, owner, name string) ([]*models.CommitAuthor, error) {
	if owner == "" {
		return nil, NewValidationError("owner", "cannot be empty")
	}
	if name == "" {
		return nil, NewValidationError("name", "cannot be empty")
	}

	// Get all commits to extract unique authors
	authors := make(map[string]*models.CommitAuthor)
	err := c.GetCommits(ctx, owner, name, time.Time{}, 100, func(commits []*models.Commit) error {
		for _, commit := range commits {
			key := commit.AuthorEmail
			if _, exists := authors[key]; !exists {
				authors[key] = &models.CommitAuthor{
					Name:  commit.AuthorName,
					Email: commit.AuthorEmail,
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Convert map to slice
	result := make([]*models.CommitAuthor, 0, len(authors))
	for _, author := range authors {
		result = append(result, author)
	}
	return result, nil
}