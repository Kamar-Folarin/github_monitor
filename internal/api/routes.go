package api

import (
	"github.com/gorilla/mux"
	httpSwagger "github.com/swaggo/http-swagger"
)

func RegisterRoutes(router *mux.Router, handler *Handler) {
	apiRouter := router.PathPrefix("/api/v1").Subrouter()

	// Repository endpoints
	apiRouter.HandleFunc("/repos/{owner}/{repo}/top-authors", handler.GetTopCommitAuthors).Methods("GET")
	apiRouter.HandleFunc("/repos/{owner}/{repo}/commits", handler.GetCommits).Methods("GET")
	apiRouter.HandleFunc("/repos/{owner}/{repo}/reset", handler.ResetRepository).Methods("POST")

	// Swagger documentation
	router.PathPrefix("/swagger/").Handler(httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"), //The url pointing to API definition
	))
}