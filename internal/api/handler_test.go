package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/Kamar-Folarin/github-monitor/internal/github"
	"github.com/Kamar-Folarin/github-monitor/internal/models"
)

// MockRepositoryService is a mock implementation of github.RepositoryService
type MockRepositoryService struct {
	mock.Mock
}

func (m *MockRepositoryService) ListRepositories(ctx context.Context) ([]models.Repository, error) {
	args := m.Called(ctx)
	return args.Get(0).([]models.Repository), args.Error(1)
}

func (m *MockRepositoryService) AddRepository(ctx context.Context, req github.AddRepositoryRequest) (*models.Repository, error) {
	args := m.Called(ctx, req)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Repository), args.Error(1)
}

func (m *MockRepositoryService) GetRepositoryByID(ctx context.Context, id uint) (*models.Repository, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.Repository), args.Error(1)
}

func (m *MockRepositoryService) DeleteRepository(ctx context.Context, repoURL string) error {
	args := m.Called(ctx, repoURL)
	return args.Error(0)
}

func (m *MockRepositoryService) DeleteRepositoryByID(ctx context.Context, id uint) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

// MockCommitService is a mock implementation of github.CommitService
type MockCommitService struct {
	mock.Mock
}

func (m *MockCommitService) GetCommits(ctx context.Context, repoURL string, limit, offset int) ([]*models.Commit, error) {
	args := m.Called(ctx, repoURL, limit, offset)
	return args.Get(0).([]*models.Commit), args.Error(1)
}

func (m *MockCommitService) GetCommitProgress() <-chan *models.BatchProgress {
	args := m.Called()
	return args.Get(0).(<-chan *models.BatchProgress)
}

// MockSyncService is a mock implementation of github.SyncService
type MockSyncService struct {
	mock.Mock
}

func (m *MockSyncService) StartSync(ctx context.Context, repoURL string) error {
	args := m.Called(ctx, repoURL)
	return args.Error(0)
}

func (m *MockSyncService) ForceClearSyncStatus(ctx context.Context, repoURL string) error {
	args := m.Called(ctx, repoURL)
	return args.Error(0)
}

func (m *MockSyncService) GetSyncStatus(ctx context.Context, repoURL string) (*models.SyncStatus, error) {
	args := m.Called(ctx, repoURL)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*models.SyncStatus), args.Error(1)
}

func (m *MockSyncService) ResetSync(ctx context.Context, repoURL string) error {
	args := m.Called(ctx, repoURL)
	return args.Error(0)
}

// MockStatusManager is a mock implementation of github.StatusManager
type MockStatusManager struct {
	mock.Mock
}

func (m *MockStatusManager) GetSyncStatus(ctx context.Context) (map[string]string, error) {
	args := m.Called(ctx)
	return args.Get(0).(map[string]string), args.Error(1)
}

func (m *MockStatusManager) ClearStatuses(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockStatusManager) DeleteStatus(ctx context.Context, repoURL string) error {
	args := m.Called(ctx, repoURL)
	return args.Error(0)
}

func (m *MockStatusManager) GetAllStatuses(ctx context.Context) (map[string]*models.SyncStatus, error) {
	args := m.Called(ctx)
	return args.Get(0).(map[string]*models.SyncStatus), args.Error(1)
}

func setupTestHandler() (*Handler, *MockRepositoryService, *MockSyncService) {
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

	return handler, mockRepoService, mockSyncService
}

func setupTestRouter(handler *Handler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	router := gin.New()
	router.GET("/repositories", handler.ListRepositories)
	router.POST("/repositories", handler.AddRepository)
	router.GET("/repositories/:id", handler.GetRepository)
	router.DELETE("/repositories/:id", handler.DeleteRepository)
	router.POST("/repositories/:id/sync", handler.SyncRepository)
	return router
}

func TestListRepositories(t *testing.T) {
	handler, mockRepoService, _ := setupTestHandler()
	router := setupTestRouter(handler)

	// Setup mock expectations
	expectedRepos := []models.Repository{
		{
			BaseModel: models.BaseModel{ID: 1},
			URL:       "https://github.com/owner1/repo1",
		},
		{
			BaseModel: models.BaseModel{ID: 2},
			URL:       "https://github.com/owner2/repo2",
		},
	}
	mockRepoService.On("ListRepositories", mock.Anything).Return(expectedRepos, nil)

	// Create test request
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/repositories", nil)
	router.ServeHTTP(w, req)

	// Assertions
	assert.Equal(t, http.StatusOK, w.Code)
	var response []models.Repository
	err := json.Unmarshal(w.Body.Bytes(), &response)
	assert.NoError(t, err)
	assert.Equal(t, expectedRepos, response)
	mockRepoService.AssertExpectations(t)
}

func TestAddRepository(t *testing.T) {
	handler, mockRepoService, _ := setupTestHandler()
	router := setupTestRouter(handler)

	// Test cases
	tests := []struct {
		name           string
		requestBody    AddRepositoryRequest
		mockResponse   *models.Repository
		mockError      error
		expectedStatus int
		expectedBody   interface{}
	}{
		{
			name: "successful repository addition",
			requestBody: AddRepositoryRequest{
				URL: "https://github.com/owner/repo",
			},
			mockResponse: &models.Repository{
				BaseModel: models.BaseModel{ID: 1},
				URL:       "https://github.com/owner/repo",
			},
			mockError:      nil,
			expectedStatus: http.StatusCreated,
			expectedBody: &models.Repository{
				BaseModel: models.BaseModel{ID: 1},
				URL:       "https://github.com/owner/repo",
			},
		},
		{
			name: "invalid request body",
			requestBody: AddRepositoryRequest{
				URL: "", // Empty URL should fail validation
			},
			mockResponse:   nil,
			mockError:      nil,
			expectedStatus: http.StatusBadRequest,
			expectedBody:   ErrorResponse{Error: "invalid request body"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mock expectations
			if tt.mockResponse != nil {
				mockRepoService.On("AddRepository", mock.Anything, mock.Anything).Return(tt.mockResponse, tt.mockError)
			}

			// Create test request
			body, _ := json.Marshal(tt.requestBody)
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/repositories", bytes.NewBuffer(body))
			req.Header.Set("Content-Type", "application/json")
			router.ServeHTTP(w, req)

			// Assertions
			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.expectedBody != nil {
				var response interface{}
				if tt.expectedStatus == http.StatusCreated {
					response = &models.Repository{}
				} else {
					response = &ErrorResponse{}
				}
				err := json.Unmarshal(w.Body.Bytes(), response)
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedBody, response)
			}
		})
	}
	mockRepoService.AssertExpectations(t)
}

func TestGetRepository(t *testing.T) {
	handler, mockRepoService, _ := setupTestHandler()
	router := setupTestRouter(handler)

	tests := []struct {
		name           string
		repoID         string
		mockResponse   *models.Repository
		mockError      error
		expectedStatus int
		expectedBody   interface{}
	}{
		{
			name:   "successful repository retrieval",
			repoID: "1",
			mockResponse: &models.Repository{
				BaseModel: models.BaseModel{ID: 1},
				URL:       "https://github.com/owner/repo",
			},
			mockError:      nil,
			expectedStatus: http.StatusOK,
			expectedBody: &models.Repository{
				BaseModel: models.BaseModel{ID: 1},
				URL:       "https://github.com/owner/repo",
			},
		},
		{
			name:           "repository not found",
			repoID:         "999",
			mockResponse:   nil,
			mockError:      errors.New("repository not found"),
			expectedStatus: http.StatusNotFound,
			expectedBody:   ErrorResponse{Error: "repository not found"},
		},
		{
			name:           "invalid repository ID",
			repoID:         "invalid",
			mockResponse:   nil,
			mockError:      nil,
			expectedStatus: http.StatusBadRequest,
			expectedBody:   ErrorResponse{Error: "invalid repository ID"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.mockResponse != nil {
				id, _ := strconv.ParseUint(tt.repoID, 10, 32)
				mockRepoService.On("GetRepositoryByID", mock.Anything, uint(id)).Return(tt.mockResponse, tt.mockError)
			}

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/repositories/"+tt.repoID, nil)
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.expectedBody != nil {
				var response interface{}
				if tt.expectedStatus == http.StatusOK {
					response = &models.Repository{}
				} else {
					response = &ErrorResponse{}
				}
				err := json.Unmarshal(w.Body.Bytes(), response)
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedBody, response)
			}
		})
	}
	mockRepoService.AssertExpectations(t)
}

func TestDeleteRepository(t *testing.T) {
	handler, mockRepoService, _ := setupTestHandler()
	router := setupTestRouter(handler)

	tests := []struct {
		name           string
		repoID         string
		mockError      error
		expectedStatus int
		expectedBody   interface{}
	}{
		{
			name:           "successful repository deletion",
			repoID:         "1",
			mockError:      nil,
			expectedStatus: http.StatusNoContent,
			expectedBody:   nil,
		},
		{
			name:           "repository not found",
			repoID:         "999",
			mockError:      errors.New("repository not found"),
			expectedStatus: http.StatusNotFound,
			expectedBody:   ErrorResponse{Error: "repository not found"},
		},
		{
			name:           "invalid repository ID",
			repoID:         "invalid",
			mockError:      nil,
			expectedStatus: http.StatusBadRequest,
			expectedBody:   ErrorResponse{Error: "invalid repository ID"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.repoID != "invalid" {
				mockRepoService.On("DeleteRepository", mock.Anything, tt.repoID).Return(tt.mockError)
			}

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("DELETE", "/repositories/"+tt.repoID, nil)
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.expectedBody != nil {
				var response ErrorResponse
				err := json.Unmarshal(w.Body.Bytes(), &response)
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedBody, response)
			}
		})
	}
	mockRepoService.AssertExpectations(t)
}

func TestSyncRepository(t *testing.T) {
	handler, mockRepoService, mockSyncService := setupTestHandler()
	router := setupTestRouter(handler)

	tests := []struct {
		name           string
		repoID         string
		mockRepo       *models.Repository
		mockRepoError  error
		mockSyncError  error
		expectedStatus int
		expectedBody   interface{}
	}{
		{
			name:   "successful sync start",
			repoID: "1",
			mockRepo: &models.Repository{
				BaseModel: models.BaseModel{ID: 1},
				URL:       "https://github.com/owner/repo",
			},
			mockRepoError:  nil,
			mockSyncError:  nil,
			expectedStatus: http.StatusAccepted,
			expectedBody:   gin.H{"status": "sync started"},
		},
		{
			name:           "repository not found",
			repoID:         "999",
			mockRepo:       nil,
			mockRepoError:  errors.New("repository not found"),
			mockSyncError:  nil,
			expectedStatus: http.StatusNotFound,
			expectedBody:   ErrorResponse{Error: "repository not found"},
		},
		{
			name:           "invalid repository ID",
			repoID:         "invalid",
			mockRepo:       nil,
			mockRepoError:  nil,
			mockSyncError:  nil,
			expectedStatus: http.StatusBadRequest,
			expectedBody:   ErrorResponse{Error: "invalid repository ID"},
		},
		{
			name:   "sync service error",
			repoID: "1",
			mockRepo: &models.Repository{
				BaseModel: models.BaseModel{ID: 1},
				URL:       "https://github.com/owner/repo",
			},
			mockRepoError:  nil,
			mockSyncError:  errors.New("sync failed"),
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   ErrorResponse{Error: "sync failed"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.repoID != "invalid" {
				id, _ := strconv.ParseUint(tt.repoID, 10, 32)
				mockRepoService.On("GetRepositoryByID", mock.Anything, uint(id)).Return(tt.mockRepo, tt.mockRepoError)
				if tt.mockRepo != nil {
					mockSyncService.On("StartSync", mock.Anything, tt.mockRepo.URL).Return(tt.mockSyncError)
				}
			}

			w := httptest.NewRecorder()
			req, _ := http.NewRequest("POST", "/repositories/"+tt.repoID+"/sync", nil)
			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)
			if tt.expectedBody != nil {
				var response interface{}
				if tt.expectedStatus == http.StatusAccepted {
					response = &gin.H{}
				} else {
					response = &ErrorResponse{}
				}
				err := json.Unmarshal(w.Body.Bytes(), response)
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedBody, response)
			}
		})
	}
	mockRepoService.AssertExpectations(t)
	mockSyncService.AssertExpectations(t)
} 