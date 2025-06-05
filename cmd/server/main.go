package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"github.com/rs/cors"
	"github.com/sirupsen/logrus"

	"github-monitor/internal/api"
	"github-monitor/internal/config"
	"github-monitor/internal/db"
	"github-monitor/internal/github"

	_ "github-monitor/docs"
)

// @title GitHub Monitor API
// @version 1.0
// @description API for monitoring GitHub repositories and commits
// @contact.name API Support
// @contact.url http://github.com/Kamar-Folarin
// @contact.email omofolarinwa.kamar@gmail.com
// @license.name MIT
// @license.url https://opensource.org/licenses/MIT
// @host localhost:8080
// @BasePath /api/v1
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
	cfg, err := config.Load()
	if err != nil {
		logger.Fatalf("Failed to load configuration: %v", err)
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

	// Initialize services
	githubService := github.NewService(cfg.GitHubToken, dbStore, logger)
	apiHandler := api.NewHandler(githubService, logger)

	// Setup router with middleware
	router := mux.NewRouter()
	api.RegisterRoutes(router, apiHandler)
	corsHandler := cors.New(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"*"},
		AllowCredentials: true,
	}).Handler(router)

	// Create HTTP server
	server := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      corsHandler,
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

	// Start background sync with test repository
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go githubService.StartSync(ctx, cfg.SyncInterval, cfg.DefaultSyncRepo)

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

// retry retries a function up to a certain number of attempts with a delay between attempts
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