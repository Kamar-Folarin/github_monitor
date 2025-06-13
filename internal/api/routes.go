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
			repos.GET("/commits", h.GetRepositoryCommitsByOwnerRepo)
			repos.POST("/reset", h.ResetRepositorySyncByOwnerRepo)
			repos.GET("/sync-status", h.GetRepositorySyncStatusByOwnerRepo)
			repos.GET("/top-authors", h.GetRepositoryAuthorsByOwnerRepo)
		}

		// Current endpoints using repository ID format
		repositories := v1.Group("/repositories")
		{
			repositories.GET("", h.ListRepositories)
			repositories.POST("", h.AddRepository)
			repositories.GET("/:id", h.GetRepository)
			repositories.DELETE("/:id", h.DeleteRepository)
			repositories.POST("/:id/sync", h.SyncRepository)
			repositories.GET("/:id/sync", h.GetSyncStatus)
			repositories.POST("/:id/reset", h.ResetRepositorySync)
			repositories.POST("/:id/clear-sync", h.ForceClearSyncStatus)
			repositories.GET("/:id/commits", h.GetRepositoryCommits)
			repositories.GET("/:id/authors", h.GetRepositoryAuthors)
			repositories.PUT("/:id/sync-config", h.UpdateRepositorySyncConfig)
		}

		// Sync endpoints
		sync := v1.Group("/sync")
		{
			sync.GET("", h.GetAllSyncStatuses)
			sync.POST("", h.SyncAllRepositories)
		}
	}

	return r
}
