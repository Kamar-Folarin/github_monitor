package models

import "time"

// Commit represents a Git commit
type Commit struct {
	BaseModel
	SHA         string    `json:"sha"`
	Message     string    `json:"message"`
	AuthorName  string    `json:"author_name"`
	AuthorEmail string    `json:"author_email"`
	AuthorDate  time.Time `json:"author_date"`
	CommitURL   string    `json:"commit_url"`
	RepositoryID int64    `json:"repository_id"`
}

// CommitAuthor represents commit author statistics
type CommitAuthor struct {
	Name           string `json:"name"`
	Email          string `json:"email"`
	CommitCount    int    `json:"commit_count"`
	FirstCommitAt  time.Time `json:"first_commit_at"`
	LastCommitAt   time.Time `json:"last_commit_at"`
	RepositoryID   int64  `json:"repository_id"`
}

// AuthorStats represents author statistics for a repository
type AuthorStats struct {
	Name           string `json:"name"`
	Email          string `json:"email"`
	CommitCount    int    `json:"commit_count"`
	FirstCommitAt  time.Time `json:"first_commit_at"`
	LastCommitAt   time.Time `json:"last_commit_at"`
	RepositoryID   int64  `json:"repository_id"`
}