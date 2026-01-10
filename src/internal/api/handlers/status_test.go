package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/eduardo/nfse-nacional/internal/domain/emission"
	"github.com/eduardo/nfse-nacional/internal/infrastructure/mongodb"
)

// MockEmissionRepository is a mock implementation of the EmissionRepositoryReader interface.
type MockEmissionRepository struct {
	mock.Mock
}

// FindByRequestID mocks the FindByRequestID method.
func (m *MockEmissionRepository) FindByRequestID(ctx context.Context, requestID string) (*mongodb.EmissionRequest, error) {
	args := m.Called(ctx, requestID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*mongodb.EmissionRequest), args.Error(1)
}

// FindByAPIKeyID mocks the FindByAPIKeyID method.
func (m *MockEmissionRepository) FindByAPIKeyID(ctx context.Context, apiKeyID primitive.ObjectID, params mongodb.PaginationParams) (*mongodb.PaginatedResult, error) {
	args := m.Called(ctx, apiKeyID, params)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*mongodb.PaginatedResult), args.Error(1)
}

// setAPIKeyInContext sets the API key in the Gin context for testing.
func setAPIKeyInContext(c *gin.Context, apiKey *mongodb.APIKey) {
	c.Set(apiKeyContextKey, apiKey)
}

// createTestAPIKey creates a test API key with the given ID.
func createTestAPIKey(id primitive.ObjectID) *mongodb.APIKey {
	return &mongodb.APIKey{
		ID:             id,
		KeyPrefix:      "test1234",
		IntegratorName: "Test Integrator",
		Environment:    "homologation",
		Active:         true,
		RateLimit: mongodb.RateLimitConfig{
			RequestsPerMinute: 100,
			Burst:             20,
		},
	}
}

// createTestEmissionRequest creates a test emission request.
func createTestEmissionRequest(requestID string, apiKeyID primitive.ObjectID, status string) *mongodb.EmissionRequest {
	now := time.Now().UTC()
	req := &mongodb.EmissionRequest{
		ID:        primitive.NewObjectID(),
		RequestID: requestID,
		APIKeyID:  apiKeyID,
		Status:    status,
		CreatedAt: now,
		UpdatedAt: now,
	}

	if status == emission.StatusSuccess {
		processedAt := now.Add(time.Second)
		req.ProcessedAt = &processedAt
		req.Result = &mongodb.EmissionResult{
			NFSeAccessKey: "NFSe35503081234567800019900001000000000000001",
			NFSeNumber:    "000001",
		}
	}

	if status == emission.StatusFailed {
		processedAt := now.Add(time.Second)
		req.ProcessedAt = &processedAt
		req.Rejection = &mongodb.RejectionInfo{
			Code:           "GOVERNMENT_REJECTION",
			Message:        "Invalid CNPJ",
			GovernmentCode: "E123",
			Details:        "The CNPJ provided is not registered",
		}
	}

	return req
}

func init() {
	gin.SetMode(gin.TestMode)
}

// TestNewStatusHandler tests the NewStatusHandler constructor.
func TestNewStatusHandler(t *testing.T) {
	t.Run("creates handler with all config options", func(t *testing.T) {
		mockRepo := &MockEmissionRepository{}
		logger := log.New(os.Stdout, "test: ", log.LstdFlags)

		handler := NewStatusHandler(StatusHandlerConfig{
			EmissionRepo: mockRepo,
			BaseURL:      "https://api.example.com",
			Logger:       logger,
		})

		assert.NotNil(t, handler)
		assert.Equal(t, "https://api.example.com", handler.baseURL)
		assert.Equal(t, logger, handler.logger)
		assert.Equal(t, mockRepo, handler.emissionRepo)
	})

	t.Run("creates handler with minimal config", func(t *testing.T) {
		handler := NewStatusHandler(StatusHandlerConfig{})

		assert.NotNil(t, handler)
		assert.Empty(t, handler.baseURL)
		assert.Nil(t, handler.logger)
		assert.Nil(t, handler.emissionRepo)
	})
}

// TestStatusHandler_Get tests the Get method of StatusHandler.
func TestStatusHandler_Get(t *testing.T) {
	testAPIKeyID := primitive.NewObjectID()
	otherAPIKeyID := primitive.NewObjectID()

	tests := []struct {
		name           string
		requestID      string
		apiKey         *mongodb.APIKey
		mockSetup      func(*MockEmissionRepository)
		expectedStatus int
		expectedType   string
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:      "valid request ID returns status",
			requestID: "req-123",
			apiKey:    createTestAPIKey(testAPIKeyID),
			mockSetup: func(m *MockEmissionRepository) {
				m.On("FindByRequestID", mock.Anything, "req-123").Return(
					createTestEmissionRequest("req-123", testAPIKeyID, emission.StatusPending),
					nil,
				)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var resp emission.StatusResponse
				err := json.Unmarshal(w.Body.Bytes(), &resp)
				require.NoError(t, err)
				assert.Equal(t, "req-123", resp.RequestID)
				assert.Equal(t, emission.StatusPending, resp.Status)
				assert.Nil(t, resp.Result)
				assert.Nil(t, resp.Error)
			},
		},
		{
			name:           "missing request ID returns 400",
			requestID:      "",
			apiKey:         createTestAPIKey(testAPIKeyID),
			mockSetup:      func(m *MockEmissionRepository) {},
			expectedStatus: http.StatusBadRequest,
			expectedType:   "application/problem+json",
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var problem ProblemDetails
				err := json.Unmarshal(w.Body.Bytes(), &problem)
				require.NoError(t, err)
				assert.Equal(t, http.StatusBadRequest, problem.Status)
				assert.Contains(t, problem.Detail, "Request ID is required")
			},
		},
		{
			name:      "request not found returns 404",
			requestID: "non-existent",
			apiKey:    createTestAPIKey(testAPIKeyID),
			mockSetup: func(m *MockEmissionRepository) {
				m.On("FindByRequestID", mock.Anything, "non-existent").Return(
					nil,
					mongodb.ErrEmissionRequestNotFound,
				)
			},
			expectedStatus: http.StatusNotFound,
			expectedType:   "application/problem+json",
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var problem ProblemDetails
				err := json.Unmarshal(w.Body.Bytes(), &problem)
				require.NoError(t, err)
				assert.Equal(t, http.StatusNotFound, problem.Status)
				assert.Contains(t, problem.Detail, "not found")
			},
		},
		{
			name:      "unauthorized (different API key) returns 404",
			requestID: "req-456",
			apiKey:    createTestAPIKey(testAPIKeyID),
			mockSetup: func(m *MockEmissionRepository) {
				// Return a request owned by a different API key
				m.On("FindByRequestID", mock.Anything, "req-456").Return(
					createTestEmissionRequest("req-456", otherAPIKeyID, emission.StatusPending),
					nil,
				)
			},
			expectedStatus: http.StatusNotFound,
			expectedType:   "application/problem+json",
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				// Should return 404 to prevent information leakage
				var problem ProblemDetails
				err := json.Unmarshal(w.Body.Bytes(), &problem)
				require.NoError(t, err)
				assert.Equal(t, http.StatusNotFound, problem.Status)
				assert.Contains(t, problem.Detail, "not found")
			},
		},
		{
			name:      "success status includes result with NFSeAccessKey, NFSeNumber, NFSeXMLURL",
			requestID: "req-success",
			apiKey:    createTestAPIKey(testAPIKeyID),
			mockSetup: func(m *MockEmissionRepository) {
				m.On("FindByRequestID", mock.Anything, "req-success").Return(
					createTestEmissionRequest("req-success", testAPIKeyID, emission.StatusSuccess),
					nil,
				)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var resp emission.StatusResponse
				err := json.Unmarshal(w.Body.Bytes(), &resp)
				require.NoError(t, err)
				assert.Equal(t, "req-success", resp.RequestID)
				assert.Equal(t, emission.StatusSuccess, resp.Status)
				require.NotNil(t, resp.Result)
				assert.Equal(t, "NFSe35503081234567800019900001000000000000001", resp.Result.NFSeAccessKey)
				assert.Equal(t, "000001", resp.Result.NFSeNumber)
				assert.NotEmpty(t, resp.Result.NFSeXMLURL)
				assert.Contains(t, resp.Result.NFSeXMLURL, "/v1/nfse/NFSe35503081234567800019900001000000000000001")
				assert.Nil(t, resp.Error)
				assert.NotNil(t, resp.ProcessedAt)
			},
		},
		{
			name:      "failed status includes error details",
			requestID: "req-failed",
			apiKey:    createTestAPIKey(testAPIKeyID),
			mockSetup: func(m *MockEmissionRepository) {
				m.On("FindByRequestID", mock.Anything, "req-failed").Return(
					createTestEmissionRequest("req-failed", testAPIKeyID, emission.StatusFailed),
					nil,
				)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var resp emission.StatusResponse
				err := json.Unmarshal(w.Body.Bytes(), &resp)
				require.NoError(t, err)
				assert.Equal(t, "req-failed", resp.RequestID)
				assert.Equal(t, emission.StatusFailed, resp.Status)
				assert.Nil(t, resp.Result)
				require.NotNil(t, resp.Error)
				assert.Equal(t, "GOVERNMENT_REJECTION", resp.Error.Code)
				assert.Equal(t, "Invalid CNPJ", resp.Error.Message)
				assert.Equal(t, "E123", resp.Error.GovernmentCode)
				assert.Equal(t, "The CNPJ provided is not registered", resp.Error.Details)
				assert.NotNil(t, resp.ProcessedAt)
			},
		},
		{
			name:      "pending status has no result or error",
			requestID: "req-pending",
			apiKey:    createTestAPIKey(testAPIKeyID),
			mockSetup: func(m *MockEmissionRepository) {
				m.On("FindByRequestID", mock.Anything, "req-pending").Return(
					createTestEmissionRequest("req-pending", testAPIKeyID, emission.StatusPending),
					nil,
				)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var resp emission.StatusResponse
				err := json.Unmarshal(w.Body.Bytes(), &resp)
				require.NoError(t, err)
				assert.Equal(t, "req-pending", resp.RequestID)
				assert.Equal(t, emission.StatusPending, resp.Status)
				assert.Nil(t, resp.Result)
				assert.Nil(t, resp.Error)
				assert.Nil(t, resp.ProcessedAt)
			},
		},
		{
			name:      "processing status has no result or error",
			requestID: "req-processing",
			apiKey:    createTestAPIKey(testAPIKeyID),
			mockSetup: func(m *MockEmissionRepository) {
				m.On("FindByRequestID", mock.Anything, "req-processing").Return(
					createTestEmissionRequest("req-processing", testAPIKeyID, emission.StatusProcessing),
					nil,
				)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var resp emission.StatusResponse
				err := json.Unmarshal(w.Body.Bytes(), &resp)
				require.NoError(t, err)
				assert.Equal(t, "req-processing", resp.RequestID)
				assert.Equal(t, emission.StatusProcessing, resp.Status)
				assert.Nil(t, resp.Result)
				assert.Nil(t, resp.Error)
			},
		},
		{
			name:           "missing API key in context returns 500",
			requestID:      "req-123",
			apiKey:         nil, // No API key set
			mockSetup:      func(m *MockEmissionRepository) {},
			expectedStatus: http.StatusInternalServerError,
			expectedType:   "application/problem+json",
		},
		{
			name:      "database error returns 500",
			requestID: "req-db-error",
			apiKey:    createTestAPIKey(testAPIKeyID),
			mockSetup: func(m *MockEmissionRepository) {
				m.On("FindByRequestID", mock.Anything, "req-db-error").Return(
					nil,
					errors.New("database connection failed"),
				)
			},
			expectedStatus: http.StatusInternalServerError,
			expectedType:   "application/problem+json",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := &MockEmissionRepository{}
			tt.mockSetup(mockRepo)

			handler := NewStatusHandler(StatusHandlerConfig{
				EmissionRepo: mockRepo,
				BaseURL:      "https://api.example.com",
			})

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			c.Request = httptest.NewRequest(http.MethodGet, "/v1/nfse/status/"+tt.requestID, nil)
			c.Params = gin.Params{{Key: "requestId", Value: tt.requestID}}

			if tt.apiKey != nil {
				setAPIKeyInContext(c, tt.apiKey)
			}

			handler.Get(c)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedType != "" {
				assert.Contains(t, w.Header().Get("Content-Type"), tt.expectedType)
			}

			if tt.checkResponse != nil {
				tt.checkResponse(t, w)
			}

			mockRepo.AssertExpectations(t)
		})
	}
}

// TestStatusHandler_Get_WithLogger tests Get method with logging enabled.
func TestStatusHandler_Get_WithLogger(t *testing.T) {
	testAPIKeyID := primitive.NewObjectID()

	t.Run("logs status query start and success", func(t *testing.T) {
		var logBuf bytes.Buffer
		logger := log.New(&logBuf, "", 0)

		mockRepo := &MockEmissionRepository{}
		mockRepo.On("FindByRequestID", mock.Anything, "req-123").Return(
			createTestEmissionRequest("req-123", testAPIKeyID, emission.StatusPending),
			nil,
		)

		handler := NewStatusHandler(StatusHandlerConfig{
			EmissionRepo: mockRepo,
			BaseURL:      "https://api.example.com",
			Logger:       logger,
		})

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/v1/nfse/status/req-123", nil)
		c.Params = gin.Params{{Key: "requestId", Value: "req-123"}}
		setAPIKeyInContext(c, createTestAPIKey(testAPIKeyID))

		handler.Get(c)

		assert.Equal(t, http.StatusOK, w.Code)

		logOutput := logBuf.String()
		assert.Contains(t, logOutput, "status_query_start")
		assert.Contains(t, logOutput, "status_query_success")
		assert.Contains(t, logOutput, "req-123")
	})

	t.Run("logs not found error", func(t *testing.T) {
		var logBuf bytes.Buffer
		logger := log.New(&logBuf, "", 0)

		mockRepo := &MockEmissionRepository{}
		mockRepo.On("FindByRequestID", mock.Anything, "not-found").Return(
			nil,
			mongodb.ErrEmissionRequestNotFound,
		)

		handler := NewStatusHandler(StatusHandlerConfig{
			EmissionRepo: mockRepo,
			BaseURL:      "https://api.example.com",
			Logger:       logger,
		})

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/v1/nfse/status/not-found", nil)
		c.Params = gin.Params{{Key: "requestId", Value: "not-found"}}
		setAPIKeyInContext(c, createTestAPIKey(testAPIKeyID))

		handler.Get(c)

		assert.Equal(t, http.StatusNotFound, w.Code)

		logOutput := logBuf.String()
		assert.Contains(t, logOutput, "status_query_not_found")
	})

	t.Run("logs database error", func(t *testing.T) {
		var logBuf bytes.Buffer
		logger := log.New(&logBuf, "", 0)

		mockRepo := &MockEmissionRepository{}
		mockRepo.On("FindByRequestID", mock.Anything, "db-error").Return(
			nil,
			errors.New("connection refused"),
		)

		handler := NewStatusHandler(StatusHandlerConfig{
			EmissionRepo: mockRepo,
			BaseURL:      "https://api.example.com",
			Logger:       logger,
		})

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/v1/nfse/status/db-error", nil)
		c.Params = gin.Params{{Key: "requestId", Value: "db-error"}}
		setAPIKeyInContext(c, createTestAPIKey(testAPIKeyID))

		handler.Get(c)

		assert.Equal(t, http.StatusInternalServerError, w.Code)

		logOutput := logBuf.String()
		assert.Contains(t, logOutput, "status_query_error")
		assert.Contains(t, logOutput, "connection refused")
	})

	t.Run("logs unauthorized access attempt", func(t *testing.T) {
		var logBuf bytes.Buffer
		logger := log.New(&logBuf, "", 0)

		otherAPIKeyID := primitive.NewObjectID()

		mockRepo := &MockEmissionRepository{}
		mockRepo.On("FindByRequestID", mock.Anything, "other-user-req").Return(
			createTestEmissionRequest("other-user-req", otherAPIKeyID, emission.StatusPending),
			nil,
		)

		handler := NewStatusHandler(StatusHandlerConfig{
			EmissionRepo: mockRepo,
			BaseURL:      "https://api.example.com",
			Logger:       logger,
		})

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/v1/nfse/status/other-user-req", nil)
		c.Params = gin.Params{{Key: "requestId", Value: "other-user-req"}}
		setAPIKeyInContext(c, createTestAPIKey(testAPIKeyID))

		handler.Get(c)

		assert.Equal(t, http.StatusNotFound, w.Code)

		logOutput := logBuf.String()
		assert.Contains(t, logOutput, "status_query_unauthorized")
	})

	t.Run("logs invalid request with missing request ID", func(t *testing.T) {
		var logBuf bytes.Buffer
		logger := log.New(&logBuf, "", 0)

		mockRepo := &MockEmissionRepository{}

		handler := NewStatusHandler(StatusHandlerConfig{
			EmissionRepo: mockRepo,
			BaseURL:      "https://api.example.com",
			Logger:       logger,
		})

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/v1/nfse/status/", nil)
		c.Params = gin.Params{{Key: "requestId", Value: ""}}
		setAPIKeyInContext(c, createTestAPIKey(testAPIKeyID))

		handler.Get(c)

		assert.Equal(t, http.StatusBadRequest, w.Code)

		logOutput := logBuf.String()
		assert.Contains(t, logOutput, "status_query_invalid")
		assert.Contains(t, logOutput, "missing_request_id")
	})
}

// TestStatusHandler_List tests the List method of StatusHandler.
func TestStatusHandler_List(t *testing.T) {
	testAPIKeyID := primitive.NewObjectID()

	tests := []struct {
		name           string
		queryParams    map[string]string
		apiKey         *mongodb.APIKey
		mockSetup      func(*MockEmissionRepository)
		expectedStatus int
		checkResponse  func(*testing.T, *httptest.ResponseRecorder)
	}{
		{
			name:        "returns paginated list of requests",
			queryParams: map[string]string{},
			apiKey:      createTestAPIKey(testAPIKeyID),
			mockSetup: func(m *MockEmissionRepository) {
				m.On("FindByAPIKeyID", mock.Anything, testAPIKeyID, mongodb.PaginationParams{
					Page:     1,
					PageSize: 20,
				}).Return(&mongodb.PaginatedResult{
					Items: []*mongodb.EmissionRequest{
						createTestEmissionRequest("req-1", testAPIKeyID, emission.StatusPending),
						createTestEmissionRequest("req-2", testAPIKeyID, emission.StatusSuccess),
					},
					TotalCount: 2,
					Page:       1,
					PageSize:   20,
					TotalPages: 1,
				}, nil)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var resp struct {
					Items      []emission.StatusResponse `json:"items"`
					Pagination struct {
						Page       int64 `json:"page"`
						PageSize   int64 `json:"page_size"`
						TotalCount int64 `json:"total_count"`
						TotalPages int64 `json:"total_pages"`
					} `json:"pagination"`
				}
				err := json.Unmarshal(w.Body.Bytes(), &resp)
				require.NoError(t, err)
				assert.Len(t, resp.Items, 2)
				assert.Equal(t, int64(1), resp.Pagination.Page)
				assert.Equal(t, int64(20), resp.Pagination.PageSize)
				assert.Equal(t, int64(2), resp.Pagination.TotalCount)
				assert.Equal(t, int64(1), resp.Pagination.TotalPages)
			},
		},
		{
			name:        "default pagination (page=1, page_size=20)",
			queryParams: map[string]string{},
			apiKey:      createTestAPIKey(testAPIKeyID),
			mockSetup: func(m *MockEmissionRepository) {
				m.On("FindByAPIKeyID", mock.Anything, testAPIKeyID, mongodb.PaginationParams{
					Page:     1,
					PageSize: 20,
				}).Return(&mongodb.PaginatedResult{
					Items:      []*mongodb.EmissionRequest{},
					TotalCount: 0,
					Page:       1,
					PageSize:   20,
					TotalPages: 0,
				}, nil)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var resp struct {
					Pagination struct {
						Page     int64 `json:"page"`
						PageSize int64 `json:"page_size"`
					} `json:"pagination"`
				}
				err := json.Unmarshal(w.Body.Bytes(), &resp)
				require.NoError(t, err)
				assert.Equal(t, int64(1), resp.Pagination.Page)
				assert.Equal(t, int64(20), resp.Pagination.PageSize)
			},
		},
		{
			name: "custom pagination parameters",
			queryParams: map[string]string{
				"page":      "2",
				"page_size": "50",
			},
			apiKey: createTestAPIKey(testAPIKeyID),
			mockSetup: func(m *MockEmissionRepository) {
				m.On("FindByAPIKeyID", mock.Anything, testAPIKeyID, mongodb.PaginationParams{
					Page:     2,
					PageSize: 50,
				}).Return(&mongodb.PaginatedResult{
					Items:      []*mongodb.EmissionRequest{},
					TotalCount: 75,
					Page:       2,
					PageSize:   50,
					TotalPages: 2,
				}, nil)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var resp struct {
					Pagination struct {
						Page       int64 `json:"page"`
						PageSize   int64 `json:"page_size"`
						TotalCount int64 `json:"total_count"`
						TotalPages int64 `json:"total_pages"`
					} `json:"pagination"`
				}
				err := json.Unmarshal(w.Body.Bytes(), &resp)
				require.NoError(t, err)
				assert.Equal(t, int64(2), resp.Pagination.Page)
				assert.Equal(t, int64(50), resp.Pagination.PageSize)
				assert.Equal(t, int64(75), resp.Pagination.TotalCount)
				assert.Equal(t, int64(2), resp.Pagination.TotalPages)
			},
		},
		{
			name:        "empty list returns empty array",
			queryParams: map[string]string{},
			apiKey:      createTestAPIKey(testAPIKeyID),
			mockSetup: func(m *MockEmissionRepository) {
				m.On("FindByAPIKeyID", mock.Anything, testAPIKeyID, mock.Anything).Return(&mongodb.PaginatedResult{
					Items:      []*mongodb.EmissionRequest{},
					TotalCount: 0,
					Page:       1,
					PageSize:   20,
					TotalPages: 0,
				}, nil)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var resp struct {
					Items []emission.StatusResponse `json:"items"`
				}
				err := json.Unmarshal(w.Body.Bytes(), &resp)
				require.NoError(t, err)
				assert.NotNil(t, resp.Items)
				assert.Empty(t, resp.Items)
			},
		},
		{
			name:           "missing API key in context returns 500",
			queryParams:    map[string]string{},
			apiKey:         nil,
			mockSetup:      func(m *MockEmissionRepository) {},
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:        "database error returns 500",
			queryParams: map[string]string{},
			apiKey:      createTestAPIKey(testAPIKeyID),
			mockSetup: func(m *MockEmissionRepository) {
				m.On("FindByAPIKeyID", mock.Anything, testAPIKeyID, mock.Anything).Return(
					nil,
					errors.New("database connection failed"),
				)
			},
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name:        "list includes success items with results",
			queryParams: map[string]string{},
			apiKey:      createTestAPIKey(testAPIKeyID),
			mockSetup: func(m *MockEmissionRepository) {
				m.On("FindByAPIKeyID", mock.Anything, testAPIKeyID, mock.Anything).Return(&mongodb.PaginatedResult{
					Items: []*mongodb.EmissionRequest{
						createTestEmissionRequest("req-success", testAPIKeyID, emission.StatusSuccess),
					},
					TotalCount: 1,
					Page:       1,
					PageSize:   20,
					TotalPages: 1,
				}, nil)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var resp struct {
					Items []emission.StatusResponse `json:"items"`
				}
				err := json.Unmarshal(w.Body.Bytes(), &resp)
				require.NoError(t, err)
				require.Len(t, resp.Items, 1)
				assert.Equal(t, emission.StatusSuccess, resp.Items[0].Status)
				require.NotNil(t, resp.Items[0].Result)
				assert.Equal(t, "NFSe35503081234567800019900001000000000000001", resp.Items[0].Result.NFSeAccessKey)
				assert.Contains(t, resp.Items[0].Result.NFSeXMLURL, "/v1/nfse/")
			},
		},
		{
			name:        "list includes failed items with errors",
			queryParams: map[string]string{},
			apiKey:      createTestAPIKey(testAPIKeyID),
			mockSetup: func(m *MockEmissionRepository) {
				m.On("FindByAPIKeyID", mock.Anything, testAPIKeyID, mock.Anything).Return(&mongodb.PaginatedResult{
					Items: []*mongodb.EmissionRequest{
						createTestEmissionRequest("req-failed", testAPIKeyID, emission.StatusFailed),
					},
					TotalCount: 1,
					Page:       1,
					PageSize:   20,
					TotalPages: 1,
				}, nil)
			},
			expectedStatus: http.StatusOK,
			checkResponse: func(t *testing.T, w *httptest.ResponseRecorder) {
				var resp struct {
					Items []emission.StatusResponse `json:"items"`
				}
				err := json.Unmarshal(w.Body.Bytes(), &resp)
				require.NoError(t, err)
				require.Len(t, resp.Items, 1)
				assert.Equal(t, emission.StatusFailed, resp.Items[0].Status)
				require.NotNil(t, resp.Items[0].Error)
				assert.Equal(t, "GOVERNMENT_REJECTION", resp.Items[0].Error.Code)
			},
		},
		{
			name: "invalid page parameter uses default",
			queryParams: map[string]string{
				"page": "invalid",
			},
			apiKey: createTestAPIKey(testAPIKeyID),
			mockSetup: func(m *MockEmissionRepository) {
				m.On("FindByAPIKeyID", mock.Anything, testAPIKeyID, mongodb.PaginationParams{
					Page:     1, // Falls back to default
					PageSize: 20,
				}).Return(&mongodb.PaginatedResult{
					Items:      []*mongodb.EmissionRequest{},
					TotalCount: 0,
					Page:       1,
					PageSize:   20,
					TotalPages: 0,
				}, nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "negative page parameter uses default",
			queryParams: map[string]string{
				"page": "-1",
			},
			apiKey: createTestAPIKey(testAPIKeyID),
			mockSetup: func(m *MockEmissionRepository) {
				m.On("FindByAPIKeyID", mock.Anything, testAPIKeyID, mongodb.PaginationParams{
					Page:     1, // Falls back to default
					PageSize: 20,
				}).Return(&mongodb.PaginatedResult{
					Items:      []*mongodb.EmissionRequest{},
					TotalCount: 0,
					Page:       1,
					PageSize:   20,
					TotalPages: 0,
				}, nil)
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "zero page parameter uses default",
			queryParams: map[string]string{
				"page": "0",
			},
			apiKey: createTestAPIKey(testAPIKeyID),
			mockSetup: func(m *MockEmissionRepository) {
				m.On("FindByAPIKeyID", mock.Anything, testAPIKeyID, mongodb.PaginationParams{
					Page:     1, // Falls back to default
					PageSize: 20,
				}).Return(&mongodb.PaginatedResult{
					Items:      []*mongodb.EmissionRequest{},
					TotalCount: 0,
					Page:       1,
					PageSize:   20,
					TotalPages: 0,
				}, nil)
			},
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := &MockEmissionRepository{}
			tt.mockSetup(mockRepo)

			handler := NewStatusHandler(StatusHandlerConfig{
				EmissionRepo: mockRepo,
				BaseURL:      "https://api.example.com",
			})

			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			// Build URL with query parameters
			url := "/v1/nfse/status"
			if len(tt.queryParams) > 0 {
				url += "?"
				first := true
				for k, v := range tt.queryParams {
					if !first {
						url += "&"
					}
					url += k + "=" + v
					first = false
				}
			}

			c.Request = httptest.NewRequest(http.MethodGet, url, nil)

			if tt.apiKey != nil {
				setAPIKeyInContext(c, tt.apiKey)
			}

			handler.List(c)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.checkResponse != nil {
				tt.checkResponse(t, w)
			}

			mockRepo.AssertExpectations(t)
		})
	}
}

// TestStatusHandler_List_WithLogger tests List method with logging enabled.
func TestStatusHandler_List_WithLogger(t *testing.T) {
	testAPIKeyID := primitive.NewObjectID()

	t.Run("logs list start and success", func(t *testing.T) {
		var logBuf bytes.Buffer
		logger := log.New(&logBuf, "", 0)

		mockRepo := &MockEmissionRepository{}
		mockRepo.On("FindByAPIKeyID", mock.Anything, testAPIKeyID, mock.Anything).Return(&mongodb.PaginatedResult{
			Items:      []*mongodb.EmissionRequest{},
			TotalCount: 0,
			Page:       1,
			PageSize:   20,
			TotalPages: 0,
		}, nil)

		handler := NewStatusHandler(StatusHandlerConfig{
			EmissionRepo: mockRepo,
			BaseURL:      "https://api.example.com",
			Logger:       logger,
		})

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/v1/nfse/status", nil)
		setAPIKeyInContext(c, createTestAPIKey(testAPIKeyID))

		handler.List(c)

		assert.Equal(t, http.StatusOK, w.Code)

		logOutput := logBuf.String()
		assert.Contains(t, logOutput, "status_list_start")
		assert.Contains(t, logOutput, "status_list_success")
	})

	t.Run("logs database error", func(t *testing.T) {
		var logBuf bytes.Buffer
		logger := log.New(&logBuf, "", 0)

		mockRepo := &MockEmissionRepository{}
		mockRepo.On("FindByAPIKeyID", mock.Anything, testAPIKeyID, mock.Anything).Return(
			nil,
			errors.New("connection timeout"),
		)

		handler := NewStatusHandler(StatusHandlerConfig{
			EmissionRepo: mockRepo,
			BaseURL:      "https://api.example.com",
			Logger:       logger,
		})

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/v1/nfse/status", nil)
		setAPIKeyInContext(c, createTestAPIKey(testAPIKeyID))

		handler.List(c)

		assert.Equal(t, http.StatusInternalServerError, w.Code)

		logOutput := logBuf.String()
		assert.Contains(t, logOutput, "status_list_error")
		assert.Contains(t, logOutput, "connection timeout")
	})
}

// TestBuildNFSeQueryURL tests the buildNFSeQueryURL method.
func TestBuildNFSeQueryURL(t *testing.T) {
	tests := []struct {
		name        string
		baseURL     string
		chaveAcesso string
		want        string
	}{
		{
			name:        "builds correct URL with base URL",
			baseURL:     "https://api.example.com",
			chaveAcesso: "NFSe35503081234567800019900001000000000000001",
			want:        "https://api.example.com/v1/nfse/NFSe35503081234567800019900001000000000000001",
		},
		{
			name:        "builds relative URL without base URL",
			baseURL:     "",
			chaveAcesso: "NFSe35503081234567800019900001000000000000001",
			want:        "/v1/nfse/NFSe35503081234567800019900001000000000000001",
		},
		{
			name:        "handles empty access key",
			baseURL:     "https://api.example.com",
			chaveAcesso: "",
			want:        "",
		},
		{
			name:        "empty access key with empty base URL",
			baseURL:     "",
			chaveAcesso: "",
			want:        "",
		},
		{
			name:        "base URL with trailing slash is preserved",
			baseURL:     "https://api.example.com/",
			chaveAcesso: "NFSe123",
			want:        "https://api.example.com//v1/nfse/NFSe123",
		},
		{
			name:        "handles special characters in access key",
			baseURL:     "https://api.example.com",
			chaveAcesso: "NFSe-123_456",
			want:        "https://api.example.com/v1/nfse/NFSe-123_456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := &StatusHandler{baseURL: tt.baseURL}
			got := handler.buildNFSeQueryURL(tt.chaveAcesso)

			assert.Equal(t, tt.want, got)
		})
	}
}

// TestParseIntQuery tests the parseIntQuery function.
func TestParseIntQuery(t *testing.T) {
	tests := []struct {
		name         string
		queryParams  map[string]string
		key          string
		defaultValue int64
		want         int64
	}{
		{
			name:         "returns default for empty value",
			queryParams:  map[string]string{},
			key:          "page",
			defaultValue: 1,
			want:         1,
		},
		{
			name:         "parses valid integer",
			queryParams:  map[string]string{"page": "5"},
			key:          "page",
			defaultValue: 1,
			want:         5,
		},
		{
			name:         "returns default for invalid integer",
			queryParams:  map[string]string{"page": "invalid"},
			key:          "page",
			defaultValue: 1,
			want:         1,
		},
		{
			name:         "returns default for negative value",
			queryParams:  map[string]string{"page": "-5"},
			key:          "page",
			defaultValue: 1,
			want:         1,
		},
		{
			name:         "returns default for zero",
			queryParams:  map[string]string{"page": "0"},
			key:          "page",
			defaultValue: 1,
			want:         1,
		},
		{
			name:         "parses large integer",
			queryParams:  map[string]string{"page": "999999"},
			key:          "page",
			defaultValue: 1,
			want:         999999,
		},
		{
			name:         "returns default for float value",
			queryParams:  map[string]string{"page": "1.5"},
			key:          "page",
			defaultValue: 1,
			want:         1,
		},
		{
			name:         "returns default for empty string value",
			queryParams:  map[string]string{"page": ""},
			key:          "page",
			defaultValue: 10,
			want:         10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)

			url := "/test"
			if len(tt.queryParams) > 0 {
				url += "?"
				first := true
				for k, v := range tt.queryParams {
					if !first {
						url += "&"
					}
					url += k + "=" + v
					first = false
				}
			}

			c.Request = httptest.NewRequest(http.MethodGet, url, nil)

			got := parseIntQuery(c, tt.key, tt.defaultValue)

			assert.Equal(t, tt.want, got)
		})
	}
}

// TestParseIntValue tests the parseIntValue function.
func TestParseIntValue(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantValue int64
		wantOK    bool
	}{
		{
			name:      "parses positive integer",
			input:     "123",
			wantValue: 123,
			wantOK:    true,
		},
		{
			name:      "parses zero",
			input:     "0",
			wantValue: 0,
			wantOK:    true,
		},
		{
			name:      "returns false for non-numeric",
			input:     "abc",
			wantValue: 0,
			wantOK:    false,
		},
		{
			name:      "returns false for negative (has minus sign)",
			input:     "-5",
			wantValue: 0,
			wantOK:    false,
		},
		{
			name:      "returns false for decimal",
			input:     "12.5",
			wantValue: 0,
			wantOK:    false,
		},
		{
			name:      "parses large number",
			input:     "9999999999",
			wantValue: 9999999999,
			wantOK:    true,
		},
		{
			name:      "returns false for mixed alphanumeric",
			input:     "12a34",
			wantValue: 0,
			wantOK:    false,
		},
		{
			name:      "parses single digit",
			input:     "5",
			wantValue: 5,
			wantOK:    true,
		},
		{
			name:      "returns false for space in middle",
			input:     "12 34",
			wantValue: 0,
			wantOK:    false,
		},
		{
			name:      "parses empty string as zero",
			input:     "",
			wantValue: 0,
			wantOK:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var result int64
			ok, _ := parseIntValue(tt.input, &result)

			assert.Equal(t, tt.wantOK, ok)
			if ok {
				assert.Equal(t, tt.wantValue, result)
			}
		})
	}
}

// TestLogStatus tests the logStatus method.
func TestLogStatus(t *testing.T) {
	t.Run("does nothing when logger is nil", func(t *testing.T) {
		handler := &StatusHandler{logger: nil}

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/test", nil)

		// Should not panic
		handler.logStatus(c, "test_event", map[string]interface{}{
			"key": "value",
		})
	})

	t.Run("logs when logger is present", func(t *testing.T) {
		var buf bytes.Buffer
		logger := log.New(&buf, "", 0)
		handler := &StatusHandler{logger: logger}

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/test", nil)
		c.Set("request_id", "test-req-id")

		handler.logStatus(c, "test_event", map[string]interface{}{
			"custom_field": "custom_value",
		})

		logOutput := buf.String()
		assert.Contains(t, logOutput, "test_event")
		assert.Contains(t, logOutput, "custom_field")
		assert.Contains(t, logOutput, "custom_value")
		assert.Contains(t, logOutput, "test-req-id")
		assert.Contains(t, logOutput, "timestamp")
		assert.Contains(t, logOutput, "client_ip")
	})

	t.Run("logs without request_id when not set", func(t *testing.T) {
		var buf bytes.Buffer
		logger := log.New(&buf, "", 0)
		handler := &StatusHandler{logger: logger}

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/test", nil)

		handler.logStatus(c, "test_event", map[string]interface{}{
			"data": "value",
		})

		logOutput := buf.String()
		assert.Contains(t, logOutput, "test_event")
		assert.Contains(t, logOutput, "data")
	})

	t.Run("handles unmarshalable fields gracefully", func(t *testing.T) {
		var buf bytes.Buffer
		logger := log.New(&buf, "", 0)
		handler := &StatusHandler{logger: logger}

		w := httptest.NewRecorder()
		c, _ := gin.CreateTestContext(w)
		c.Request = httptest.NewRequest(http.MethodGet, "/test", nil)

		// Create a map with a function value that cannot be marshaled to JSON
		handler.logStatus(c, "test_event", map[string]interface{}{
			"unmarshalable": func() {},
		})

		logOutput := buf.String()
		// Should log the marshal error instead of the event
		assert.Contains(t, logOutput, "failed to marshal")
	})
}

// TestStatusHandler_Get_SuccessWithoutResult tests edge case where status is success but result is nil.
func TestStatusHandler_Get_SuccessWithoutResult(t *testing.T) {
	testAPIKeyID := primitive.NewObjectID()

	mockRepo := &MockEmissionRepository{}

	// Create a success request but with nil result (edge case)
	req := createTestEmissionRequest("req-edge", testAPIKeyID, emission.StatusSuccess)
	req.Result = nil

	mockRepo.On("FindByRequestID", mock.Anything, "req-edge").Return(req, nil)

	handler := NewStatusHandler(StatusHandlerConfig{
		EmissionRepo: mockRepo,
		BaseURL:      "https://api.example.com",
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/nfse/status/req-edge", nil)
	c.Params = gin.Params{{Key: "requestId", Value: "req-edge"}}
	setAPIKeyInContext(c, createTestAPIKey(testAPIKeyID))

	handler.Get(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp emission.StatusResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, emission.StatusSuccess, resp.Status)
	assert.Nil(t, resp.Result) // Should be nil when result is nil
}

// TestStatusHandler_Get_FailedWithoutRejection tests edge case where status is failed but rejection is nil.
func TestStatusHandler_Get_FailedWithoutRejection(t *testing.T) {
	testAPIKeyID := primitive.NewObjectID()

	mockRepo := &MockEmissionRepository{}

	// Create a failed request but with nil rejection (edge case)
	req := createTestEmissionRequest("req-edge", testAPIKeyID, emission.StatusFailed)
	req.Rejection = nil

	mockRepo.On("FindByRequestID", mock.Anything, "req-edge").Return(req, nil)

	handler := NewStatusHandler(StatusHandlerConfig{
		EmissionRepo: mockRepo,
		BaseURL:      "https://api.example.com",
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/nfse/status/req-edge", nil)
	c.Params = gin.Params{{Key: "requestId", Value: "req-edge"}}
	setAPIKeyInContext(c, createTestAPIKey(testAPIKeyID))

	handler.Get(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp emission.StatusResponse
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	assert.Equal(t, emission.StatusFailed, resp.Status)
	assert.Nil(t, resp.Error) // Should be nil when rejection is nil
}

// TestStatusHandler_List_MixedStatuses tests listing with various statuses.
func TestStatusHandler_List_MixedStatuses(t *testing.T) {
	testAPIKeyID := primitive.NewObjectID()

	mockRepo := &MockEmissionRepository{}

	pendingReq := createTestEmissionRequest("req-pending", testAPIKeyID, emission.StatusPending)
	processingReq := createTestEmissionRequest("req-processing", testAPIKeyID, emission.StatusProcessing)
	successReq := createTestEmissionRequest("req-success", testAPIKeyID, emission.StatusSuccess)
	failedReq := createTestEmissionRequest("req-failed", testAPIKeyID, emission.StatusFailed)

	mockRepo.On("FindByAPIKeyID", mock.Anything, testAPIKeyID, mock.Anything).Return(&mongodb.PaginatedResult{
		Items:      []*mongodb.EmissionRequest{pendingReq, processingReq, successReq, failedReq},
		TotalCount: 4,
		Page:       1,
		PageSize:   20,
		TotalPages: 1,
	}, nil)

	handler := NewStatusHandler(StatusHandlerConfig{
		EmissionRepo: mockRepo,
		BaseURL:      "https://api.example.com",
	})

	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodGet, "/v1/nfse/status", nil)
	setAPIKeyInContext(c, createTestAPIKey(testAPIKeyID))

	handler.List(c)

	assert.Equal(t, http.StatusOK, w.Code)

	var resp struct {
		Items []emission.StatusResponse `json:"items"`
	}
	err := json.Unmarshal(w.Body.Bytes(), &resp)
	require.NoError(t, err)
	require.Len(t, resp.Items, 4)

	// Verify each item has appropriate fields
	for _, item := range resp.Items {
		switch item.Status {
		case emission.StatusPending, emission.StatusProcessing:
			assert.Nil(t, item.Result)
			assert.Nil(t, item.Error)
		case emission.StatusSuccess:
			assert.NotNil(t, item.Result)
			assert.Nil(t, item.Error)
		case emission.StatusFailed:
			assert.Nil(t, item.Result)
			assert.NotNil(t, item.Error)
		}
	}
}
