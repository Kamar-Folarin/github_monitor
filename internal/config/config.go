package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port              string
	DBConnectionString string
	GitHubToken       string
	SyncInterval      time.Duration
	DefaultSyncRepo   string
}

func Load() (*Config, error) {
	port := getEnv("PORT", "8080")
	dbConnStr := getEnv("DB_CONNECTION_STRING", "")
	githubToken := getEnv("GITHUB_TOKEN", "")
	defaultSyncRepo := getEnv("DEFAULT_SYNC_REPO", "https://github.com/chromium/chromium")

	syncInterval, err := strconv.Atoi(getEnv("SYNC_INTERVAL_MINUTES", "60"))
	if err != nil {
		return nil, err
	}

	return &Config{
		Port:              port,
		DBConnectionString: dbConnStr,
		GitHubToken:       githubToken,
		SyncInterval:      time.Duration(syncInterval) * time.Minute,
		DefaultSyncRepo:   defaultSyncRepo,
	}, nil
}

func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}