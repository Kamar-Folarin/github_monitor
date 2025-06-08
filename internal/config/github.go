package config

import "time"

// GitHubConfig holds GitHub-specific configuration
type GitHubConfig struct {
	Token string
	APIBaseURL string
	RateLimit RateLimitConfig
}

// RateLimitConfig holds rate limit configuration
type RateLimitConfig struct {
	MaxRetries int
	InitialBackoff time.Duration
	MaxBackoff time.Duration
	RetryMultiplier float64
}

// DefaultGitHubConfig returns the default GitHub configuration
func DefaultGitHubConfig() *GitHubConfig {
	return &GitHubConfig{
		APIBaseURL: "https://api.github.com",
		RateLimit: RateLimitConfig{
			MaxRetries:      3,
			InitialBackoff:  time.Second,
			MaxBackoff:      time.Minute,
			RetryMultiplier: 2.0,
		},
	}
} 