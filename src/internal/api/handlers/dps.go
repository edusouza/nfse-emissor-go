// Package handlers provides HTTP request handlers for the NFS-e API.
package handlers

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/pkcs12"

	"github.com/eduardo/nfse-nacional/internal/domain/query"
	"github.com/eduardo/nfse-nacional/internal/infrastructure/sefin"
	"github.com/eduardo/nfse-nacional/pkg/dpsid"
)

// DPSHandler handles DPS lookup requests.
// It provides functionality to recover NFS-e access keys using DPS identifiers
// when integrators don't have the access key available.
type DPSHandler struct {
	sefinClient sefin.SefinClient
	baseURL     string
	logger      *log.Logger
}

// DPSHandlerConfig configures the DPS handler.
type DPSHandlerConfig struct {
	// SefinClient is the client for communicating with the government API.
	SefinClient sefin.SefinClient

	// BaseURL is the base URL for constructing NFS-e URLs in responses.
	BaseURL string

	// Logger is an optional logger for request logging.
	Logger *log.Logger
}

// NewDPSHandler creates a new DPS handler.
func NewDPSHandler(config DPSHandlerConfig) *DPSHandler {
	return &DPSHandler{
		sefinClient: config.SefinClient,
		baseURL:     config.BaseURL,
		logger:      config.Logger,
	}
}

// Lookup handles GET /v1/dps/:id requests.
// It looks up an NFS-e access key using the DPS identifier.
//
// The request must include:
//   - DPS ID in the URL path (42-character numeric identifier)
//   - Digital certificate as multipart form data (PFX file + password)
//
// Response codes:
//   - 200 OK: DPS found, returns access key and NFS-e URL
//   - 400 Bad Request: Invalid DPS ID format or invalid certificate
//   - 403 Forbidden: Not authorized to access this DPS (actor restriction)
//   - 404 Not Found: DPS not found in the government system
//   - 503 Service Unavailable: Government API temporarily unavailable
func (h *DPSHandler) Lookup(c *gin.Context) {
	// Log the incoming request
	h.logRequest(c, "DPS lookup started")

	// T021: Validate DPS ID from path parameter
	dpsIDParam := c.Param("id")
	if dpsIDParam == "" {
		h.logRequest(c, "DPS lookup failed: missing DPS ID")
		BadRequest(c, "DPS ID is required in the path")
		return
	}

	// Parse and validate the DPS ID using the dpsid package
	parsedDPSID, err := dpsid.Parse(dpsIDParam)
	if err != nil {
		h.logRequest(c, fmt.Sprintf("DPS lookup failed: invalid DPS ID format - %v", err))
		h.handleDPSIDValidationError(c, err)
		return
	}

	// Additional validation
	if err := parsedDPSID.Validate(); err != nil {
		h.logRequest(c, fmt.Sprintf("DPS lookup failed: invalid DPS ID - %v", err))
		h.handleDPSIDValidationError(c, err)
		return
	}

	// T022 & T023: Extract and validate certificate from multipart form
	cert, err := h.extractCertificate(c)
	if err != nil {
		h.logRequest(c, fmt.Sprintf("DPS lookup failed: certificate error - %v", err))
		h.handleCertificateError(c, err)
		return
	}

	// Call SEFIN API to lookup DPS
	h.logRequest(c, fmt.Sprintf("Looking up DPS: %s", dpsIDParam))

	result, err := h.sefinClient.LookupDPS(c.Request.Context(), dpsIDParam, cert)
	if err != nil {
		h.logRequest(c, fmt.Sprintf("DPS lookup failed: SEFIN error - %v", err))
		h.handleSefinError(c, err)
		return
	}

	// T024: Build response
	nfseURL := h.buildNFSeURL(result.ChaveAcesso)
	response := query.NewDPSLookupResponse(result.DPSID, result.ChaveAcesso, nfseURL)

	h.logRequest(c, fmt.Sprintf("DPS lookup successful: %s -> %s", dpsIDParam, result.ChaveAcesso))

	c.JSON(http.StatusOK, response)
}

// extractCertificate extracts and parses the digital certificate from the multipart form request.
// The form must contain:
//   - "certificate": PFX file upload
//   - "certificate_password": password for the PFX file
func (h *DPSHandler) extractCertificate(c *gin.Context) (*tls.Certificate, error) {
	// Get the certificate file from the form
	file, header, err := c.Request.FormFile("certificate")
	if err != nil {
		if errors.Is(err, http.ErrMissingFile) {
			return nil, &certificateError{
				code:    CertificateCodeMissing,
				message: "Certificate file is required. Include 'certificate' field in multipart form.",
			}
		}
		return nil, &certificateError{
			code:    CertificateCodeInvalidFormat,
			message: fmt.Sprintf("Failed to read certificate file: %v", err),
		}
	}
	defer file.Close()

	// Validate file size (max 50KB for PFX files)
	const maxCertSize = 50 * 1024 // 50KB
	if header.Size > maxCertSize {
		return nil, &certificateError{
			code:    CertificateCodeInvalidFormat,
			message: fmt.Sprintf("Certificate file too large: %d bytes (max %d bytes)", header.Size, maxCertSize),
		}
	}

	// Read the PFX data
	pfxData, err := io.ReadAll(file)
	if err != nil {
		return nil, &certificateError{
			code:    CertificateCodeInvalidFormat,
			message: fmt.Sprintf("Failed to read certificate data: %v", err),
		}
	}

	if len(pfxData) == 0 {
		return nil, &certificateError{
			code:    CertificateCodeInvalidFormat,
			message: "Certificate file is empty",
		}
	}

	// Get the password from the form
	password := c.PostForm("certificate_password")
	if password == "" {
		return nil, &certificateError{
			code:    CertificateCodeMissingPassword,
			message: "Certificate password is required. Include 'certificate_password' field in the form.",
		}
	}

	// Parse the PFX file
	return parsePFX(pfxData, password)
}

// certificateError represents a certificate-related error.
type certificateError struct {
	code    string
	message string
}

func (e *certificateError) Error() string {
	return e.message
}

// Certificate error codes.
const (
	CertificateCodeMissing         = "CERTIFICATE_MISSING"
	CertificateCodeMissingPassword = "CERTIFICATE_PASSWORD_MISSING"
	// CertificateCodeInvalidFormat, CertificateCodeInvalidPassword, CertificateCodeExpired
	// are imported from handlers/errors.go
)

// parsePFX parses a PFX file and returns a TLS certificate.
// It validates that the certificate is not expired.
func parsePFX(pfxData []byte, password string) (*tls.Certificate, error) {
	// Decode the PFX data
	privateKey, cert, err := pkcs12.Decode(pfxData, password)
	if err != nil {
		// Check if it's a password error
		if isPasswordError(err) {
			return nil, &certificateError{
				code:    CertificateCodeInvalidPassword,
				message: "Invalid certificate password",
			}
		}
		return nil, &certificateError{
			code:    CertificateCodeInvalidFormat,
			message: fmt.Sprintf("Failed to decode PFX file: %v", err),
		}
	}

	// Validate that we have a certificate
	if cert == nil {
		return nil, &certificateError{
			code:    CertificateCodeInvalidFormat,
			message: "PFX file does not contain a certificate",
		}
	}

	// Validate that we have a private key
	if privateKey == nil {
		return nil, &certificateError{
			code:    CertificateCodeMissingKey,
			message: "PFX file does not contain a private key",
		}
	}

	// T023: Validate certificate is not expired
	now := time.Now()
	if now.After(cert.NotAfter) {
		return nil, &certificateError{
			code:    CertificateCodeExpired,
			message: fmt.Sprintf("Certificate expired on %s", cert.NotAfter.Format(time.RFC3339)),
		}
	}

	// Validate certificate is not yet valid
	if now.Before(cert.NotBefore) {
		return nil, &certificateError{
			code:    CertificateCodeNotYetValid,
			message: fmt.Sprintf("Certificate not valid until %s", cert.NotBefore.Format(time.RFC3339)),
		}
	}

	// Build the TLS certificate
	tlsCert := &tls.Certificate{
		Certificate: [][]byte{cert.Raw},
		PrivateKey:  privateKey,
		Leaf:        cert,
	}

	return tlsCert, nil
}

// isPasswordError checks if the error is related to an incorrect password.
func isPasswordError(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	// Common password-related error messages from pkcs12 package
	return contains(errStr, "password") ||
		contains(errStr, "incorrect") ||
		contains(errStr, "wrong") ||
		contains(errStr, "decryption")
}

// contains checks if s contains substr (case-insensitive).
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		(len(s) > 0 && len(substr) > 0 && containsCI(s, substr)))
}

// containsCI performs a case-insensitive contains check.
func containsCI(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if equalFold(s[i:i+len(substr)], substr) {
			return true
		}
	}
	return false
}

// equalFold compares two strings case-insensitively (ASCII only).
func equalFold(s, t string) bool {
	if len(s) != len(t) {
		return false
	}
	for i := 0; i < len(s); i++ {
		c1, c2 := s[i], t[i]
		if c1 >= 'A' && c1 <= 'Z' {
			c1 += 'a' - 'A'
		}
		if c2 >= 'A' && c2 <= 'Z' {
			c2 += 'a' - 'A'
		}
		if c1 != c2 {
			return false
		}
	}
	return true
}

// handleDPSIDValidationError handles DPS ID validation errors.
func (h *DPSHandler) handleDPSIDValidationError(c *gin.Context, err error) {
	var detail string

	switch {
	case errors.Is(err, dpsid.ErrEmptyDPSID):
		detail = "DPS ID cannot be empty"
	case errors.Is(err, dpsid.ErrInvalidLength):
		detail = "DPS ID must be exactly 42 characters"
	case errors.Is(err, dpsid.ErrInvalidCharacters):
		detail = "DPS ID must contain only numeric characters"
	case errors.Is(err, dpsid.ErrInvalidRegistrationType):
		detail = "Invalid registration type in DPS ID (must be 1 for CNPJ or 2 for CPF)"
	case errors.Is(err, dpsid.ErrInvalidCPFPadding):
		detail = "Invalid CPF padding in DPS ID"
	default:
		detail = err.Error()
	}

	ValidationFailed(c, []ValidationError{
		{
			Field:   "id",
			Code:    string(query.ErrorCodeInvalidDPSID),
			Message: detail,
		},
	})
}

// handleCertificateError handles certificate-related errors.
func (h *DPSHandler) handleCertificateError(c *gin.Context, err error) {
	var certErr *certificateError
	if errors.As(err, &certErr) {
		ValidationFailed(c, []ValidationError{
			{
				Field:   "certificate",
				Code:    certErr.code,
				Message: certErr.message,
			},
		})
		return
	}

	// Generic certificate error
	BadRequest(c, fmt.Sprintf("Certificate error: %v", err))
}

// handleSefinError handles errors from the SEFIN client.
func (h *DPSHandler) handleSefinError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, sefin.ErrDPSNotFound):
		NotFound(c, "DPS not found. The identifier may be incorrect or the DPS has not been processed yet.")

	case errors.Is(err, sefin.ErrForbidden):
		Forbidden(c, "Access denied. Only the service provider who submitted the DPS can look it up.")

	case errors.Is(err, sefin.ErrServiceUnavailable):
		ServiceUnavailable(c, "Government service is temporarily unavailable. Please try again later.")

	case errors.Is(err, sefin.ErrTimeout):
		// Use 504 Gateway Timeout for upstream timeout
		problem := NewProblemDetails(
			"https://api.nfse.gov.br/problems/gateway-timeout",
			"Gateway Timeout",
			http.StatusGatewayTimeout,
		).WithDetail("Request to government API timed out. Please try again later.").
			WithInstance(c.Request.URL.Path)
		problem.Respond(c)

	default:
		// Check if it's a QueryError from the domain
		var queryErr *query.QueryError
		if errors.As(err, &queryErr) {
			h.respondWithQueryError(c, queryErr)
			return
		}

		// Generic internal error
		InternalError(c, "An error occurred while looking up the DPS")
	}
}

// respondWithQueryError responds with a query error.
func (h *DPSHandler) respondWithQueryError(c *gin.Context, err *query.QueryError) {
	problem := NewProblemDetails(
		fmt.Sprintf("https://api.nfse.gov.br/problems/%s", err.Code),
		err.Message,
		err.HTTPStatus(),
	).WithDetail(err.Detail).WithInstance(c.Request.URL.Path)

	problem.Respond(c)
}

// buildNFSeURL constructs the URL for retrieving the NFS-e by access key.
func (h *DPSHandler) buildNFSeURL(chaveAcesso string) string {
	if h.baseURL != "" {
		return fmt.Sprintf("%s/v1/nfse/%s", h.baseURL, chaveAcesso)
	}
	return fmt.Sprintf("/v1/nfse/%s", chaveAcesso)
}

// logRequest logs a request-related message with context.
func (h *DPSHandler) logRequest(c *gin.Context, message string) {
	if h.logger == nil {
		return
	}

	requestID := getRequestIDFromContext(c)
	clientIP := c.ClientIP()
	method := c.Request.Method
	path := c.Request.URL.Path

	h.logger.Printf("[%s] %s %s %s - %s", requestID, method, path, clientIP, message)
}

// getRequestIDFromContext retrieves the request ID from the Gin context.
// This is a local helper to avoid importing the middleware package.
func getRequestIDFromContext(c *gin.Context) string {
	value, exists := c.Get("request_id")
	if !exists {
		return ""
	}
	requestID, ok := value.(string)
	if !ok {
		return ""
	}
	return requestID
}

// CheckExists handles HEAD /v1/dps/:id requests.
// It checks if a DPS exists without returning the full lookup data.
// This operation has no actor restriction - any valid certificate can check.
//
// Response codes:
//   - 200 OK: DPS exists
//   - 400 Bad Request: Invalid DPS ID format or invalid certificate
//   - 404 Not Found: DPS not found
//   - 503 Service Unavailable: Government API temporarily unavailable
func (h *DPSHandler) CheckExists(c *gin.Context) {
	h.logRequest(c, "DPS existence check started")

	// Validate DPS ID from path parameter
	dpsIDParam := c.Param("id")
	if dpsIDParam == "" {
		h.logRequest(c, "DPS check failed: missing DPS ID")
		BadRequest(c, "DPS ID is required in the path")
		return
	}

	// Parse and validate the DPS ID
	parsedDPSID, err := dpsid.Parse(dpsIDParam)
	if err != nil {
		h.logRequest(c, fmt.Sprintf("DPS check failed: invalid DPS ID format - %v", err))
		h.handleDPSIDValidationError(c, err)
		return
	}

	if err := parsedDPSID.Validate(); err != nil {
		h.logRequest(c, fmt.Sprintf("DPS check failed: invalid DPS ID - %v", err))
		h.handleDPSIDValidationError(c, err)
		return
	}

	// Extract and validate certificate
	cert, err := h.extractCertificate(c)
	if err != nil {
		h.logRequest(c, fmt.Sprintf("DPS check failed: certificate error - %v", err))
		h.handleCertificateError(c, err)
		return
	}

	// Call SEFIN API to check if DPS exists
	exists, err := h.sefinClient.CheckDPSExists(c.Request.Context(), dpsIDParam, cert)
	if err != nil {
		h.logRequest(c, fmt.Sprintf("DPS check failed: SEFIN error - %v", err))
		h.handleSefinError(c, err)
		return
	}

	if !exists {
		h.logRequest(c, fmt.Sprintf("DPS check: not found - %s", dpsIDParam))
		NotFound(c, "DPS not found")
		return
	}

	h.logRequest(c, fmt.Sprintf("DPS check successful: exists - %s", dpsIDParam))
	c.Status(http.StatusOK)
}

// ValidateCertificate is a helper that validates a certificate without making an API call.
// This can be used for pre-validation before expensive operations.
func ValidateCertificate(pfxData []byte, password string) (*x509.Certificate, error) {
	_, cert, err := pkcs12.Decode(pfxData, password)
	if err != nil {
		if isPasswordError(err) {
			return nil, &certificateError{
				code:    CertificateCodeInvalidPassword,
				message: "Invalid certificate password",
			}
		}
		return nil, &certificateError{
			code:    CertificateCodeInvalidFormat,
			message: fmt.Sprintf("Failed to decode PFX file: %v", err),
		}
	}

	if cert == nil {
		return nil, &certificateError{
			code:    CertificateCodeInvalidFormat,
			message: "PFX file does not contain a certificate",
		}
	}

	now := time.Now()
	if now.After(cert.NotAfter) {
		return nil, &certificateError{
			code:    CertificateCodeExpired,
			message: fmt.Sprintf("Certificate expired on %s", cert.NotAfter.Format(time.RFC3339)),
		}
	}

	if now.Before(cert.NotBefore) {
		return nil, &certificateError{
			code:    CertificateCodeNotYetValid,
			message: fmt.Sprintf("Certificate not valid until %s", cert.NotBefore.Format(time.RFC3339)),
		}
	}

	return cert, nil
}
