package models

import "time"

// BaseModel contains common fields for all database models
type BaseModel struct {
	ID        int       `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// SyncTracking contains common fields for tracking sync operations
type SyncTracking struct {
	LastSyncedAt time.Time `json:"last_synced_at"`
	IsSyncing    bool      `json:"is_syncing"`
	LastError    string    `json:"last_error,omitempty"`
}

// ProgressTracking contains common fields for tracking progress
type ProgressTracking struct {
	StartTime      time.Time `json:"start_time"`
	LastUpdateTime time.Time `json:"last_update_time"`
	Status         string    `json:"status"` // e.g., "in_progress", "completed", "failed", "paused"
	Error          string    `json:"error,omitempty"`
}

// BatchTracking contains common fields for batch processing
type BatchTracking struct {
	BatchSize         int    `json:"batch_size"`
	CurrentBatch      int    `json:"current_batch"`
	TotalBatches      int    `json:"total_batches"`
	ProcessedBatches  int    `json:"processed_batches"`
	TotalItems        int    `json:"total_items"`
	ProcessedItems    int    `json:"processed_items"`
	LastProcessedItem string `json:"last_processed_item"`
} 