package github

import "time"

type Repository struct {
	ID              int       `json:"-"`
	Name            string    `json:"name"`
	Description     string    `json:"description"`
	URL             string    `json:"html_url"`
	Language        string    `json:"language"`
	ForksCount      int       `json:"forks_count"`
	StarsCount      int       `json:"stargazers_count"`
	OpenIssuesCount int       `json:"open_issues_count"`
	WatchersCount   int       `json:"watchers_count"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	LastSyncedAt    time.Time `json:"-"`
}

type Commit struct {
	ID          int       `json:"-"`
	RepositoryID int       `json:"-"`
	SHA         string    `json:"sha"`
	Message     string    `json:"message"`
	AuthorName  string    `json:"author_name"`
	AuthorEmail string    `json:"author_email"`
	AuthorDate  time.Time `json:"author_date"`
	CommitURL   string    `json:"html_url"`
}

type AuthorStats struct {
	Name        string `json:"name"`
	Email       string `json:"email"`
	CommitCount int    `json:"commit_count"`
}