// Package dpsid provides utilities for parsing and validating DPS (Declaracao de
// Prestacao de Servico) identifiers used in Brazil's Sistema Nacional NFS-e.
//
// A DPS identifier is a 42-character numeric string with the following structure:
//   - Municipality Code (7 digits): IBGE municipality code
//   - Registration Type (1 digit): 1=CNPJ, 2=CPF
//   - Federal Registration (14 digits): CNPJ or CPF padded with leading zeros
//   - Series (5 digits): DPS series number
//   - Number (15 digits): DPS sequential number
//
// Example: "3550308112345678000199000010000000000000001"
//
//	          |     |              |    |
//	          7     1      14      5   15 = 42 characters
package dpsid

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// DPS ID length constants.
const (
	// DPSIDLength is the total length of a DPS identifier (42 characters).
	DPSIDLength = 42

	// MunicipalityCodeLength is the length of the IBGE municipality code (7 digits).
	MunicipalityCodeLength = 7

	// RegistrationTypeLength is the length of the registration type field (1 digit).
	RegistrationTypeLength = 1

	// FederalRegistrationLength is the length of the federal registration field (14 digits).
	FederalRegistrationLength = 14

	// SeriesLength is the length of the DPS series field (5 digits).
	SeriesLength = 5

	// NumberLength is the length of the DPS number field (15 digits).
	NumberLength = 15
)

// Registration type constants.
const (
	// RegistrationTypeCNPJ indicates the provider is identified by CNPJ (company).
	RegistrationTypeCNPJ = 1

	// RegistrationTypeCPF indicates the provider is identified by CPF (individual).
	RegistrationTypeCPF = 2
)

// Byte offsets for parsing the DPS ID.
const (
	offsetMunicipalityCode    = 0
	offsetRegistrationType    = MunicipalityCodeLength
	offsetFederalRegistration = offsetRegistrationType + RegistrationTypeLength
	offsetSeries              = offsetFederalRegistration + FederalRegistrationLength
	offsetNumber              = offsetSeries + SeriesLength
)

// Error definitions for DPS ID parsing and validation.
var (
	// ErrInvalidLength indicates the DPS ID does not have exactly 42 characters.
	ErrInvalidLength = errors.New("dps id must be exactly 42 characters")

	// ErrInvalidCharacters indicates the DPS ID contains non-numeric characters.
	ErrInvalidCharacters = errors.New("dps id must contain only numeric characters")

	// ErrInvalidMunicipalityCode indicates an invalid IBGE municipality code.
	ErrInvalidMunicipalityCode = errors.New("invalid municipality code: must be 7 digits")

	// ErrInvalidRegistrationType indicates an invalid registration type.
	ErrInvalidRegistrationType = errors.New("invalid registration type: must be 1 (CNPJ) or 2 (CPF)")

	// ErrInvalidFederalRegistration indicates an invalid federal registration format.
	ErrInvalidFederalRegistration = errors.New("invalid federal registration: must be 14 digits")

	// ErrInvalidSeries indicates an invalid DPS series format.
	ErrInvalidSeries = errors.New("invalid series: must be 5 digits")

	// ErrInvalidNumber indicates an invalid DPS number format.
	ErrInvalidNumber = errors.New("invalid number: must be 15 digits")

	// ErrEmptyDPSID indicates an empty DPS ID was provided.
	ErrEmptyDPSID = errors.New("dps id cannot be empty")

	// ErrInvalidCPFPadding indicates that a CPF registration does not have the required "000" prefix.
	ErrInvalidCPFPadding = errors.New("invalid cpf federal registration: must start with 000")
)

// numericRegex validates that a string contains only digits.
var numericRegex = regexp.MustCompile(`^\d+$`)

// DPSIdentifier represents the parsed components of a DPS identifier.
// Each field corresponds to a segment of the 42-character DPS ID.
type DPSIdentifier struct {
	// MunicipalityCode is the 7-digit IBGE code of the municipality where
	// the service provider is registered.
	MunicipalityCode string

	// RegistrationType indicates the type of federal registration:
	// 1 = CNPJ (company), 2 = CPF (individual/MEI).
	RegistrationType int

	// FederalRegistration is the 14-digit federal registration number.
	// For CNPJ: the full 14-digit number.
	// For CPF: the 11-digit number padded with 3 leading zeros.
	FederalRegistration string

	// Series is the 5-digit DPS series number, typically starting from "00001".
	Series string

	// Number is the 15-digit sequential DPS number within the series.
	Number string
}

// Parse parses a 42-character DPS identifier string and returns its components.
// Returns an error if the format is invalid.
//
// Example:
//
//	id, err := dpsid.Parse("3550308112345678000199000010000000000000001")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Printf("Municipality: %s, CNPJ: %s\n", id.MunicipalityCode, id.FederalRegistration)
func Parse(id string) (*DPSIdentifier, error) {
	// Remove any whitespace
	id = strings.TrimSpace(id)

	if id == "" {
		return nil, ErrEmptyDPSID
	}

	if len(id) != DPSIDLength {
		return nil, fmt.Errorf("%w: got %d characters", ErrInvalidLength, len(id))
	}

	if !numericRegex.MatchString(id) {
		return nil, ErrInvalidCharacters
	}

	// Extract components
	municipalityCode := id[offsetMunicipalityCode : offsetMunicipalityCode+MunicipalityCodeLength]
	registrationTypeStr := id[offsetRegistrationType : offsetRegistrationType+RegistrationTypeLength]
	federalRegistration := id[offsetFederalRegistration : offsetFederalRegistration+FederalRegistrationLength]
	series := id[offsetSeries : offsetSeries+SeriesLength]
	number := id[offsetNumber : offsetNumber+NumberLength]

	// Parse registration type
	registrationType := int(registrationTypeStr[0] - '0')
	if registrationType != RegistrationTypeCNPJ && registrationType != RegistrationTypeCPF {
		return nil, ErrInvalidRegistrationType
	}

	// For CPF registrations, the first 3 digits must be "000" (11-digit CPF padded to 14)
	if registrationType == RegistrationTypeCPF && federalRegistration[:3] != "000" {
		return nil, ErrInvalidCPFPadding
	}

	return &DPSIdentifier{
		MunicipalityCode:    municipalityCode,
		RegistrationType:    registrationType,
		FederalRegistration: federalRegistration,
		Series:              series,
		Number:              number,
	}, nil
}

// String serializes the DPSIdentifier back to its 42-character string representation.
// This method implements the fmt.Stringer interface.
//
// Example:
//
//	id := &DPSIdentifier{
//	    MunicipalityCode:    "3550308",
//	    RegistrationType:    1,
//	    FederalRegistration: "12345678000199",
//	    Series:              "00001",
//	    Number:              "000000000000001",
//	}
//	fmt.Println(id.String()) // "3550308112345678000199000010000000000000001"
func (d *DPSIdentifier) String() string {
	if d == nil {
		return ""
	}

	return fmt.Sprintf("%s%d%s%s%s",
		d.MunicipalityCode,
		d.RegistrationType,
		d.FederalRegistration,
		d.Series,
		d.Number,
	)
}

// Validate checks if the DPSIdentifier has valid values for all fields.
// Returns nil if valid, or an error describing the validation failure.
//
// Validation rules:
//   - MunicipalityCode: exactly 7 numeric digits
//   - RegistrationType: 1 (CNPJ) or 2 (CPF)
//   - FederalRegistration: exactly 14 numeric digits
//   - Series: exactly 5 numeric digits
//   - Number: exactly 15 numeric digits
func (d *DPSIdentifier) Validate() error {
	if d == nil {
		return ErrEmptyDPSID
	}

	// Validate municipality code
	if len(d.MunicipalityCode) != MunicipalityCodeLength {
		return fmt.Errorf("%w: got %d digits", ErrInvalidMunicipalityCode, len(d.MunicipalityCode))
	}
	if !numericRegex.MatchString(d.MunicipalityCode) {
		return fmt.Errorf("%w: contains non-numeric characters", ErrInvalidMunicipalityCode)
	}

	// Validate registration type
	if d.RegistrationType != RegistrationTypeCNPJ && d.RegistrationType != RegistrationTypeCPF {
		return fmt.Errorf("%w: got %d", ErrInvalidRegistrationType, d.RegistrationType)
	}

	// Validate federal registration
	if len(d.FederalRegistration) != FederalRegistrationLength {
		return fmt.Errorf("%w: got %d digits", ErrInvalidFederalRegistration, len(d.FederalRegistration))
	}
	if !numericRegex.MatchString(d.FederalRegistration) {
		return fmt.Errorf("%w: contains non-numeric characters", ErrInvalidFederalRegistration)
	}

	// For CPF registrations, the first 3 digits must be "000" (11-digit CPF padded to 14)
	if d.RegistrationType == RegistrationTypeCPF && d.FederalRegistration[:3] != "000" {
		return ErrInvalidCPFPadding
	}

	// Validate series
	if len(d.Series) != SeriesLength {
		return fmt.Errorf("%w: got %d digits", ErrInvalidSeries, len(d.Series))
	}
	if !numericRegex.MatchString(d.Series) {
		return fmt.Errorf("%w: contains non-numeric characters", ErrInvalidSeries)
	}

	// Validate number
	if len(d.Number) != NumberLength {
		return fmt.Errorf("%w: got %d digits", ErrInvalidNumber, len(d.Number))
	}
	if !numericRegex.MatchString(d.Number) {
		return fmt.Errorf("%w: contains non-numeric characters", ErrInvalidNumber)
	}

	return nil
}

// IsCNPJ returns true if the provider is identified by CNPJ (company).
func (d *DPSIdentifier) IsCNPJ() bool {
	return d != nil && d.RegistrationType == RegistrationTypeCNPJ
}

// IsCPF returns true if the provider is identified by CPF (individual/MEI).
func (d *DPSIdentifier) IsCPF() bool {
	return d != nil && d.RegistrationType == RegistrationTypeCPF
}

// GetCNPJ returns the CNPJ if this is a company registration, otherwise empty string.
func (d *DPSIdentifier) GetCNPJ() string {
	if d.IsCNPJ() {
		return d.FederalRegistration
	}
	return ""
}

// GetCPF returns the CPF (without leading zeros) if this is an individual registration.
// CPF is stored as 14 digits with 3 leading zeros, so we strip them to get the 11-digit CPF.
func (d *DPSIdentifier) GetCPF() string {
	if d.IsCPF() {
		// Remove the 3 leading zeros to get the 11-digit CPF
		return strings.TrimLeft(d.FederalRegistration, "0")
	}
	return ""
}

// New creates a new DPSIdentifier with the provided values.
// Use this constructor when building a DPS ID from individual components.
// The function normalizes inputs (pads numbers with zeros) and validates the result.
//
// Parameters:
//   - municipalityCode: 7-digit IBGE code
//   - registrationType: 1 for CNPJ, 2 for CPF
//   - federalRegistration: CNPJ (14 digits) or CPF (11 digits)
//   - series: DPS series (will be padded to 5 digits)
//   - number: DPS number (will be padded to 15 digits)
func New(municipalityCode string, registrationType int, federalRegistration string, series string, number string) (*DPSIdentifier, error) {
	// Normalize municipality code
	municipalityCode = strings.TrimSpace(municipalityCode)

	// Normalize federal registration based on type
	federalRegistration = strings.TrimSpace(federalRegistration)
	// Remove any formatting characters
	federalRegistration = strings.ReplaceAll(federalRegistration, ".", "")
	federalRegistration = strings.ReplaceAll(federalRegistration, "-", "")
	federalRegistration = strings.ReplaceAll(federalRegistration, "/", "")

	// Pad CPF to 14 digits if needed
	if registrationType == RegistrationTypeCPF && len(federalRegistration) == 11 {
		federalRegistration = fmt.Sprintf("%014s", federalRegistration)
	}

	// Normalize series (pad to 5 digits)
	series = strings.TrimSpace(series)
	if len(series) > 0 && len(series) < SeriesLength {
		series = fmt.Sprintf("%05s", series)
	}

	// Normalize number (pad to 15 digits)
	number = strings.TrimSpace(number)
	if len(number) > 0 && len(number) < NumberLength {
		number = fmt.Sprintf("%015s", number)
	}

	id := &DPSIdentifier{
		MunicipalityCode:    municipalityCode,
		RegistrationType:    registrationType,
		FederalRegistration: federalRegistration,
		Series:              series,
		Number:              number,
	}

	// Validate the constructed identifier
	if err := id.Validate(); err != nil {
		return nil, err
	}

	return id, nil
}

// MustParse is like Parse but panics on error.
// Use only when the input is guaranteed to be valid (e.g., in tests or for constants).
func MustParse(id string) *DPSIdentifier {
	parsed, err := Parse(id)
	if err != nil {
		panic(fmt.Sprintf("dpsid.MustParse: %v", err))
	}
	return parsed
}
