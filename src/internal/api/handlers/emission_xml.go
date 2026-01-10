// Package handlers provides HTTP request handlers for the NFS-e API.
package handlers

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/eduardo/nfse-nacional/internal/domain/emission"
	"github.com/eduardo/nfse-nacional/internal/domain/validation"
	"github.com/eduardo/nfse-nacional/internal/infrastructure/mongodb"
	infraredis "github.com/eduardo/nfse-nacional/internal/infrastructure/redis"
	"github.com/eduardo/nfse-nacional/internal/infrastructure/xmlsigner"
	"github.com/eduardo/nfse-nacional/internal/jobs"
)

// EmissionXMLHandler handles pre-signed XML emission requests.
type EmissionXMLHandler struct {
	emissionRepo *mongodb.EmissionRepository
	jobClient    *infraredis.JobClient
	verifier     *xmlsigner.XMLVerifier
	xsdValidator *validation.XSDValidator
	baseURL      string
}

// EmissionXMLHandlerConfig configures the emission XML handler.
type EmissionXMLHandlerConfig struct {
	// EmissionRepo is the repository for emission requests.
	EmissionRepo *mongodb.EmissionRepository

	// JobClient is the Asynq job client for enqueueing tasks.
	JobClient *infraredis.JobClient

	// BaseURL is the base URL for constructing status URLs.
	BaseURL string

	// SchemaDir is the directory containing XSD schema files.
	SchemaDir string

	// ValidateCertificate controls whether to validate signer certificate dates.
	ValidateCertificate bool
}

// NewEmissionXMLHandler creates a new emission XML handler.
func NewEmissionXMLHandler(config EmissionXMLHandlerConfig) (*EmissionXMLHandler, error) {
	// Create XSD validator
	xsdValidator, err := validation.NewXSDValidator(config.SchemaDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create XSD validator: %w", err)
	}

	// Create XML signature verifier
	verifier := xmlsigner.NewXMLVerifier()
	verifier.ValidateCertificate = config.ValidateCertificate

	return &EmissionXMLHandler{
		emissionRepo: config.EmissionRepo,
		jobClient:    config.JobClient,
		verifier:     verifier,
		xsdValidator: xsdValidator,
		baseURL:      config.BaseURL,
	}, nil
}

// Create handles POST /v1/nfse/xml requests.
// It accepts pre-signed DPS XML documents and queues them for processing.
//
// The handler accepts two content types:
//   - application/xml: Raw signed DPS XML in the request body
//   - application/json: JSON with base64-encoded XML in the "xml" field
//
// The handler performs the following steps:
//  1. Parse the request based on content type
//  2. Verify the XML signature
//  3. Validate the XML against XSD schema
//  4. Extract information from the XML
//  5. Create an emission request record with is_presigned=true
//  6. Enqueue the emission job
//  7. Return 202 Accepted with request details
func (h *EmissionXMLHandler) Create(c *gin.Context) {
	// Get API key from context (set by auth middleware)
	apiKey := getAPIKeyFromContext(c)
	if apiKey == nil {
		InternalError(c, "Failed to retrieve API key from context")
		return
	}

	// Get the raw XML content based on content type
	xmlContent, err := h.extractXMLContent(c)
	if err != nil {
		BadRequest(c, err.Error())
		return
	}

	// Determine webhook URL from request or use API key default
	webhookURL := h.extractWebhookURL(c, apiKey.WebhookURL)

	// Process the pre-signed XML
	response, apiErr := h.processPreSignedXML(c, apiKey, xmlContent, webhookURL)
	if apiErr != nil {
		apiErr.Respond(c)
		return
	}

	c.JSON(http.StatusAccepted, response)
}

// extractXMLContent extracts the XML content from the request based on Content-Type.
func (h *EmissionXMLHandler) extractXMLContent(c *gin.Context) (string, error) {
	contentType := c.ContentType()

	// Handle application/xml content type
	if strings.HasPrefix(contentType, "application/xml") || strings.HasPrefix(contentType, "text/xml") {
		body, err := io.ReadAll(c.Request.Body)
		if err != nil {
			return "", fmt.Errorf("failed to read request body: %v", err)
		}
		if len(body) == 0 {
			return "", fmt.Errorf("request body is empty")
		}
		return string(body), nil
	}

	// Handle application/json content type
	if strings.HasPrefix(contentType, "application/json") {
		var req emission.PreSignedXMLRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			return "", fmt.Errorf("invalid JSON request body: %v", err)
		}

		// Decode base64 XML
		xmlContent, err := req.DecodeXML()
		if err != nil {
			return "", fmt.Errorf("failed to decode base64 XML: %v", err)
		}

		return xmlContent, nil
	}

	return "", fmt.Errorf("unsupported Content-Type: %s (expected application/xml or application/json)", contentType)
}

// extractWebhookURL extracts the webhook URL from the request or uses the default.
func (h *EmissionXMLHandler) extractWebhookURL(c *gin.Context, defaultURL string) string {
	contentType := c.ContentType()

	// For JSON requests, check if webhook_url is provided
	if strings.HasPrefix(contentType, "application/json") {
		// Re-read the body to extract webhook URL
		// Note: This is a simplified approach. In production, you'd want to
		// parse the JSON once and extract both fields.
		// For now, we'll use a query parameter fallback
		if webhookURL := c.Query("webhook_url"); webhookURL != "" {
			return webhookURL
		}
	}

	return defaultURL
}

// processPreSignedXML processes a pre-signed XML document.
func (h *EmissionXMLHandler) processPreSignedXML(
	c *gin.Context,
	apiKey *mongodb.APIKey,
	xmlContent string,
	webhookURL string,
) (*emission.PreSignedXMLResponse, *ProblemDetails) {

	// Step 1: Verify XML signature
	verificationResult, err := h.verifier.VerifyDPSSignature(xmlContent)
	if err != nil {
		return nil, NewProblemDetails(
			ProblemTypeBadRequest,
			"Invalid XML",
			http.StatusBadRequest,
		).WithDetail(fmt.Sprintf("Failed to parse XML document: %v", err)).WithInstance(c.Request.URL.Path)
	}

	// Check if signature verification failed
	if !verificationResult.Valid {
		// Build error details from verification errors
		detail := "XML signature verification failed"
		if len(verificationResult.Errors) > 0 {
			detail = fmt.Sprintf("XML signature verification failed: %s", strings.Join(verificationResult.Errors, "; "))
		}

		return nil, NewProblemDetails(
			ProblemTypeValidationFailed,
			"Signature Verification Failed",
			http.StatusBadRequest,
		).WithDetail(detail).WithInstance(c.Request.URL.Path).WithErrors(convertSignatureErrors(verificationResult.Errors))
	}

	// Step 2: Validate XML against XSD schema
	xsdErrors := h.xsdValidator.ValidateDPS(xmlContent)
	if len(xsdErrors) > 0 {
		return nil, NewProblemDetails(
			ProblemTypeValidationFailed,
			"XSD Validation Failed",
			http.StatusBadRequest,
		).WithDetail("XML document failed schema validation").WithInstance(c.Request.URL.Path).WithErrors(convertXSDErrors(xsdErrors))
	}

	// Step 3: Extract information from the XML
	preSignedInfo, err := emission.ParsePreSignedXML(xmlContent)
	if err != nil {
		return nil, NewProblemDetails(
			ProblemTypeBadRequest,
			"Invalid DPS XML",
			http.StatusBadRequest,
		).WithDetail(fmt.Sprintf("Failed to extract information from XML: %v", err)).WithInstance(c.Request.URL.Path)
	}

	// Step 4: Validate extracted information
	validationErrors := preSignedInfo.Validate()
	if len(validationErrors) > 0 {
		errors := make([]ValidationError, len(validationErrors))
		for i, msg := range validationErrors {
			errors[i] = ValidationError{
				Field:   "xml",
				Code:    ValidationCodeInvalid,
				Message: msg,
			}
		}
		return nil, NewProblemDetails(
			ProblemTypeValidationFailed,
			"Validation Failed",
			http.StatusBadRequest,
		).WithDetail("Pre-signed XML validation failed").WithInstance(c.Request.URL.Path).WithErrors(errors)
	}

	// Step 5: Generate unique request ID
	requestID := uuid.New().String()

	// Step 6: Determine environment
	// Use the environment from the XML if valid, otherwise fall back to API key environment
	environment := preSignedInfo.GetEnvironmentString()
	if apiKey.Environment != "" && apiKey.Environment != environment {
		// Log warning but allow - the XML environment takes precedence
		fmt.Printf("Warning: API key environment (%s) differs from XML environment (%s) for request %s\n",
			apiKey.Environment, environment, requestID)
	}

	// Step 7: Create emission request record
	emissionReq := &mongodb.EmissionRequest{
		RequestID:    requestID,
		APIKeyID:     apiKey.ID,
		Status:       emission.StatusPending,
		Environment:  environment,
		IsPreSigned:  true,
		PreSignedXML: xmlContent,
		Provider: mongodb.ProviderData{
			CNPJ: preSignedInfo.ProviderCNPJ,
			Name: preSignedInfo.ProviderName,
		},
		Service: mongodb.ServiceData{
			NationalCode:     preSignedInfo.NationalServiceCode,
			Description:      preSignedInfo.ServiceDescription,
			MunicipalityCode: preSignedInfo.ServiceMunicipalityCode,
		},
		Values: mongodb.ValuesData{
			ServiceValue: preSignedInfo.ServiceValue,
		},
		DPS: mongodb.DPSData{
			Series: preSignedInfo.Series,
			Number: preSignedInfo.Number,
		},
		WebhookURL: webhookURL,
		RetryCount: 0,
		// Store certificate info from signature verification
		Certificate: &mongodb.CertificateData{
			HasCertificate: true,
			IsSigned:       true,
			SubjectCN:      verificationResult.SignerCN,
			SerialNumber:   verificationResult.SignerSerial,
		},
	}

	// Handle CPF if CNPJ is not present
	if preSignedInfo.ProviderCNPJ == "" && preSignedInfo.ProviderCPF != "" {
		// Store CPF in a way that's compatible with the existing model
		// For now, we'll store it in the CNPJ field with a prefix indicator
		// In a real implementation, you might want to add a separate field
		emissionReq.Provider.CNPJ = preSignedInfo.ProviderCPF
	}

	// Step 8: Save to database
	if err := h.emissionRepo.Create(c.Request.Context(), emissionReq); err != nil {
		return nil, NewProblemDetails(
			ProblemTypeInternalError,
			"Internal Server Error",
			http.StatusInternalServerError,
		).WithDetail("Failed to create emission request").WithInstance(c.Request.URL.Path)
	}

	// Step 9: Enqueue processing job
	task, err := jobs.NewEmissionTask(requestID)
	if err != nil {
		// Log error but don't fail - request is saved and can be retried
		fmt.Printf("Warning: Failed to create emission task for request %s: %v\n", requestID, err)
	} else {
		_, err = h.jobClient.Enqueue(c.Request.Context(), task, &infraredis.EnqueueOptions{
			Queue:    infraredis.QueueDefault,
			MaxRetry: 3,
		})
		if err != nil {
			// Log error but don't fail - request is saved and can be processed later
			fmt.Printf("Warning: Failed to enqueue emission task for request %s: %v\n", requestID, err)
		}
	}

	// Step 10: Build and return response
	statusURL := h.buildStatusURL(requestID)

	return &emission.PreSignedXMLResponse{
		RequestID: requestID,
		Status:    emission.StatusPending,
		Message:   "Pre-signed XML request accepted and queued for processing",
		StatusURL: statusURL,
		DPSID:     preSignedInfo.DPSID,
		Provider:  preSignedInfo.GetProviderID(),
	}, nil
}

// buildStatusURL constructs the status URL for a request.
func (h *EmissionXMLHandler) buildStatusURL(requestID string) string {
	if h.baseURL != "" {
		return fmt.Sprintf("%s/v1/nfse/status/%s", h.baseURL, requestID)
	}
	return fmt.Sprintf("/v1/nfse/status/%s", requestID)
}

// convertSignatureErrors converts signature verification errors to validation errors.
func convertSignatureErrors(errors []string) []ValidationError {
	result := make([]ValidationError, len(errors))
	for i, err := range errors {
		result[i] = ValidationError{
			Field:   "xml.signature",
			Code:    emission.ErrorCodeSignatureInvalid,
			Message: err,
		}
	}
	return result
}

// convertXSDErrors converts XSD validation errors to handler validation errors.
func convertXSDErrors(xsdErrors []validation.XSDValidationError) []ValidationError {
	result := make([]ValidationError, len(xsdErrors))
	for i, err := range xsdErrors {
		result[i] = ValidationError{
			Field:   err.Element,
			Code:    err.Code,
			Message: err.Message,
		}
	}
	return result
}
