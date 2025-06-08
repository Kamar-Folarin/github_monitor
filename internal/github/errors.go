package github

import (
	"fmt"
	"time"
)

// Error types for GitHub client operations
type GitHubError struct {
	StatusCode int
	Message    string
	Err        error
}

func (e *GitHubError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("GitHub API error (status %d): %s: %v", e.StatusCode, e.Message, e.Err)
	}
	return fmt.Sprintf("GitHub API error (status %d): %s", e.StatusCode, e.Message)
}

func (e *GitHubError) Unwrap() error {
	return e.Err
}

// RateLimitError represents when we hit GitHub's rate limits
type RateLimitError struct {
	ResetTime time.Time
	Limit     int
	Remaining int
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("GitHub API rate limit exceeded. Reset at %v. Limit: %d, Remaining: %d",
		e.ResetTime, e.Limit, e.Remaining)
}

// ValidationError represents invalid input to GitHub client methods
type ValidationError struct {
	Field string
	Value string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error: invalid %s: %s", e.Field, e.Value)
}

// RepositoryNotFoundError represents when a repository cannot be found
type RepositoryNotFoundError struct {
	Owner string
	Name  string
}

func (e *RepositoryNotFoundError) Error() string {
	return fmt.Sprintf("repository not found: %s/%s", e.Owner, e.Name)
}

// NewGitHubError creates a new GitHubError with the given status code and message
func NewGitHubError(statusCode int, message string, err error) error {
	return &GitHubError{
		StatusCode: statusCode,
		Message:    message,
		Err:        err,
	}
}

// NewRateLimitError creates a new RateLimitError
func NewRateLimitError(resetTime time.Time, limit, remaining int) error {
	return &RateLimitError{
		ResetTime: resetTime,
		Limit:     limit,
		Remaining: remaining,
	}
}

// NewValidationError creates a new ValidationError
func NewValidationError(field, value string) error {
	return &ValidationError{
		Field: field,
		Value: value,
	}
}

// NewRepositoryNotFoundError creates a new RepositoryNotFoundError
func NewRepositoryNotFoundError(owner, name string) error {
	return &RepositoryNotFoundError{
		Owner: owner,
		Name:  name,
	}
} 