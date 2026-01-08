// Package emission provides DTOs and business logic for NFS-e emission operations.
package emission

import (
	"time"
)

// EmissionAccepted represents the response when an emission request is accepted (202).
type EmissionAccepted struct {
	// RequestID is the unique identifier for tracking this emission request.
	RequestID string `json:"request_id"`

	// Status indicates the current status of the request (always "pending" on creation).
	Status string `json:"status"`

	// Message provides additional context about the request.
	Message string `json:"message"`

	// StatusURL is the URL to poll for status updates.
	StatusURL string `json:"status_url"`
}

// StatusResponse represents the response from GET /v1/nfse/status/{requestId}.
type StatusResponse struct {
	// RequestID is the unique identifier for this emission request.
	RequestID string `json:"request_id"`

	// Status indicates the current status: pending, processing, success, or failed.
	Status string `json:"status"`

	// CreatedAt is when the request was first submitted.
	CreatedAt time.Time `json:"created_at"`

	// UpdatedAt is when the request was last updated.
	UpdatedAt time.Time `json:"updated_at"`

	// ProcessedAt is when the request was processed (only if completed).
	ProcessedAt *time.Time `json:"processed_at,omitempty"`

	// Result contains the successful emission result (only on success).
	Result *EmissionResultDTO `json:"result,omitempty"`

	// Error contains the error details (only on failure).
	Error *EmissionErrorDTO `json:"error,omitempty"`
}

// EmissionResultDTO contains the successful emission result.
type EmissionResultDTO struct {
	// NFSeAccessKey is the 66-character access key for the emitted NFS-e.
	NFSeAccessKey string `json:"nfse_access_key"`

	// NFSeNumber is the official NFS-e number assigned by SEFIN.
	NFSeNumber string `json:"nfse_number"`

	// NFSeXMLURL is the URL to retrieve the signed XML document.
	NFSeXMLURL string `json:"nfse_xml_url,omitempty"`
}

// EmissionErrorDTO contains error details when emission fails.
type EmissionErrorDTO struct {
	// Code is the error classification code (e.g., GOVERNMENT_REJECTION, VALIDATION_ERROR).
	Code string `json:"code"`

	// Message is a human-readable error message.
	Message string `json:"message"`

	// GovernmentCode is the error code returned by SEFIN (if applicable).
	GovernmentCode string `json:"government_code,omitempty"`

	// Details provides additional context about the error.
	Details string `json:"details,omitempty"`
}

// WebhookPayload represents the payload sent to webhook endpoints.
type WebhookPayload struct {
	// Event indicates the type of webhook event.
	Event string `json:"event"`

	// RequestID is the unique identifier for this emission request.
	RequestID string `json:"request_id"`

	// Timestamp is when this webhook was generated.
	Timestamp time.Time `json:"timestamp"`

	// Status indicates the final status: success or failed.
	Status string `json:"status"`

	// Result contains the successful emission result (only on success).
	Result *EmissionResultDTO `json:"result,omitempty"`

	// Error contains the error details (only on failure).
	Error *EmissionErrorDTO `json:"error,omitempty"`
}

// WebhookEvent constants define the types of webhook events.
const (
	// WebhookEventEmissionCompleted indicates the emission was successful.
	WebhookEventEmissionCompleted = "emission.completed"

	// WebhookEventEmissionFailed indicates the emission failed.
	WebhookEventEmissionFailed = "emission.failed"
)

// EmissionStatus constants define the possible statuses of an emission request.
const (
	// StatusPending indicates the request is queued for processing.
	StatusPending = "pending"

	// StatusProcessing indicates the request is currently being processed.
	StatusProcessing = "processing"

	// StatusSuccess indicates the emission was successful.
	StatusSuccess = "success"

	// StatusFailed indicates the emission failed.
	StatusFailed = "failed"
)

// ErrorCode constants define the error classifications.
const (
	// ErrorCodeValidation indicates a validation error in the request.
	ErrorCodeValidation = "VALIDATION_ERROR"

	// ErrorCodeGovernmentRejection indicates the government API rejected the request.
	ErrorCodeGovernmentRejection = "GOVERNMENT_REJECTION"

	// ErrorCodeGovernmentUnavailable indicates the government API is unavailable.
	ErrorCodeGovernmentUnavailable = "GOVERNMENT_UNAVAILABLE"

	// ErrorCodeInternalError indicates an internal processing error.
	ErrorCodeInternalError = "INTERNAL_ERROR"

	// ErrorCodeCertificateError indicates a digital certificate error.
	ErrorCodeCertificateError = "CERTIFICATE_ERROR"

	// ErrorCodeXMLBuildError indicates an error building the DPS XML.
	ErrorCodeXMLBuildError = "XML_BUILD_ERROR"
)
