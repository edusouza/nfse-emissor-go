// Package validation provides validation logic for NFS-e domain objects.
package validation

import (
	"regexp"
	"strings"

	"github.com/eduardo/nfse-nacional/internal/domain/emission"
	"github.com/eduardo/nfse-nacional/pkg/cnpjcpf"
)

// Validation patterns for emission request fields.
var (
	// nationalCodePattern matches exactly 6 digits for cTribNac.
	nationalCodePattern = regexp.MustCompile(`^\d{6}$`)

	// municipalityCodePattern matches exactly 7 digits for IBGE municipality code.
	municipalityCodePattern = regexp.MustCompile(`^\d{7}$`)

	// dpsSeriesPattern matches exactly 5 digits for DPS series.
	dpsSeriesPattern = regexp.MustCompile(`^\d{5}$`)

	// dpsNumberPattern matches 1-15 digits for DPS number.
	dpsNumberPattern = regexp.MustCompile(`^\d{1,15}$`)
)

// Valid tax regime values.
const (
	TaxRegimeMEI   = "mei"
	TaxRegimeMEEPP = "me_epp"
)

// ValidationError represents a single field validation error.
type ValidationError struct {
	// Field is the name of the field that failed validation.
	Field string `json:"field"`

	// Code is a machine-readable error code.
	Code string `json:"code"`

	// Message is a human-readable error message.
	Message string `json:"message"`
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

// EmissionValidator validates emission requests.
type EmissionValidator struct {
	takerValidator *TakerValidator
}

// NewEmissionValidator creates a new emission validator.
func NewEmissionValidator() *EmissionValidator {
	return &EmissionValidator{
		takerValidator: NewTakerValidator(),
	}
}

// Validate performs comprehensive validation of an emission request.
// Returns a slice of ValidationErrors if any validation fails.
func (v *EmissionValidator) Validate(req *emission.EmissionRequest) []ValidationError {
	var errors []ValidationError

	// Validate provider
	errors = append(errors, v.validateProvider(&req.Provider)...)

	// Validate taker (if present)
	if req.Taker != nil {
		errors = append(errors, v.validateTaker(req.Taker)...)
	}

	// Validate service
	errors = append(errors, v.validateService(&req.Service)...)

	// Validate values
	errors = append(errors, v.validateValues(&req.Values)...)

	// Validate DPS
	errors = append(errors, v.validateDPS(&req.DPS)...)

	// Validate certificate (if present)
	if req.Certificate != nil {
		errors = append(errors, v.validateCertificate(req.Certificate)...)
	}

	// Validate webhook URL (if present)
	if req.WebhookURL != "" {
		errors = append(errors, v.validateWebhookURL(req.WebhookURL)...)
	}

	return errors
}

// validateProvider validates the provider section of the request.
func (v *EmissionValidator) validateProvider(provider *emission.ProviderRequest) []ValidationError {
	var errors []ValidationError

	// Validate CNPJ is present
	if provider.CNPJ == "" {
		errors = append(errors, NewValidationError(
			"provider.cnpj",
			ValidationCodeRequired,
			"Provider CNPJ is required",
		))
	} else {
		// Clean and validate CNPJ format and check digit
		cleanCNPJ := cnpjcpf.CleanCNPJ(provider.CNPJ)
		if !cnpjcpf.ValidateCNPJ(cleanCNPJ) {
			errors = append(errors, NewValidationError(
				"provider.cnpj",
				ValidationCodeInvalid,
				"Provider CNPJ is invalid (check digit mismatch or incorrect format)",
			))
		}
	}

	// Validate tax regime
	if provider.TaxRegime == "" {
		errors = append(errors, NewValidationError(
			"provider.tax_regime",
			ValidationCodeRequired,
			"Provider tax regime is required",
		))
	} else if provider.TaxRegime != TaxRegimeMEI && provider.TaxRegime != TaxRegimeMEEPP {
		errors = append(errors, NewValidationError(
			"provider.tax_regime",
			ValidationCodeInvalid,
			"Provider tax regime must be 'mei' or 'me_epp'",
		))
	}

	// Validate name
	if provider.Name == "" {
		errors = append(errors, NewValidationError(
			"provider.name",
			ValidationCodeRequired,
			"Provider name is required",
		))
	} else if len(provider.Name) > 150 {
		errors = append(errors, NewValidationError(
			"provider.name",
			ValidationCodeTooLong,
			"Provider name must not exceed 150 characters",
		))
	}

	return errors
}

// validateTaker validates the taker section of the request.
// This method delegates to the TakerValidator for comprehensive taker validation
// including identification (CNPJ/CPF/NIF), name, phone, email, and address.
func (v *EmissionValidator) validateTaker(taker *emission.TakerRequest) []ValidationError {
	return v.takerValidator.ValidateTaker(taker)
}

// validateService validates the service section of the request.
func (v *EmissionValidator) validateService(service *emission.ServiceRequest) []ValidationError {
	var errors []ValidationError

	// Validate national code (cTribNac) - exactly 6 digits
	if service.NationalCode == "" {
		errors = append(errors, NewValidationError(
			"service.national_code",
			ValidationCodeRequired,
			"Service national code (cTribNac) is required",
		))
	} else if !nationalCodePattern.MatchString(service.NationalCode) {
		errors = append(errors, NewValidationError(
			"service.national_code",
			ValidationCodeInvalidFormat,
			"Service national code must be exactly 6 digits",
		))
	}

	// Validate description
	if service.Description == "" {
		errors = append(errors, NewValidationError(
			"service.description",
			ValidationCodeRequired,
			"Service description is required",
		))
	} else if len(service.Description) > 2000 {
		errors = append(errors, NewValidationError(
			"service.description",
			ValidationCodeTooLong,
			"Service description must not exceed 2000 characters",
		))
	}

	// Validate municipality code (IBGE) - exactly 7 digits
	if service.MunicipalityCode == "" {
		errors = append(errors, NewValidationError(
			"service.municipality_code",
			ValidationCodeRequired,
			"Service municipality code (IBGE) is required",
		))
	} else if !municipalityCodePattern.MatchString(service.MunicipalityCode) {
		errors = append(errors, NewValidationError(
			"service.municipality_code",
			ValidationCodeInvalidFormat,
			"Service municipality code must be exactly 7 digits (IBGE code)",
		))
	}

	return errors
}

// validateValues validates the monetary values section of the request.
func (v *EmissionValidator) validateValues(values *emission.ValuesRequest) []ValidationError {
	var errors []ValidationError

	// Validate service value is positive
	if values.ServiceValue <= 0 {
		errors = append(errors, NewValidationError(
			"values.service_value",
			ValidationCodeOutOfRange,
			"Service value must be greater than zero",
		))
	}

	// Validate service value has at most 2 decimal places (monetary precision)
	// This is important for financial systems to avoid rounding issues
	if values.ServiceValue > 0 && !isValidMonetaryValue(values.ServiceValue) {
		errors = append(errors, NewValidationError(
			"values.service_value",
			ValidationCodeInvalid,
			"Service value must have at most 2 decimal places",
		))
	}

	// Validate discounts are not negative
	if values.UnconditionalDiscount < 0 {
		errors = append(errors, NewValidationError(
			"values.unconditional_discount",
			ValidationCodeOutOfRange,
			"Unconditional discount cannot be negative",
		))
	}

	if values.ConditionalDiscount < 0 {
		errors = append(errors, NewValidationError(
			"values.conditional_discount",
			ValidationCodeOutOfRange,
			"Conditional discount cannot be negative",
		))
	}

	if values.Deductions < 0 {
		errors = append(errors, NewValidationError(
			"values.deductions",
			ValidationCodeOutOfRange,
			"Deductions cannot be negative",
		))
	}

	// Validate that discounts don't exceed service value
	totalDeductions := values.UnconditionalDiscount + values.ConditionalDiscount + values.Deductions
	if totalDeductions > values.ServiceValue {
		errors = append(errors, NewValidationError(
			"values",
			ValidationCodeOutOfRange,
			"Total discounts and deductions cannot exceed service value",
		))
	}

	return errors
}

// validateDPS validates the DPS (Documento de Prestacao de Servicos) section.
func (v *EmissionValidator) validateDPS(dps *emission.DPSRequest) []ValidationError {
	var errors []ValidationError

	// Validate series - exactly 5 digits
	if dps.Series == "" {
		errors = append(errors, NewValidationError(
			"dps.series",
			ValidationCodeRequired,
			"DPS series is required",
		))
	} else if !dpsSeriesPattern.MatchString(dps.Series) {
		errors = append(errors, NewValidationError(
			"dps.series",
			ValidationCodeInvalidFormat,
			"DPS series must be exactly 5 digits",
		))
	}

	// Validate number - 1 to 15 digits
	if dps.Number == "" {
		errors = append(errors, NewValidationError(
			"dps.number",
			ValidationCodeRequired,
			"DPS number is required",
		))
	} else if !dpsNumberPattern.MatchString(dps.Number) {
		errors = append(errors, NewValidationError(
			"dps.number",
			ValidationCodeInvalidFormat,
			"DPS number must be 1 to 15 digits",
		))
	}

	return errors
}

// validateCertificate validates the digital certificate section.
func (v *EmissionValidator) validateCertificate(cert *emission.CertificateRequest) []ValidationError {
	var errors []ValidationError

	// Validate PFX is provided
	if cert.PFXBase64 == "" {
		errors = append(errors, NewValidationError(
			"certificate.pfx_base64",
			ValidationCodeRequired,
			"Certificate PFX (base64 encoded) is required",
		))
	}

	// Validate password is provided
	if cert.Password == "" {
		errors = append(errors, NewValidationError(
			"certificate.password",
			ValidationCodeRequired,
			"Certificate password is required",
		))
	}

	return errors
}

// validateWebhookURL validates the optional webhook URL override.
func (v *EmissionValidator) validateWebhookURL(url string) []ValidationError {
	var errors []ValidationError

	// Basic URL validation - must start with https:// for security
	if !strings.HasPrefix(url, "https://") {
		errors = append(errors, NewValidationError(
			"webhook_url",
			ValidationCodeInvalid,
			"Webhook URL must use HTTPS protocol",
		))
	}

	// Check URL length
	if len(url) > 2048 {
		errors = append(errors, NewValidationError(
			"webhook_url",
			ValidationCodeTooLong,
			"Webhook URL must not exceed 2048 characters",
		))
	}

	return errors
}

// isValidMonetaryValue checks if a float64 value has at most 2 decimal places.
// This is a simplified check; in production, decimal.Decimal should be used.
func isValidMonetaryValue(value float64) bool {
	// Multiply by 100 and check if it's effectively an integer
	scaled := value * 100
	return scaled == float64(int64(scaled+0.5))
}
