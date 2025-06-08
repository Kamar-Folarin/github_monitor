package models

import (
	"encoding/json"
	"fmt"
	"time"
)

// SyncStatus tracks the sync state of a repository
type SyncStatus struct {
	RepositoryID int64 `json:"repository_id"`
	RepositoryURL string `json:"repository_url"`
	Status string `json:"status"`
	LastSyncAt time.Time `json:"last_sync_at"`
	LastError string `json:"last_error,omitempty"`
	IsSyncing bool `json:"is_syncing"`
	CurrentPage int `json:"current_page"`
	TotalCommits int `json:"total_commits"`
	LastCommitSHA string `json:"last_commit_sha"`
	SyncDuration int64 `json:"sync_duration"`
	StartTime time.Time `json:"start_time,omitempty"`
	BatchProgress *BatchProgress `json:"batch_progress,omitempty"`
}

// BatchProgress tracks the progress of batch processing
type BatchProgress struct {
	TotalBatches     int       `json:"total_batches"`
	ProcessedBatches int       `json:"processed_batches"`
	TotalCommits     int       `json:"total_commits"`
	ProcessedCommits int       `json:"processed_commits"`
	LastProcessedSHA string    `json:"last_processed_sha"`
	StartTime        time.Time `json:"start_time"`
	LastUpdateTime   time.Time `json:"last_update_time"`
	Errors           []error   `json:"errors,omitempty"`
}

// String returns the JSON string representation of the sync status
func (s *SyncStatus) String() string {
	data, err := json.Marshal(s)
	if err != nil {
		return fmt.Sprintf(`{"error":"failed to marshal sync status: %v"}`, err)
	}
	return string(data)
} 