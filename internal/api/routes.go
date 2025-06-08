package api

import (
	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
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
// @description Type "Bearer" followed by a space and JWT token.

// SetupRouter configures the API routes
// @Summary Setup API routes
// @Description Configures all API endpoints and middleware
// @Tags setup
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "Router setup successful"
// @Router / [get]
func SetupRouter(h *Handler) *gin.Engine {
	r := gin.Default()

	// API documentation
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	// API v1 group
	v1 := r.Group("/api/v1")
	{
		// Legacy endpoints using owner/repo format
		repos := v1.Group("/repos/:owner/:repo")
		{
			// @Summary Get repository commits
			// @Description Get a list of commits for a repository with optional date filtering
			// @Tags repository
			// @Accept json
			// @Produce json
			// @Param owner path string true "Repository owner"
			// @Param repo path string true "Repository name"
			// @Param limit query int false "Number of commits to return" default(50)
			// @Param offset query int false "Number of commits to skip" default(0)
			// @Param since query string false "Filter commits since this date (RFC3339)" example("2024-03-20T00:00:00Z")
			// @Param until query string false "Filter commits until this date (RFC3339)" example("2024-03-21T00:00:00Z")
			// @Success 200 {object} CommitListResponse
			// @Failure 400 {object} ErrorResponse
			// @Failure 404 {object} ErrorResponse
			// @Failure 500 {object} ErrorResponse
			// @Router /repos/{owner}/{repo}/commits [get]
			repos.GET("/commits", h.GetRepositoryCommitsByOwnerRepo)

			// @Summary Reset repository sync
			// @Description Reset repository sync to start from a specific date
			// @Tags repository
			// @Accept json
			// @Produce json
			// @Param owner path string true "Repository owner"
			// @Param repo path string true "Repository name"
			// @Param request body object true "Reset request" SchemaExample({"since": "2024-03-20T00:00:00Z"})
			// @Success 202 {object} map[string]string "Reset status"
			// @Failure 400 {object} ErrorResponse
			// @Failure 404 {object} ErrorResponse
			// @Failure 500 {object} ErrorResponse
			// @Router /repos/{owner}/{repo}/reset [post]
			repos.POST("/reset", h.ResetRepositorySyncByOwnerRepo)

			// @Summary Get repository sync status
			// @Description Get the current sync status of a repository
			// @Tags repository
			// @Accept json
			// @Produce json
			// @Param owner path string true "Repository owner"
			// @Param repo path string true "Repository name"
			// @Success 200 {object} SyncStatus
			// @Failure 404 {object} ErrorResponse
			// @Failure 500 {object} ErrorResponse
			// @Router /repos/{owner}/{repo}/sync-status [get]
			repos.GET("/sync-status", h.GetRepositorySyncStatusByOwnerRepo)

			// @Summary Get repository commit authors
			// @Description Get statistics about commit authors with optional date filtering
			// @Tags repository
			// @Accept json
			// @Produce json
			// @Param owner path string true "Repository owner"
			// @Param repo path string true "Repository name"
			// @Param limit query int false "Number of authors to return" default(10)
			// @Param since query string false "Filter commits since this date (RFC3339)" example("2024-03-20T00:00:00Z")
			// @Param until query string false "Filter commits until this date (RFC3339)" example("2024-03-21T00:00:00Z")
			// @Success 200 {object} AuthorListResponse
			// @Failure 400 {object} ErrorResponse
			// @Failure 404 {object} ErrorResponse
			// @Failure 500 {object} ErrorResponse
			// @Router /repos/{owner}/{repo}/top-authors [get]
			repos.GET("/top-authors", h.GetRepositoryAuthorsByOwnerRepo)
		}

		// Current endpoints using repository ID format
		repositories := v1.Group("/repositories")
		{
			// @Summary List all monitored repositories
			// @Description Get a list of all repositories being monitored
			// @Tags repositories
			// @Accept json
			// @Produce json
			// @Success 200 {array} Repository
			// @Failure 500 {object} ErrorResponse
			// @Router /repositories [get]
			repositories.GET("", h.ListRepositories)

			// @Summary Add a new repository to monitor
			// @Description Start monitoring a new GitHub repository
			// @Tags repositories
			// @Accept json
			// @Produce json
			// @Param url query string true "Repository URL"
			// @Success 201 {object} Repository
			// @Failure 400 {object} ErrorResponse
			// @Failure 500 {object} ErrorResponse
			// @Router /repositories [post]
			repositories.POST("", h.AddRepository)

			// @Summary Get repository details
			// @Description Get detailed information about a specific repository
			// @Tags repositories
			// @Accept json
			// @Produce json
			// @Param id path int true "Repository ID"
			// @Success 200 {object} Repository
			// @Failure 404 {object} ErrorResponse
			// @Failure 500 {object} ErrorResponse
			// @Router /repositories/{id} [get]
			repositories.GET("/:id", h.GetRepository)

			// @Summary Delete a repository
			// @Description Stop monitoring a repository and remove it from the system
			// @Tags repositories
			// @Accept json
			// @Produce json
			// @Param id path int true "Repository ID"
			// @Success 204 "No Content"
			// @Failure 404 {object} ErrorResponse
			// @Failure 500 {object} ErrorResponse
			// @Router /repositories/{id} [delete]
			repositories.DELETE("/:id", h.DeleteRepository)

			// @Summary Sync a repository
			// @Description Trigger a manual sync of a repository
			// @Tags repositories
			// @Accept json
			// @Produce json
			// @Param id path int true "Repository ID"
			// @Success 202 {object} SyncStatus
			// @Failure 404 {object} ErrorResponse
			// @Failure 500 {object} ErrorResponse
			// @Router /repositories/{id}/sync [post]
			repositories.POST("/:id/sync", h.SyncRepository)

			// @Summary Get repository sync status
			// @Description Get the current sync status of a repository
			// @Tags repositories
			// @Accept json
			// @Produce json
			// @Param id path int true "Repository ID"
			// @Success 200 {object} SyncStatus
			// @Failure 404 {object} ErrorResponse
			// @Failure 500 {object} ErrorResponse
			// @Router /repositories/{id}/sync [get]
			repositories.GET("/:id/sync", h.GetSyncStatus)

			// @Summary Reset repository sync
			// @Description Reset repository sync to start from a specific date
			// @Tags repositories
			// @Accept json
			// @Produce json
			// @Param id path int true "Repository ID"
			// @Param request body object true "Reset request" SchemaExample({"since": "2024-03-20T00:00:00Z"})
			// @Success 202 {object} map[string]string "Reset status"
			// @Failure 400 {object} ErrorResponse
			// @Failure 404 {object} ErrorResponse
			// @Failure 500 {object} ErrorResponse
			// @Router /repositories/{id}/reset [post]
			repositories.POST("/:id/reset", h.ResetRepositorySync)

			// @Summary Force clear repository sync status
			// @Description Forcefully clear a stuck sync status for a repository
			// @Tags repositories
			// @Accept json
			// @Produce json
			// @Param id path int true "Repository ID"
			// @Success 200 {object} SyncStatus "Sync status cleared"
			// @Failure 404 {object} ErrorResponse "Repository not found"
			// @Failure 500 {object} ErrorResponse "Internal server error"
			// @Router /repositories/{id}/clear-sync [post]
			repositories.POST("/:id/clear-sync", h.ForceClearSyncStatus)

			// @Summary Get repository commits
			// @Description Get a list of commits for a repository with optional date filtering
			// @Tags repositories
			// @Accept json
			// @Produce json
			// @Param id path int true "Repository ID"
			// @Param limit query int false "Number of commits to return" default(100)
			// @Param offset query int false "Number of commits to skip" default(0)
			// @Param since query string false "Filter commits since this date (RFC3339)" example("2024-03-20T00:00:00Z")
			// @Param until query string false "Filter commits until this date (RFC3339)" example("2024-03-21T00:00:00Z")
			// @Success 200 {object} CommitListResponse
			// @Failure 400 {object} ErrorResponse
			// @Failure 404 {object} ErrorResponse
			// @Failure 500 {object} ErrorResponse
			// @Router /repositories/{id}/commits [get]
			repositories.GET("/:id/commits", h.GetRepositoryCommits)

			// @Summary Get repository commit authors
			// @Description Get statistics about commit authors with optional date filtering
			// @Tags repositories
			// @Accept json
			// @Produce json
			// @Param id path int true "Repository ID"
			// @Param limit query int false "Number of authors to return" default(10)
			// @Param since query string false "Filter commits since this date (RFC3339)" example("2024-03-20T00:00:00Z")
			// @Param until query string false "Filter commits until this date (RFC3339)" example("2024-03-21T00:00:00Z")
			// @Success 200 {object} AuthorListResponse
			// @Failure 400 {object} ErrorResponse
			// @Failure 404 {object} ErrorResponse
			// @Failure 500 {object} ErrorResponse
			// @Router /repositories/{id}/authors [get]
			repositories.GET("/:id/authors", h.GetRepositoryAuthors)

			// @Summary Update repository sync configuration
			// @Description Update the sync configuration for a repository
			// @Tags repositories
			// @Accept json
			// @Produce json
			// @Param id path int true "Repository ID"
			// @Param config body models.RepositorySyncConfig true "Sync configuration"
			// @Success 200 {object} models.Repository
			// @Failure 400 {object} ErrorResponse
			// @Failure 404 {object} ErrorResponse
			// @Failure 500 {object} ErrorResponse
			// @Router /repositories/{id}/sync-config [put]
			repositories.PUT("/:id/sync-config", h.UpdateRepositorySyncConfig)
		}

		// Sync endpoints
		sync := v1.Group("/sync")
		{
			// @Summary Get all sync statuses
			// @Description Get the sync status of all repositories
			// @Tags sync
			// @Accept json
			// @Produce json
			// @Success 200 {array} SyncStatus
			// @Failure 500 {object} ErrorResponse
			// @Router /sync [get]
			sync.GET("", h.GetAllSyncStatuses)

			// @Summary Sync all repositories
			// @Description Trigger a sync of all monitored repositories
			// @Tags sync
			// @Accept json
			// @Produce json
			// @Success 202 {array} SyncStatus
			// @Failure 500 {object} ErrorResponse
			// @Router /sync [post]
			sync.POST("", h.SyncAllRepositories)
		}
	}

	return r
}