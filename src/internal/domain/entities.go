// Package domain provides core domain entities and business logic for the NFS-e API.
// These entities represent the fundamental data structures used throughout the application.
package domain

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Address represents an address that can be either Brazilian (national) or foreign.
// For national addresses, MunicipalityCode, State, and PostalCode are required.
// For foreign addresses, only CountryCode, Street, Number, and Neighborhood are required.
type Address struct {
	// Street is the street name (logradouro / xLgr).
	Street string `json:"street" bson:"street"`

	// Number is the building/house number (nro).
	Number string `json:"number" bson:"number"`

	// Complement is additional address information (apartment, suite, etc.) (xCpl).
	Complement string `json:"complement,omitempty" bson:"complement,omitempty"`

	// Neighborhood is the district or neighborhood (bairro / xBairro).
	Neighborhood string `json:"neighborhood" bson:"neighborhood"`

	// MunicipalityCode is the 7-digit IBGE municipality code (cMun).
	// Required for national addresses, empty for foreign addresses.
	MunicipalityCode string `json:"municipality_code,omitempty" bson:"municipality_code,omitempty"`

	// State is the 2-letter state abbreviation (UF).
	// Required for national addresses, empty for foreign addresses.
	State string `json:"state,omitempty" bson:"state,omitempty"`

	// PostalCode is the 8-digit CEP (postal code) without formatting.
	// Required for national addresses, empty for foreign addresses.
	PostalCode string `json:"postal_code,omitempty" bson:"postal_code,omitempty"`

	// CountryCode is the ISO 3166-1 alpha-2 country code (cPais).
	// Defaults to "BR" for national addresses.
	CountryCode string `json:"country_code,omitempty" bson:"country_code,omitempty"`
}

// IsForeign returns true if the address is outside Brazil.
// An address is considered foreign if CountryCode is set and is not "BR".
func (a *Address) IsForeign() bool {
	if a == nil {
		return false
	}
	return a.CountryCode != "" && a.CountryCode != "BR"
}

// GetCountryCode returns the country code, defaulting to "BR" if not set.
func (a *Address) GetCountryCode() string {
	if a == nil || a.CountryCode == "" {
		return "BR"
	}
	return a.CountryCode
}

// Provider represents a service provider (prestador) in an NFS-e transaction.
// Providers must be registered with a valid CNPJ and municipal registration.
type Provider struct {
	// CNPJ is the 14-digit tax ID of the service provider (without formatting).
	CNPJ string `json:"cnpj" bson:"cnpj"`

	// TaxRegime indicates the tax regime: "mei" (Microempreendedor Individual)
	// or "me_epp" (Microempresa ou Empresa de Pequeno Porte).
	TaxRegime string `json:"tax_regime" bson:"tax_regime"`

	// Name is the legal name (razao social) of the provider.
	Name string `json:"name" bson:"name"`

	// MunicipalRegistration is the municipal service provider registration number.
	MunicipalRegistration string `json:"municipal_registration,omitempty" bson:"municipal_registration,omitempty"`

	// Address is the provider's business address.
	Address *Address `json:"address,omitempty" bson:"address,omitempty"`
}

// TakerIdentificationType represents the type of identification used for a taker.
type TakerIdentificationType string

const (
	// TakerIdentificationCNPJ indicates the taker is a Brazilian company (CNPJ).
	TakerIdentificationCNPJ TakerIdentificationType = "cnpj"

	// TakerIdentificationCPF indicates the taker is a Brazilian individual (CPF).
	TakerIdentificationCPF TakerIdentificationType = "cpf"

	// TakerIdentificationNIF indicates the taker is a foreign entity (NIF).
	TakerIdentificationNIF TakerIdentificationType = "nif"

	// TakerIdentificationNone indicates no identification was provided.
	TakerIdentificationNone TakerIdentificationType = ""
)

// Taker represents a service taker (tomador) in an NFS-e transaction.
// A taker can be identified by CNPJ (company), CPF (individual), or NIF (foreign).
// These identification types are mutually exclusive.
type Taker struct {
	// CNPJ is the 14-digit tax ID for company takers (without formatting).
	// Mutually exclusive with CPF and NIF.
	CNPJ string `json:"cnpj,omitempty" bson:"cnpj,omitempty"`

	// CPF is the 11-digit tax ID for individual takers (without formatting).
	// Mutually exclusive with CNPJ and NIF.
	CPF string `json:"cpf,omitempty" bson:"cpf,omitempty"`

	// NIF is the foreign tax identification number for non-Brazilian takers.
	// Valid NIF: 1-40 alphanumeric characters.
	// Mutually exclusive with CNPJ and CPF.
	NIF string `json:"nif,omitempty" bson:"nif,omitempty"`

	// Name is the legal name or full name of the taker (xNome).
	// Required field, max 300 characters.
	Name string `json:"name" bson:"name"`

	// Phone is the taker's phone number (fone).
	// Optional. For Brazilian numbers: 10-11 digits.
	Phone string `json:"phone,omitempty" bson:"phone,omitempty"`

	// Email is the taker's email address.
	// Optional. Must be a valid email format if provided.
	Email string `json:"email,omitempty" bson:"email,omitempty"`

	// Address is the taker's address (end).
	// Required for B2B transactions (CNPJ takers).
	// Optional for B2C transactions (CPF takers).
	// For foreign takers (NIF), a simplified foreign address is required.
	Address *Address `json:"address,omitempty" bson:"address,omitempty"`
}

// GetIdentificationType returns the type of identification used for this taker.
// Returns TakerIdentificationNone if no identification is provided.
// If multiple identifications are set (invalid state), returns the first found
// in order: CNPJ, CPF, NIF.
func (t *Taker) GetIdentificationType() TakerIdentificationType {
	if t == nil {
		return TakerIdentificationNone
	}
	if t.CNPJ != "" {
		return TakerIdentificationCNPJ
	}
	if t.CPF != "" {
		return TakerIdentificationCPF
	}
	if t.NIF != "" {
		return TakerIdentificationNIF
	}
	return TakerIdentificationNone
}

// IsForeign returns true if the taker has a NIF (foreign tax ID).
// Foreign takers are non-Brazilian entities identified by NIF instead of CNPJ/CPF.
func (t *Taker) IsForeign() bool {
	if t == nil {
		return false
	}
	return t.NIF != ""
}

// IsCompany returns true if the taker is a Brazilian company (CNPJ).
func (t *Taker) IsCompany() bool {
	if t == nil {
		return false
	}
	return t.CNPJ != ""
}

// IsIndividual returns true if the taker is a Brazilian individual (CPF).
func (t *Taker) IsIndividual() bool {
	if t == nil {
		return false
	}
	return t.CPF != ""
}

// GetIdentification returns the identification value (CNPJ, CPF, or NIF).
// Returns an empty string if no identification is set.
func (t *Taker) GetIdentification() string {
	if t == nil {
		return ""
	}
	if t.CNPJ != "" {
		return t.CNPJ
	}
	if t.CPF != "" {
		return t.CPF
	}
	if t.NIF != "" {
		return t.NIF
	}
	return ""
}

// CountIdentifications counts how many identification fields are set.
// Valid takers should have exactly 1 identification.
func (t *Taker) CountIdentifications() int {
	if t == nil {
		return 0
	}
	count := 0
	if t.CNPJ != "" {
		count++
	}
	if t.CPF != "" {
		count++
	}
	if t.NIF != "" {
		count++
	}
	return count
}

// Service represents a service being invoiced in an NFS-e.
// Services are identified by national and municipal codes.
type Service struct {
	// NationalCode is the 6-digit cTribNac (national service code).
	NationalCode string `json:"national_code" bson:"national_code"`

	// MunicipalCode is the local service code assigned by the municipality.
	MunicipalCode string `json:"municipal_code,omitempty" bson:"municipal_code,omitempty"`

	// Description is a detailed description of the service provided.
	Description string `json:"description" bson:"description"`

	// MunicipalityCode is the 7-digit IBGE code of the municipality where
	// the service was provided (local de prestacao).
	MunicipalityCode string `json:"municipality_code" bson:"municipality_code"`

	// CountryCode is the ISO country code for services exported abroad.
	// Required when the service is provided to a foreign taker.
	CountryCode string `json:"country_code,omitempty" bson:"country_code,omitempty"`
}

// Values represents the monetary values in an NFS-e.
// All values are in Brazilian Reais (BRL).
type Values struct {
	// ServiceValue is the gross value of the service (valor do servico).
	ServiceValue float64 `json:"service_value" bson:"service_value"`

	// UnconditionalDiscount is a discount applied regardless of payment conditions.
	UnconditionalDiscount float64 `json:"unconditional_discount,omitempty" bson:"unconditional_discount,omitempty"`

	// ConditionalDiscount is a discount conditional on payment terms.
	ConditionalDiscount float64 `json:"conditional_discount,omitempty" bson:"conditional_discount,omitempty"`

	// Deductions are legally permitted deductions from the service value.
	Deductions float64 `json:"deductions,omitempty" bson:"deductions,omitempty"`
}

// NetValue calculates the net value after discounts and deductions.
func (v *Values) NetValue() float64 {
	return v.ServiceValue - v.UnconditionalDiscount - v.ConditionalDiscount - v.Deductions
}

// DPSInfo contains information about the DPS (Documento de Prestacao de Servicos).
// This is the initial service document before conversion to NFS-e.
type DPSInfo struct {
	// Series is a 5-digit series identifier.
	Series string `json:"series" bson:"series"`

	// Number is a 1-15 digit document number.
	Number string `json:"number" bson:"number"`
}

// TaxRegime constants for service providers.
const (
	// TaxRegimeMEI represents Microempreendedor Individual regime.
	TaxRegimeMEI = "mei"

	// TaxRegimeMEEPP represents Microempresa ou Empresa de Pequeno Porte regime.
	TaxRegimeMEEPP = "me_epp"
)

// EmissionStatus represents the status of an NFS-e emission.
type EmissionStatus string

const (
	// EmissionStatusPending indicates the emission is queued for processing.
	EmissionStatusPending EmissionStatus = "pending"

	// EmissionStatusProcessing indicates the emission is currently being processed.
	EmissionStatusProcessing EmissionStatus = "processing"

	// EmissionStatusCompleted indicates the emission was successful.
	EmissionStatusCompleted EmissionStatus = "completed"

	// EmissionStatusFailed indicates the emission failed.
	EmissionStatusFailed EmissionStatus = "failed"

	// EmissionStatusCancelled indicates the emission was cancelled.
	EmissionStatusCancelled EmissionStatus = "cancelled"
)

// EmissionRequest represents a request to emit an NFS-e.
type EmissionRequest struct {
	// ID is the unique identifier for this emission request.
	ID primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`

	// IntegratorID identifies the API key/integrator that submitted this request.
	IntegratorID string `json:"integrator_id" bson:"integrator_id"`

	// IdempotencyKey is a client-provided key to prevent duplicate emissions.
	IdempotencyKey string `json:"idempotency_key" bson:"idempotency_key"`

	// Provider is the service provider information.
	Provider Provider `json:"provider" bson:"provider"`

	// Taker is the service taker information.
	Taker *Taker `json:"taker,omitempty" bson:"taker,omitempty"`

	// Service contains service details.
	Service Service `json:"service" bson:"service"`

	// Values contains monetary values.
	Values Values `json:"values" bson:"values"`

	// DPS contains the source document information.
	DPS DPSInfo `json:"dps" bson:"dps"`

	// Status is the current emission status.
	Status EmissionStatus `json:"status" bson:"status"`

	// NFSeNumber is the official NFS-e number assigned after successful emission.
	NFSeNumber string `json:"nfse_number,omitempty" bson:"nfse_number,omitempty"`

	// NFSeKey is the access key for the emitted NFS-e.
	NFSeKey string `json:"nfse_key,omitempty" bson:"nfse_key,omitempty"`

	// ErrorCode is the error code if emission failed.
	ErrorCode string `json:"error_code,omitempty" bson:"error_code,omitempty"`

	// ErrorMessage is the error description if emission failed.
	ErrorMessage string `json:"error_message,omitempty" bson:"error_message,omitempty"`

	// Attempts tracks the number of emission attempts.
	Attempts int `json:"attempts" bson:"attempts"`

	// CreatedAt is when the request was created.
	CreatedAt time.Time `json:"created_at" bson:"created_at"`

	// UpdatedAt is when the request was last updated.
	UpdatedAt time.Time `json:"updated_at" bson:"updated_at"`

	// CompletedAt is when the emission was completed (success or final failure).
	CompletedAt *time.Time `json:"completed_at,omitempty" bson:"completed_at,omitempty"`
}

// Event represents a generic event that occurred during NFS-e processing.
type Event struct {
	// ID is the unique identifier for this event.
	ID primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`

	// EmissionRequestID is the related emission request.
	EmissionRequestID primitive.ObjectID `json:"emission_request_id" bson:"emission_request_id"`

	// Type is the event type (e.g., "status_changed", "sefin_response").
	Type string `json:"type" bson:"type"`

	// Data contains event-specific data.
	Data map[string]interface{} `json:"data,omitempty" bson:"data,omitempty"`

	// Timestamp is when the event occurred.
	Timestamp time.Time `json:"timestamp" bson:"timestamp"`
}
