package api

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func setupTestRoutes(t *testing.T) (*gin.Engine, *MockRepositoryService, *MockSyncService) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	mockRepoService := new(MockRepositoryService)
	mockSyncService := new(MockSyncService)
	mockCommitService := new(MockCommitService)
	mockStatusManager := new(MockStatusManager)
	logger := logrus.New()
	logger.SetOutput(bytes.NewBuffer(nil)) // Discard logs during tests

	handler := NewHandler(
		mockRepoService,
		mockCommitService,
		mockSyncService,
		mockStatusManager,
		logger,
	)

	// Register routes
	router.GET("/repositories", handler.ListRepositories)
	router.POST("/repositories", handler.AddRepository)
	router.GET("/repositories/:id", handler.GetRepository)
	router.DELETE("/repositories/:id", handler.DeleteRepository)
	router.POST("/repositories/:id/sync", handler.SyncRepository)

	return router, mockRepoService, mockSyncService
}

func TestRouteRegistration(t *testing.T) {
	router, _, _ := setupTestRoutes(t)

	tests := []struct {
		name           string
		method         string
		path           string
		expectedStatus int
	}{
		{
			name:           "list repositories",
			method:         "GET",
			path:           "/repositories",
			expectedStatus: http.StatusOK,
		},
		{
			name:           "add repository",
			method:         "POST",
			path:           "/repositories",
			expectedStatus: http.StatusBadRequest, // Expect 400 due to missing request body
		},
		{
			name:           "get repository",
			method:         "GET",
			path:           "/repositories/1",
			expectedStatus: http.StatusNotFound, // Expect 404 due to non-existent repository
		},
		{
			name:           "delete repository",
			method:         "DELETE",
			path:           "/repositories/1",
			expectedStatus: http.StatusNotFound, // Expect 404 due to non-existent repository
		},
		{
			name:           "sync repository",
			method:         "POST",
			path:           "/repositories/1/sync",
			expectedStatus: http.StatusNotFound, // Expect 404 due to non-existent repository
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest(tt.method, tt.path, nil)
			router.ServeHTTP(w, req)
			assert.Equal(t, tt.expectedStatus, w.Code)
		})
	}
}

func TestMiddlewareSetup(t *testing.T) {
	router, _, _ := setupTestRoutes(t)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/repositories", nil)
	router.ServeHTTP(w, req)

	// Test CORS headers
	assert.Equal(t, "*", w.Header().Get("Access-Control-Allow-Origin"))
	assert.Equal(t, "GET, POST, PUT, DELETE, OPTIONS", w.Header().Get("Access-Control-Allow-Methods"))
	assert.Equal(t, "Content-Type, Authorization", w.Header().Get("Access-Control-Allow-Headers"))
}

// MockCommitService is a mock implementation of github.CommitService
type MockCommitService struct {
	mock.Mock
}

// MockStatusManager is a mock implementation of github.StatusManager
type MockStatusManager struct {
	mock.Mock
}

// Add mock method implementations as needed for route tests
func (m *MockCommitService) GetCommits(ctx context.Context, repoURL string) ([]models.Commit, error) {
	args := m.Called(ctx, repoURL)
	return args.Get(0).([]models.Commit), args.Error(1)
}

func (m *MockStatusManager) GetSyncStatus(ctx context.Context) (map[string]string, error) {
	args := m.Called(ctx)
	return args.Get(0).(map[string]string), args.Error(1)
}

func (m *MockStatusManager) ClearSyncStatus(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
} 