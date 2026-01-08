// Package validation provides validation logic for NFS-e domain objects.
package validation

import (
	"regexp"
	"strings"
	"unicode"

	"github.com/eduardo/nfse-nacional/internal/domain/emission"
	"github.com/eduardo/nfse-nacional/pkg/cnpjcpf"
)

// Validation patterns for taker fields.
var (
	// nifPattern matches 1-40 alphanumeric characters for foreign tax ID.
	nifPattern = regexp.MustCompile(`^[A-Za-z0-9]{1,40}$`)

	// emailPattern is a basic email validation regex.
	// More comprehensive validation should use net/mail.ParseAddress or similar.
	emailPattern = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}$`)

	// brazilianPhonePattern matches Brazilian phone numbers: 10-11 digits.
	// Format: DDD (2 digits) + number (8-9 digits)
	brazilianPhonePattern = regexp.MustCompile(`^\d{10,11}$`)

	// internationalPhonePattern matches international phone numbers.
	// Allows digits with optional + prefix, 7-15 digits total.
	internationalPhonePattern = regexp.MustCompile(`^\+?\d{7,15}$`)

	// postalCodePattern matches Brazilian CEP: exactly 8 digits.
	postalCodePattern = regexp.MustCompile(`^\d{8}$`)

	// statePattern matches Brazilian state codes: exactly 2 uppercase letters.
	statePattern = regexp.MustCompile(`^[A-Z]{2}$`)

	// countryCodePattern matches ISO 3166-1 alpha-2 country codes: exactly 2 uppercase letters.
	countryCodePattern = regexp.MustCompile(`^[A-Z]{2}$`)
)

// Taker validation constants.
const (
	// TakerNameMaxLength is the maximum length for taker name.
	TakerNameMaxLength = 300

	// AddressStreetMaxLength is the maximum length for street name.
	AddressStreetMaxLength = 255

	// AddressNumberMaxLength is the maximum length for address number.
	AddressNumberMaxLength = 60

	// AddressComplementMaxLength is the maximum length for address complement.
	AddressComplementMaxLength = 156

	// AddressNeighborhoodMaxLength is the maximum length for neighborhood.
	AddressNeighborhoodMaxLength = 60

	// EmailMaxLength is the maximum length for email.
	EmailMaxLength = 254

	// NIFMaxLength is the maximum length for NIF.
	NIFMaxLength = 40
)

// Valid Brazilian state codes.
var validBrazilianStates = map[string]bool{
	"AC": true, "AL": true, "AP": true, "AM": true, "BA": true,
	"CE": true, "DF": true, "ES": true, "GO": true, "MA": true,
	"MT": true, "MS": true, "MG": true, "PA": true, "PB": true,
	"PR": true, "PE": true, "PI": true, "RJ": true, "RN": true,
	"RS": true, "RO": true, "RR": true, "SC": true, "SP": true,
	"SE": true, "TO": true,
}

// TakerValidator validates taker information in emission requests.
type TakerValidator struct {
	// No external dependencies needed; CNPJ/CPF validation is done via package functions.
}

// NewTakerValidator creates a new TakerValidator instance.
func NewTakerValidator() *TakerValidator {
	return &TakerValidator{}
}

// ValidateTaker validates a taker and returns all validation errors found.
// The taker parameter must not be nil.
func (v *TakerValidator) ValidateTaker(taker *emission.TakerRequest) []ValidationError {
	if taker == nil {
		return []ValidationError{
			NewValidationError("taker", ValidationCodeRequired, "Taker is required"),
		}
	}

	var errors []ValidationError

	// Validate identification (CNPJ, CPF, or NIF)
	errors = append(errors, v.validateIdentification(taker)...)

	// Validate name
	errors = append(errors, v.validateName(taker.Name)...)

	// Validate phone (if provided)
	if taker.Phone != "" {
		errors = append(errors, v.validatePhone(taker.Phone, taker.NIF != "")...)
	}

	// Validate email (if provided)
	if taker.Email != "" {
		errors = append(errors, v.validateEmail(taker.Email)...)
	}

	// Validate address based on taker type
	errors = append(errors, v.validateTakerAddress(taker)...)

	return errors
}

// validateIdentification validates that exactly one identification type is provided
// and that the provided identification is valid.
func (v *TakerValidator) validateIdentification(taker *emission.TakerRequest) []ValidationError {
	var errors []ValidationError

	// Count provided identifiers
	identifierCount := 0
	if taker.CNPJ != "" {
		identifierCount++
	}
	if taker.CPF != "" {
		identifierCount++
	}
	if taker.NIF != "" {
		identifierCount++
	}

	// Check that exactly one identifier is provided
	if identifierCount == 0 {
		errors = append(errors, NewValidationError(
			"taker",
			ValidationCodeRequired,
			"Taker must have exactly one of CNPJ, CPF, or NIF",
		))
		return errors
	}

	if identifierCount > 1 {
		errors = append(errors, NewValidationError(
			"taker",
			ValidationCodeInvalid,
			"Taker must have only one of CNPJ, CPF, or NIF (they are mutually exclusive)",
		))
		return errors
	}

	// Validate the provided identifier
	if taker.CNPJ != "" {
		cleanCNPJ := cnpjcpf.CleanCNPJ(taker.CNPJ)
		if !cnpjcpf.ValidateCNPJ(cleanCNPJ) {
			errors = append(errors, NewValidationError(
				"taker.cnpj",
				ValidationCodeInvalid,
				"Taker CNPJ is invalid (check digit mismatch or incorrect format)",
			))
		}
	}

	if taker.CPF != "" {
		cleanCPF := cnpjcpf.CleanCPF(taker.CPF)
		if !cnpjcpf.ValidateCPF(cleanCPF) {
			errors = append(errors, NewValidationError(
				"taker.cpf",
				ValidationCodeInvalid,
				"Taker CPF is invalid (check digit mismatch or incorrect format)",
			))
		}
	}

	if taker.NIF != "" {
		errors = append(errors, v.validateNIF(taker.NIF)...)
	}

	return errors
}

// validateNIF validates a foreign tax identification number.
// NIF must be 1-40 alphanumeric characters.
func (v *TakerValidator) validateNIF(nif string) []ValidationError {
	var errors []ValidationError

	// Check length
	if len(nif) == 0 {
		errors = append(errors, NewValidationError(
			"taker.nif",
			ValidationCodeRequired,
			"NIF cannot be empty",
		))
		return errors
	}

	if len(nif) > NIFMaxLength {
		errors = append(errors, NewValidationError(
			"taker.nif",
			ValidationCodeTooLong,
			"NIF must not exceed 40 characters",
		))
		return errors
	}

	// Check format (alphanumeric only)
	if !nifPattern.MatchString(nif) {
		errors = append(errors, NewValidationError(
			"taker.nif",
			ValidationCodeInvalidFormat,
			"NIF must contain only alphanumeric characters (1-40 chars)",
		))
	}

	return errors
}

// validateName validates the taker name.
func (v *TakerValidator) validateName(name string) []ValidationError {
	var errors []ValidationError

	// Check required
	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		errors = append(errors, NewValidationError(
			"taker.name",
			ValidationCodeRequired,
			"Taker name is required",
		))
		return errors
	}

	// Check length
	if len(trimmedName) > TakerNameMaxLength {
		errors = append(errors, NewValidationError(
			"taker.name",
			ValidationCodeTooLong,
			"Taker name must not exceed 300 characters",
		))
	}

	// Check for printable characters (no control characters)
	for _, r := range trimmedName {
		if unicode.IsControl(r) {
			errors = append(errors, NewValidationError(
				"taker.name",
				ValidationCodeInvalid,
				"Taker name contains invalid control characters",
			))
			break
		}
	}

	return errors
}

// validatePhone validates the taker phone number.
// For Brazilian takers (CNPJ/CPF), phone should be 10-11 digits.
// For foreign takers (NIF), international format is accepted.
func (v *TakerValidator) validatePhone(phone string, isForeign bool) []ValidationError {
	var errors []ValidationError

	// Clean phone number (remove common formatting)
	cleanPhone := cleanPhoneNumber(phone)

	if cleanPhone == "" {
		// Phone is optional, empty is valid
		return errors
	}

	if isForeign {
		// International phone: allow + prefix, 7-15 digits
		if !internationalPhonePattern.MatchString(cleanPhone) {
			errors = append(errors, NewValidationError(
				"taker.phone",
				ValidationCodeInvalidFormat,
				"Phone number must be 7-15 digits (international format)",
			))
		}
	} else {
		// Brazilian phone: 10-11 digits
		if !brazilianPhonePattern.MatchString(cleanPhone) {
			errors = append(errors, NewValidationError(
				"taker.phone",
				ValidationCodeInvalidFormat,
				"Phone number must be 10-11 digits (Brazilian format: DDD + number)",
			))
		}
	}

	return errors
}

// validateEmail validates the taker email address.
func (v *TakerValidator) validateEmail(email string) []ValidationError {
	var errors []ValidationError

	trimmedEmail := strings.TrimSpace(email)
	if trimmedEmail == "" {
		// Email is optional, empty is valid
		return errors
	}

	// Check length
	if len(trimmedEmail) > EmailMaxLength {
		errors = append(errors, NewValidationError(
			"taker.email",
			ValidationCodeTooLong,
			"Email must not exceed 254 characters",
		))
		return errors
	}

	// Check format
	if !emailPattern.MatchString(trimmedEmail) {
		errors = append(errors, NewValidationError(
			"taker.email",
			ValidationCodeInvalidFormat,
			"Email address is not in a valid format",
		))
	}

	return errors
}

// validateTakerAddress validates the taker address based on taker type.
// - CNPJ takers (B2B): address is required, must be national address
// - CPF takers (B2C): address is optional, if provided must be national
// - NIF takers (foreign): address is required, must be foreign address
func (v *TakerValidator) validateTakerAddress(taker *emission.TakerRequest) []ValidationError {
	var errors []ValidationError

	// Determine if address is required
	if taker.CNPJ != "" {
		// B2B: address is required for company takers
		if taker.Address == nil {
			errors = append(errors, NewValidationError(
				"taker.address",
				ValidationCodeRequired,
				"Address is required for company takers (CNPJ)",
			))
			return errors
		}
		// Validate as national address
		errors = append(errors, v.validateNationalAddress(taker.Address)...)
	} else if taker.CPF != "" {
		// B2C: address is optional for individual takers
		if taker.Address != nil {
			// If provided, validate as national address
			errors = append(errors, v.validateNationalAddress(taker.Address)...)
		}
	} else if taker.NIF != "" {
		// Foreign: address is required for foreign takers
		if taker.Address == nil {
			errors = append(errors, NewValidationError(
				"taker.address",
				ValidationCodeRequired,
				"Address is required for foreign takers (NIF)",
			))
			return errors
		}
		// Validate as foreign address
		errors = append(errors, v.validateForeignAddress(taker.Address)...)
	}

	return errors
}

// ValidateAddress validates an address and returns validation errors.
// The isForeign parameter determines which validation rules to apply.
func (v *TakerValidator) ValidateAddress(addr *emission.AddressRequest, isForeign bool) []ValidationError {
	if addr == nil {
		return nil
	}

	if isForeign {
		return v.validateForeignAddress(addr)
	}
	return v.validateNationalAddress(addr)
}

// validateNationalAddress validates a Brazilian (national) address.
func (v *TakerValidator) validateNationalAddress(addr *emission.AddressRequest) []ValidationError {
	var errors []ValidationError

	// Validate common fields
	errors = append(errors, v.validateAddressCommonFields(addr)...)

	// Municipality code is required (7 digits IBGE code)
	if addr.MunicipalityCode == "" {
		errors = append(errors, NewValidationError(
			"taker.address.municipality_code",
			ValidationCodeRequired,
			"Municipality code (IBGE) is required for Brazilian addresses",
		))
	} else if !municipalityCodePattern.MatchString(addr.MunicipalityCode) {
		errors = append(errors, NewValidationError(
			"taker.address.municipality_code",
			ValidationCodeInvalidFormat,
			"Municipality code must be exactly 7 digits (IBGE code)",
		))
	}

	// State is required (2 chars)
	if addr.State == "" {
		errors = append(errors, NewValidationError(
			"taker.address.state",
			ValidationCodeRequired,
			"State (UF) is required for Brazilian addresses",
		))
	} else {
		upperState := strings.ToUpper(addr.State)
		if !statePattern.MatchString(upperState) {
			errors = append(errors, NewValidationError(
				"taker.address.state",
				ValidationCodeInvalidFormat,
				"State must be exactly 2 uppercase letters",
			))
		} else if !validBrazilianStates[upperState] {
			errors = append(errors, NewValidationError(
				"taker.address.state",
				ValidationCodeInvalid,
				"State is not a valid Brazilian state code",
			))
		}
	}

	// Postal code is required (8 digits CEP)
	if addr.PostalCode == "" {
		errors = append(errors, NewValidationError(
			"taker.address.postal_code",
			ValidationCodeRequired,
			"Postal code (CEP) is required for Brazilian addresses",
		))
	} else {
		cleanPostal := strings.ReplaceAll(addr.PostalCode, "-", "")
		if !postalCodePattern.MatchString(cleanPostal) {
			errors = append(errors, NewValidationError(
				"taker.address.postal_code",
				ValidationCodeInvalidFormat,
				"Postal code (CEP) must be exactly 8 digits",
			))
		}
	}

	// Country code should be BR or empty for national addresses
	if addr.CountryCode != "" && addr.CountryCode != "BR" {
		errors = append(errors, NewValidationError(
			"taker.address.country_code",
			ValidationCodeInvalid,
			"Country code must be 'BR' or empty for Brazilian addresses",
		))
	}

	return errors
}

// validateForeignAddress validates a foreign (non-Brazilian) address.
func (v *TakerValidator) validateForeignAddress(addr *emission.AddressRequest) []ValidationError {
	var errors []ValidationError

	// Validate common fields
	errors = append(errors, v.validateAddressCommonFields(addr)...)

	// Country code is required and must NOT be BR
	if addr.CountryCode == "" {
		errors = append(errors, NewValidationError(
			"taker.address.country_code",
			ValidationCodeRequired,
			"Country code is required for foreign addresses",
		))
	} else if !countryCodePattern.MatchString(addr.CountryCode) {
		errors = append(errors, NewValidationError(
			"taker.address.country_code",
			ValidationCodeInvalidFormat,
			"Country code must be exactly 2 uppercase letters (ISO 3166-1 alpha-2)",
		))
	} else if addr.CountryCode == "BR" {
		errors = append(errors, NewValidationError(
			"taker.address.country_code",
			ValidationCodeInvalid,
			"Foreign address cannot have 'BR' as country code",
		))
	}

	// Municipality code should NOT be provided for foreign addresses
	if addr.MunicipalityCode != "" {
		errors = append(errors, NewValidationError(
			"taker.address.municipality_code",
			ValidationCodeInvalid,
			"Municipality code (IBGE) should not be provided for foreign addresses",
		))
	}

	// State should NOT be provided for foreign addresses
	if addr.State != "" {
		errors = append(errors, NewValidationError(
			"taker.address.state",
			ValidationCodeInvalid,
			"State (UF) should not be provided for foreign addresses",
		))
	}

	// Postal code is optional for foreign addresses
	// (no validation on format since it varies by country)

	return errors
}

// validateAddressCommonFields validates fields common to both national and foreign addresses.
func (v *TakerValidator) validateAddressCommonFields(addr *emission.AddressRequest) []ValidationError {
	var errors []ValidationError

	// Street is required
	if strings.TrimSpace(addr.Street) == "" {
		errors = append(errors, NewValidationError(
			"taker.address.street",
			ValidationCodeRequired,
			"Street is required",
		))
	} else if len(addr.Street) > AddressStreetMaxLength {
		errors = append(errors, NewValidationError(
			"taker.address.street",
			ValidationCodeTooLong,
			"Street must not exceed 255 characters",
		))
	}

	// Number is required
	if strings.TrimSpace(addr.Number) == "" {
		errors = append(errors, NewValidationError(
			"taker.address.number",
			ValidationCodeRequired,
			"Number is required",
		))
	} else if len(addr.Number) > AddressNumberMaxLength {
		errors = append(errors, NewValidationError(
			"taker.address.number",
			ValidationCodeTooLong,
			"Number must not exceed 60 characters",
		))
	}

	// Complement is optional, but validate length if provided
	if addr.Complement != "" && len(addr.Complement) > AddressComplementMaxLength {
		errors = append(errors, NewValidationError(
			"taker.address.complement",
			ValidationCodeTooLong,
			"Complement must not exceed 156 characters",
		))
	}

	// Neighborhood is required
	if strings.TrimSpace(addr.Neighborhood) == "" {
		errors = append(errors, NewValidationError(
			"taker.address.neighborhood",
			ValidationCodeRequired,
			"Neighborhood is required",
		))
	} else if len(addr.Neighborhood) > AddressNeighborhoodMaxLength {
		errors = append(errors, NewValidationError(
			"taker.address.neighborhood",
			ValidationCodeTooLong,
			"Neighborhood must not exceed 60 characters",
		))
	}

	return errors
}

// cleanPhoneNumber removes common formatting characters from phone numbers.
func cleanPhoneNumber(phone string) string {
	phone = strings.ReplaceAll(phone, " ", "")
	phone = strings.ReplaceAll(phone, "-", "")
	phone = strings.ReplaceAll(phone, "(", "")
	phone = strings.ReplaceAll(phone, ")", "")
	phone = strings.ReplaceAll(phone, ".", "")
	return phone
}
