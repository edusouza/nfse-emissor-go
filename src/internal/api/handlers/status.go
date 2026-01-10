// Package handlers provides HTTP request handlers for the NFS-e API.
package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/eduardo/nfse-nacional/internal/domain/emission"
	"github.com/eduardo/nfse-nacional/internal/infrastructure/mongodb"
)

// EmissionRepositoryReader defines the read operations needed by StatusHandler.
// This interface allows for easier testing by enabling mock implementations.
type EmissionRepositoryReader interface {
	FindByRequestID(ctx context.Context, requestID string) (*mongodb.EmissionRequest, error)
	FindByAPIKeyID(ctx context.Context, apiKeyID primitive.ObjectID, params mongodb.PaginationParams) (*mongodb.PaginatedResult, error)
}

// StatusHandler handles NFS-e emission status requests.
type StatusHandler struct {
	emissionRepo EmissionRepositoryReader
	baseURL      string
	logger       *log.Logger
}

// StatusHandlerConfig configures the status handler.
type StatusHandlerConfig struct {
	// EmissionRepo is the repository for emission requests.
	// Can be *mongodb.EmissionRepository or any type implementing EmissionRepositoryReader.
	EmissionRepo EmissionRepositoryReader

	// BaseURL is the base URL for constructing URLs.
	BaseURL string

	// Logger is an optional logger for structured logging (can be nil).
	Logger *log.Logger
}

// NewStatusHandler creates a new status handler.
func NewStatusHandler(config StatusHandlerConfig) *StatusHandler {
	return &StatusHandler{
		emissionRepo: config.EmissionRepo,
		baseURL:      config.BaseURL,
		logger:       config.Logger,
	}
}

// Get handles GET /v1/nfse/status/:requestId requests.
// It returns the current status of an emission request.
func (h *StatusHandler) Get(c *gin.Context) {
	start := time.Now()

	// Get API key from context (set by auth middleware)
	apiKey := getAPIKeyFromContext(c)
	if apiKey == nil {
		InternalError(c, "Failed to retrieve API key from context")
		return
	}

	// Get request ID from path
	requestID := c.Param("requestId")
	if requestID == "" {
		h.logStatus(c, "status_query_invalid", map[string]interface{}{
			"error": "missing_request_id",
		})
		BadRequest(c, "Request ID is required in the path")
		return
	}

	// Log the status query request
	h.logStatus(c, "status_query_start", map[string]interface{}{
		"request_id": requestID,
		"api_key_id": apiKey.KeyPrefix,
	})

	// Find emission request
	emissionReq, err := h.emissionRepo.FindByRequestID(c.Request.Context(), requestID)
	if err != nil {
		if errors.Is(err, mongodb.ErrEmissionRequestNotFound) {
			h.logStatus(c, "status_query_not_found", map[string]interface{}{
				"request_id": requestID,
				"latency_ms": time.Since(start).Milliseconds(),
			})
			NotFound(c, "Emission request not found")
			return
		}
		h.logStatus(c, "status_query_error", map[string]interface{}{
			"request_id": requestID,
			"error":      err.Error(),
			"latency_ms": time.Since(start).Milliseconds(),
		})
		InternalError(c, "Failed to retrieve emission request")
		return
	}

	// Verify ownership - the API key must own this request
	if emissionReq.APIKeyID != apiKey.ID {
		// Return 404 instead of 403 to prevent information leakage
		h.logStatus(c, "status_query_unauthorized", map[string]interface{}{
			"request_id":   requestID,
			"owner_key_id": emissionReq.APIKeyID,
			"latency_ms":   time.Since(start).Milliseconds(),
		})
		NotFound(c, "Emission request not found")
		return
	}

	// Build response
	response := emission.StatusResponse{
		RequestID:   emissionReq.RequestID,
		Status:      emissionReq.Status,
		CreatedAt:   emissionReq.CreatedAt,
		UpdatedAt:   emissionReq.UpdatedAt,
		ProcessedAt: emissionReq.ProcessedAt,
	}

	// Add result if successful
	if emissionReq.Status == emission.StatusSuccess && emissionReq.Result != nil {
		response.Result = &emission.EmissionResultDTO{
			NFSeAccessKey: emissionReq.Result.NFSeAccessKey,
			NFSeNumber:    emissionReq.Result.NFSeNumber,
			NFSeXMLURL:    h.buildNFSeQueryURL(emissionReq.Result.NFSeAccessKey),
		}
	}

	// Add error if failed
	if emissionReq.Status == emission.StatusFailed && emissionReq.Rejection != nil {
		response.Error = &emission.EmissionErrorDTO{
			Code:           emissionReq.Rejection.Code,
			Message:        emissionReq.Rejection.Message,
			GovernmentCode: emissionReq.Rejection.GovernmentCode,
			Details:        emissionReq.Rejection.Details,
		}
	}

	// Log successful status query
	h.logStatus(c, "status_query_success", map[string]interface{}{
		"request_id":     requestID,
		"emission_status": emissionReq.Status,
		"latency_ms":     time.Since(start).Milliseconds(),
	})

	c.JSON(http.StatusOK, response)
}

// List handles GET /v1/nfse/status requests.
// It returns a paginated list of emission requests for the authenticated API key.
func (h *StatusHandler) List(c *gin.Context) {
	start := time.Now()

	// Get API key from context (set by auth middleware)
	apiKey := getAPIKeyFromContext(c)
	if apiKey == nil {
		InternalError(c, "Failed to retrieve API key from context")
		return
	}

	// Parse pagination parameters
	params := mongodb.PaginationParams{
		Page:     parseIntQuery(c, "page", 1),
		PageSize: parseIntQuery(c, "page_size", 20),
	}

	// Log the list request
	h.logStatus(c, "status_list_start", map[string]interface{}{
		"api_key_id": apiKey.KeyPrefix,
		"page":       params.Page,
		"page_size":  params.PageSize,
	})

	// Find emission requests
	result, err := h.emissionRepo.FindByAPIKeyID(c.Request.Context(), apiKey.ID, params)
	if err != nil {
		h.logStatus(c, "status_list_error", map[string]interface{}{
			"api_key_id": apiKey.KeyPrefix,
			"error":      err.Error(),
			"latency_ms": time.Since(start).Milliseconds(),
		})
		InternalError(c, "Failed to retrieve emission requests")
		return
	}

	// Build response list
	items := make([]emission.StatusResponse, 0, len(result.Items))
	for _, req := range result.Items {
		item := emission.StatusResponse{
			RequestID:   req.RequestID,
			Status:      req.Status,
			CreatedAt:   req.CreatedAt,
			UpdatedAt:   req.UpdatedAt,
			ProcessedAt: req.ProcessedAt,
		}

		// Add result if successful
		if req.Status == emission.StatusSuccess && req.Result != nil {
			item.Result = &emission.EmissionResultDTO{
				NFSeAccessKey: req.Result.NFSeAccessKey,
				NFSeNumber:    req.Result.NFSeNumber,
				NFSeXMLURL:    h.buildNFSeQueryURL(req.Result.NFSeAccessKey),
			}
		}

		// Add error if failed
		if req.Status == emission.StatusFailed && req.Rejection != nil {
			item.Error = &emission.EmissionErrorDTO{
				Code:           req.Rejection.Code,
				Message:        req.Rejection.Message,
				GovernmentCode: req.Rejection.GovernmentCode,
				Details:        req.Rejection.Details,
			}
		}

		items = append(items, item)
	}

	// Log successful list operation
	h.logStatus(c, "status_list_success", map[string]interface{}{
		"api_key_id":  apiKey.KeyPrefix,
		"total_count": result.TotalCount,
		"items_count": len(items),
		"latency_ms":  time.Since(start).Milliseconds(),
	})

	// Return paginated response
	c.JSON(http.StatusOK, gin.H{
		"items": items,
		"pagination": gin.H{
			"page":        result.Page,
			"page_size":   result.PageSize,
			"total_count": result.TotalCount,
			"total_pages": result.TotalPages,
		},
	})
}

// parseIntQuery parses an integer query parameter with a default value.
func parseIntQuery(c *gin.Context, key string, defaultValue int64) int64 {
	value := c.Query(key)
	if value == "" {
		return defaultValue
	}

	var result int64
	_, err := parseIntValue(value, &result)
	if err != nil {
		return defaultValue
	}

	if result < 1 {
		return defaultValue
	}

	return result
}

// parseIntValue parses an integer from a string.
func parseIntValue(s string, result *int64) (bool, error) {
	var v int64
	for _, c := range s {
		if c < '0' || c > '9' {
			return false, nil
		}
		v = v*10 + int64(c-'0')
	}
	*result = v
	return true, nil
}

// buildNFSeQueryURL constructs the URL to retrieve an NFS-e by its access key.
// The URL points to GET /v1/nfse/{chaveAcesso} endpoint.
func (h *StatusHandler) buildNFSeQueryURL(chaveAcesso string) string {
	if chaveAcesso == "" {
		return ""
	}
	if h.baseURL != "" {
		return fmt.Sprintf("%s/v1/nfse/%s", h.baseURL, chaveAcesso)
	}
	return fmt.Sprintf("/v1/nfse/%s", chaveAcesso)
}

// logStatus logs a status operation with structured fields.
func (h *StatusHandler) logStatus(c *gin.Context, event string, fields map[string]interface{}) {
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
