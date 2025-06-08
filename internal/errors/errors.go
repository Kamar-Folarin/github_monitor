package errors

import (
	"fmt"
	"time"
)

// ErrorType represents the type of error
type ErrorType string

const (
	ErrNotFound     ErrorType = "NOT_FOUND"
	ErrRateLimit    ErrorType = "RATE_LIMIT"
	ErrInvalidInput ErrorType = "INVALID_INPUT"
	ErrInternal     ErrorType = "INTERNAL"
	ErrUnauthorized ErrorType = "UNAUTHORIZED"
)

// AppError represents an application error
type AppError struct {
	Type      ErrorType
	Message   string
	Cause     error
	Timestamp time.Time
}

func (e *AppError) Error() string {
	if e.Cause != nil {
		return fmt.Sprintf("%s: %s (caused by: %v)", e.Type, e.Message, e.Cause)
	}
	return fmt.Sprintf("%s: %s", e.Type, e.Message)
}

// New creates a new AppError
func New(errType ErrorType, message string, cause error) *AppError {
	return &AppError{
		Type:      errType,
		Message:   message,
		Cause:     cause,
		Timestamp: time.Now(),
	}
}

// IsNotFound checks if the error is a not found error
func IsNotFound(err error) bool {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Type == ErrNotFound
	}
	return false
}

// IsRateLimit checks if the error is a rate limit error
func IsRateLimit(err error) bool {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Type == ErrRateLimit
	}
	return false
}

// IsInvalidInput checks if the error is an invalid input error
func IsInvalidInput(err error) bool {
	if appErr, ok := err.(*AppError); ok {
		return appErr.Type == ErrInvalidInput
	}
	return false
}

// IsValidationError checks if the error is a validation error
// This is an alias for IsInvalidInput since validation errors are a type of invalid input error
func IsValidationError(err error) bool {
	return IsInvalidInput(err)
}

// RateLimitError represents a GitHub API rate limit error
type RateLimitError struct {
	ResetTime time.Time
	Limit     int
	Remaining int
}

func (e *RateLimitError) Error() string {
	return fmt.Sprintf("rate limit exceeded, resets at %v (limit: %d, remaining: %d)",
		e.ResetTime, e.Limit, e.Remaining)
}

// NewRateLimitError creates a new RateLimitError
func NewRateLimitError(resetTime time.Time, limit, remaining int) *RateLimitError {
	return &RateLimitError{
		ResetTime: resetTime,
		Limit:     limit,
		Remaining: remaining,
	}
}

// RepositoryNotFoundError represents a repository not found error
type RepositoryNotFoundError struct {
	Owner string
	Name  string
}

func (e *RepositoryNotFoundError) Error() string {
	return fmt.Sprintf("repository not found: %s/%s", e.Owner, e.Name)
}

// NewRepositoryNotFoundError creates a new RepositoryNotFoundError
func NewRepositoryNotFoundError(owner, name string) *RepositoryNotFoundError {
	return &RepositoryNotFoundError{
		Owner: owner,
		Name:  name,
	}
}

// NewNotFoundError creates a new not found error
func NewNotFoundError(message string, err error) *AppError {
	return New(ErrNotFound, message, err)
}

// NewValidationError creates a new validation error
func NewValidationError(message string, err error) *AppError {
	return New(ErrInvalidInput, message, err)
}

// NewUnauthorizedError creates a new unauthorized error
func NewUnauthorizedError(message string, err error) *AppError {
	return New(ErrUnauthorized, message, err)
}

// NewForbiddenError creates a new forbidden error
func NewForbiddenError(message string, err error) *AppError {
	return New(ErrInvalidInput, message, err)
}

// NewInternalError creates a new internal error
func NewInternalError(message string, err error) *AppError {
	return New(ErrInternal, message, err)
}

// SyncInProgressError represents an error when a sync operation is already in progress
type SyncInProgressError struct {
	RepositoryURL string
}

func (e *SyncInProgressError) Error() string {
	return fmt.Sprintf("sync already in progress for repository: %s", e.RepositoryURL)
}

// NewSyncInProgressError creates a new SyncInProgressError
func NewSyncInProgressError(repoURL string) error {
	return &SyncInProgressError{
		RepositoryURL: repoURL,
	}
}

// NotFoundError represents a not found error
type NotFoundError struct {
	Resource string
	ID       string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("%s not found: %s", e.Resource, e.ID)
}

// NewResourceNotFoundError creates a new NotFoundError for a specific resource
func NewResourceNotFoundError(resource, id string) error {
	return &NotFoundError{
		Resource: resource,
		ID:       id,
	}
} 