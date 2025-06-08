package config

import "time"

// SyncConfig holds synchronization configuration
type SyncConfig struct {
	Interval time.Duration
	BatchSize int
	MaxConcurrentSyncs int
	StatusPersistenceInterval time.Duration
	BatchConfig BatchConfig
}

// BatchConfig holds batch processing configuration
type BatchConfig struct {
	Size int
	Workers int
	MaxRetries int
	BatchDelay time.Duration
	MaxCommits int
}

// DefaultSyncConfig returns the default sync configuration
func DefaultSyncConfig() *SyncConfig {
	return &SyncConfig{
		Interval:                  time.Minute * 5,
		BatchSize:                100,
		MaxConcurrentSyncs:       3,
		StatusPersistenceInterval: time.Minute,
		BatchConfig: BatchConfig{
			Size:       1000,
			Workers:    3,
			MaxRetries: 3,
			BatchDelay: time.Second,
			MaxCommits: 0,
		},
	}
} 