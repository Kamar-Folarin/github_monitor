package models

import "time"

// SyncInterval represents a duration in seconds for sync operations
// @swagger:model SyncInterval
type SyncInterval int64

// ToDuration converts SyncInterval to time.Duration
func (si SyncInterval) ToDuration() time.Duration {
	return time.Duration(si) * time.Second
}

// FromDuration creates a SyncInterval from time.Duration
func FromDuration(d time.Duration) SyncInterval {
	return SyncInterval(d.Seconds())
}

// RepositorySyncConfig holds repository-specific sync configuration
// @swagger:model RepositorySyncConfig
// @Description Repository-specific sync configuration (for example, to enable or disable sync, set a custom interval, or a custom batch size)
type RepositorySyncConfig struct {
	// SyncInterval is the custom sync interval for this repository in seconds (0 means use global default)
	// @example 3600
	// @minimum 0
	SyncInterval SyncInterval `json:"sync_interval,omitempty"`
	
	// InitialSyncDate is when to start syncing from (if not specified, syncs from beginning)
	// @example 2024-03-20T00:00:00Z
	// @format date-time
	InitialSyncDate *time.Time `json:"initial_sync_date,omitempty"`
	
	// BatchSize is the custom batch size for this repository (0 means use global default)
	// @example 100
	// @minimum 0
	BatchSize int `json:"batch_size,omitempty"`
	
	// Priority determines sync order (higher number = higher priority)
	// @example 0
	Priority int `json:"priority"`
	
	// Enabled determines if the repository should be synced
	// @example true
	Enabled bool `json:"enabled"`
}

type Repository struct {
	BaseModel

	Name            string    `json:"name"`
	Description     string    `json:"description"`
	URL             string    `json:"html_url"`
	Language        string    `json:"language"`
	ForksCount      int       `json:"forks_count"`
	StarsCount      int       `json:"stargazers_count"`
	OpenIssuesCount int       `json:"open_issues_count"`
	WatchersCount   int       `json:"watchers_count"`
	LastSyncedAt    time.Time `json:"-"`
	
	// SyncConfig holds repository-specific sync configuration
	SyncConfig RepositorySyncConfig `json:"sync_config"`
}