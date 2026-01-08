// Package emission provides DTOs and business logic for NFS-e emission operations.
package emission

// EmissionRequest represents the incoming request to emit an NFS-e.
// This DTO matches the OpenAPI specification for POST /v1/nfse.
type EmissionRequest struct {
	// Provider contains the service provider (prestador) information.
	Provider ProviderRequest `json:"provider" binding:"required"`

	// Taker contains the service taker (tomador) information. Optional.
	Taker *TakerRequest `json:"taker,omitempty"`

	// Service contains the service details being invoiced.
	Service ServiceRequest `json:"service" binding:"required"`

	// Values contains the monetary values for the invoice.
	Values ValuesRequest `json:"values" binding:"required"`

	// DPS contains the source document (DPS) information.
	DPS DPSRequest `json:"dps" binding:"required"`

	// Certificate contains the digital certificate for signing. Optional for Phase 3.
	Certificate *CertificateRequest `json:"certificate,omitempty"`

	// WebhookURL is an optional override for the webhook URL configured in the API key.
	WebhookURL string `json:"webhook_url,omitempty"`
}

// ProviderRequest contains the service provider information in the emission request.
type ProviderRequest struct {
	// CNPJ is the 14-digit tax ID of the service provider (without formatting).
	CNPJ string `json:"cnpj" binding:"required"`

	// TaxRegime indicates the tax regime: "mei" or "me_epp".
	TaxRegime string `json:"tax_regime" binding:"required"`

	// Name is the legal name (razao social) of the provider.
	Name string `json:"name" binding:"required"`

	// MunicipalRegistration is the optional municipal service provider registration number.
	MunicipalRegistration string `json:"municipal_registration,omitempty"`
}

// TakerRequest contains the service taker information in the emission request.
type TakerRequest struct {
	// CNPJ is the 14-digit tax ID for company takers (without formatting).
	// Mutually exclusive with CPF and NIF.
	CNPJ string `json:"cnpj,omitempty"`

	// CPF is the 11-digit tax ID for individual takers (without formatting).
	// Mutually exclusive with CNPJ and NIF.
	CPF string `json:"cpf,omitempty"`

	// NIF is the foreign tax identification number for non-Brazilian takers.
	// Valid NIF: 1-40 alphanumeric characters.
	// Mutually exclusive with CNPJ and CPF.
	NIF string `json:"nif,omitempty"`

	// Name is the legal name or full name of the taker (max 300 chars).
	Name string `json:"name" binding:"required"`

	// Phone is the taker's phone number.
	// For Brazilian numbers: 10-11 digits (DDD + number).
	// For international: include country code.
	Phone string `json:"phone,omitempty"`

	// Email is the taker's email address.
	Email string `json:"email,omitempty"`

	// Address is the taker's address.
	// Required for B2B (CNPJ takers), optional for B2C (CPF takers).
	// For foreign takers (NIF), requires foreign address format.
	Address *AddressRequest `json:"address,omitempty"`
}

// AddressRequest contains address information in the emission request.
type AddressRequest struct {
	// Street is the street name (xLgr). Required.
	Street string `json:"street" binding:"required"`

	// Number is the building/house number (nro). Required.
	Number string `json:"number" binding:"required"`

	// Complement is additional address information (xCpl). Optional.
	Complement string `json:"complement,omitempty"`

	// Neighborhood is the district or neighborhood (xBairro). Required.
	Neighborhood string `json:"neighborhood" binding:"required"`

	// MunicipalityCode is the 7-digit IBGE municipality code (cMun).
	// Required for national (Brazilian) addresses.
	MunicipalityCode string `json:"municipality_code,omitempty"`

	// State is the 2-letter state abbreviation (UF).
	// Required for national (Brazilian) addresses.
	State string `json:"state,omitempty"`

	// PostalCode is the 8-digit CEP without formatting.
	// Required for national (Brazilian) addresses.
	PostalCode string `json:"postal_code,omitempty"`

	// CountryCode is the ISO 3166-1 alpha-2 country code (cPais).
	// Defaults to "BR" if not provided.
	// For foreign addresses, must be a non-BR code.
	CountryCode string `json:"country_code,omitempty"`
}

// ServiceRequest contains the service details in the emission request.
type ServiceRequest struct {
	// NationalCode is the 6-digit cTribNac (national service code).
	NationalCode string `json:"national_code" binding:"required"`

	// Description is a detailed description of the service provided.
	Description string `json:"description" binding:"required"`

	// MunicipalityCode is the 7-digit IBGE code of the municipality where
	// the service was provided (local de prestacao).
	MunicipalityCode string `json:"municipality_code" binding:"required"`
}

// ValuesRequest contains the monetary values in the emission request.
// These values are used to calculate the tax base according to Brazilian NFS-e rules:
// Tax Base (vBCCalc) = ServiceValue - UnconditionalDiscount - Deductions
// Note: ConditionalDiscount does NOT affect the tax base.
type ValuesRequest struct {
	// ServiceValue is the gross value of the service (valor do servico / vServ).
	// Required, must be greater than 0.
	ServiceValue float64 `json:"service_value" binding:"required,gt=0"`

	// UnconditionalDiscount is a discount applied regardless of payment conditions (vDescIncond).
	// Reduces the tax base. Optional, must be >= 0.
	UnconditionalDiscount float64 `json:"unconditional_discount,omitempty"`

	// ConditionalDiscount is a discount conditional on payment terms (vDescCond).
	// Does NOT reduce the tax base. Optional, must be >= 0.
	ConditionalDiscount float64 `json:"conditional_discount,omitempty"`

	// Deductions are legally permitted deductions from the service value (vDedRed / vDR).
	// Reduces the tax base. Optional, must be >= 0.
	Deductions float64 `json:"deductions,omitempty"`
}

// HasUnconditionalDiscount returns true if an unconditional discount is present.
func (v *ValuesRequest) HasUnconditionalDiscount() bool {
	return v != nil && v.UnconditionalDiscount > 0
}

// HasConditionalDiscount returns true if a conditional discount is present.
func (v *ValuesRequest) HasConditionalDiscount() bool {
	return v != nil && v.ConditionalDiscount > 0
}

// HasDeductions returns true if deductions are present.
func (v *ValuesRequest) HasDeductions() bool {
	return v != nil && v.Deductions > 0
}

// HasAnyDiscount returns true if any discount or deduction is present.
func (v *ValuesRequest) HasAnyDiscount() bool {
	return v.HasUnconditionalDiscount() || v.HasConditionalDiscount() || v.HasDeductions()
}

// CalculateTaxBase calculates the tax base according to Brazilian NFS-e rules.
// Tax Base = ServiceValue - UnconditionalDiscount - Deductions
// Note: ConditionalDiscount does NOT reduce the tax base.
// Returns 0 if the result would be negative (invalid state).
func (v *ValuesRequest) CalculateTaxBase() float64 {
	if v == nil {
		return 0
	}

	taxBase := v.ServiceValue - v.UnconditionalDiscount - v.Deductions

	// Tax base cannot be negative
	if taxBase < 0 {
		return 0
	}

	return taxBase
}

// CalculateDeductionPercentage calculates the deduction percentage.
// DeductionPercentage = (Deductions / ServiceValue) * 100
// Returns 0 if service value is 0 or no deductions are present.
func (v *ValuesRequest) CalculateDeductionPercentage() float64 {
	if v == nil || v.ServiceValue == 0 || v.Deductions == 0 {
		return 0
	}

	return (v.Deductions / v.ServiceValue) * 100
}

// CalculateNetValue calculates the net value after all discounts and deductions.
// NetValue = ServiceValue - UnconditionalDiscount - ConditionalDiscount - Deductions
// Note: This is different from tax base (which excludes conditional discount).
// Returns 0 if the result would be negative (invalid state).
func (v *ValuesRequest) CalculateNetValue() float64 {
	if v == nil {
		return 0
	}

	netValue := v.ServiceValue - v.UnconditionalDiscount - v.ConditionalDiscount - v.Deductions

	// Net value cannot be negative
	if netValue < 0 {
		return 0
	}

	return netValue
}

// TotalTaxBaseDeductions returns the total amount that reduces the tax base.
// This is UnconditionalDiscount + Deductions (not ConditionalDiscount).
func (v *ValuesRequest) TotalTaxBaseDeductions() float64 {
	if v == nil {
		return 0
	}

	return v.UnconditionalDiscount + v.Deductions
}

// TotalDiscounts returns the total of all discounts and deductions.
func (v *ValuesRequest) TotalDiscounts() float64 {
	if v == nil {
		return 0
	}

	return v.UnconditionalDiscount + v.ConditionalDiscount + v.Deductions
}

// DPSRequest contains the DPS (Documento de Prestacao de Servicos) information.
type DPSRequest struct {
	// Series is a 5-digit series identifier.
	Series string `json:"series" binding:"required"`

	// Number is a 1-15 digit document number.
	Number string `json:"number" binding:"required"`
}

// CertificateRequest contains the digital certificate for XML signing.
type CertificateRequest struct {
	// PFXBase64 is the PFX certificate encoded in base64.
	PFXBase64 string `json:"pfx_base64" binding:"required"`

	// Password is the certificate password.
	Password string `json:"password" binding:"required"`
}
