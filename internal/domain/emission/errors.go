// Package emission provides domain logic for NFS-e emission processing.
package emission

import (
	"fmt"
	"strings"
	"sync"
)

// RejectionCode represents a government rejection error code with translations.
type RejectionCode struct {
	// Code is the official government error code (e.g., "E001").
	Code string `json:"code"`

	// Message is the original Portuguese message from the government.
	Message string `json:"message"`

	// Description is an English description of the error.
	Description string `json:"description"`

	// Action is a suggested action for the integrator to resolve the issue.
	Action string `json:"action"`

	// Category classifies the error type for programmatic handling.
	Category RejectionCategory `json:"category"`

	// Retryable indicates if the operation can be retried.
	Retryable bool `json:"retryable"`
}

// RejectionCategory classifies rejection errors for handling.
type RejectionCategory string

const (
	// CategoryValidation indicates a data validation error.
	CategoryValidation RejectionCategory = "validation"

	// CategoryCertificate indicates a digital certificate error.
	CategoryCertificate RejectionCategory = "certificate"

	// CategoryDuplicate indicates a duplicate submission error.
	CategoryDuplicate RejectionCategory = "duplicate"

	// CategoryNotFound indicates a resource not found error.
	CategoryNotFound RejectionCategory = "not_found"

	// CategoryPermission indicates a permission or authorization error.
	CategoryPermission RejectionCategory = "permission"

	// CategoryService indicates a service or infrastructure error.
	CategoryService RejectionCategory = "service"

	// CategoryUnknown indicates an unknown or unmapped error.
	CategoryUnknown RejectionCategory = "unknown"
)

// rejectionCodesMu protects concurrent access to the rejection codes map.
var rejectionCodesMu sync.RWMutex

// rejectionCodes is the map of government error codes to RejectionCode details.
// This map is populated with known codes and can be extended at runtime.
var rejectionCodes = map[string]RejectionCode{
	// Provider/Contributor Errors (E001-E099)
	"E001": {
		Code:        "E001",
		Message:     "CNPJ nao encontrado",
		Description: "Provider CNPJ not found in government registry (Cadastro Nacional)",
		Action:      "Verify the CNPJ is correctly registered with the municipal tax authority and has NFS-e emission permission",
		Category:    CategoryNotFound,
		Retryable:   false,
	},
	"E002": {
		Code:        "E002",
		Message:     "CPF invalido",
		Description: "Invalid CPF check digits or format",
		Action:      "Verify the CPF format (11 digits) and check digit calculation",
		Category:    CategoryValidation,
		Retryable:   false,
	},
	"E003": {
		Code:        "E003",
		Message:     "DPS duplicado",
		Description: "Duplicate DPS number for this provider - same series/number combination already submitted",
		Action:      "Use a unique DPS series/number combination. Check if this DPS was already processed successfully",
		Category:    CategoryDuplicate,
		Retryable:   false,
	},
	"E004": {
		Code:        "E004",
		Message:     "Certificado invalido",
		Description: "Invalid digital certificate - may be expired, revoked, or not authorized for NFS-e",
		Action:      "Check certificate expiration date, verify it is an A1 certificate with NFS-e signing permissions, and ensure the certificate belongs to the provider",
		Category:    CategoryCertificate,
		Retryable:   false,
	},
	"E005": {
		Code:        "E005",
		Message:     "XML mal formado",
		Description: "XML structure error - document does not conform to DPS schema",
		Action:      "Validate XML against the DPS_v1.00.xsd schema before submission. Check for missing required elements or invalid values",
		Category:    CategoryValidation,
		Retryable:   false,
	},
	"E006": {
		Code:        "E006",
		Message:     "Assinatura invalida",
		Description: "Invalid XMLDSig signature - signature verification failed",
		Action:      "Regenerate signature ensuring: correct canonicalization (exc-c14n), proper digest calculation, valid certificate chain, and certificate matches provider CNPJ",
		Category:    CategoryCertificate,
		Retryable:   false,
	},
	"E007": {
		Code:        "E007",
		Message:     "Codigo de servico invalido",
		Description: "Invalid national service code (cTribNac) - code not found in LC 116/2003 list",
		Action:      "Use a valid service code from the Lei Complementar 116/2003 national service list (NBS). Verify the code format and existence",
		Category:    CategoryValidation,
		Retryable:   false,
	},
	"E008": {
		Code:        "E008",
		Message:     "Municipio nao conveniado",
		Description: "Municipality not integrated with the national NFS-e system",
		Action:      "Contact the municipality to verify their participation status. Some municipalities use their own NFS-e systems",
		Category:    CategoryService,
		Retryable:   false,
	},
	"E009": {
		Code:        "E009",
		Message:     "Valor do servico invalido",
		Description: "Invalid service value - must be greater than zero",
		Action:      "Ensure service value (vServ) is positive and properly formatted with up to 2 decimal places",
		Category:    CategoryValidation,
		Retryable:   false,
	},
	"E010": {
		Code:        "E010",
		Message:     "Data de competencia invalida",
		Description: "Invalid competency date - date is in the future or too far in the past",
		Action:      "Use a competency date within the allowed range. Generally, use the current month or at most the previous month",
		Category:    CategoryValidation,
		Retryable:   false,
	},

	// Additional Provider/Contributor Errors
	"E011": {
		Code:        "E011",
		Message:     "Inscricao Municipal invalida",
		Description: "Invalid municipal registration (IM) for the provider",
		Action:      "Verify the municipal registration number is correct and active for the specified municipality",
		Category:    CategoryValidation,
		Retryable:   false,
	},
	"E012": {
		Code:        "E012",
		Message:     "Prestador nao autorizado a emitir NFS-e",
		Description: "Provider not authorized to emit NFS-e in this municipality",
		Action:      "Contact the municipal tax authority to request NFS-e emission authorization",
		Category:    CategoryPermission,
		Retryable:   false,
	},
	"E013": {
		Code:        "E013",
		Message:     "Regime tributario incompativel",
		Description: "Tax regime incompatible with the operation or service type",
		Action:      "Verify the declared tax regime (cRegTrib) matches your registration and the service being provided",
		Category:    CategoryValidation,
		Retryable:   false,
	},
	"E014": {
		Code:        "E014",
		Message:     "CNPJ do prestador diferente do certificado",
		Description: "Provider CNPJ does not match the digital certificate owner",
		Action:      "Use a certificate that belongs to the provider CNPJ or update the provider CNPJ in the request",
		Category:    CategoryCertificate,
		Retryable:   false,
	},

	// Taker (Customer) Errors (E020-E039)
	"E020": {
		Code:        "E020",
		Message:     "CNPJ do tomador invalido",
		Description: "Invalid taker (customer) CNPJ",
		Action:      "Verify the taker CNPJ format and check digits",
		Category:    CategoryValidation,
		Retryable:   false,
	},
	"E021": {
		Code:        "E021",
		Message:     "CPF do tomador invalido",
		Description: "Invalid taker (customer) CPF",
		Action:      "Verify the taker CPF format and check digits",
		Category:    CategoryValidation,
		Retryable:   false,
	},
	"E022": {
		Code:        "E022",
		Message:     "Identificacao do tomador obrigatoria",
		Description: "Taker identification is required for this service type",
		Action:      "Provide either CNPJ or CPF for the taker. Anonymous services may not be allowed for this service code",
		Category:    CategoryValidation,
		Retryable:   false,
	},
	"E023": {
		Code:        "E023",
		Message:     "Endereco do tomador obrigatorio",
		Description: "Taker address is required for this service type",
		Action:      "Provide complete taker address including municipality code (cMun) and other required fields",
		Category:    CategoryValidation,
		Retryable:   false,
	},

	// Value and Tax Errors (E040-E059)
	"E040": {
		Code:        "E040",
		Message:     "Base de calculo invalida",
		Description: "Invalid tax calculation base - does not match service value minus deductions",
		Action:      "Verify that vBC (base) = vServ - vDesc - deductions. Check arithmetic calculations",
		Category:    CategoryValidation,
		Retryable:   false,
	},
	"E041": {
		Code:        "E041",
		Message:     "Aliquota ISS invalida",
		Description: "Invalid ISS (service tax) rate for this municipality and service code",
		Action:      "Use the correct ISS rate defined by the municipality for this service code. Rates vary by municipality and service type",
		Category:    CategoryValidation,
		Retryable:   false,
	},
	"E042": {
		Code:        "E042",
		Message:     "Valor ISS calculado incorretamente",
		Description: "Calculated ISS value does not match base * rate",
		Action:      "Verify that vISS = vBC * pISS. Check for rounding issues (use 2 decimal places)",
		Category:    CategoryValidation,
		Retryable:   false,
	},
	"E043": {
		Code:        "E043",
		Message:     "Retencao ISS invalida",
		Description: "Invalid ISS withholding configuration",
		Action:      "Verify ISS withholding rules for the service type and taker location. Some services require mandatory withholding",
		Category:    CategoryValidation,
		Retryable:   false,
	},
	"E044": {
		Code:        "E044",
		Message:     "Valor total inconsistente",
		Description: "Total value inconsistent with service value and deductions",
		Action:      "Verify that all value fields are consistent: vServ, vDesc, vBC, vISS, and total values",
		Category:    CategoryValidation,
		Retryable:   false,
	},

	// Document Errors (E060-E079)
	"E060": {
		Code:        "E060",
		Message:     "Serie DPS invalida",
		Description: "Invalid DPS series - must be 5 numeric characters",
		Action:      "Use a valid 5-character numeric series (e.g., '00001'). Series must be pre-authorized by the municipality",
		Category:    CategoryValidation,
		Retryable:   false,
	},
	"E061": {
		Code:        "E061",
		Message:     "Numero DPS invalido",
		Description: "Invalid DPS number - must be numeric and sequential",
		Action:      "Use a sequential numeric DPS number. Do not skip numbers in the sequence",
		Category:    CategoryValidation,
		Retryable:   false,
	},
	"E062": {
		Code:        "E062",
		Message:     "Sequencia DPS quebrada",
		Description: "DPS sequence broken - missing numbers in sequence",
		Action:      "Submit DPS documents in sequential order. If numbers were skipped, contact the municipality",
		Category:    CategoryValidation,
		Retryable:   false,
	},
	"E063": {
		Code:        "E063",
		Message:     "Id DPS invalido",
		Description: "Invalid DPS ID format - does not match expected pattern",
		Action:      "Ensure Id attribute follows the format: DPS + UF + AAMM + CNPJ + Mod + Serie + Num + CodVer",
		Category:    CategoryValidation,
		Retryable:   false,
	},

	// Service Location Errors (E080-E089)
	"E080": {
		Code:        "E080",
		Message:     "Codigo municipio invalido",
		Description: "Invalid municipality code (IBGE code)",
		Action:      "Use a valid 7-digit IBGE municipality code from the official list",
		Category:    CategoryValidation,
		Retryable:   false,
	},
	"E081": {
		Code:        "E081",
		Message:     "Municipio de incidencia incorreto",
		Description: "Incorrect ISS incidence municipality for this service type",
		Action:      "Verify ISS incidence rules: some services are taxed at provider location, others at service execution location",
		Category:    CategoryValidation,
		Retryable:   false,
	},
	"E082": {
		Code:        "E082",
		Message:     "Pais invalido para operacao",
		Description: "Invalid country code for this operation type",
		Action:      "Use ISO 3166-1 country codes. Verify if export rules apply for foreign takers",
		Category:    CategoryValidation,
		Retryable:   false,
	},

	// System and Infrastructure Errors (E100-E199)
	"E100": {
		Code:        "E100",
		Message:     "Servico temporariamente indisponivel",
		Description: "Government service temporarily unavailable",
		Action:      "Wait and retry the submission. Check government portal for maintenance notices",
		Category:    CategoryService,
		Retryable:   true,
	},
	"E101": {
		Code:        "E101",
		Message:     "Timeout na comunicacao",
		Description: "Communication timeout with government server",
		Action:      "The request may have been processed. Query status before retrying to avoid duplicates",
		Category:    CategoryService,
		Retryable:   true,
	},
	"E102": {
		Code:        "E102",
		Message:     "Erro interno do sistema",
		Description: "Internal government system error",
		Action:      "Wait and retry. If persistent, contact government support with the protocol number",
		Category:    CategoryService,
		Retryable:   true,
	},
	"E103": {
		Code:        "E103",
		Message:     "Sistema em manutencao",
		Description: "Government system under maintenance",
		Action:      "Wait for maintenance window to complete. Check government portal for estimated restoration time",
		Category:    CategoryService,
		Retryable:   true,
	},

	// Authorization Errors (E200-E249)
	"E200": {
		Code:        "E200",
		Message:     "Acesso negado",
		Description: "Access denied - not authorized for this operation",
		Action:      "Verify your credentials and permissions. Contact the municipality if access was recently granted",
		Category:    CategoryPermission,
		Retryable:   false,
	},
	"E201": {
		Code:        "E201",
		Message:     "Certificado revogado",
		Description: "Digital certificate has been revoked",
		Action:      "Obtain a new digital certificate from an authorized certificate authority",
		Category:    CategoryCertificate,
		Retryable:   false,
	},
	"E202": {
		Code:        "E202",
		Message:     "Certificado expirado",
		Description: "Digital certificate has expired",
		Action:      "Renew your digital certificate before it expires. A1 certificates are valid for 1 year",
		Category:    CategoryCertificate,
		Retryable:   false,
	},
	"E203": {
		Code:        "E203",
		Message:     "Certificado nao confiavel",
		Description: "Digital certificate not from a trusted authority (ICP-Brasil)",
		Action:      "Use a certificate issued by an ICP-Brasil accredited certificate authority",
		Category:    CategoryCertificate,
		Retryable:   false,
	},
}

// TranslateRejection translates a government error code to a user-friendly RejectionCode.
// Returns nil if the code is not found in the known codes map.
func TranslateRejection(governmentCode string) *RejectionCode {
	rejectionCodesMu.RLock()
	defer rejectionCodesMu.RUnlock()

	code := strings.TrimSpace(strings.ToUpper(governmentCode))
	if rejection, exists := rejectionCodes[code]; exists {
		return &rejection
	}
	return nil
}

// TranslateRejectionWithDefault translates a government error code, returning a default
// unknown error if the code is not found.
func TranslateRejectionWithDefault(governmentCode, originalMessage string) *RejectionCode {
	if translated := TranslateRejection(governmentCode); translated != nil {
		return translated
	}

	// Return a generic unknown error with the original message
	return &RejectionCode{
		Code:        governmentCode,
		Message:     originalMessage,
		Description: fmt.Sprintf("Unknown government error code: %s", governmentCode),
		Action:      "Contact support with the error code and original message for assistance",
		Category:    CategoryUnknown,
		Retryable:   false,
	}
}

// TranslateMultiple translates multiple government error codes.
// Returns a slice of RejectionCode pointers; nil entries indicate unknown codes.
func TranslateMultiple(codes []string) []*RejectionCode {
	results := make([]*RejectionCode, len(codes))
	for i, code := range codes {
		results[i] = TranslateRejection(code)
	}
	return results
}

// TranslateMultipleWithMessages translates multiple error codes with their original messages.
// Each code is paired with its message for proper translation.
type CodeMessage struct {
	Code    string
	Message string
}

// TranslateMultipleWithDefaults translates multiple codes, providing defaults for unknown codes.
func TranslateMultipleWithDefaults(codeMessages []CodeMessage) []*RejectionCode {
	results := make([]*RejectionCode, len(codeMessages))
	for i, cm := range codeMessages {
		results[i] = TranslateRejectionWithDefault(cm.Code, cm.Message)
	}
	return results
}

// RegisterRejectionCode adds or updates a rejection code in the map.
// This allows runtime extension of known codes.
func RegisterRejectionCode(code RejectionCode) {
	rejectionCodesMu.Lock()
	defer rejectionCodesMu.Unlock()

	rejectionCodes[strings.ToUpper(code.Code)] = code
}

// GetAllRejectionCodes returns a copy of all known rejection codes.
func GetAllRejectionCodes() map[string]RejectionCode {
	rejectionCodesMu.RLock()
	defer rejectionCodesMu.RUnlock()

	result := make(map[string]RejectionCode, len(rejectionCodes))
	for k, v := range rejectionCodes {
		result[k] = v
	}
	return result
}

// IsRetryable checks if a government error code represents a retryable error.
func IsRetryable(governmentCode string) bool {
	if translated := TranslateRejection(governmentCode); translated != nil {
		return translated.Retryable
	}
	// Unknown errors are not retryable by default
	return false
}

// GetCategory returns the category of a government error code.
func GetCategory(governmentCode string) RejectionCategory {
	if translated := TranslateRejection(governmentCode); translated != nil {
		return translated.Category
	}
	return CategoryUnknown
}

// FormatErrorResponse creates a formatted error response for API clients.
type FormattedError struct {
	Code        string `json:"code"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Action      string `json:"action"`
	Retryable   bool   `json:"retryable"`
}

// FormatForAPI formats a RejectionCode for API response.
func (r *RejectionCode) FormatForAPI() FormattedError {
	return FormattedError{
		Code:        r.Code,
		Title:       r.Message,
		Description: r.Description,
		Action:      r.Action,
		Retryable:   r.Retryable,
	}
}

// FormatMultipleForAPI formats multiple rejection codes for API response.
func FormatMultipleForAPI(rejections []*RejectionCode) []FormattedError {
	results := make([]FormattedError, 0, len(rejections))
	for _, r := range rejections {
		if r != nil {
			results = append(results, r.FormatForAPI())
		}
	}
	return results
}
