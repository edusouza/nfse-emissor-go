// Package handlers provides HTTP request handlers for the NFS-e API.
package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
)

// ProblemDetails represents an RFC 7807 Problem Details response.
// This provides a standardized format for API error responses.
type ProblemDetails struct {
	// Type is a URI reference that identifies the problem type.
	Type string `json:"type"`

	// Title is a short, human-readable summary of the problem type.
	Title string `json:"title"`

	// Status is the HTTP status code for this occurrence of the problem.
	Status int `json:"status"`

	// Detail is a human-readable explanation specific to this occurrence.
	Detail string `json:"detail,omitempty"`

	// Instance is a URI reference that identifies the specific occurrence.
	Instance string `json:"instance,omitempty"`

	// Errors contains validation errors for request body validation failures.
	Errors []ValidationError `json:"errors,omitempty"`
}

// ValidationError represents a single field validation error.
type ValidationError struct {
	// Field is the name of the field that failed validation.
	Field string `json:"field"`

	// Code is a machine-readable error code.
	Code string `json:"code"`

	// Message is a human-readable error message.
	Message string `json:"message"`
}

// Problem type URIs following RFC 7807 conventions.
const (
	ProblemTypeBadRequest          = "https://api.nfse.gov.br/problems/bad-request"
	ProblemTypeUnauthorized        = "https://api.nfse.gov.br/problems/unauthorized"
	ProblemTypeForbidden           = "https://api.nfse.gov.br/problems/forbidden"
	ProblemTypeNotFound            = "https://api.nfse.gov.br/problems/not-found"
	ProblemTypeMethodNotAllowed    = "https://api.nfse.gov.br/problems/method-not-allowed"
	ProblemTypeConflict            = "https://api.nfse.gov.br/problems/conflict"
	ProblemTypeUnprocessableEntity = "https://api.nfse.gov.br/problems/unprocessable-entity"
	ProblemTypeTooManyRequests     = "https://api.nfse.gov.br/problems/too-many-requests"
	ProblemTypeInternalError       = "https://api.nfse.gov.br/problems/internal-error"
	ProblemTypeServiceUnavailable  = "https://api.nfse.gov.br/problems/service-unavailable"
	ProblemTypeValidationFailed    = "https://api.nfse.gov.br/problems/validation-failed"
)

// NewProblemDetails creates a new ProblemDetails instance.
func NewProblemDetails(problemType, title string, status int) *ProblemDetails {
	return &ProblemDetails{
		Type:   problemType,
		Title:  title,
		Status: status,
	}
}

// WithDetail adds a detail message to the problem.
func (p *ProblemDetails) WithDetail(detail string) *ProblemDetails {
	p.Detail = detail
	return p
}

// WithInstance adds an instance URI to the problem.
func (p *ProblemDetails) WithInstance(instance string) *ProblemDetails {
	p.Instance = instance
	return p
}

// WithErrors adds validation errors to the problem.
func (p *ProblemDetails) WithErrors(errors []ValidationError) *ProblemDetails {
	p.Errors = errors
	return p
}

// Respond sends the problem details as a JSON response.
func (p *ProblemDetails) Respond(c *gin.Context) {
	c.Header("Content-Type", "application/problem+json")
	c.JSON(p.Status, p)
}

// BadRequest responds with a 400 Bad Request error.
func BadRequest(c *gin.Context, detail string) {
	problem := NewProblemDetails(
		ProblemTypeBadRequest,
		"Bad Request",
		http.StatusBadRequest,
	).WithDetail(detail).WithInstance(c.Request.URL.Path)

	problem.Respond(c)
}

// BadRequestWithErrors responds with a 400 Bad Request error including validation errors.
func BadRequestWithErrors(c *gin.Context, detail string, errors []ValidationError) {
	problem := NewProblemDetails(
		ProblemTypeValidationFailed,
		"Validation Failed",
		http.StatusBadRequest,
	).WithDetail(detail).WithInstance(c.Request.URL.Path).WithErrors(errors)

	problem.Respond(c)
}

// Unauthorized responds with a 401 Unauthorized error.
func Unauthorized(c *gin.Context, detail string) {
	problem := NewProblemDetails(
		ProblemTypeUnauthorized,
		"Unauthorized",
		http.StatusUnauthorized,
	).WithDetail(detail).WithInstance(c.Request.URL.Path)

	problem.Respond(c)
}

// Forbidden responds with a 403 Forbidden error.
func Forbidden(c *gin.Context, detail string) {
	problem := NewProblemDetails(
		ProblemTypeForbidden,
		"Forbidden",
		http.StatusForbidden,
	).WithDetail(detail).WithInstance(c.Request.URL.Path)

	problem.Respond(c)
}

// NotFound responds with a 404 Not Found error.
func NotFound(c *gin.Context, detail string) {
	problem := NewProblemDetails(
		ProblemTypeNotFound,
		"Not Found",
		http.StatusNotFound,
	).WithDetail(detail).WithInstance(c.Request.URL.Path)

	problem.Respond(c)
}

// MethodNotAllowed responds with a 405 Method Not Allowed error.
func MethodNotAllowed(c *gin.Context, detail string) {
	problem := NewProblemDetails(
		ProblemTypeMethodNotAllowed,
		"Method Not Allowed",
		http.StatusMethodNotAllowed,
	).WithDetail(detail).WithInstance(c.Request.URL.Path)

	problem.Respond(c)
}

// Conflict responds with a 409 Conflict error.
func Conflict(c *gin.Context, detail string) {
	problem := NewProblemDetails(
		ProblemTypeConflict,
		"Conflict",
		http.StatusConflict,
	).WithDetail(detail).WithInstance(c.Request.URL.Path)

	problem.Respond(c)
}

// UnprocessableEntity responds with a 422 Unprocessable Entity error.
func UnprocessableEntity(c *gin.Context, detail string) {
	problem := NewProblemDetails(
		ProblemTypeUnprocessableEntity,
		"Unprocessable Entity",
		http.StatusUnprocessableEntity,
	).WithDetail(detail).WithInstance(c.Request.URL.Path)

	problem.Respond(c)
}

// TooManyRequests responds with a 429 Too Many Requests error.
// The retryAfter parameter indicates when the client can retry (in seconds).
func TooManyRequests(c *gin.Context, detail string, retryAfter int) {
	c.Header("Retry-After", fmt.Sprintf("%d", retryAfter))

	problem := NewProblemDetails(
		ProblemTypeTooManyRequests,
		"Too Many Requests",
		http.StatusTooManyRequests,
	).WithDetail(detail).WithInstance(c.Request.URL.Path)

	problem.Respond(c)
}

// InternalError responds with a 500 Internal Server Error.
// The detail should not expose sensitive information in production.
func InternalError(c *gin.Context, detail string) {
	problem := NewProblemDetails(
		ProblemTypeInternalError,
		"Internal Server Error",
		http.StatusInternalServerError,
	).WithDetail(detail).WithInstance(c.Request.URL.Path)

	problem.Respond(c)
}

// ServiceUnavailable responds with a 503 Service Unavailable error.
func ServiceUnavailable(c *gin.Context, detail string) {
	problem := NewProblemDetails(
		ProblemTypeServiceUnavailable,
		"Service Unavailable",
		http.StatusServiceUnavailable,
	).WithDetail(detail).WithInstance(c.Request.URL.Path)

	problem.Respond(c)
}

// ValidationFailed responds with a 400 Bad Request error for validation failures.
func ValidationFailed(c *gin.Context, errors []ValidationError) {
	detail := "One or more fields failed validation"
	if len(errors) == 1 {
		detail = errors[0].Message
	}

	problem := NewProblemDetails(
		ProblemTypeValidationFailed,
		"Validation Failed",
		http.StatusBadRequest,
	).WithDetail(detail).WithInstance(c.Request.URL.Path).WithErrors(errors)

	problem.Respond(c)
}

// NewValidationError creates a new validation error.
func NewValidationError(field, code, message string) ValidationError {
	return ValidationError{
		Field:   field,
		Code:    code,
		Message: message,
	}
}

// Common validation error codes.
const (
	ValidationCodeRequired      = "required"
	ValidationCodeInvalid       = "invalid"
	ValidationCodeTooShort      = "too_short"
	ValidationCodeTooLong       = "too_long"
	ValidationCodeOutOfRange    = "out_of_range"
	ValidationCodeInvalidFormat = "invalid_format"
	ValidationCodeDuplicate     = "duplicate"
)

// Certificate-specific error codes for digital certificate validation.
const (
	// CertificateCodeInvalidFormat indicates the PFX format is invalid.
	CertificateCodeInvalidFormat = "INVALID_CERTIFICATE_FORMAT"

	// CertificateCodeInvalidPassword indicates the password is incorrect.
	CertificateCodeInvalidPassword = "INVALID_CERTIFICATE_PASSWORD"

	// CertificateCodeExpired indicates the certificate has expired.
	CertificateCodeExpired = "CERTIFICATE_EXPIRED"

	// CertificateCodeNotYetValid indicates the certificate is not yet valid.
	CertificateCodeNotYetValid = "CERTIFICATE_NOT_YET_VALID"

	// CertificateCodeMissingKey indicates the certificate is missing a private key.
	CertificateCodeMissingKey = "CERTIFICATE_MISSING_KEY"
)
