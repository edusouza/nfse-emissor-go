// Package handlers provides HTTP request handlers for the NFS-e API.
package handlers

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/eduardo/nfse-nacional/internal/infrastructure/mongodb"
	"github.com/eduardo/nfse-nacional/internal/infrastructure/sefin"
)

// ================================================================================
// Mock SefinClient
// ================================================================================

// MockSefinClient implements sefin.SefinClient for testing.
type MockSefinClient struct {
	mock.Mock
}

// SubmitDPS mocks the DPS submission.
func (m *MockSefinClient) SubmitDPS(ctx context.Context, dpsXML string, environment string) (*sefin.SefinResponse, error) {
	args := m.Called(ctx, dpsXML, environment)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*sefin.SefinResponse), args.Error(1)
}

// QueryNFSe mocks the NFS-e query operation.
func (m *MockSefinClient) QueryNFSe(ctx context.Context, chaveAcesso string, cert *tls.Certificate) (*sefin.NFSeQueryResult, error) {
	args := m.Called(ctx, chaveAcesso, cert)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*sefin.NFSeQueryResult), args.Error(1)
}

// LookupDPS mocks the DPS lookup operation.
func (m *MockSefinClient) LookupDPS(ctx context.Context, dpsID string, cert *tls.Certificate) (*sefin.DPSLookupResult, error) {
	args := m.Called(ctx, dpsID, cert)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*sefin.DPSLookupResult), args.Error(1)
}

// CheckDPSExists mocks the DPS existence check.
func (m *MockSefinClient) CheckDPSExists(ctx context.Context, dpsID string, cert *tls.Certificate) (bool, error) {
	args := m.Called(ctx, dpsID, cert)
	return args.Bool(0), args.Error(1)
}

// QueryEvents mocks the events query operation.
func (m *MockSefinClient) QueryEvents(ctx context.Context, chaveAcesso string, cert *tls.Certificate) (*sefin.EventsQueryResult, error) {
	args := m.Called(ctx, chaveAcesso, cert)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*sefin.EventsQueryResult), args.Error(1)
}

// ================================================================================
// Test Helpers
// ================================================================================

// testAPIKey returns a valid API key for testing.
func testAPIKey() *mongodb.APIKey {
	return &mongodb.APIKey{
		KeyPrefix:      "test1234",
		IntegratorName: "Test Integrator",
		WebhookURL:     "https://example.com/webhook",
		Environment:    "homologacao",
		Active:         true,
		CreatedAt:      time.Now().Add(-24 * time.Hour),
		UpdatedAt:      time.Now(),
	}
}

// validAccessKey returns a valid 50-character access key for testing.
func validAccessKey() string {
	// NFSe + 46 alphanumeric characters = 50 total
	return "NFSe3550308202601081123456789012300000000000012310"
}

// setupTestRouter creates a test router with the query handler and optional API key.
func setupTestRouter(handler *QueryHandler, apiKey *mongodb.APIKey) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	// Middleware to inject API key into context
	r.Use(func(c *gin.Context) {
		if apiKey != nil {
			c.Set(apiKeyContextKey, apiKey)
		}
		c.Next()
	})

	r.GET("/v1/nfse/:chaveAcesso", handler.GetNFSe)
	r.GET("/v1/nfse/:chaveAcesso/eventos", handler.GetEvents)

	return r
}

// createTestHandler creates a QueryHandler with the provided mock client.
func createTestHandler(mockClient *MockSefinClient) *QueryHandler {
	return NewQueryHandler(QueryHandlerConfig{
		SefinClient: mockClient,
		BaseURL:     "http://localhost:8080",
		Logger:      nil, // No logging in tests
	})
}

// createSampleNFSeResult returns a sample NFSeQueryResult for testing.
func createSampleNFSeResult() *sefin.NFSeQueryResult {
	return &sefin.NFSeQueryResult{
		ChaveAcesso: validAccessKey(),
		Numero:      "000000001",
		DataEmissao: time.Date(2024, 1, 15, 10, 30, 0, 0, time.FixedZone("BRT", -3*60*60)),
		Status:      "active",
		XML:         "<NFSe>sample</NFSe>",
		Prestador: sefin.PrestadorData{
			Documento:       "12345678000199",
			Nome:            "Empresa Teste Ltda",
			Municipio:       "Sao Paulo",
			MunicipioCodigo: "3550308",
		},
		Tomador: &sefin.TomadorData{
			Documento:     "98765432000188",
			TipoDocumento: "cnpj",
			Nome:          "Cliente Teste S.A.",
		},
		Servico: sefin.ServicoData{
			CodigoNacional:  "010201",
			Descricao:       "Consultoria em tecnologia da informacao",
			LocalPrestacao:  "Sao Paulo - SP",
			MunicipioCodigo: "3550308",
		},
		Valores: sefin.ValoresData{
			ValorServico: 1000.00,
			BaseCalculo:  1000.00,
			Aliquota:     5.00,
			ValorISSQN:   50.00,
			ValorLiquido: 950.00,
		},
	}
}

// createSampleEventsResult returns a sample EventsQueryResult for testing.
func createSampleEventsResult() *sefin.EventsQueryResult {
	return &sefin.EventsQueryResult{
		ChaveAcesso: validAccessKey(),
		Events: []sefin.EventData{
			{
				Tipo:      "EMISSAO",
				Descricao: "NFS-e emitida",
				Sequencia: 1,
				Data:      time.Date(2024, 1, 15, 10, 30, 0, 0, time.FixedZone("BRT", -3*60*60)),
				XML:       "<evento>emission</evento>",
			},
			{
				Tipo:      "e101101",
				Descricao: "Cancelamento de NFS-e",
				Sequencia: 2,
				Data:      time.Date(2024, 1, 16, 14, 0, 0, 0, time.FixedZone("BRT", -3*60*60)),
				XML:       "<evento>cancellation</evento>",
			},
		},
	}
}

// ================================================================================
// NewQueryHandler Tests
// ================================================================================

func TestNewQueryHandler(t *testing.T) {
	mockClient := new(MockSefinClient)

	config := QueryHandlerConfig{
		SefinClient: mockClient,
		BaseURL:     "http://localhost:8080",
		Logger:      nil,
	}

	handler := NewQueryHandler(config)

	assert.NotNil(t, handler)
	assert.Equal(t, mockClient, handler.sefinClient)
	assert.Equal(t, "http://localhost:8080", handler.baseURL)
	assert.Nil(t, handler.logger)
}

// ================================================================================
// GetNFSe Tests
// ================================================================================

func TestGetNFSe_Success(t *testing.T) {
	mockClient := new(MockSefinClient)
	handler := createTestHandler(mockClient)
	apiKey := testAPIKey()

	expectedResult := createSampleNFSeResult()

	// Setup mock expectation
	mockClient.On("QueryNFSe", mock.Anything, validAccessKey(), (*tls.Certificate)(nil)).
		Return(expectedResult, nil)

	router := setupTestRouter(handler, apiKey)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/nfse/"+validAccessKey(), nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Parse response body
	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	// Verify response fields
	assert.Equal(t, validAccessKey(), response["chave_acesso"])
	assert.Equal(t, "000000001", response["numero"])
	assert.Equal(t, "active", response["status"])
	assert.Equal(t, "<NFSe>sample</NFSe>", response["xml"])

	// Verify prestador
	prestador := response["prestador"].(map[string]interface{})
	assert.Equal(t, "12345678000199", prestador["documento"])
	assert.Equal(t, "Empresa Teste Ltda", prestador["nome"])
	assert.Equal(t, "Sao Paulo", prestador["municipio"])

	// Verify servico
	servico := response["servico"].(map[string]interface{})
	assert.Equal(t, "010201", servico["codigo_nacional"])
	assert.Equal(t, "Consultoria em tecnologia da informacao", servico["descricao"])

	// Verify valores
	valores := response["valores"].(map[string]interface{})
	assert.Equal(t, float64(1000), valores["valor_servico"])
	assert.Equal(t, float64(950), valores["valor_liquido"])

	mockClient.AssertExpectations(t)
}

func TestGetNFSe_SuccessWithTomador(t *testing.T) {
	mockClient := new(MockSefinClient)
	handler := createTestHandler(mockClient)
	apiKey := testAPIKey()

	expectedResult := createSampleNFSeResult()
	expectedResult.Tomador = &sefin.TomadorData{
		Documento:     "98765432000188",
		TipoDocumento: "cnpj",
		Nome:          "Cliente Teste S.A.",
	}

	mockClient.On("QueryNFSe", mock.Anything, validAccessKey(), (*tls.Certificate)(nil)).
		Return(expectedResult, nil)

	router := setupTestRouter(handler, apiKey)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/nfse/"+validAccessKey(), nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	// Verify tomador is present
	tomador, exists := response["tomador"].(map[string]interface{})
	assert.True(t, exists)
	assert.Equal(t, "Cliente Teste S.A.", tomador["nome"])

	mockClient.AssertExpectations(t)
}

func TestGetNFSe_SuccessWithoutTomador(t *testing.T) {
	mockClient := new(MockSefinClient)
	handler := createTestHandler(mockClient)
	apiKey := testAPIKey()

	expectedResult := createSampleNFSeResult()
	expectedResult.Tomador = nil // No taker (anonymous B2C)

	mockClient.On("QueryNFSe", mock.Anything, validAccessKey(), (*tls.Certificate)(nil)).
		Return(expectedResult, nil)

	router := setupTestRouter(handler, apiKey)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/nfse/"+validAccessKey(), nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	// Verify tomador is not present
	_, exists := response["tomador"]
	assert.False(t, exists)

	mockClient.AssertExpectations(t)
}

func TestGetNFSe_EmptyAccessKey(t *testing.T) {
	mockClient := new(MockSefinClient)
	handler := createTestHandler(mockClient)
	apiKey := testAPIKey()

	// Gin treats empty path param differently - use a whitespace-only key
	router := setupTestRouter(handler, apiKey)
	w := httptest.NewRecorder()

	// Use path with just spaces (will be trimmed to empty)
	req, _ := http.NewRequest("GET", "/v1/nfse/%20%20%20", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, float64(400), response["status"])
	assert.Contains(t, response["detail"], "Access key is required")

	// Mock should not be called for invalid keys
	mockClient.AssertNotCalled(t, "QueryNFSe")
}

func TestGetNFSe_InvalidAccessKeyLength(t *testing.T) {
	tests := []struct {
		name        string
		accessKey   string
		expectedMsg string
	}{
		{
			name:        "too short",
			accessKey:   "NFSe12345",
			expectedMsg: "Access key must be exactly 50 characters",
		},
		{
			name:        "too long",
			accessKey:   "NFSe355030820260108112345678901230000000000001231012345",
			expectedMsg: "Access key must be exactly 50 characters",
		},
		{
			name:        "49 characters",
			accessKey:   "NFSe35503082026010811234567890123000000000000123",
			expectedMsg: "Access key must be exactly 50 characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(MockSefinClient)
			handler := createTestHandler(mockClient)
			apiKey := testAPIKey()

			router := setupTestRouter(handler, apiKey)
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/v1/nfse/"+tt.accessKey, nil)

			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusBadRequest, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			assert.Equal(t, float64(400), response["status"])
			assert.Contains(t, response["detail"], tt.expectedMsg)

			mockClient.AssertNotCalled(t, "QueryNFSe")
		})
	}
}

func TestGetNFSe_InvalidAccessKeyPrefix(t *testing.T) {
	mockClient := new(MockSefinClient)
	handler := createTestHandler(mockClient)
	apiKey := testAPIKey()

	// 50 chars but wrong prefix
	invalidKey := "ABCD3550308202601081123456789012300000000000012310"

	router := setupTestRouter(handler, apiKey)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/nfse/"+invalidKey, nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Contains(t, response["detail"], "Access key must start with 'NFSe' prefix")

	mockClient.AssertNotCalled(t, "QueryNFSe")
}

func TestGetNFSe_InvalidAccessKeyCharacters(t *testing.T) {
	mockClient := new(MockSefinClient)
	handler := createTestHandler(mockClient)
	apiKey := testAPIKey()

	// 50 chars with special characters
	invalidKey := "NFSe3550308-02601081!23456789012300000000000012310"

	router := setupTestRouter(handler, apiKey)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/nfse/"+invalidKey, nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Contains(t, response["detail"], "Access key must contain only alphanumeric characters")

	mockClient.AssertNotCalled(t, "QueryNFSe")
}

func TestGetNFSe_NotFound(t *testing.T) {
	mockClient := new(MockSefinClient)
	handler := createTestHandler(mockClient)
	apiKey := testAPIKey()

	mockClient.On("QueryNFSe", mock.Anything, validAccessKey(), (*tls.Certificate)(nil)).
		Return(nil, sefin.ErrNFSeNotFound)

	router := setupTestRouter(handler, apiKey)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/nfse/"+validAccessKey(), nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, float64(404), response["status"])
	assert.Contains(t, response["detail"], "NFS-e not found")

	mockClient.AssertExpectations(t)
}

func TestGetNFSe_ServiceUnavailable(t *testing.T) {
	mockClient := new(MockSefinClient)
	handler := createTestHandler(mockClient)
	apiKey := testAPIKey()

	mockClient.On("QueryNFSe", mock.Anything, validAccessKey(), (*tls.Certificate)(nil)).
		Return(nil, sefin.ErrServiceUnavailable)

	router := setupTestRouter(handler, apiKey)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/nfse/"+validAccessKey(), nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, float64(503), response["status"])
	assert.Contains(t, response["detail"], "temporarily unavailable")

	mockClient.AssertExpectations(t)
}

func TestGetNFSe_Timeout(t *testing.T) {
	mockClient := new(MockSefinClient)
	handler := createTestHandler(mockClient)
	apiKey := testAPIKey()

	mockClient.On("QueryNFSe", mock.Anything, validAccessKey(), (*tls.Certificate)(nil)).
		Return(nil, sefin.ErrTimeout)

	router := setupTestRouter(handler, apiKey)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/nfse/"+validAccessKey(), nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusGatewayTimeout, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, float64(504), response["status"])
	assert.Contains(t, response["detail"], "timed out")

	mockClient.AssertExpectations(t)
}

func TestGetNFSe_ContextDeadlineExceeded(t *testing.T) {
	mockClient := new(MockSefinClient)
	handler := createTestHandler(mockClient)
	apiKey := testAPIKey()

	mockClient.On("QueryNFSe", mock.Anything, validAccessKey(), (*tls.Certificate)(nil)).
		Return(nil, context.DeadlineExceeded)

	router := setupTestRouter(handler, apiKey)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/nfse/"+validAccessKey(), nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusGatewayTimeout, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, float64(504), response["status"])
	assert.Contains(t, response["detail"], "timed out")

	mockClient.AssertExpectations(t)
}

func TestGetNFSe_Forbidden(t *testing.T) {
	mockClient := new(MockSefinClient)
	handler := createTestHandler(mockClient)
	apiKey := testAPIKey()

	mockClient.On("QueryNFSe", mock.Anything, validAccessKey(), (*tls.Certificate)(nil)).
		Return(nil, sefin.ErrForbidden)

	router := setupTestRouter(handler, apiKey)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/nfse/"+validAccessKey(), nil)

	router.ServeHTTP(w, req)

	// Forbidden is treated as 404 to prevent information leakage
	assert.Equal(t, http.StatusNotFound, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Contains(t, response["detail"], "not found")

	mockClient.AssertExpectations(t)
}

func TestGetNFSe_InternalError(t *testing.T) {
	mockClient := new(MockSefinClient)
	handler := createTestHandler(mockClient)
	apiKey := testAPIKey()

	// Return an unexpected error
	mockClient.On("QueryNFSe", mock.Anything, validAccessKey(), (*tls.Certificate)(nil)).
		Return(nil, errors.New("unexpected database error"))

	router := setupTestRouter(handler, apiKey)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/nfse/"+validAccessKey(), nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, float64(500), response["status"])

	mockClient.AssertExpectations(t)
}

func TestGetNFSe_NoAPIKeyInContext(t *testing.T) {
	mockClient := new(MockSefinClient)
	handler := createTestHandler(mockClient)

	// No API key in context (nil)
	router := setupTestRouter(handler, nil)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/nfse/"+validAccessKey(), nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Contains(t, response["detail"], "API key")

	mockClient.AssertNotCalled(t, "QueryNFSe")
}

func TestGetNFSe_ResponseMapping(t *testing.T) {
	mockClient := new(MockSefinClient)
	handler := createTestHandler(mockClient)
	apiKey := testAPIKey()

	// Create result with specific values to verify mapping
	expectedResult := &sefin.NFSeQueryResult{
		ChaveAcesso: validAccessKey(),
		Numero:      "123456789",
		DataEmissao: time.Date(2024, 3, 20, 15, 45, 30, 0, time.FixedZone("BRT", -3*60*60)),
		Status:      "cancelled",
		XML:         "<NFSe><infNFSe>test</infNFSe></NFSe>",
		Prestador: sefin.PrestadorData{
			Documento:       "11222333000144",
			Nome:            "Prestador ABC",
			Municipio:       "Rio de Janeiro",
			MunicipioCodigo: "3304557",
		},
		Tomador: &sefin.TomadorData{
			Documento:     "12312312300",
			TipoDocumento: "cpf",
			Nome:          "Joao Silva",
		},
		Servico: sefin.ServicoData{
			CodigoNacional:  "020101",
			Descricao:       "Desenvolvimento de software",
			LocalPrestacao:  "Rio de Janeiro - RJ",
			MunicipioCodigo: "3304557",
		},
		Valores: sefin.ValoresData{
			ValorServico: 5000.50,
			BaseCalculo:  4500.00,
			Aliquota:     2.00,
			ValorISSQN:   90.00,
			ValorLiquido: 4410.00,
		},
	}

	mockClient.On("QueryNFSe", mock.Anything, validAccessKey(), (*tls.Certificate)(nil)).
		Return(expectedResult, nil)

	router := setupTestRouter(handler, apiKey)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/nfse/"+validAccessKey(), nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	// Verify all mapped fields
	assert.Equal(t, validAccessKey(), response["chave_acesso"])
	assert.Equal(t, "123456789", response["numero"])
	assert.Equal(t, "cancelled", response["status"])
	assert.NotEmpty(t, response["data_emissao"])

	// Verify prestador mapping
	prestador := response["prestador"].(map[string]interface{})
	assert.Equal(t, "11222333000144", prestador["documento"])
	assert.Equal(t, "Prestador ABC", prestador["nome"])
	assert.Equal(t, "Rio de Janeiro", prestador["municipio"])

	// Verify tomador mapping
	tomador := response["tomador"].(map[string]interface{})
	assert.Equal(t, "Joao Silva", tomador["nome"])

	// Verify servico mapping
	servico := response["servico"].(map[string]interface{})
	assert.Equal(t, "020101", servico["codigo_nacional"])
	assert.Equal(t, "Desenvolvimento de software", servico["descricao"])
	assert.Equal(t, "Rio de Janeiro - RJ", servico["local_prestacao"])

	// Verify valores mapping
	valores := response["valores"].(map[string]interface{})
	assert.Equal(t, float64(5000.50), valores["valor_servico"])
	assert.Equal(t, float64(4500), valores["base_calculo"])
	assert.Equal(t, float64(4410), valores["valor_liquido"])
	assert.Equal(t, float64(2), valores["aliquota"])
	assert.Equal(t, float64(90), valores["valor_issqn"])

	mockClient.AssertExpectations(t)
}

func TestGetNFSe_OptionalTaxValues(t *testing.T) {
	tests := []struct {
		name            string
		aliquota        float64
		valorISSQN      float64
		expectAliquota  bool
		expectValorISSQ bool
	}{
		{
			name:            "both tax values present",
			aliquota:        5.00,
			valorISSQN:      50.00,
			expectAliquota:  true,
			expectValorISSQ: true,
		},
		{
			name:            "zero tax values - exempt",
			aliquota:        0,
			valorISSQN:      0,
			expectAliquota:  false,
			expectValorISSQ: false,
		},
		{
			name:            "only aliquota present",
			aliquota:        2.00,
			valorISSQN:      0,
			expectAliquota:  true,
			expectValorISSQ: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(MockSefinClient)
			handler := createTestHandler(mockClient)
			apiKey := testAPIKey()

			expectedResult := createSampleNFSeResult()
			expectedResult.Valores.Aliquota = tt.aliquota
			expectedResult.Valores.ValorISSQN = tt.valorISSQN

			mockClient.On("QueryNFSe", mock.Anything, validAccessKey(), (*tls.Certificate)(nil)).
				Return(expectedResult, nil)

			router := setupTestRouter(handler, apiKey)
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/v1/nfse/"+validAccessKey(), nil)

			router.ServeHTTP(w, req)

			assert.Equal(t, http.StatusOK, w.Code)

			var response map[string]interface{}
			err := json.Unmarshal(w.Body.Bytes(), &response)
			require.NoError(t, err)

			valores := response["valores"].(map[string]interface{})

			_, hasAliquota := valores["aliquota"]
			_, hasValorISSQN := valores["valor_issqn"]

			assert.Equal(t, tt.expectAliquota, hasAliquota, "aliquota presence mismatch")
			assert.Equal(t, tt.expectValorISSQ, hasValorISSQN, "valor_issqn presence mismatch")

			mockClient.AssertExpectations(t)
		})
	}
}

// ================================================================================
// GetEvents Tests
// ================================================================================

func TestGetEvents_Success(t *testing.T) {
	mockClient := new(MockSefinClient)
	handler := createTestHandler(mockClient)
	apiKey := testAPIKey()

	expectedResult := createSampleEventsResult()

	mockClient.On("QueryEvents", mock.Anything, validAccessKey(), (*tls.Certificate)(nil)).
		Return(expectedResult, nil)

	router := setupTestRouter(handler, apiKey)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/nfse/"+validAccessKey()+"/eventos", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, validAccessKey(), response["chave_acesso"])
	assert.Equal(t, float64(2), response["total"])

	eventos := response["eventos"].([]interface{})
	assert.Len(t, eventos, 2)

	// Verify first event
	event1 := eventos[0].(map[string]interface{})
	assert.Equal(t, "EMISSAO", event1["tipo"])
	assert.Equal(t, float64(1), event1["sequencia"])
	assert.NotEmpty(t, event1["xml"])

	// Verify second event
	event2 := eventos[1].(map[string]interface{})
	assert.Equal(t, "e101101", event2["tipo"])
	assert.Equal(t, float64(2), event2["sequencia"])

	mockClient.AssertExpectations(t)
}

func TestGetEvents_EmptyList(t *testing.T) {
	mockClient := new(MockSefinClient)
	handler := createTestHandler(mockClient)
	apiKey := testAPIKey()

	// Return empty events list (NFS-e exists but has no events)
	expectedResult := &sefin.EventsQueryResult{
		ChaveAcesso: validAccessKey(),
		Events:      []sefin.EventData{},
	}

	mockClient.On("QueryEvents", mock.Anything, validAccessKey(), (*tls.Certificate)(nil)).
		Return(expectedResult, nil)

	router := setupTestRouter(handler, apiKey)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/nfse/"+validAccessKey()+"/eventos", nil)

	router.ServeHTTP(w, req)

	// Should return 200 with empty array, NOT 404
	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, validAccessKey(), response["chave_acesso"])
	assert.Equal(t, float64(0), response["total"])

	eventos := response["eventos"].([]interface{})
	assert.Empty(t, eventos)
	assert.NotNil(t, eventos) // Should be empty array, not nil

	mockClient.AssertExpectations(t)
}

func TestGetEvents_FilterByEventType(t *testing.T) {
	mockClient := new(MockSefinClient)
	handler := createTestHandler(mockClient)
	apiKey := testAPIKey()

	expectedResult := createSampleEventsResult()

	mockClient.On("QueryEvents", mock.Anything, validAccessKey(), (*tls.Certificate)(nil)).
		Return(expectedResult, nil)

	router := setupTestRouter(handler, apiKey)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/nfse/"+validAccessKey()+"/eventos?tipo=e101101", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	// Should only return filtered events
	assert.Equal(t, float64(1), response["total"])

	eventos := response["eventos"].([]interface{})
	assert.Len(t, eventos, 1)

	event := eventos[0].(map[string]interface{})
	assert.Equal(t, "e101101", event["tipo"])

	mockClient.AssertExpectations(t)
}

func TestGetEvents_FilterNoMatch(t *testing.T) {
	mockClient := new(MockSefinClient)
	handler := createTestHandler(mockClient)
	apiKey := testAPIKey()

	expectedResult := createSampleEventsResult()

	mockClient.On("QueryEvents", mock.Anything, validAccessKey(), (*tls.Certificate)(nil)).
		Return(expectedResult, nil)

	router := setupTestRouter(handler, apiKey)
	w := httptest.NewRecorder()
	// Filter for event type that doesn't exist
	req, _ := http.NewRequest("GET", "/v1/nfse/"+validAccessKey()+"/eventos?tipo=NONEXISTENT", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	// Should return 200 with empty array
	assert.Equal(t, float64(0), response["total"])
	eventos := response["eventos"].([]interface{})
	assert.Empty(t, eventos)

	mockClient.AssertExpectations(t)
}

func TestGetEvents_InvalidAccessKey(t *testing.T) {
	mockClient := new(MockSefinClient)
	handler := createTestHandler(mockClient)
	apiKey := testAPIKey()

	invalidKey := "INVALID123"

	router := setupTestRouter(handler, apiKey)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/nfse/"+invalidKey+"/eventos", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, float64(400), response["status"])

	mockClient.AssertNotCalled(t, "QueryEvents")
}

func TestGetEvents_NFSeNotFound(t *testing.T) {
	mockClient := new(MockSefinClient)
	handler := createTestHandler(mockClient)
	apiKey := testAPIKey()

	mockClient.On("QueryEvents", mock.Anything, validAccessKey(), (*tls.Certificate)(nil)).
		Return(nil, sefin.ErrNFSeNotFound)

	router := setupTestRouter(handler, apiKey)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/nfse/"+validAccessKey()+"/eventos", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, float64(404), response["status"])
	assert.Contains(t, response["detail"], "not found")

	mockClient.AssertExpectations(t)
}

func TestGetEvents_ServiceUnavailable(t *testing.T) {
	mockClient := new(MockSefinClient)
	handler := createTestHandler(mockClient)
	apiKey := testAPIKey()

	mockClient.On("QueryEvents", mock.Anything, validAccessKey(), (*tls.Certificate)(nil)).
		Return(nil, sefin.ErrServiceUnavailable)

	router := setupTestRouter(handler, apiKey)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/nfse/"+validAccessKey()+"/eventos", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	mockClient.AssertExpectations(t)
}

func TestGetEvents_Timeout(t *testing.T) {
	mockClient := new(MockSefinClient)
	handler := createTestHandler(mockClient)
	apiKey := testAPIKey()

	mockClient.On("QueryEvents", mock.Anything, validAccessKey(), (*tls.Certificate)(nil)).
		Return(nil, sefin.ErrTimeout)

	router := setupTestRouter(handler, apiKey)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/nfse/"+validAccessKey()+"/eventos", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusGatewayTimeout, w.Code)

	mockClient.AssertExpectations(t)
}

func TestGetEvents_ContextDeadlineExceeded(t *testing.T) {
	mockClient := new(MockSefinClient)
	handler := createTestHandler(mockClient)
	apiKey := testAPIKey()

	mockClient.On("QueryEvents", mock.Anything, validAccessKey(), (*tls.Certificate)(nil)).
		Return(nil, context.DeadlineExceeded)

	router := setupTestRouter(handler, apiKey)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/nfse/"+validAccessKey()+"/eventos", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusGatewayTimeout, w.Code)

	mockClient.AssertExpectations(t)
}

func TestGetEvents_InternalError(t *testing.T) {
	mockClient := new(MockSefinClient)
	handler := createTestHandler(mockClient)
	apiKey := testAPIKey()

	mockClient.On("QueryEvents", mock.Anything, validAccessKey(), (*tls.Certificate)(nil)).
		Return(nil, errors.New("unexpected error"))

	router := setupTestRouter(handler, apiKey)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/nfse/"+validAccessKey()+"/eventos", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	mockClient.AssertExpectations(t)
}

func TestGetEvents_NoAPIKeyInContext(t *testing.T) {
	mockClient := new(MockSefinClient)
	handler := createTestHandler(mockClient)

	router := setupTestRouter(handler, nil)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/nfse/"+validAccessKey()+"/eventos", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	mockClient.AssertNotCalled(t, "QueryEvents")
}

func TestGetEvents_EventDescriptionMapping(t *testing.T) {
	mockClient := new(MockSefinClient)
	handler := createTestHandler(mockClient)
	apiKey := testAPIKey()

	// Create event without description - should use EventTypeDescriptions
	expectedResult := &sefin.EventsQueryResult{
		ChaveAcesso: validAccessKey(),
		Events: []sefin.EventData{
			{
				Tipo:      "e101101",
				Descricao: "", // Empty - should be populated from EventTypeDescriptions
				Sequencia: 1,
				Data:      time.Now(),
				XML:       "<evento/>",
			},
			{
				Tipo:      "UNKNOWN_TYPE",
				Descricao: "", // Empty and unknown - should use tipo as description
				Sequencia: 2,
				Data:      time.Now(),
				XML:       "<evento/>",
			},
		},
	}

	mockClient.On("QueryEvents", mock.Anything, validAccessKey(), (*tls.Certificate)(nil)).
		Return(expectedResult, nil)

	router := setupTestRouter(handler, apiKey)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/nfse/"+validAccessKey()+"/eventos", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	eventos := response["eventos"].([]interface{})

	// First event should have description from EventTypeDescriptions
	event1 := eventos[0].(map[string]interface{})
	assert.Equal(t, "Cancelamento de NFS-e", event1["descricao"])

	// Second event should use tipo as description (unknown type)
	event2 := eventos[1].(map[string]interface{})
	assert.Equal(t, "UNKNOWN_TYPE", event2["descricao"])

	mockClient.AssertExpectations(t)
}

func TestGetEvents_PreservesExistingDescription(t *testing.T) {
	mockClient := new(MockSefinClient)
	handler := createTestHandler(mockClient)
	apiKey := testAPIKey()

	// Create event with existing description
	expectedResult := &sefin.EventsQueryResult{
		ChaveAcesso: validAccessKey(),
		Events: []sefin.EventData{
			{
				Tipo:      "e101101",
				Descricao: "Custom cancellation description", // Has description - should not be overwritten
				Sequencia: 1,
				Data:      time.Now(),
				XML:       "<evento/>",
			},
		},
	}

	mockClient.On("QueryEvents", mock.Anything, validAccessKey(), (*tls.Certificate)(nil)).
		Return(expectedResult, nil)

	router := setupTestRouter(handler, apiKey)
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/nfse/"+validAccessKey()+"/eventos", nil)

	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	eventos := response["eventos"].([]interface{})
	event := eventos[0].(map[string]interface{})

	// Should preserve existing description
	assert.Equal(t, "Custom cancellation description", event["descricao"])

	mockClient.AssertExpectations(t)
}

// ================================================================================
// Table-Driven Tests for Access Key Validation
// ================================================================================

func TestGetNFSe_AccessKeyValidation_TableDriven(t *testing.T) {
	tests := []struct {
		name           string
		accessKey      string
		expectedStatus int
		expectedError  string
	}{
		{
			name:           "valid access key",
			accessKey:      "NFSe3550308202601081123456789012300000000000012310",
			expectedStatus: http.StatusOK,
			expectedError:  "",
		},
		{
			name:           "empty access key (whitespace)",
			accessKey:      "%20%20%20",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "Access key is required",
		},
		{
			name:           "too short",
			accessKey:      "NFSe1234567890",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "exactly 50 characters",
		},
		{
			name:           "wrong prefix lowercase",
			accessKey:      "nfse3550308202601081123456789012300000000000012310",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "start with 'NFSe'",
		},
		{
			name:           "wrong prefix different",
			accessKey:      "NFSE3550308202601081123456789012300000000000012310",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "start with 'NFSe'",
		},
		{
			name:           "special characters in key",
			accessKey:      "NFSe3550308-02601081123456789012300000000000012310",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "alphanumeric characters",
		},
		{
			name:           "spaces in key",
			accessKey:      "NFSe3550308 02601081123456789012300000000000012310",
			expectedStatus: http.StatusBadRequest,
			expectedError:  "alphanumeric characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(MockSefinClient)
			handler := createTestHandler(mockClient)
			apiKey := testAPIKey()

			// Only setup mock for valid keys
			if tt.expectedStatus == http.StatusOK {
				mockClient.On("QueryNFSe", mock.Anything, mock.AnythingOfType("string"), (*tls.Certificate)(nil)).
					Return(createSampleNFSeResult(), nil)
			}

			router := setupTestRouter(handler, apiKey)
			w := httptest.NewRecorder()
			req, _ := http.NewRequest("GET", "/v1/nfse/"+tt.accessKey, nil)

			router.ServeHTTP(w, req)

			assert.Equal(t, tt.expectedStatus, w.Code)

			if tt.expectedError != "" {
				var response map[string]interface{}
				err := json.Unmarshal(w.Body.Bytes(), &response)
				require.NoError(t, err)
				assert.Contains(t, response["detail"], tt.expectedError)
			}

			mockClient.AssertExpectations(t)
		})
	}
}

// ================================================================================
// Helper Function Tests
// ================================================================================

func TestMaskAccessKey(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "standard 50 char key",
			input:    "NFSe3550308202601081123456789012300000000000012310",
			expected: "NFSe355030...2310",
		},
		{
			name:     "short key",
			input:    "NFSe123",
			expected: "NFSe123",
		},
		{
			name:     "exactly 14 chars",
			input:    "NFSe1234567890",
			expected: "NFSe1234567890",
		},
		{
			name:     "15 chars - just over threshold",
			input:    "NFSe12345678901",
			expected: "NFSe123456...8901",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := maskAccessKey(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFormatDateTime(t *testing.T) {
	tests := []struct {
		name          string
		input         time.Time
		expectEmpty   bool
		expectedYear  string
		expectedMonth string
		expectedDay   string
	}{
		{
			name:        "zero time",
			input:       time.Time{},
			expectEmpty: true,
		},
		{
			name:          "valid time",
			input:         time.Date(2024, 1, 15, 10, 30, 45, 0, time.FixedZone("BRT", -3*60*60)),
			expectEmpty:   false,
			expectedYear:  "2024",
			expectedMonth: "01",
			expectedDay:   "15",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatDateTime(tt.input)
			if tt.expectEmpty {
				assert.Equal(t, "", result)
			} else {
				// The function uses a hardcoded -03:00 offset for Brazil timezone
				// Verify the date components are formatted correctly
				assert.Contains(t, result, tt.expectedYear)
				assert.Contains(t, result, "-"+tt.expectedMonth+"-")
				assert.Contains(t, result, "-"+tt.expectedDay+"T")
				// Verify it ends with timezone offset format
				assert.Regexp(t, `.*[+-]\d{2}:\d{2}$`, result)
			}
		})
	}
}

func TestFormatAccessKeyError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "empty key error",
			err:      errors.New("access key cannot be empty"),
			expected: "access key cannot be empty",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatAccessKeyError(tt.err)
			assert.Contains(t, result, tt.expected)
		})
	}
}

// ================================================================================
// Logging Tests
// ================================================================================

func TestGetNFSe_SuccessWithLogger(t *testing.T) {
	mockClient := new(MockSefinClient)

	// Create handler with a logger
	handler := NewQueryHandler(QueryHandlerConfig{
		SefinClient: mockClient,
		BaseURL:     "http://localhost:8080",
		Logger:      nil, // Even with nil logger, the code path is tested
	})
	apiKey := testAPIKey()

	expectedResult := createSampleNFSeResult()

	mockClient.On("QueryNFSe", mock.Anything, validAccessKey(), (*tls.Certificate)(nil)).
		Return(expectedResult, nil)

	// Add request_id to context to test that logging path
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(apiKeyContextKey, apiKey)
		c.Set("request_id", "test-request-123")
		c.Next()
	})
	r.GET("/v1/nfse/:chaveAcesso", handler.GetNFSe)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/nfse/"+validAccessKey(), nil)

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	mockClient.AssertExpectations(t)
}

func TestGetEvents_SuccessWithRequestID(t *testing.T) {
	mockClient := new(MockSefinClient)
	handler := createTestHandler(mockClient)
	apiKey := testAPIKey()

	expectedResult := createSampleEventsResult()

	mockClient.On("QueryEvents", mock.Anything, validAccessKey(), (*tls.Certificate)(nil)).
		Return(expectedResult, nil)

	// Create router with request_id in context
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(apiKeyContextKey, apiKey)
		c.Set("request_id", "test-request-456")
		c.Next()
	})
	r.GET("/v1/nfse/:chaveAcesso/eventos", handler.GetEvents)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/nfse/"+validAccessKey()+"/eventos", nil)

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	mockClient.AssertExpectations(t)
}

func TestGetNFSe_WithActiveLogger(t *testing.T) {
	mockClient := new(MockSefinClient)

	// Create a buffer to capture log output
	var logBuffer bytes.Buffer
	testLogger := log.New(&logBuffer, "", 0)

	// Create handler with an actual logger
	handler := NewQueryHandler(QueryHandlerConfig{
		SefinClient: mockClient,
		BaseURL:     "http://localhost:8080",
		Logger:      testLogger,
	})
	apiKey := testAPIKey()

	expectedResult := createSampleNFSeResult()

	mockClient.On("QueryNFSe", mock.Anything, validAccessKey(), (*tls.Certificate)(nil)).
		Return(expectedResult, nil)

	// Create router with request_id to exercise all logging paths
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(apiKeyContextKey, apiKey)
		c.Set("request_id", "test-log-request")
		c.Next()
	})
	r.GET("/v1/nfse/:chaveAcesso", handler.GetNFSe)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/nfse/"+validAccessKey(), nil)

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify logs were written
	logOutput := logBuffer.String()
	assert.Contains(t, logOutput, "nfse_query_start")
	assert.Contains(t, logOutput, "nfse_query_success")
	assert.Contains(t, logOutput, "test-log-request") // request_id

	mockClient.AssertExpectations(t)
}

func TestGetNFSe_LoggingOnError(t *testing.T) {
	mockClient := new(MockSefinClient)

	var logBuffer bytes.Buffer
	testLogger := log.New(&logBuffer, "", 0)

	handler := NewQueryHandler(QueryHandlerConfig{
		SefinClient: mockClient,
		BaseURL:     "http://localhost:8080",
		Logger:      testLogger,
	})
	apiKey := testAPIKey()

	// Return not found error
	mockClient.On("QueryNFSe", mock.Anything, validAccessKey(), (*tls.Certificate)(nil)).
		Return(nil, sefin.ErrNFSeNotFound)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(apiKeyContextKey, apiKey)
		c.Next()
	})
	r.GET("/v1/nfse/:chaveAcesso", handler.GetNFSe)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/nfse/"+validAccessKey(), nil)

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	// Verify error was logged
	logOutput := logBuffer.String()
	assert.Contains(t, logOutput, "nfse_query_not_found")

	mockClient.AssertExpectations(t)
}

func TestGetNFSe_LoggingOnInvalidKey(t *testing.T) {
	mockClient := new(MockSefinClient)

	var logBuffer bytes.Buffer
	testLogger := log.New(&logBuffer, "", 0)

	handler := NewQueryHandler(QueryHandlerConfig{
		SefinClient: mockClient,
		BaseURL:     "http://localhost:8080",
		Logger:      testLogger,
	})
	apiKey := testAPIKey()

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(apiKeyContextKey, apiKey)
		c.Next()
	})
	r.GET("/v1/nfse/:chaveAcesso", handler.GetNFSe)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/nfse/INVALID", nil)

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	// Verify invalid key was logged
	logOutput := logBuffer.String()
	assert.Contains(t, logOutput, "nfse_query_invalid_key")

	mockClient.AssertNotCalled(t, "QueryNFSe")
}

func TestGetEvents_WithActiveLogger(t *testing.T) {
	mockClient := new(MockSefinClient)

	var logBuffer bytes.Buffer
	testLogger := log.New(&logBuffer, "", 0)

	handler := NewQueryHandler(QueryHandlerConfig{
		SefinClient: mockClient,
		BaseURL:     "http://localhost:8080",
		Logger:      testLogger,
	})
	apiKey := testAPIKey()

	expectedResult := createSampleEventsResult()

	mockClient.On("QueryEvents", mock.Anything, validAccessKey(), (*tls.Certificate)(nil)).
		Return(expectedResult, nil)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(apiKeyContextKey, apiKey)
		c.Set("request_id", "events-log-test")
		c.Next()
	})
	r.GET("/v1/nfse/:chaveAcesso/eventos", handler.GetEvents)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/nfse/"+validAccessKey()+"/eventos?tipo=e101101", nil)

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)

	// Verify logs
	logOutput := logBuffer.String()
	assert.Contains(t, logOutput, "events_query_start")
	assert.Contains(t, logOutput, "events_query_success")
	assert.Contains(t, logOutput, "events-log-test")

	mockClient.AssertExpectations(t)
}

func TestGetEvents_LoggingOnNFSeNotFound(t *testing.T) {
	mockClient := new(MockSefinClient)

	var logBuffer bytes.Buffer
	testLogger := log.New(&logBuffer, "", 0)

	handler := NewQueryHandler(QueryHandlerConfig{
		SefinClient: mockClient,
		BaseURL:     "http://localhost:8080",
		Logger:      testLogger,
	})
	apiKey := testAPIKey()

	mockClient.On("QueryEvents", mock.Anything, validAccessKey(), (*tls.Certificate)(nil)).
		Return(nil, sefin.ErrNFSeNotFound)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(apiKeyContextKey, apiKey)
		c.Next()
	})
	r.GET("/v1/nfse/:chaveAcesso/eventos", handler.GetEvents)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/nfse/"+validAccessKey()+"/eventos", nil)

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)

	logOutput := logBuffer.String()
	assert.Contains(t, logOutput, "events_query_nfse_not_found")

	mockClient.AssertExpectations(t)
}

func TestGetEvents_LoggingOnServiceUnavailable(t *testing.T) {
	mockClient := new(MockSefinClient)

	var logBuffer bytes.Buffer
	testLogger := log.New(&logBuffer, "", 0)

	handler := NewQueryHandler(QueryHandlerConfig{
		SefinClient: mockClient,
		BaseURL:     "http://localhost:8080",
		Logger:      testLogger,
	})
	apiKey := testAPIKey()

	mockClient.On("QueryEvents", mock.Anything, validAccessKey(), (*tls.Certificate)(nil)).
		Return(nil, sefin.ErrServiceUnavailable)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(apiKeyContextKey, apiKey)
		c.Next()
	})
	r.GET("/v1/nfse/:chaveAcesso/eventos", handler.GetEvents)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/nfse/"+validAccessKey()+"/eventos", nil)

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	logOutput := logBuffer.String()
	assert.Contains(t, logOutput, "events_query_service_unavailable")

	mockClient.AssertExpectations(t)
}

func TestGetEvents_LoggingOnTimeout(t *testing.T) {
	mockClient := new(MockSefinClient)

	var logBuffer bytes.Buffer
	testLogger := log.New(&logBuffer, "", 0)

	handler := NewQueryHandler(QueryHandlerConfig{
		SefinClient: mockClient,
		BaseURL:     "http://localhost:8080",
		Logger:      testLogger,
	})
	apiKey := testAPIKey()

	mockClient.On("QueryEvents", mock.Anything, validAccessKey(), (*tls.Certificate)(nil)).
		Return(nil, sefin.ErrTimeout)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(apiKeyContextKey, apiKey)
		c.Next()
	})
	r.GET("/v1/nfse/:chaveAcesso/eventos", handler.GetEvents)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/nfse/"+validAccessKey()+"/eventos", nil)

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusGatewayTimeout, w.Code)

	logOutput := logBuffer.String()
	assert.Contains(t, logOutput, "events_query_timeout")

	mockClient.AssertExpectations(t)
}

func TestGetEvents_LoggingOnContextDeadline(t *testing.T) {
	mockClient := new(MockSefinClient)

	var logBuffer bytes.Buffer
	testLogger := log.New(&logBuffer, "", 0)

	handler := NewQueryHandler(QueryHandlerConfig{
		SefinClient: mockClient,
		BaseURL:     "http://localhost:8080",
		Logger:      testLogger,
	})
	apiKey := testAPIKey()

	mockClient.On("QueryEvents", mock.Anything, validAccessKey(), (*tls.Certificate)(nil)).
		Return(nil, context.DeadlineExceeded)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(apiKeyContextKey, apiKey)
		c.Next()
	})
	r.GET("/v1/nfse/:chaveAcesso/eventos", handler.GetEvents)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/nfse/"+validAccessKey()+"/eventos", nil)

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusGatewayTimeout, w.Code)

	logOutput := logBuffer.String()
	assert.Contains(t, logOutput, "events_query_context_timeout")

	mockClient.AssertExpectations(t)
}

func TestGetEvents_LoggingOnInternalError(t *testing.T) {
	mockClient := new(MockSefinClient)

	var logBuffer bytes.Buffer
	testLogger := log.New(&logBuffer, "", 0)

	handler := NewQueryHandler(QueryHandlerConfig{
		SefinClient: mockClient,
		BaseURL:     "http://localhost:8080",
		Logger:      testLogger,
	})
	apiKey := testAPIKey()

	mockClient.On("QueryEvents", mock.Anything, validAccessKey(), (*tls.Certificate)(nil)).
		Return(nil, errors.New("internal database error"))

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(apiKeyContextKey, apiKey)
		c.Next()
	})
	r.GET("/v1/nfse/:chaveAcesso/eventos", handler.GetEvents)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/nfse/"+validAccessKey()+"/eventos", nil)

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	logOutput := logBuffer.String()
	assert.Contains(t, logOutput, "events_query_error")

	mockClient.AssertExpectations(t)
}

func TestGetEvents_LoggingOnInvalidKey(t *testing.T) {
	mockClient := new(MockSefinClient)

	var logBuffer bytes.Buffer
	testLogger := log.New(&logBuffer, "", 0)

	handler := NewQueryHandler(QueryHandlerConfig{
		SefinClient: mockClient,
		BaseURL:     "http://localhost:8080",
		Logger:      testLogger,
	})
	apiKey := testAPIKey()

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(apiKeyContextKey, apiKey)
		c.Next()
	})
	r.GET("/v1/nfse/:chaveAcesso/eventos", handler.GetEvents)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/nfse/INVALID/eventos", nil)

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)

	logOutput := logBuffer.String()
	assert.Contains(t, logOutput, "events_query_invalid_key")

	mockClient.AssertNotCalled(t, "QueryEvents")
}

func TestGetNFSe_LoggingOnServiceUnavailable(t *testing.T) {
	mockClient := new(MockSefinClient)

	var logBuffer bytes.Buffer
	testLogger := log.New(&logBuffer, "", 0)

	handler := NewQueryHandler(QueryHandlerConfig{
		SefinClient: mockClient,
		BaseURL:     "http://localhost:8080",
		Logger:      testLogger,
	})
	apiKey := testAPIKey()

	mockClient.On("QueryNFSe", mock.Anything, validAccessKey(), (*tls.Certificate)(nil)).
		Return(nil, sefin.ErrServiceUnavailable)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(apiKeyContextKey, apiKey)
		c.Next()
	})
	r.GET("/v1/nfse/:chaveAcesso", handler.GetNFSe)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/nfse/"+validAccessKey(), nil)

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)

	logOutput := logBuffer.String()
	assert.Contains(t, logOutput, "nfse_query_service_unavailable")

	mockClient.AssertExpectations(t)
}

func TestGetNFSe_LoggingOnTimeout(t *testing.T) {
	mockClient := new(MockSefinClient)

	var logBuffer bytes.Buffer
	testLogger := log.New(&logBuffer, "", 0)

	handler := NewQueryHandler(QueryHandlerConfig{
		SefinClient: mockClient,
		BaseURL:     "http://localhost:8080",
		Logger:      testLogger,
	})
	apiKey := testAPIKey()

	mockClient.On("QueryNFSe", mock.Anything, validAccessKey(), (*tls.Certificate)(nil)).
		Return(nil, sefin.ErrTimeout)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(apiKeyContextKey, apiKey)
		c.Next()
	})
	r.GET("/v1/nfse/:chaveAcesso", handler.GetNFSe)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/nfse/"+validAccessKey(), nil)

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusGatewayTimeout, w.Code)

	logOutput := logBuffer.String()
	assert.Contains(t, logOutput, "nfse_query_timeout")

	mockClient.AssertExpectations(t)
}

func TestGetNFSe_LoggingOnContextDeadline(t *testing.T) {
	mockClient := new(MockSefinClient)

	var logBuffer bytes.Buffer
	testLogger := log.New(&logBuffer, "", 0)

	handler := NewQueryHandler(QueryHandlerConfig{
		SefinClient: mockClient,
		BaseURL:     "http://localhost:8080",
		Logger:      testLogger,
	})
	apiKey := testAPIKey()

	mockClient.On("QueryNFSe", mock.Anything, validAccessKey(), (*tls.Certificate)(nil)).
		Return(nil, context.DeadlineExceeded)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(apiKeyContextKey, apiKey)
		c.Next()
	})
	r.GET("/v1/nfse/:chaveAcesso", handler.GetNFSe)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/nfse/"+validAccessKey(), nil)

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusGatewayTimeout, w.Code)

	logOutput := logBuffer.String()
	assert.Contains(t, logOutput, "nfse_query_context_timeout")

	mockClient.AssertExpectations(t)
}

func TestGetNFSe_LoggingOnForbidden(t *testing.T) {
	mockClient := new(MockSefinClient)

	var logBuffer bytes.Buffer
	testLogger := log.New(&logBuffer, "", 0)

	handler := NewQueryHandler(QueryHandlerConfig{
		SefinClient: mockClient,
		BaseURL:     "http://localhost:8080",
		Logger:      testLogger,
	})
	apiKey := testAPIKey()

	mockClient.On("QueryNFSe", mock.Anything, validAccessKey(), (*tls.Certificate)(nil)).
		Return(nil, sefin.ErrForbidden)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(apiKeyContextKey, apiKey)
		c.Next()
	})
	r.GET("/v1/nfse/:chaveAcesso", handler.GetNFSe)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/nfse/"+validAccessKey(), nil)

	r.ServeHTTP(w, req)

	// Forbidden is mapped to 404 to prevent information leakage
	assert.Equal(t, http.StatusNotFound, w.Code)

	logOutput := logBuffer.String()
	assert.Contains(t, logOutput, "nfse_query_forbidden")

	mockClient.AssertExpectations(t)
}

func TestGetNFSe_LoggingOnInternalError(t *testing.T) {
	mockClient := new(MockSefinClient)

	var logBuffer bytes.Buffer
	testLogger := log.New(&logBuffer, "", 0)

	handler := NewQueryHandler(QueryHandlerConfig{
		SefinClient: mockClient,
		BaseURL:     "http://localhost:8080",
		Logger:      testLogger,
	})
	apiKey := testAPIKey()

	mockClient.On("QueryNFSe", mock.Anything, validAccessKey(), (*tls.Certificate)(nil)).
		Return(nil, errors.New("database connection failed"))

	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(apiKeyContextKey, apiKey)
		c.Next()
	})
	r.GET("/v1/nfse/:chaveAcesso", handler.GetNFSe)

	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/v1/nfse/"+validAccessKey(), nil)

	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusInternalServerError, w.Code)

	logOutput := logBuffer.String()
	assert.Contains(t, logOutput, "nfse_query_error")

	mockClient.AssertExpectations(t)
}

// ================================================================================
// GatewayTimeout Function Test
// ================================================================================

func TestGatewayTimeout(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest("GET", "/test", nil)

	GatewayTimeout(c, "Test timeout message")

	assert.Equal(t, http.StatusGatewayTimeout, w.Code)

	var response map[string]interface{}
	err := json.Unmarshal(w.Body.Bytes(), &response)
	require.NoError(t, err)

	assert.Equal(t, float64(504), response["status"])
	assert.Equal(t, "Gateway Timeout", response["title"])
	assert.Equal(t, "Test timeout message", response["detail"])
	assert.Equal(t, "https://api.nfse.gov.br/problems/gateway-timeout", response["type"])
}
