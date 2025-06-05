package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"

	"github-monitor/internal/github"
)

type Handler struct {
	githubService *github.Service
	logger       *logrus.Logger
}

func NewHandler(githubService *github.Service, logger *logrus.Logger) *Handler {
	return &Handler{
		githubService: githubService,
		logger:       logger,
	}
}

func (h *Handler) GetTopCommitAuthors(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	owner := vars["owner"]
	repo := vars["repo"]

	limit, err := getIntQueryParam(r, "limit", 10)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid limit parameter")
		return
	}

	repoURL := fmt.Sprintf("https://github.com/%s/%s", owner, repo)
	authors, err := h.githubService.GetTopCommitAuthors(r.Context(), repoURL, limit)
	if err != nil {
		h.logger.Errorf("Failed to get top authors: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to get top authors")
		return
	}

	respondWithJSON(w, http.StatusOK, authors)
}

func (h *Handler) GetCommits(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	owner := vars["owner"]
	repo := vars["repo"]

	limit, err := getIntQueryParam(r, "limit", 50)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid limit parameter")
		return
	}

	offset, err := getIntQueryParam(r, "offset", 0)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid offset parameter")
		return
	}

	repoURL := fmt.Sprintf("https://github.com/%s/%s", owner, repo)
	commits, err := h.githubService.GetCommits(r.Context(), repoURL, limit, offset)
	if err != nil {
		h.logger.Errorf("Failed to get commits: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to get commits")
		return
	}

	respondWithJSON(w, http.StatusOK, commits)
}

func (h *Handler) ResetRepository(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	owner := vars["owner"]
	repo := vars["repo"]

	sinceParam := r.URL.Query().Get("since")
	since, err := time.Parse(time.RFC3339, sinceParam)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid since parameter (use RFC3339 format)")
		return
	}

	repoURL := fmt.Sprintf("https://github.com/%s/%s", owner, repo)
	if err := h.githubService.ResetRepository(r.Context(), repoURL, since); err != nil {
		h.logger.Errorf("Failed to reset repository: %v", err)
		respondWithError(w, http.StatusInternalServerError, "Failed to reset repository")
		return
	}

	respondWithJSON(w, http.StatusOK, map[string]string{"status": "success"})
}

func respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(payload)
}

func respondWithError(w http.ResponseWriter, code int, message string) {
	respondWithJSON(w, code, map[string]string{"error": message})
}

func getIntQueryParam(r *http.Request, param string, defaultValue int) (int, error) {
	value := r.URL.Query().Get(param)
	if value == "" {
		return defaultValue, nil
	}
	return strconv.Atoi(value)
}