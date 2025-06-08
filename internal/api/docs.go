package api

import (
	"time"

	_ "github.com/Kamar-Folarin/github-monitor/docs"
)

// @title GitHub Monitor API
// @version 1.0
// @description API for monitoring GitHub repositories and commits
// @host localhost:8080
// @BasePath /api/v1

// Repository represents a monitored GitHub repository
// @Description A GitHub repository being monitored
// @swagger:model Repository
type Repository struct {
	// ID of the repository
	// @example 1
	ID uint `json:"id" example:"1"`
	// Name of the repository
	// @example github-monitor
	Name string `json:"name" example:"chromium"`
	// Description of the repository
	// @example A tool for monitoring GitHub repositories
	Description string `json:"description"`
	// URL of the repository
	// @example https://github.com/owner/repo
	URL string `json:"url" example:"https://github.com/chromium/chromium"`
	// Primary programming language
	// @example Go
	Language string `json:"language" example:"C++"`
	// Number of forks
	// @example 42
	ForksCount int `json:"forks_count" example:"1234"`
	// Number of stars
	// @example 100
	StarsCount int `json:"stars_count" example:"5678"`
	// Number of open issues
	// @example 5
	OpenIssuesCount int `json:"open_issues" example:"100"`
	// Number of watchers
	// @example 50
	WatchersCount int `json:"watchers" example:"2000"`
	// When the repository was last synced
	// @example 2024-01-01T12:00:00Z
	LastSyncedAt time.Time `json:"last_synced_at"`
	// When the repository was created in the system
	// @example 2024-01-01T12:00:00Z
	CreatedAt time.Time `json:"created_at" example:"2024-03-20T00:00:00Z"`
	// When the repository was last updated in the system
	// @example 2024-01-01T12:00:00Z
	UpdatedAt time.Time `json:"updated_at" example:"2024-03-21T00:00:00Z"`
}

// AuthorStats represents commit author statistics
// @Description Statistics for a commit author
// @swagger:model AuthorStats
type AuthorStats struct {
	// Name of the author
	// @example John Doe
	Name string `json:"author" example:"John Doe"`
	// Email of the author
	// @example john@example.com
	Email string `json:"email" example:"john@example.com"`
	// Number of commits by this author
	// @example 42
	CommitCount int `json:"commits" example:"42"`
	// First seen date
	// @example 2024-03-20T00:00:00Z
	FirstSeen string `json:"first_seen" example:"2024-03-20T00:00:00Z"`
	// Last seen date
	// @example 2024-03-21T00:00:00Z
	LastSeen string `json:"last_seen" example:"2024-03-21T00:00:00Z"`
}

// Commit represents a GitHub commit
// @Description A commit from a GitHub repository
// @swagger:model Commit
type Commit struct {
	// ID of the commit
	// @example 1
	ID uint `json:"id"`
	// SHA of the commit
	// @example abc123def456
	SHA string `json:"sha" example:"a1b2c3d4e5f6"`
	// Commit message
	// @example Fix bug in login flow
	Message string `json:"message" example:"Fix bug in feature X"`
	// Name of the commit author
	// @example John Doe
	AuthorName string `json:"author" example:"John Doe"`
	// Email of the commit author
	// @example john@example.com
	AuthorEmail string `json:"email" example:"john@example.com"`
	// Date when the commit was authored
	// @example 2024-01-01T12:00:00Z
	AuthorDate time.Time `json:"date" example:"2024-03-20T12:00:00Z"`
	// URL to view the commit on GitHub
	// @example https://github.com/owner/repo/commit/abc123
	CommitURL string `json:"url" example:"https://github.com/owner/repo/commit/a1b2c3d4e5f6"`
	// When the commit was created in the system
	// @example 2024-01-01T12:00:00Z
	CreatedAt time.Time `json:"created_at"`
	// When the commit was last updated in the system
	// @example 2024-01-01T12:00:00Z
	UpdatedAt time.Time `json:"updated_at"`
	// URL of the repository
	// @example https://github.com/owner/repo
	RepoURL string `json:"repo_url" example:"https://github.com/owner/repo"`
}

// ErrorResponse represents an API error
// @Description Error response from the API
// @swagger:model ErrorResponse
type ErrorResponse struct {
	// Error message
	// @example Invalid repository URL
	Error string `json:"error" example:"Failed to process request"`
}

// SyncStatus represents the sync status of a repository
// @Description Current sync status of a repository
// @swagger:model SyncStatus
type SyncStatus struct {
	// URL of the repository
	// @example https://github.com/owner/repo
	RepositoryURL string `json:"repository_url" example:"https://github.com/owner/repo"`
	// Current sync status
	// @example in_progress
	Status string `json:"status" example:"syncing" enums:"idle,syncing,error"`
	// Whether the repository is currently syncing
	// @example true
	IsSyncing bool `json:"is_syncing"`
	// Last sync time
	// @example 2024-01-01T12:00:00Z
	LastSyncAt time.Time `json:"last_sync" example:"2024-03-21T00:00:00Z"`
	// Last error message, if any
	// @example Failed to fetch commits
	LastError string `json:"error,omitempty" example:"Rate limit exceeded"`
	// Total number of commits synced
	// @example 1000
	TotalCommits int `json:"total_commits"`
	// SHA of the last commit synced
	// @example abc123def456
	LastCommitSHA string `json:"last_commit_sha,omitempty"`
	// Duration of the last sync in seconds
	// @example 330
	SyncDuration int64 `json:"sync_duration"`
	// When the sync started
	// @example 2024-01-01T12:00:00Z
	StartTime time.Time `json:"start_time"`
	// Next sync time
	// @example 2024-03-21T01:00:00Z
	NextSync string `json:"next_sync" example:"2024-03-21T01:00:00Z"`
	// Rate limit information
	RateLimit struct {
		// Total limit
		// @example 5000
		Limit int `json:"limit" example:"5000"`
		// Remaining requests
		// @example 4500
		Remaining int `json:"remaining" example:"4500"`
		// Reset time in Unix timestamp
		// @example 1616284800
		Reset int `json:"reset" example:"1616284800"`
	} `json:"rate_limit"`
}

// CommitListResponse represents a paginated list of commits
// @Description A paginated list of commits
// @swagger:model CommitListResponse
type CommitListResponse struct {
	// Data contains the list of commits
	// @example [{"id":1,"sha":"a1b2c3d4e5f6","message":"Fix bug in feature X","author_name":"John Doe","author_email":"john@example.com","author_date":"2024-03-20T12:00:00Z","commit_url":"https://github.com/owner/repo/commit/a1b2c3d4e5f6","created_at":"2024-03-20T12:00:00Z","updated_at":"2024-03-20T12:00:00Z","repo_url":"https://github.com/owner/repo"}]
	Data []Commit `json:"data"`
	// Pagination contains pagination metadata
	// @example {"total":1000,"limit":100,"offset":0}
	Pagination struct {
		// Total number of commits
		// @example 1000
		Total int `json:"total" example:"1000"`
		// Number of commits per page
		// @example 100
		Limit int `json:"limit" example:"100"`
		// Offset for pagination
		// @example 0
		Offset int `json:"offset" example:"0"`
	} `json:"pagination"`
}

// AuthorListResponse represents a list of commit authors with metadata
// @Description A list of commit authors with metadata
// @swagger:model AuthorListResponse
type AuthorListResponse struct {
	// Data contains the list of authors
	// @example [{"author":"John Doe","email":"john@example.com","commits":42,"first_seen":"2024-03-20T00:00:00Z","last_seen":"2024-03-21T00:00:00Z"}]
	Data []AuthorStats `json:"data"`
	// Metadata contains repository and limit information
	// @example {"repository":"https://github.com/owner/repo","limit":10}
	Metadata struct {
		// URL of the repository
		// @example https://github.com/owner/repo
		Repository string  `json:"repository" example:"https://github.com/owner/repo"`
		// Number of authors to return
		// @example 10
		Limit int     `json:"limit" example:"10"`
		// Start date for filtering commits
		// @example 2024-03-20T00:00:00Z
		Since *string `json:"since,omitempty" example:"2024-03-20T00:00:00Z"`
		// End date for filtering commits
		// @example 2024-03-21T00:00:00Z
		Until *string `json:"until,omitempty" example:"2024-03-21T00:00:00Z"`
	} `json:"metadata"`
}

// @Description Get a list of all monitored repositories
// @Tags repositories
// @Accept json
// @Produce json
// @Success 200 {array} Repository "List of repositories"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /repositories [get]
// Note: Implementation is in handler.go