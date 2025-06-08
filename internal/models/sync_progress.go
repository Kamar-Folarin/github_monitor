package models

import "time"

// SyncProgress represents the progress of a repository sync operation
type SyncProgress struct {
	RepoURL string `json:"repo_url"`
	CurrentPage int `json:"current_page"`
	TotalPages int `json:"total_pages"`
	ProcessedCommits int `json:"processed_commits"`
	TotalCommits int `json:"total_commits"`
	LastProcessedSHA string `json:"last_processed_sha"`
	StartDate time.Time `json:"start_date"`
	LastUpdated time.Time `json:"last_updated"`
	Status string `json:"status"`
	Error string `json:"error,omitempty"`
	BatchSize int `json:"batch_size"`
	CurrentBatch int `json:"current_batch"`
	TotalBatches int `json:"total_batches"`
} 