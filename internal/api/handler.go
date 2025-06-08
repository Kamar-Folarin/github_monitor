package api

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"github.com/Kamar-Folarin/github-monitor/internal/errors"
	"github.com/Kamar-Folarin/github-monitor/internal/github"
	"github.com/Kamar-Folarin/github-monitor/internal/models"
)

// AddRepositoryRequest represents the request to add a new repository
// @Description Request to add a new repository with optional sync configuration
type AddRepositoryRequest struct {
	// URL is the GitHub repository URL
	// @Description The full URL of the GitHub repository to monitor
	// @Example https://github.com/owner/repo
	// @required
	URL string `json:"url" binding:"required"`

	// SyncConfig is the optional repository-specific sync configuration
	// @Description Optional configuration for repository-specific sync settings
	SyncConfig *models.RepositorySyncConfig `json:"sync_config,omitempty"`
}

// Handler handles HTTP requests
type Handler struct {
	repoService   github.RepositoryService
	commitService github.CommitService
	syncService   github.SyncService
	statusManager github.StatusManager
	logger        *logrus.Logger
}

// NewHandler creates a new API handler
func NewHandler(
	repoService github.RepositoryService,
	commitService github.CommitService,
	syncService github.SyncService,
	statusManager github.StatusManager,
	logger *logrus.Logger,
) *Handler {
	return &Handler{
		repoService:   repoService,
		commitService: commitService,
		syncService:   syncService,
		statusManager: statusManager,
		logger:        logger,
	}
}

// ListRepositories handles GET /repositories
// @Summary List all monitored repositories
// @Description Get a list of all repositories being monitored
// @Tags repositories
// @Accept json
// @Produce json
// @Success 200 {array} Repository "List of repositories"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /repositories [get]
func (h *Handler) ListRepositories(c *gin.Context) {
	repos, err := h.repoService.ListRepositories(c.Request.Context())
	if err != nil {
		h.logger.Errorf("Failed to list repositories: %v", err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, repos)
}

// AddRepository handles POST /repositories
// @Summary Add a new repository
// @Description Add a new repository to monitor with optional sync configuration
// @Tags repositories
// @Accept json
// @Produce json
// @Param request body AddRepositoryRequest true "Add repository request"
// @Success 201 {object} models.Repository "Repository added successfully"
// @Failure 400 {object} ErrorResponse "Invalid request"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /repositories [post]
func (h *Handler) AddRepository(c *gin.Context) {
	var req github.AddRepositoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request body"})
		return
	}

	repo, err := h.repoService.AddRepository(c.Request.Context(), req)
	if err != nil {
		if errors.IsValidationError(err) {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, repo)
}

// GetRepository handles GET /repositories/:id
// @Summary Get repository details
// @Description Get detailed information about a specific repository
// @Tags repositories
// @Accept json
// @Produce json
// @Param id path int true "Repository ID"
// @Success 200 {object} Repository "Repository details"
// @Failure 404 {object} ErrorResponse "Repository not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /repositories/{id} [get]
func (h *Handler) GetRepository(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid repository ID"})
		return
	}

	repo, err := h.repoService.GetRepositoryByID(c.Request.Context(), uint(id))
	if err != nil {
		if errors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, ErrorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	c.JSON(http.StatusOK, repo)
}

// DeleteRepository handles DELETE /repositories/:id
// @Summary Delete a repository
// @Description Stop monitoring a repository and remove it from the system
// @Tags repositories
// @Accept json
// @Produce json
// @Param id path int true "Repository ID"
// @Success 204 "No Content"
// @Failure 404 {object} ErrorResponse "Repository not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /repositories/{id} [delete]
func (h *Handler) DeleteRepository(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid repository ID"})
		return
	}

	if err := h.repoService.DeleteRepositoryByID(c.Request.Context(), uint(id)); err != nil {
		if errors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, ErrorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}
	c.Status(http.StatusNoContent)
}

// SyncRepository handles POST /repositories/:id/sync
// @Summary Sync a repository
// @Description Trigger a manual sync of a repository
// @Tags repositories
// @Accept json
// @Produce json
// @Param id path int true "Repository ID"
// @Success 202 {object} SyncStatus "Sync started"
// @Failure 404 {object} ErrorResponse "Repository not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /repositories/{id}/sync [post]
func (h *Handler) SyncRepository(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid repository ID"})
		return
	}

	repo, err := h.repoService.GetRepositoryByID(c.Request.Context(), uint(id))
	if err != nil {
		if errors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, ErrorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	if err := h.syncService.StartSync(c.Request.Context(), repo.URL); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{"status": "sync started"})
}

// GetSyncStatus handles GET /repositories/:id/sync
// @Summary Get repository sync status
// @Description Get the current sync status of a repository
// @Tags repositories
// @Accept json
// @Produce json
// @Param id path int true "Repository ID"
// @Success 200 {object} SyncStatus "Sync status"
// @Failure 404 {object} ErrorResponse "Repository not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /repositories/{id}/sync [get]
func (h *Handler) GetSyncStatus(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid repository ID"})
		return
	}

	repo, err := h.repoService.GetRepositoryByID(c.Request.Context(), uint(id))
	if err != nil {
		if errors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, ErrorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	status, err := h.syncService.GetSyncStatus(c.Request.Context(), repo.URL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, status)
}

// ResetRepositorySync handles POST /repositories/:id/reset
// @Summary Reset repository sync
// @Description Reset repository sync to start from a specific date
// @Tags repositories
// @Accept json
// @Produce json
// @Param id path int true "Repository ID"
// @Param request body object true "Reset request" SchemaExample({"since": "2024-03-20T00:00:00Z"})
// @Success 202 {object} map[string]string "Reset status"
// @Failure 400 {object} ErrorResponse "Invalid request"
// @Failure 404 {object} ErrorResponse "Repository not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /repositories/{id}/reset [post]
func (h *Handler) ResetRepositorySync(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid repository ID"})
		return
	}

	var req struct {
		Since string `json:"since" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request body: since date is required"})
		return
	}

	since, err := time.Parse(time.RFC3339, req.Since)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid date format, use RFC3339 (e.g. 2024-03-20T00:00:00Z)"})
		return
	}

	repo, err := h.repoService.GetRepositoryByID(c.Request.Context(), uint(id))
	if err != nil {
		if errors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, ErrorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	if err := h.syncService.ResetSync(c.Request.Context(), repo.URL, since); err != nil {
		h.logger.WithFields(logrus.Fields{
			"repository": repo.URL,
			"since":     since,
			"error":     err,
		}).Error("Failed to reset repository sync")
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to reset repository sync"})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"status": "sync reset initiated",
		"since":  since.Format(time.RFC3339),
	})
}

// GetRepositoryCommits handles GET /repositories/:id/commits
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
// @Success 200 {object} CommitListResponse "List of commits"
// @Failure 400 {object} ErrorResponse "Invalid request"
// @Failure 404 {object} ErrorResponse "Repository not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /repositories/{id}/commits [get]
func (h *Handler) GetRepositoryCommits(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid repository ID"})
		return
	}

	repo, err := h.repoService.GetRepositoryByID(c.Request.Context(), uint(id))
	if err != nil {
		if errors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, ErrorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	// Get query parameters
	limit := 100 // default limit
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	offset := 0 // default offset
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	// Parse date filters
	var since, until *time.Time
	if sinceStr := c.Query("since"); sinceStr != "" {
		if t, err := time.Parse(time.RFC3339, sinceStr); err == nil {
			since = &t
		} else {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid since date format, use RFC3339"})
			return
		}
	}
	if untilStr := c.Query("until"); untilStr != "" {
		if t, err := time.Parse(time.RFC3339, untilStr); err == nil {
			until = &t
		} else {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid until date format, use RFC3339"})
			return
		}
	}

	// Get commits with pagination
	commits, total, err := h.commitService.GetCommitsWithPagination(c.Request.Context(), repo.URL, limit, offset, since, until)
	if err != nil {
		h.logger.WithFields(logrus.Fields{
			"repository": repo.URL,
			"limit":     limit,
			"offset":    offset,
			"error":     err,
		}).Error("Failed to get repository commits")
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to get repository commits"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": commits,
		"pagination": gin.H{
			"total":  total,
			"limit":  limit,
			"offset": offset,
		},
	})
}

// GetRepositoryAuthors handles GET /repositories/:id/authors
// @Summary Get repository commit authors
// @Description Get statistics about commit authors with optional date filtering
// @Tags repositories
// @Accept json
// @Produce json
// @Param id path int true "Repository ID"
// @Param limit query int false "Number of authors to return" default(10)
// @Param since query string false "Filter commits since this date (RFC3339)" example("2024-03-20T00:00:00Z")
// @Param until query string false "Filter commits until this date (RFC3339)" example("2024-03-21T00:00:00Z")
// @Success 200 {object} AuthorListResponse "List of authors"
// @Failure 400 {object} ErrorResponse "Invalid request"
// @Failure 404 {object} ErrorResponse "Repository not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /repositories/{id}/authors [get]
func (h *Handler) GetRepositoryAuthors(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "Invalid repository ID"})
		return
	}

	repo, err := h.repoService.GetRepositoryByID(c.Request.Context(), uint(id))
	if err != nil {
		h.logger.WithFields(logrus.Fields{
			"repository_id": id,
			"error":        err,
		}).Error("Failed to get repository")
		if errors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, ErrorResponse{Error: "Repository not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to get repository"})
		return
	}

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if limit <= 0 {
		limit = 10
	}

	// Parse date filters
	var since, until *time.Time
	if sinceStr := c.Query("since"); sinceStr != "" {
		if t, err := time.Parse(time.RFC3339, sinceStr); err == nil {
			since = &t
		} else {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid since date format, use RFC3339"})
			return
		}
	}
	if untilStr := c.Query("until"); untilStr != "" {
		if t, err := time.Parse(time.RFC3339, untilStr); err == nil {
			until = &t
		} else {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid until date format, use RFC3339"})
			return
		}
	}

	authors, err := h.commitService.GetTopCommitAuthors(c.Request.Context(), repo.URL, limit, since, until)
	if err != nil {
		h.logger.WithFields(logrus.Fields{
			"repository": repo.URL,
			"limit":     limit,
			"error":     err,
		}).Error("Failed to get top authors")
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to get top authors"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": authors,
		"metadata": gin.H{
			"repository": repo.URL,
			"limit":     limit,
			"since":     since,
			"until":     until,
		},
	})
}

// GetAllSyncStatuses handles GET /sync
// @Summary Get all sync statuses
// @Description Get the sync status of all repositories
// @Tags sync
// @Accept json
// @Produce json
// @Success 200 {array} SyncStatus "List of sync statuses"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /sync [get]
func (h *Handler) GetAllSyncStatuses(c *gin.Context) {
	statuses, err := h.statusManager.ListStatuses(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, statuses)
}

// SyncAllRepositories handles POST /sync
// @Summary Sync all repositories
// @Description Trigger a sync of all monitored repositories
// @Tags sync
// @Accept json
// @Produce json
// @Success 202 {array} SyncStatus "Sync started for all repositories"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /sync [post]
func (h *Handler) SyncAllRepositories(c *gin.Context) {
	repos, err := h.repoService.ListRepositories(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	var (
		syncedRepos    []string
		disabledRepos  []string
		failedRepos    []string
	)

	for _, repo := range repos {
		if !repo.SyncConfig.Enabled {
			disabledRepos = append(disabledRepos, repo.URL)
			continue
		}

		if err := h.syncService.StartSync(c.Request.Context(), repo.URL); err != nil {
			h.logger.WithFields(logrus.Fields{
				"repository": repo.URL,
				"error":     err,
			}).Error("Failed to start sync for repository")
			failedRepos = append(failedRepos, repo.URL)
			continue
		}
		syncedRepos = append(syncedRepos, repo.URL)
	}

	// Log summary
	h.logger.WithFields(logrus.Fields{
		"total_repos":     len(repos),
		"synced_repos":    len(syncedRepos),
		"disabled_repos":  len(disabledRepos),
		"failed_repos":    len(failedRepos),
		"synced":          syncedRepos,
		"disabled":        disabledRepos,
		"failed":          failedRepos,
	}).Info("Sync all repositories completed")

	c.JSON(http.StatusAccepted, gin.H{
		"status": "sync initiated",
		"summary": gin.H{
			"total_repos":    len(repos),
			"synced_repos":   len(syncedRepos),
			"disabled_repos": len(disabledRepos),
			"failed_repos":   len(failedRepos),
		},
		"details": gin.H{
			"synced":   syncedRepos,
			"disabled": disabledRepos,
			"failed":   failedRepos,
		},
	})
}

// GetRepositoryCommitsByOwnerRepo handles GET /repos/:owner/:repo/commits
// @Summary Get repository commits
// @Description Get a list of commits for a repository with optional date filtering using owner/repo format
// @Tags repository
// @Accept json
// @Produce json
// @Param owner path string true "Repository owner"
// @Param repo path string true "Repository name"
// @Param limit query int false "Number of commits to return" default(50)
// @Param offset query int false "Number of commits to skip" default(0)
// @Param since query string false "Filter commits since this date (RFC3339)" example("2024-03-20T00:00:00Z")
// @Param until query string false "Filter commits until this date (RFC3339)" example("2024-03-21T00:00:00Z")
// @Success 200 {object} CommitListResponse "List of commits"
// @Failure 400 {object} ErrorResponse "Invalid request"
// @Failure 404 {object} ErrorResponse "Repository not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /repos/{owner}/{repo}/commits [get]
func (h *Handler) GetRepositoryCommitsByOwnerRepo(c *gin.Context) {
	owner := c.Param("owner")
	repo := c.Param("repo")
	repoURL := fmt.Sprintf("https://github.com/%s/%s", owner, repo)

	// Get query parameters
	limit := 50 // default limit for legacy endpoint
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	offset := 0 // default offset
	if offsetStr := c.Query("offset"); offsetStr != "" {
		if o, err := strconv.Atoi(offsetStr); err == nil && o >= 0 {
			offset = o
		}
	}

	// Parse date filters
	var since, until *time.Time
	if sinceStr := c.Query("since"); sinceStr != "" {
		if t, err := time.Parse(time.RFC3339, sinceStr); err == nil {
			since = &t
		} else {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid since date format, use RFC3339"})
			return
		}
	}
	if untilStr := c.Query("until"); untilStr != "" {
		if t, err := time.Parse(time.RFC3339, untilStr); err == nil {
			until = &t
		} else {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid until date format, use RFC3339"})
			return
		}
	}

	// Get commits with pagination
	commits, total, err := h.commitService.GetCommitsWithPagination(c.Request.Context(), repoURL, limit, offset, since, until)
	if err != nil {
		h.logger.WithFields(logrus.Fields{
			"repository": repoURL,
			"limit":     limit,
			"offset":    offset,
			"error":     err,
		}).Error("Failed to get repository commits")
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to get repository commits"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": commits,
		"pagination": gin.H{
			"total":  total,
			"limit":  limit,
			"offset": offset,
		},
	})
}

// ResetRepositorySyncByOwnerRepo handles POST /repos/:owner/:repo/reset
// @Summary Reset repository sync
// @Description Reset repository sync to start from a specific date using owner/repo format
// @Tags repository
// @Accept json
// @Produce json
// @Param owner path string true "Repository owner"
// @Param repo path string true "Repository name"
// @Param request body object true "Reset request" SchemaExample({"since": "2024-03-20T00:00:00Z"})
// @Success 202 {object} map[string]string "Reset status"
// @Failure 400 {object} ErrorResponse "Invalid request"
// @Failure 404 {object} ErrorResponse "Repository not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /repos/{owner}/{repo}/reset [post]
func (h *Handler) ResetRepositorySyncByOwnerRepo(c *gin.Context) {
	owner := c.Param("owner")
	repo := c.Param("repo")
	repoURL := fmt.Sprintf("https://github.com/%s/%s", owner, repo)

	// Check if repository exists
	existingRepo, err := h.repoService.GetRepository(c.Request.Context(), repoURL)
	if err != nil {
		if errors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, ErrorResponse{Error: "Repository not found"})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to check repository existence"})
		return
	}

	// Check current sync status
	status, err := h.syncService.GetSyncStatus(c.Request.Context(), repoURL)
	if err != nil && !errors.IsNotFound(err) {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to get sync status"})
		return
	}
	if status != nil && status.IsSyncing {
		c.JSON(http.StatusConflict, ErrorResponse{Error: "Repository sync is currently in progress"})
		return
	}

	var req struct {
		Since string `json:"since" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid request body: since date is required"})
		return
	}

	since, err := time.Parse(time.RFC3339, req.Since)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid date format, use RFC3339 (e.g. 2024-03-20T00:00:00Z)"})
		return
	}

	// Validate since date is not in the future
	if since.After(time.Now()) {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "since date cannot be in the future"})
		return
	}

	if err := h.syncService.ResetSync(c.Request.Context(), repoURL, since); err != nil {
		h.logger.WithFields(logrus.Fields{
			"repository": repoURL,
			"since":     since,
			"error":     err,
		}).Error("Failed to reset repository sync")
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to reset repository sync"})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"status": "sync reset initiated",
		"since":  since.Format(time.RFC3339),
		"repository": existingRepo.URL,
	})
}

// GetRepositorySyncStatusByOwnerRepo handles GET /repos/:owner/:repo/sync-status
// @Summary Get repository sync status
// @Description Get the current sync status of a repository using owner/repo format
// @Tags repository
// @Accept json
// @Produce json
// @Param owner path string true "Repository owner"
// @Param repo path string true "Repository name"
// @Success 200 {object} SyncStatus "Sync status"
// @Failure 404 {object} ErrorResponse "Repository not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /repos/{owner}/{repo}/sync-status [get]
func (h *Handler) GetRepositorySyncStatusByOwnerRepo(c *gin.Context) {
	owner := c.Param("owner")
	repo := c.Param("repo")
	repoURL := fmt.Sprintf("https://github.com/%s/%s", owner, repo)

	status, err := h.syncService.GetSyncStatus(c.Request.Context(), repoURL)
	if err != nil {
		h.logger.WithFields(logrus.Fields{
			"repository": repoURL,
			"error":     err,
		}).Error("Failed to get repository sync status")
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to get repository sync status"})
		return
	}

	c.JSON(http.StatusOK, status)
}

// GetRepositoryAuthorsByOwnerRepo handles GET /repos/:owner/:repo/top-authors
// @Summary Get repository commit authors
// @Description Get statistics about commit authors with optional date filtering using owner/repo format
// @Tags repository
// @Accept json
// @Produce json
// @Param owner path string true "Repository owner"
// @Param repo path string true "Repository name"
// @Param limit query int false "Number of authors to return" default(10)
// @Param since query string false "Filter commits since this date (RFC3339)" example("2024-03-20T00:00:00Z")
// @Param until query string false "Filter commits until this date (RFC3339)" example("2024-03-21T00:00:00Z")
// @Success 200 {object} AuthorListResponse "List of authors"
// @Failure 400 {object} ErrorResponse "Invalid request"
// @Failure 404 {object} ErrorResponse "Repository not found"
// @Failure 500 {object} ErrorResponse "Internal server error"
// @Router /repos/{owner}/{repo}/top-authors [get]
func (h *Handler) GetRepositoryAuthorsByOwnerRepo(c *gin.Context) {
	owner := c.Param("owner")
	repo := c.Param("repo")
	repoURL := fmt.Sprintf("https://github.com/%s/%s", owner, repo)

	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	if limit <= 0 {
		limit = 10
	}

	// Parse date filters
	var since, until *time.Time
	if sinceStr := c.Query("since"); sinceStr != "" {
		if t, err := time.Parse(time.RFC3339, sinceStr); err == nil {
			since = &t
		} else {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid since date format, use RFC3339"})
			return
		}
	}
	if untilStr := c.Query("until"); untilStr != "" {
		if t, err := time.Parse(time.RFC3339, untilStr); err == nil {
			until = &t
		} else {
			c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid until date format, use RFC3339"})
			return
		}
	}

	authors, err := h.commitService.GetTopCommitAuthors(c.Request.Context(), repoURL, limit, since, until)
	if err != nil {
		h.logger.WithFields(logrus.Fields{
			"repository": repoURL,
			"limit":     limit,
			"error":     err,
		}).Error("Failed to get top authors")
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to get top authors"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": authors,
		"metadata": gin.H{
			"repository": repoURL,
			"limit":     limit,
			"since":     since,
			"until":     until,
		},
	})
}

// UpdateRepositorySyncConfig handles PUT /repositories/:id/sync-config
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
func (h *Handler) UpdateRepositorySyncConfig(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid repository ID"})
		return
	}

	var config models.RepositorySyncConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid sync configuration"})
		return
	}

	repo, err := h.repoService.GetRepositoryByID(c.Request.Context(), uint(id))
	if err != nil {
		if errors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, ErrorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	if err := h.repoService.UpdateRepositorySyncConfig(c.Request.Context(), repo.URL, config); err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	// If sync was disabled, stop any ongoing sync
	if !config.Enabled {
		if err := h.syncService.StopSync(c.Request.Context(), repo.URL); err != nil {
			// Log error but don't fail the request
			// TODO: Add proper logging
		}
	}

	// Get updated repository
	updatedRepo, err := h.repoService.GetRepositoryByID(c.Request.Context(), uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	c.JSON(http.StatusOK, updatedRepo)
}

// ForceClearSyncStatus handles POST /repositories/:id/clear-sync
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
func (h *Handler) ForceClearSyncStatus(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{Error: "invalid repository ID"})
		return
	}

	repo, err := h.repoService.GetRepositoryByID(c.Request.Context(), uint(id))
	if err != nil {
		if errors.IsNotFound(err) {
			c.JSON(http.StatusNotFound, ErrorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: err.Error()})
		return
	}

	if err := h.syncService.ForceClearSyncStatus(c.Request.Context(), repo.URL); err != nil {
		h.logger.WithFields(logrus.Fields{
			"repository": repo.URL,
			"error":     err,
		}).Error("Failed to force clear sync status")
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to clear sync status"})
		return
	}

	// Get updated status
	status, err := h.syncService.GetSyncStatus(c.Request.Context(), repo.URL)
	if err != nil {
		c.JSON(http.StatusInternalServerError, ErrorResponse{Error: "Failed to get updated sync status"})
		return
	}

	c.JSON(http.StatusOK, status)
}
