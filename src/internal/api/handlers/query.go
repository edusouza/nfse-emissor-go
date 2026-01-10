// Package handlers provides HTTP request handlers for the NFS-e API.
package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/eduardo/nfse-nacional/internal/domain/query"
	"github.com/eduardo/nfse-nacional/internal/infrastructure/sefin"
)

// QueryHandler handles NFS-e query requests.
type QueryHandler struct {
	sefinClient sefin.SefinClient
	baseURL     string
	logger      *log.Logger
}

// QueryHandlerConfig configures the query handler.
type QueryHandlerConfig struct {
	// SefinClient is the client for communicating with the government API.
	SefinClient sefin.SefinClient

	// BaseURL is the base URL for constructing resource URLs.
	BaseURL string

	// Logger is an optional logger for debugging (can be nil).
	Logger *log.Logger
}

// NewQueryHandler creates a new query handler.
func NewQueryHandler(config QueryHandlerConfig) *QueryHandler {
	return &QueryHandler{
		sefinClient: config.SefinClient,
		baseURL:     config.BaseURL,
		logger:      config.Logger,
	}
}

// GetNFSe handles GET /v1/nfse/:chaveAcesso requests.
// It retrieves an NFS-e document by its 50-character access key.
//
// Responses:
//   - 200 OK: NFS-e found and returned successfully
//   - 400 Bad Request: Invalid access key format
//   - 404 Not Found: NFS-e not found
//   - 503 Service Unavailable: Government API unavailable
//   - 504 Gateway Timeout: Government API timeout
func (h *QueryHandler) GetNFSe(c *gin.Context) {
	start := time.Now()

	// Get API key from context (set by auth middleware)
	apiKey := getAPIKeyFromContext(c)
	if apiKey == nil {
		InternalError(c, "Failed to retrieve API key from context")
		return
	}

	// Get access key from path parameter
	chaveAcesso := c.Param("chaveAcesso")

	// Log the query request
	h.logQuery(c, "nfse_query_start", map[string]interface{}{
		"chave_acesso": maskAccessKey(chaveAcesso),
		"api_key_id":   apiKey.KeyPrefix,
	})

	// Validate access key format (T015)
	if err := query.ValidateAccessKey(chaveAcesso); err != nil {
		h.logQuery(c, "nfse_query_invalid_key", map[string]interface{}{
			"error":  err.Error(),
			"length": len(chaveAcesso),
		})

		// Return 400 with specific validation error
		BadRequest(c, formatAccessKeyError(err))
		return
	}

	// Call SEFIN API to retrieve the NFS-e
	// Note: For this query endpoint, we use the mock client or production client
	// depending on the environment. The certificate is not required for public
	// NFS-e queries by access key.
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	result, err := h.sefinClient.QueryNFSe(ctx, chaveAcesso, nil)
	if err != nil {
		h.handleQueryError(c, err, chaveAcesso, start)
		return
	}

	// Map SEFIN response to API response DTO (T016)
	response := h.mapToQueryResponse(result)

	// Log successful query
	h.logQuery(c, "nfse_query_success", map[string]interface{}{
		"chave_acesso": maskAccessKey(chaveAcesso),
		"numero":       result.Numero,
		"status":       result.Status,
		"latency_ms":   time.Since(start).Milliseconds(),
	})

	c.JSON(http.StatusOK, response)
}

// handleQueryError processes errors from the SEFIN API and returns appropriate HTTP responses (T017).
func (h *QueryHandler) handleQueryError(c *gin.Context, err error, chaveAcesso string, start time.Time) {
	latencyMs := time.Since(start).Milliseconds()

	// Check for specific SEFIN errors
	switch {
	case errors.Is(err, sefin.ErrNFSeNotFound):
		// 404 Not Found
		h.logQuery(c, "nfse_query_not_found", map[string]interface{}{
			"chave_acesso": maskAccessKey(chaveAcesso),
			"latency_ms":   latencyMs,
		})
		NotFound(c, "NFS-e not found with the specified access key")
		return

	case errors.Is(err, sefin.ErrServiceUnavailable):
		// 503 Service Unavailable
		h.logQuery(c, "nfse_query_service_unavailable", map[string]interface{}{
			"chave_acesso": maskAccessKey(chaveAcesso),
			"error":        err.Error(),
			"latency_ms":   latencyMs,
		})
		ServiceUnavailable(c, "Government service is temporarily unavailable. Please try again later.")
		return

	case errors.Is(err, sefin.ErrTimeout):
		// 504 Gateway Timeout
		h.logQuery(c, "nfse_query_timeout", map[string]interface{}{
			"chave_acesso": maskAccessKey(chaveAcesso),
			"error":        err.Error(),
			"latency_ms":   latencyMs,
		})
		GatewayTimeout(c, "Government service request timed out. Please try again later.")
		return

	case errors.Is(err, sefin.ErrForbidden):
		// 403 Forbidden - treated as 404 to prevent information leakage
		h.logQuery(c, "nfse_query_forbidden", map[string]interface{}{
			"chave_acesso": maskAccessKey(chaveAcesso),
			"error":        err.Error(),
			"latency_ms":   latencyMs,
		})
		NotFound(c, "NFS-e not found with the specified access key")
		return

	case errors.Is(err, context.DeadlineExceeded):
		// Context deadline exceeded - 504
		h.logQuery(c, "nfse_query_context_timeout", map[string]interface{}{
			"chave_acesso": maskAccessKey(chaveAcesso),
			"error":        err.Error(),
			"latency_ms":   latencyMs,
		})
		GatewayTimeout(c, "Request timed out while waiting for government service response.")
		return

	default:
		// 500 Internal Server Error for unexpected errors
		h.logQuery(c, "nfse_query_error", map[string]interface{}{
			"chave_acesso": maskAccessKey(chaveAcesso),
			"error":        err.Error(),
			"latency_ms":   latencyMs,
		})
		InternalError(c, "An error occurred while retrieving the NFS-e")
		return
	}
}

// mapToQueryResponse maps a SEFIN NFSeQueryResult to an API response DTO (T016).
func (h *QueryHandler) mapToQueryResponse(result *sefin.NFSeQueryResult) *query.NFSeQueryResponse {
	response := &query.NFSeQueryResponse{
		ChaveAcesso: result.ChaveAcesso,
		Numero:      result.Numero,
		DataEmissao: formatDateTime(result.DataEmissao),
		Status:      result.Status,
		Prestador: query.PrestadorInfo{
			Documento: result.Prestador.Documento,
			Nome:      result.Prestador.Nome,
			Municipio: result.Prestador.Municipio,
		},
		Servico: query.ServicoInfo{
			CodigoNacional: result.Servico.CodigoNacional,
			Descricao:      result.Servico.Descricao,
			LocalPrestacao: result.Servico.LocalPrestacao,
		},
		Valores: query.ValoresInfo{
			ValorServico: result.Valores.ValorServico,
			BaseCalculo:  result.Valores.BaseCalculo,
			ValorLiquido: result.Valores.ValorLiquido,
		},
		XML: result.XML,
	}

	// Set optional tax values
	if result.Valores.Aliquota > 0 {
		response.Valores.SetAliquota(result.Valores.Aliquota)
	}
	if result.Valores.ValorISSQN > 0 {
		response.Valores.SetValorISSQN(result.Valores.ValorISSQN)
	}

	// Add taker if present
	if result.Tomador != nil {
		response.Tomador = query.NewTomadorInfo(result.Tomador.Nome)
		if result.Tomador.Documento != "" {
			response.Tomador.SetDocumento(result.Tomador.Documento)
		}
	}

	return response
}

// formatDateTime formats a time.Time to ISO 8601 string with Brazil timezone offset (-03:00).
// Note: This always uses the Brazil timezone offset regardless of the input time's location.
func formatDateTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format("2006-01-02T15:04:05-03:00")
}

// formatAccessKeyError formats an access key validation error for the API response.
func formatAccessKeyError(err error) string {
	switch {
	case errors.Is(err, query.ErrAccessKeyEmpty):
		return "Access key is required"
	case errors.Is(err, query.ErrAccessKeyInvalidLength):
		return "Access key must be exactly 50 characters"
	case errors.Is(err, query.ErrAccessKeyInvalidPrefix):
		return "Access key must start with 'NFSe' prefix"
	case errors.Is(err, query.ErrAccessKeyInvalidCharacters):
		return "Access key must contain only alphanumeric characters"
	default:
		return err.Error()
	}
}

// maskAccessKey masks the access key for logging, showing only first 10 and last 4 characters.
func maskAccessKey(key string) string {
	if len(key) <= 14 {
		return key
	}
	return key[:10] + "..." + key[len(key)-4:]
}

// logQuery logs a query operation with structured fields (T019).
func (h *QueryHandler) logQuery(c *gin.Context, event string, fields map[string]interface{}) {
	if h.logger == nil {
		return
	}

	// Add common fields
	fields["event"] = event
	fields["timestamp"] = time.Now().UTC().Format(time.RFC3339)

	// Add request ID if available
	if requestID, exists := c.Get("request_id"); exists {
		fields["request_id"] = requestID
	}

	// Add client IP
	fields["client_ip"] = c.ClientIP()

	// Marshal to JSON for structured logging
	jsonBytes, err := json.Marshal(fields)
	if err != nil {
		h.logger.Printf("failed to marshal log entry: %v", err)
		return
	}

	h.logger.Println(string(jsonBytes))
}

// GetEvents handles GET /v1/nfse/:chaveAcesso/eventos requests.
// It retrieves all events (cancellations, substitutions, etc.) for an NFS-e by its access key.
//
// Query Parameters:
//   - tipo: Optional filter by event type code (e.g., "e101101" for cancellation)
//
// Responses:
//   - 200 OK: Events found and returned successfully (empty list if NFS-e has no events)
//   - 400 Bad Request: Invalid access key format
//   - 404 Not Found: NFS-e not found
//   - 503 Service Unavailable: Government API unavailable
//   - 504 Gateway Timeout: Government API timeout
func (h *QueryHandler) GetEvents(c *gin.Context) {
	start := time.Now()

	// Get API key from context (set by auth middleware)
	apiKey := getAPIKeyFromContext(c)
	if apiKey == nil {
		InternalError(c, "Failed to retrieve API key from context")
		return
	}

	// Get access key from path parameter
	chaveAcesso := c.Param("chaveAcesso")

	// Get optional event type filter
	eventType := c.Query("tipo")

	// Log the events query request (T042)
	h.logQuery(c, "events_query_start", map[string]interface{}{
		"chave_acesso": maskAccessKey(chaveAcesso),
		"api_key_id":   apiKey.KeyPrefix,
		"event_type":   eventType,
	})

	// Validate access key format (T037)
	if err := query.ValidateAccessKey(chaveAcesso); err != nil {
		h.logQuery(c, "events_query_invalid_key", map[string]interface{}{
			"error":  err.Error(),
			"length": len(chaveAcesso),
		})

		BadRequest(c, formatAccessKeyError(err))
		return
	}

	// Call SEFIN API to retrieve events
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	result, err := h.sefinClient.QueryEvents(ctx, chaveAcesso, nil)
	if err != nil {
		h.handleEventsQueryError(c, err, chaveAcesso, start)
		return
	}

	// Map SEFIN response to API response DTO (T039)
	response := h.mapToEventsQueryResponse(result, eventType)

	// Log successful query (T042)
	h.logQuery(c, "events_query_success", map[string]interface{}{
		"chave_acesso":   maskAccessKey(chaveAcesso),
		"total_events":   response.Total,
		"event_type":     eventType,
		"filtered_count": len(response.Eventos),
		"latency_ms":     time.Since(start).Milliseconds(),
	})

	c.JSON(http.StatusOK, response)
}

// handleEventsQueryError processes errors from the SEFIN API for events queries.
func (h *QueryHandler) handleEventsQueryError(c *gin.Context, err error, chaveAcesso string, start time.Time) {
	latencyMs := time.Since(start).Milliseconds()

	// Check for specific SEFIN errors
	switch {
	case errors.Is(err, sefin.ErrNFSeNotFound):
		// 404 Not Found - NFS-e itself doesn't exist
		h.logQuery(c, "events_query_nfse_not_found", map[string]interface{}{
			"chave_acesso": maskAccessKey(chaveAcesso),
			"latency_ms":   latencyMs,
		})
		NotFound(c, "NFS-e not found with the specified access key")
		return

	case errors.Is(err, sefin.ErrServiceUnavailable):
		// 503 Service Unavailable
		h.logQuery(c, "events_query_service_unavailable", map[string]interface{}{
			"chave_acesso": maskAccessKey(chaveAcesso),
			"error":        err.Error(),
			"latency_ms":   latencyMs,
		})
		ServiceUnavailable(c, "Government service is temporarily unavailable. Please try again later.")
		return

	case errors.Is(err, sefin.ErrTimeout):
		// 504 Gateway Timeout
		h.logQuery(c, "events_query_timeout", map[string]interface{}{
			"chave_acesso": maskAccessKey(chaveAcesso),
			"error":        err.Error(),
			"latency_ms":   latencyMs,
		})
		GatewayTimeout(c, "Government service request timed out. Please try again later.")
		return

	case errors.Is(err, context.DeadlineExceeded):
		// Context deadline exceeded - 504
		h.logQuery(c, "events_query_context_timeout", map[string]interface{}{
			"chave_acesso": maskAccessKey(chaveAcesso),
			"error":        err.Error(),
			"latency_ms":   latencyMs,
		})
		GatewayTimeout(c, "Request timed out while waiting for government service response.")
		return

	default:
		// 500 Internal Server Error for unexpected errors
		h.logQuery(c, "events_query_error", map[string]interface{}{
			"chave_acesso": maskAccessKey(chaveAcesso),
			"error":        err.Error(),
			"latency_ms":   latencyMs,
		})
		InternalError(c, "An error occurred while retrieving NFS-e events")
		return
	}
}

// mapToEventsQueryResponse maps a SEFIN EventsQueryResult to an API response DTO (T039).
// If eventType is non-empty, filters the events to only include that type (T038).
// Returns an empty list (not nil) if the NFS-e has no events (T040).
func (h *QueryHandler) mapToEventsQueryResponse(result *sefin.EventsQueryResult, eventType string) *query.EventsQueryResponse {
	eventos := make([]query.EventInfo, 0, len(result.Events))

	for _, evt := range result.Events {
		// Apply event type filter if specified (T038)
		if eventType != "" && evt.Tipo != eventType {
			continue
		}

		// Map event data to DTO
		eventInfo := query.EventInfo{
			Tipo:      evt.Tipo,
			Descricao: evt.Descricao,
			Sequencia: evt.Sequencia,
			Data:      formatDateTime(evt.Data),
			XML:       evt.XML,
		}

		// Use description from EventTypeDescriptions if available and not already set
		if eventInfo.Descricao == "" {
			if desc, ok := query.EventTypeDescriptions[evt.Tipo]; ok {
				eventInfo.Descricao = desc
			} else {
				eventInfo.Descricao = evt.Tipo
			}
		}

		eventos = append(eventos, eventInfo)
	}

	// Return response with empty list if no events (T040)
	return &query.EventsQueryResponse{
		ChaveAcesso: result.ChaveAcesso,
		Total:       len(eventos),
		Eventos:     eventos,
	}
}

// GatewayTimeout responds with a 504 Gateway Timeout error.
func GatewayTimeout(c *gin.Context, detail string) {
	problem := NewProblemDetails(
		"https://api.nfse.gov.br/problems/gateway-timeout",
		"Gateway Timeout",
		http.StatusGatewayTimeout,
	).WithDetail(detail).WithInstance(c.Request.URL.Path)

	problem.Respond(c)
}
