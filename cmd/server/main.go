package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/sirupsen/logrus"

	"github.com/Kamar-Folarin/github-monitor/internal/api"
	"github.com/Kamar-Folarin/github-monitor/internal/config"
	"github.com/Kamar-Folarin/github-monitor/internal/db"
	"github.com/Kamar-Folarin/github-monitor/internal/github"

	_ "github.com/Kamar-Folarin/github-monitor/docs"
)

// Constants
const (
	defaultPort    = "8080"
	defaultSyncInt = 1 * time.Hour
)

// @title GitHub Monitor API
// @version 1.0
// @description API for monitoring GitHub repositories and commits
// @contact.name API Support
// @contact.url http://github.com/Kamar-Folarin
// @contact.email omofolarinwa.kamar@gamil.com
// @license.name MIT
// @license.url https://opensource.org/licenses/MIT
// @host localhost:8080
// @BasePath /api/v1
// @schemes http https
// @securityDefinitions.apikey ApiKeyAuth
// @in header
// @name Authorization
func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	// Initialize logger
	logger := logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{
		TimestampFormat: time.RFC3339,
	})
	logger.SetOutput(os.Stdout)

	// Load configuration with defaults
	cfg := &config.Config{
		Port:              getEnv("PORT", defaultPort),
		DBConnectionString: os.Getenv("DB_CONNECTION_STRING"),
		GitHubToken:       os.Getenv("GITHUB_TOKEN"),
		SyncInterval:      defaultSyncInt,
	}

	// Validate minimum required config
	if cfg.DBConnectionString == "" || cfg.GitHubToken == "" {
		logger.Fatal("Missing required configuration (DB_CONNECTION_STRING and GITHUB_TOKEN must be set)")
	}

	// Initialize database
	dbStore, err := db.NewPostgresStore(cfg.DBConnectionString)
	if err != nil {
		logger.Fatalf("Failed to initialize database: %v", err)
	}

	// Run migrations with retry logic
	if err := retry(3, 5*time.Second, func() error {
		return dbStore.Migrate()
	}); err != nil {
		logger.Fatalf("Failed to run migrations after retries: %v", err)
	}

	// Initialize GitHub client
	client := github.NewGitHubClient(cfg.GitHubToken, logger)

	// Initialize services
	syncCfg := &config.SyncConfig{
		MaxConcurrentSyncs: 3,
		BatchSize:         100,
		StatusPersistenceInterval: time.Minute,
		BatchConfig: config.BatchConfig{
			Size:       1000,
			Workers:    3,
			MaxRetries: 3,
			BatchDelay: time.Second,
			MaxCommits: 0,
		},
	}

	// Create status manager first
	statusManager := github.NewStatusManager(dbStore)

	// Create repository service with status manager
	repoService := github.NewRepositoryService(client, dbStore, &config.GitHubConfig{}, statusManager)

	// Create commit service
	commitService := github.NewCommitService(client, dbStore, syncCfg)

	// Create sync service
	syncService := github.NewSyncService(repoService, commitService, statusManager, syncCfg)

	// Create API handler with all required services
	apiHandler := api.NewHandler(
		repoService,    // RepositoryService
		commitService,  // CommitService
		syncService,    // SyncService
		statusManager,  // StatusManager
		logger,
	)

	// Setup router with middleware
	router := api.SetupRouter(apiHandler)

	// Create HTTP server
	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		logger.Infof("Server starting on port %s", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatalf("Server failed: %v", err)
		}
	}()

	// Start background sync with all monitored repositories
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start sync for all repositories
	repos, err := repoService.ListRepositories(ctx)
	if err != nil {
		logger.Errorf("Failed to list repositories: %v", err)
	} else {
		for _, repo := range repos {
			if err := syncService.StartSync(ctx, repo.URL); err != nil {
				logger.Errorf("Failed to start sync for repository %s: %v", repo.URL, err)
			}
		}
	}

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	logger.Info("Shutting down server...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer shutdownCancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Errorf("Server shutdown failed: %v", err)
	}
	logger.Info("Server exited properly")
}

// Helper functions
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func retry(attempts int, sleep time.Duration, fn func() error) error {
	if err := fn(); err != nil {
		if attempts--; attempts > 0 {
			time.Sleep(sleep)
			return retry(attempts, sleep, fn)
		}
		return err
	}
	return nil
}