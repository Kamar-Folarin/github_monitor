package api

import (
	"time"

	_ "github-monitor/docs" 
)

// @title GitHub Monitor API
// @version 1.0
// @description API for monitoring GitHub repositories and commits
// @host localhost:8080
// @BasePath /api/v1

// AuthorStats represents commit author statistics
// swagger:model AuthorStats
type AuthorStats struct {
	Name        string `json:"name"`
	Email       string `json:"email"`
	CommitCount int    `json:"commit_count"`
}

// Commit represents a GitHub commit
// swagger:model Commit
type Commit struct {
	SHA        string    `json:"sha"`
	Message    string    `json:"message"`
	AuthorName string    `json:"author_name"`
	AuthorDate time.Time `json:"author_date"`
	CommitURL  string    `json:"html_url"`
}

// ErrorResponse represents an API error
// swagger:model ErrorResponse
type ErrorResponse struct {
	Error string `json:"error"`
}