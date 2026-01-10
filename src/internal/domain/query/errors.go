package query

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
)

// QueryErrorCode represents a machine-readable error code for query operations.
type QueryErrorCode string

// Query-specific error codes with their associated HTTP status codes.
const (
	// ErrorCodeInvalidAccessKey indicates the access key format is invalid (400).
	ErrorCodeInvalidAccessKey QueryErrorCode = "INVALID_ACCESS_KEY"

	// ErrorCodeInvalidDPSID indicates the DPS identifier format is invalid (400).
	ErrorCodeInvalidDPSID QueryErrorCode = "INVALID_DPS_ID"

	// ErrorCodeNFSeNotFound indicates the requested NFS-e was not found (404).
	ErrorCodeNFSeNotFound QueryErrorCode = "NFSE_NOT_FOUND"

	// ErrorCodeDPSNotFound indicates the requested DPS was not found (404).
	ErrorCodeDPSNotFound QueryErrorCode = "DPS_NOT_FOUND"

	// ErrorCodeForbiddenAccess indicates the caller does not have permission to access the resource (403).
	ErrorCodeForbiddenAccess QueryErrorCode = "FORBIDDEN_ACCESS"

	// ErrorCodeCertificateRequired indicates a digital certificate is required for the operation (400).
	ErrorCodeCertificateRequired QueryErrorCode = "CERTIFICATE_REQUIRED"

	// ErrorCodeCertificateInvalid indicates the provided certificate is invalid or expired (400).
	ErrorCodeCertificateInvalid QueryErrorCode = "CERTIFICATE_INVALID"

	// ErrorCodeGovernmentUnavailable indicates the government service is temporarily unavailable (503).
	ErrorCodeGovernmentUnavailable QueryErrorCode = "GOVERNMENT_UNAVAILABLE"

	// ErrorCodeGovernmentTimeout indicates the government service request timed out (504).
	ErrorCodeGovernmentTimeout QueryErrorCode = "GOVERNMENT_TIMEOUT"
)

// HTTPStatus returns the appropriate HTTP status code for the error code.
func (c QueryErrorCode) HTTPStatus() int {
	switch c {
	case ErrorCodeInvalidAccessKey, ErrorCodeInvalidDPSID,
		ErrorCodeCertificateRequired, ErrorCodeCertificateInvalid:
		return http.StatusBadRequest
	case ErrorCodeForbiddenAccess:
		return http.StatusForbidden
	case ErrorCodeNFSeNotFound, ErrorCodeDPSNotFound:
		return http.StatusNotFound
	case ErrorCodeGovernmentUnavailable:
		return http.StatusServiceUnavailable
	case ErrorCodeGovernmentTimeout:
		return http.StatusGatewayTimeout
	default:
		return http.StatusInternalServerError
	}
}

// IsRetryable returns true if the error is potentially retryable.
func (c QueryErrorCode) IsRetryable() bool {
	switch c {
	case ErrorCodeGovernmentUnavailable, ErrorCodeGovernmentTimeout:
		return true
	default:
		return false
	}
}

// String returns the string representation of the error code.
func (c QueryErrorCode) String() string {
	return string(c)
}

// QueryError represents a structured error for query operations.
// It includes both machine-readable code and human-readable messages.
type QueryError struct {
	// Code is the machine-readable error code.
	Code QueryErrorCode `json:"code"`

	// Message is a human-readable error message.
	Message string `json:"message"`

	// Detail provides additional context about the error.
	Detail string `json:"detail,omitempty"`

	// GovernmentCode is the original government error code, if applicable.
	GovernmentCode string `json:"government_code,omitempty"`

	// Retryable indicates if the operation can be retried.
	Retryable bool `json:"retryable"`
}

// Error implements the error interface.
func (e *QueryError) Error() string {
	if e.Detail != "" {
		return fmt.Sprintf("%s: %s (%s)", e.Code, e.Message, e.Detail)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}

// HTTPStatus returns the HTTP status code for this error.
func (e *QueryError) HTTPStatus() int {
	return e.Code.HTTPStatus()
}

// Is implements error comparison for errors.Is().
func (e *QueryError) Is(target error) bool {
	t, ok := target.(*QueryError)
	if !ok {
		return false
	}
	return e.Code == t.Code
}

// NewQueryError creates a new QueryError with the given code and message.
func NewQueryError(code QueryErrorCode, message string) *QueryError {
	return &QueryError{
		Code:      code,
		Message:   message,
		Retryable: code.IsRetryable(),
	}
}

// NewQueryErrorWithDetail creates a new QueryError with additional detail.
func NewQueryErrorWithDetail(code QueryErrorCode, message, detail string) *QueryError {
	return &QueryError{
		Code:      code,
		Message:   message,
		Detail:    detail,
		Retryable: code.IsRetryable(),
	}
}

// NewQueryErrorFromGovernment creates a QueryError from a government error response.
func NewQueryErrorFromGovernment(code QueryErrorCode, governmentCode, originalMessage string) *QueryError {
	return &QueryError{
		Code:           code,
		Message:        originalMessage,
		GovernmentCode: governmentCode,
		Retryable:      code.IsRetryable(),
	}
}

// Predefined errors for common query scenarios.
var (
	// ErrInvalidAccessKeyFormat is returned when the access key format is invalid.
	ErrInvalidAccessKeyFormat = NewQueryError(
		ErrorCodeInvalidAccessKey,
		"Invalid access key format",
	)

	// ErrInvalidDPSIDFormat is returned when the DPS ID format is invalid.
	ErrInvalidDPSIDFormat = NewQueryError(
		ErrorCodeInvalidDPSID,
		"Invalid DPS identifier format",
	)

	// ErrNFSeNotFound is returned when the requested NFS-e does not exist.
	ErrNFSeNotFound = NewQueryError(
		ErrorCodeNFSeNotFound,
		"NFS-e not found",
	)

	// ErrDPSNotFound is returned when the requested DPS does not exist.
	ErrDPSNotFound = NewQueryError(
		ErrorCodeDPSNotFound,
		"DPS not found",
	)

	// ErrForbiddenAccess is returned when the caller lacks permission.
	ErrForbiddenAccess = NewQueryError(
		ErrorCodeForbiddenAccess,
		"Access to this resource is forbidden",
	)

	// ErrCertificateRequired is returned when a certificate is required but not provided.
	ErrCertificateRequired = NewQueryError(
		ErrorCodeCertificateRequired,
		"Digital certificate is required for this operation",
	)

	// ErrCertificateInvalid is returned when the provided certificate is invalid.
	ErrCertificateInvalid = NewQueryError(
		ErrorCodeCertificateInvalid,
		"Provided digital certificate is invalid or expired",
	)

	// ErrGovernmentUnavailable is returned when the government service is down.
	ErrGovernmentUnavailable = NewQueryError(
		ErrorCodeGovernmentUnavailable,
		"Government service is temporarily unavailable",
	)

	// ErrGovernmentTimeout is returned when the government service times out.
	ErrGovernmentTimeout = NewQueryError(
		ErrorCodeGovernmentTimeout,
		"Government service request timed out",
	)
)

// ================================================================================
// Convenience Constructor Functions
// ================================================================================

// NewInvalidAccessKeyError creates a QueryError for invalid access key format.
func NewInvalidAccessKeyError(details string) *QueryError {
	return &QueryError{
		Code:      ErrorCodeInvalidAccessKey,
		Message:   "Invalid access key format",
		Detail:    details,
		Retryable: false,
	}
}

// NewInvalidDPSIDError creates a QueryError for invalid DPS ID format.
func NewInvalidDPSIDError(details string) *QueryError {
	return &QueryError{
		Code:      ErrorCodeInvalidDPSID,
		Message:   "Invalid DPS identifier format",
		Detail:    details,
		Retryable: false,
	}
}

// NewNFSeNotFoundError creates a QueryError for NFS-e not found.
func NewNFSeNotFoundError() *QueryError {
	return &QueryError{
		Code:      ErrorCodeNFSeNotFound,
		Message:   "NFS-e not found",
		Retryable: false,
	}
}

// NewDPSNotFoundError creates a QueryError for DPS not found.
func NewDPSNotFoundError() *QueryError {
	return &QueryError{
		Code:      ErrorCodeDPSNotFound,
		Message:   "DPS not found",
		Retryable: false,
	}
}

// NewForbiddenAccessError creates a QueryError for forbidden access.
func NewForbiddenAccessError() *QueryError {
	return &QueryError{
		Code:      ErrorCodeForbiddenAccess,
		Message:   "Access to this resource is forbidden",
		Retryable: false,
	}
}

// NewCertificateRequiredError creates a QueryError for missing certificate.
func NewCertificateRequiredError() *QueryError {
	return &QueryError{
		Code:      ErrorCodeCertificateRequired,
		Message:   "Digital certificate is required for this operation",
		Retryable: false,
	}
}

// NewCertificateInvalidError creates a QueryError for invalid certificate.
func NewCertificateInvalidError(details string) *QueryError {
	return &QueryError{
		Code:      ErrorCodeCertificateInvalid,
		Message:   "Provided digital certificate is invalid or expired",
		Detail:    details,
		Retryable: false,
	}
}

// NewGovernmentUnavailableError creates a QueryError for unavailable government service.
func NewGovernmentUnavailableError() *QueryError {
	return &QueryError{
		Code:      ErrorCodeGovernmentUnavailable,
		Message:   "Government service is temporarily unavailable",
		Retryable: true,
	}
}

// NewGovernmentTimeoutError creates a QueryError for government service timeout.
func NewGovernmentTimeoutError() *QueryError {
	return &QueryError{
		Code:      ErrorCodeGovernmentTimeout,
		Message:   "Government service request timed out",
		Retryable: true,
	}
}

// IsQueryError checks if an error is a QueryError.
func IsQueryError(err error) bool {
	var qe *QueryError
	return errors.As(err, &qe)
}

// GetQueryError extracts a QueryError from an error if present.
func GetQueryError(err error) (*QueryError, bool) {
	var qe *QueryError
	if errors.As(err, &qe) {
		return qe, true
	}
	return nil, false
}

// ================================================================================
// Government Query Error Translation (T004)
// ================================================================================

// GovernmentQueryCode represents a translated government error code for query operations.
type GovernmentQueryCode struct {
	// Code is the official government error code (e.g., "Q001").
	Code string `json:"code"`

	// Message is the original Portuguese message from the government.
	Message string `json:"message"`

	// Description is an English description of the error.
	Description string `json:"description"`

	// Action is a suggested action for the integrator to resolve the issue.
	Action string `json:"action"`

	// Category classifies the error type for programmatic handling.
	Category QueryErrorCategory `json:"category"`

	// Retryable indicates if the operation can be retried.
	Retryable bool `json:"retryable"`

	// MappedCode is the QueryErrorCode this government code maps to.
	MappedCode QueryErrorCode `json:"mapped_code"`
}

// QueryErrorCategory classifies query errors for handling.
type QueryErrorCategory string

const (
	// CategoryQueryValidation indicates a query parameter validation error.
	CategoryQueryValidation QueryErrorCategory = "query_validation"

	// CategoryQueryNotFound indicates the queried resource was not found.
	CategoryQueryNotFound QueryErrorCategory = "not_found"

	// CategoryQueryPermission indicates a permission or access error.
	CategoryQueryPermission QueryErrorCategory = "permission"

	// CategoryQueryCertificate indicates a certificate-related error.
	CategoryQueryCertificate QueryErrorCategory = "certificate"

	// CategoryQueryService indicates a service infrastructure error.
	CategoryQueryService QueryErrorCategory = "service"

	// CategoryQueryUnknown indicates an unknown or unmapped error.
	CategoryQueryUnknown QueryErrorCategory = "unknown"
)

// governmentQueryCodesMu protects concurrent access to the query codes map.
var governmentQueryCodesMu sync.RWMutex

// governmentQueryCodes maps government error codes to GovernmentQueryCode details.
var governmentQueryCodes = map[string]GovernmentQueryCode{
	// Query validation errors (Q001-Q019)
	"Q001": {
		Code:        "Q001",
		Message:     "Chave de acesso invalida",
		Description: "Invalid NFS-e access key format",
		Action:      "Verify the access key format: must be 50 alphanumeric characters starting with 'NFSe'",
		Category:    CategoryQueryValidation,
		Retryable:   false,
		MappedCode:  ErrorCodeInvalidAccessKey,
	},
	"Q002": {
		Code:        "Q002",
		Message:     "Id DPS invalido",
		Description: "Invalid DPS identifier format",
		Action:      "Verify the DPS ID format: must be 42 numeric characters",
		Category:    CategoryQueryValidation,
		Retryable:   false,
		MappedCode:  ErrorCodeInvalidDPSID,
	},
	"Q003": {
		Code:        "Q003",
		Message:     "Parametros de consulta invalidos",
		Description: "Invalid query parameters provided",
		Action:      "Check the query parameters format and values",
		Category:    CategoryQueryValidation,
		Retryable:   false,
		MappedCode:  ErrorCodeInvalidAccessKey,
	},

	// Not found errors (Q020-Q039)
	"Q020": {
		Code:        "Q020",
		Message:     "NFS-e nao encontrada",
		Description: "The requested NFS-e was not found in the national system",
		Action:      "Verify the access key is correct. The NFS-e may not exist or may have been cancelled",
		Category:    CategoryQueryNotFound,
		Retryable:   false,
		MappedCode:  ErrorCodeNFSeNotFound,
	},
	"Q021": {
		Code:        "Q021",
		Message:     "DPS nao encontrado",
		Description: "The requested DPS was not found in the national system",
		Action:      "Verify the DPS ID is correct. The DPS may not exist or may not have been processed yet",
		Category:    CategoryQueryNotFound,
		Retryable:   false,
		MappedCode:  ErrorCodeDPSNotFound,
	},
	"Q022": {
		Code:        "Q022",
		Message:     "Evento nao encontrado",
		Description: "No events found for the specified NFS-e",
		Action:      "The NFS-e may not have any registered events, or the access key may be incorrect",
		Category:    CategoryQueryNotFound,
		Retryable:   false,
		MappedCode:  ErrorCodeNFSeNotFound,
	},
	"Q023": {
		Code:        "Q023",
		Message:     "NFS-e cancelada",
		Description: "The NFS-e has been cancelled and is no longer valid",
		Action:      "This NFS-e was cancelled. Query events to see cancellation details",
		Category:    CategoryQueryNotFound,
		Retryable:   false,
		MappedCode:  ErrorCodeNFSeNotFound,
	},

	// Permission errors (Q040-Q059)
	"Q040": {
		Code:        "Q040",
		Message:     "Acesso negado",
		Description: "Access denied to the requested resource",
		Action:      "Verify your credentials and permissions. You may only query NFS-e documents you are authorized to access",
		Category:    CategoryQueryPermission,
		Retryable:   false,
		MappedCode:  ErrorCodeForbiddenAccess,
	},
	"Q041": {
		Code:        "Q041",
		Message:     "Prestador nao autorizado",
		Description: "Provider not authorized to query this NFS-e",
		Action:      "You can only query NFS-e documents where you are the provider or an authorized third party",
		Category:    CategoryQueryPermission,
		Retryable:   false,
		MappedCode:  ErrorCodeForbiddenAccess,
	},
	"Q042": {
		Code:        "Q042",
		Message:     "Tomador nao autorizado",
		Description: "Taker not authorized to query this NFS-e",
		Action:      "You can only query NFS-e documents where you are the taker or an authorized third party",
		Category:    CategoryQueryPermission,
		Retryable:   false,
		MappedCode:  ErrorCodeForbiddenAccess,
	},

	// Certificate errors (Q060-Q079)
	"Q060": {
		Code:        "Q060",
		Message:     "Certificado obrigatorio",
		Description: "A digital certificate is required for this query operation",
		Action:      "Provide a valid ICP-Brasil digital certificate to authenticate the request",
		Category:    CategoryQueryCertificate,
		Retryable:   false,
		MappedCode:  ErrorCodeCertificateRequired,
	},
	"Q061": {
		Code:        "Q061",
		Message:     "Certificado invalido",
		Description: "The provided digital certificate is invalid",
		Action:      "Check the certificate format, expiration date, and ensure it is from an ICP-Brasil authority",
		Category:    CategoryQueryCertificate,
		Retryable:   false,
		MappedCode:  ErrorCodeCertificateInvalid,
	},
	"Q062": {
		Code:        "Q062",
		Message:     "Certificado expirado",
		Description: "The digital certificate has expired",
		Action:      "Renew your digital certificate. A1 certificates are valid for 1 year",
		Category:    CategoryQueryCertificate,
		Retryable:   false,
		MappedCode:  ErrorCodeCertificateInvalid,
	},
	"Q063": {
		Code:        "Q063",
		Message:     "Certificado revogado",
		Description: "The digital certificate has been revoked",
		Action:      "Obtain a new certificate from an ICP-Brasil certificate authority",
		Category:    CategoryQueryCertificate,
		Retryable:   false,
		MappedCode:  ErrorCodeCertificateInvalid,
	},
	"Q064": {
		Code:        "Q064",
		Message:     "CNPJ do certificado nao confere",
		Description: "Certificate CNPJ does not match the requester",
		Action:      "Use a certificate that belongs to the CNPJ making the query request",
		Category:    CategoryQueryCertificate,
		Retryable:   false,
		MappedCode:  ErrorCodeCertificateInvalid,
	},

	// Service errors (Q100-Q119)
	"Q100": {
		Code:        "Q100",
		Message:     "Servico temporariamente indisponivel",
		Description: "Government query service is temporarily unavailable",
		Action:      "Wait and retry the query. Check government portal for maintenance notices",
		Category:    CategoryQueryService,
		Retryable:   true,
		MappedCode:  ErrorCodeGovernmentUnavailable,
	},
	"Q101": {
		Code:        "Q101",
		Message:     "Timeout na consulta",
		Description: "Query request timed out",
		Action:      "The query may still be processing. Wait before retrying to avoid duplicate requests",
		Category:    CategoryQueryService,
		Retryable:   true,
		MappedCode:  ErrorCodeGovernmentTimeout,
	},
	"Q102": {
		Code:        "Q102",
		Message:     "Sistema em manutencao",
		Description: "Government system is under scheduled maintenance",
		Action:      "Check the government portal for maintenance window and expected restoration time",
		Category:    CategoryQueryService,
		Retryable:   true,
		MappedCode:  ErrorCodeGovernmentUnavailable,
	},
	"Q103": {
		Code:        "Q103",
		Message:     "Erro interno do sistema",
		Description: "Internal government system error during query",
		Action:      "Wait and retry. If persistent, contact government support",
		Category:    CategoryQueryService,
		Retryable:   true,
		MappedCode:  ErrorCodeGovernmentUnavailable,
	},
	"Q104": {
		Code:        "Q104",
		Message:     "Limite de requisicoes excedido",
		Description: "Query rate limit exceeded",
		Action:      "Reduce query frequency. Implement exponential backoff between requests",
		Category:    CategoryQueryService,
		Retryable:   true,
		MappedCode:  ErrorCodeGovernmentUnavailable,
	},
}

// TranslateQueryCode translates a government error code to a user-friendly GovernmentQueryCode.
// Returns nil if the code is not found in the known codes map.
func TranslateQueryCode(governmentCode string) *GovernmentQueryCode {
	governmentQueryCodesMu.RLock()
	defer governmentQueryCodesMu.RUnlock()

	code := strings.TrimSpace(strings.ToUpper(governmentCode))
	if translated, exists := governmentQueryCodes[code]; exists {
		return &translated
	}
	return nil
}

// TranslateQueryCodeWithDefault translates a government error code, returning a default
// unknown error if the code is not found.
func TranslateQueryCodeWithDefault(governmentCode, originalMessage string) *GovernmentQueryCode {
	if translated := TranslateQueryCode(governmentCode); translated != nil {
		return translated
	}

	// Return a generic unknown error with the original message
	return &GovernmentQueryCode{
		Code:        governmentCode,
		Message:     originalMessage,
		Description: fmt.Sprintf("Unknown government query error code: %s", governmentCode),
		Action:      "Contact support with the error code and original message for assistance",
		Category:    CategoryQueryUnknown,
		Retryable:   false,
		MappedCode:  ErrorCodeGovernmentUnavailable,
	}
}

// TranslateToQueryError converts a government error code directly to a QueryError.
// This is a convenience function for creating QueryErrors from government responses.
func TranslateToQueryError(governmentCode, originalMessage string) *QueryError {
	translated := TranslateQueryCodeWithDefault(governmentCode, originalMessage)
	return &QueryError{
		Code:           translated.MappedCode,
		Message:        translated.Description,
		Detail:         translated.Action,
		GovernmentCode: governmentCode,
		Retryable:      translated.Retryable,
	}
}

// ================================================================================
// Legacy Government Error Code Translation (E-prefix codes)
// ================================================================================

// legacyGovernmentCodes maps common legacy government error codes (E-prefix) to QueryErrors.
// These codes are used by some government endpoints and need translation.
var legacyGovernmentCodes = map[string]struct {
	code      QueryErrorCode
	message   string
	retryable bool
}{
	"E001": {ErrorCodeInvalidAccessKey, "Invalid data provided", false},
	"E002": {ErrorCodeNFSeNotFound, "Resource not found", false},
	"E003": {ErrorCodeForbiddenAccess, "Access denied", false},
	"E004": {ErrorCodeGovernmentUnavailable, "Service unavailable", true},
}

// TranslateGovernmentError translates a government error code to a QueryError.
// Handles both Q-prefix (query-specific) and E-prefix (legacy) error codes.
// This function provides a unified interface for translating any government error code.
func TranslateGovernmentError(govCode string, govMessage string) *QueryError {
	code := strings.TrimSpace(strings.ToUpper(govCode))

	// First try Q-prefix codes (query-specific)
	if translated := TranslateQueryCode(code); translated != nil {
		return &QueryError{
			Code:           translated.MappedCode,
			Message:        translated.Description,
			Detail:         translated.Action,
			GovernmentCode: govCode,
			Retryable:      translated.Retryable,
		}
	}

	// Try legacy E-prefix codes
	if legacy, exists := legacyGovernmentCodes[code]; exists {
		return &QueryError{
			Code:           legacy.code,
			Message:        legacy.message,
			Detail:         govMessage,
			GovernmentCode: govCode,
			Retryable:      legacy.retryable,
		}
	}

	// Return unknown error with original message preserved
	return &QueryError{
		Code:           ErrorCodeGovernmentUnavailable,
		Message:        fmt.Sprintf("Unknown government error: %s", govCode),
		Detail:         govMessage,
		GovernmentCode: govCode,
		Retryable:      false,
	}
}

// RegisterQueryCode adds or updates a query code in the map.
// This allows runtime extension of known codes.
func RegisterQueryCode(code GovernmentQueryCode) {
	governmentQueryCodesMu.Lock()
	defer governmentQueryCodesMu.Unlock()

	governmentQueryCodes[strings.ToUpper(code.Code)] = code
}

// GetAllQueryCodes returns a copy of all known query error codes.
func GetAllQueryCodes() map[string]GovernmentQueryCode {
	governmentQueryCodesMu.RLock()
	defer governmentQueryCodesMu.RUnlock()

	result := make(map[string]GovernmentQueryCode, len(governmentQueryCodes))
	for k, v := range governmentQueryCodes {
		result[k] = v
	}
	return result
}

// IsRetryableQueryCode checks if a government error code represents a retryable error.
func IsRetryableQueryCode(governmentCode string) bool {
	if translated := TranslateQueryCode(governmentCode); translated != nil {
		return translated.Retryable
	}
	return false
}

// GetQueryCategory returns the category of a government query error code.
func GetQueryCategory(governmentCode string) QueryErrorCategory {
	if translated := TranslateQueryCode(governmentCode); translated != nil {
		return translated.Category
	}
	return CategoryQueryUnknown
}

// FormattedQueryError creates a formatted error response for API clients.
type FormattedQueryError struct {
	Code           string `json:"code"`
	Title          string `json:"title"`
	Description    string `json:"description"`
	Action         string `json:"action"`
	Retryable      bool   `json:"retryable"`
	GovernmentCode string `json:"government_code,omitempty"`
}

// FormatForAPI formats a GovernmentQueryCode for API response.
func (g *GovernmentQueryCode) FormatForAPI() FormattedQueryError {
	return FormattedQueryError{
		Code:           string(g.MappedCode),
		Title:          g.Message,
		Description:    g.Description,
		Action:         g.Action,
		Retryable:      g.Retryable,
		GovernmentCode: g.Code,
	}
}

// FormatQueryErrorForAPI formats a QueryError for API response.
func FormatQueryErrorForAPI(err *QueryError) FormattedQueryError {
	return FormattedQueryError{
		Code:           string(err.Code),
		Title:          err.Message,
		Description:    err.Detail,
		Retryable:      err.Retryable,
		GovernmentCode: err.GovernmentCode,
	}
}
