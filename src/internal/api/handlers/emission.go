// Package handlers provides HTTP request handlers for the NFS-e API.
package handlers

import (
	"fmt"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/eduardo/nfse-nacional/internal/domain/emission"
	"github.com/eduardo/nfse-nacional/internal/domain/validation"
	"github.com/eduardo/nfse-nacional/internal/infrastructure/mongodb"
	infraredis "github.com/eduardo/nfse-nacional/internal/infrastructure/redis"
	"github.com/eduardo/nfse-nacional/internal/jobs"
	"github.com/eduardo/nfse-nacional/pkg/cnpjcpf"
)

// Context key for storing the authenticated API key.
const apiKeyContextKey = "api_key"

// getAPIKeyFromContext retrieves the authenticated API key from the Gin context.
// This is a package-local version to avoid import cycles with middleware.
func getAPIKeyFromContext(c *gin.Context) *mongodb.APIKey {
	value, exists := c.Get(apiKeyContextKey)
	if !exists {
		return nil
	}

	apiKey, ok := value.(*mongodb.APIKey)
	if !ok {
		return nil
	}

	return apiKey
}

// EmissionHandler handles NFS-e emission requests.
type EmissionHandler struct {
	emissionRepo *mongodb.EmissionRepository
	jobClient    *infraredis.JobClient
	validator    *validation.EmissionValidator
	baseURL      string
}

// EmissionHandlerConfig configures the emission handler.
type EmissionHandlerConfig struct {
	// EmissionRepo is the repository for emission requests.
	EmissionRepo *mongodb.EmissionRepository

	// JobClient is the Asynq job client for enqueueing tasks.
	JobClient *infraredis.JobClient

	// BaseURL is the base URL for constructing status URLs.
	BaseURL string
}

// NewEmissionHandler creates a new emission handler.
func NewEmissionHandler(config EmissionHandlerConfig) *EmissionHandler {
	return &EmissionHandler{
		emissionRepo: config.EmissionRepo,
		jobClient:    config.JobClient,
		validator:    validation.NewEmissionValidator(),
		baseURL:      config.BaseURL,
	}
}

// Create handles POST /v1/nfse requests.
// It validates the request, creates an emission record, and enqueues a processing job.
func (h *EmissionHandler) Create(c *gin.Context) {
	// Get API key from context (set by auth middleware)
	apiKey := getAPIKeyFromContext(c)
	if apiKey == nil {
		InternalError(c, "Failed to retrieve API key from context")
		return
	}

	// Bind JSON request
	var req emission.EmissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		BadRequest(c, fmt.Sprintf("Invalid JSON request body: %v", err))
		return
	}

	// Validate request using domain validator
	validationErrors := h.validator.Validate(&req)
	if len(validationErrors) > 0 {
		// Convert validation errors to handler errors
		handlerErrors := make([]ValidationError, len(validationErrors))
		for i, err := range validationErrors {
			handlerErrors[i] = ValidationError{
				Field:   err.Field,
				Code:    err.Code,
				Message: err.Message,
			}
		}
		ValidationFailed(c, handlerErrors)
		return
	}

	// Validate certificate if provided (deep validation beyond basic format)
	var certValidationResult *validation.CertificateValidationResult
	if req.Certificate != nil {
		certValidationResult = validation.ValidateCertificateWithResult(req.Certificate)
		if !certValidationResult.Valid {
			// Convert certificate validation errors to handler errors
			handlerErrors := make([]ValidationError, len(certValidationResult.Errors))
			for i, err := range certValidationResult.Errors {
				handlerErrors[i] = ValidationError{
					Field:   err.Field,
					Code:    err.Code,
					Message: err.Message,
				}
			}
			ValidationFailed(c, handlerErrors)
			return
		}
	}

	// Generate unique request ID
	requestID := uuid.New().String()

	// Determine webhook URL (request override or API key default)
	webhookURL := req.WebhookURL
	if webhookURL == "" {
		webhookURL = apiKey.WebhookURL
	}

	// Create emission request record
	emissionReq := &mongodb.EmissionRequest{
		RequestID:   requestID,
		APIKeyID:    apiKey.ID,
		Status:      emission.StatusPending,
		Environment: apiKey.Environment,
		Provider: mongodb.ProviderData{
			CNPJ:                  cnpjcpf.CleanCNPJ(req.Provider.CNPJ),
			TaxRegime:             req.Provider.TaxRegime,
			Name:                  req.Provider.Name,
			MunicipalRegistration: req.Provider.MunicipalRegistration,
		},
		Service: mongodb.ServiceData{
			NationalCode:     req.Service.NationalCode,
			Description:      req.Service.Description,
			MunicipalityCode: req.Service.MunicipalityCode,
		},
		Values: mongodb.ValuesData{
			ServiceValue:          req.Values.ServiceValue,
			UnconditionalDiscount: req.Values.UnconditionalDiscount,
			ConditionalDiscount:   req.Values.ConditionalDiscount,
			Deductions:            req.Values.Deductions,
		},
		DPS: mongodb.DPSData{
			Series: req.DPS.Series,
			Number: req.DPS.Number,
		},
		WebhookURL: webhookURL,
		RetryCount: 0,
	}

	// Add taker if provided
	if req.Taker != nil {
		emissionReq.Taker = &mongodb.TakerData{
			CNPJ: cnpjcpf.CleanCNPJ(req.Taker.CNPJ),
			CPF:  cnpjcpf.CleanCPF(req.Taker.CPF),
			NIF:  req.Taker.NIF,
			Name: req.Taker.Name,
		}
	}

	// Add certificate if provided and validated
	if req.Certificate != nil && certValidationResult != nil && certValidationResult.Valid {
		emissionReq.Certificate = &mongodb.CertificateData{
			HasCertificate: true,
			PFXBase64:      req.Certificate.PFXBase64,
			Password:       req.Certificate.Password,
			// Note: SubjectCN, IssuerCN, and SerialNumber will be populated
			// by the processor after signing is complete
		}
	}

	// Save to database
	if err := h.emissionRepo.Create(c.Request.Context(), emissionReq); err != nil {
		InternalError(c, "Failed to create emission request")
		return
	}

	// Enqueue processing job
	task, err := jobs.NewEmissionTask(requestID)
	if err != nil {
		// Log error but don't fail - request is saved and can be retried
		log.Printf("ERROR: Failed to create emission task: requestID=%s error=%v", requestID, err)
	} else {
		_, err = h.jobClient.Enqueue(c.Request.Context(), task, &infraredis.EnqueueOptions{
			Queue:    infraredis.QueueDefault,
			MaxRetry: 3,
		})
		if err != nil {
			// Log error but don't fail - request is saved and can be processed later
			log.Printf("ERROR: Failed to enqueue emission task: requestID=%s error=%v", requestID, err)
		}
	}

	// Build status URL
	statusURL := h.buildStatusURL(requestID)

	// Return 202 Accepted
	response := emission.EmissionAccepted{
		RequestID: requestID,
		Status:    emission.StatusPending,
		Message:   "Request queued for processing",
		StatusURL: statusURL,
	}

	c.JSON(http.StatusAccepted, response)
}

// buildStatusURL constructs the status URL for a request.
func (h *EmissionHandler) buildStatusURL(requestID string) string {
	if h.baseURL != "" {
		return fmt.Sprintf("%s/v1/nfse/status/%s", h.baseURL, requestID)
	}
	return fmt.Sprintf("/v1/nfse/status/%s", requestID)
}
