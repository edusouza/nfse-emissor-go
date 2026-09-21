// Copyright 2026 Eduardo Souza
// SPDX-License-Identifier: FSL-1.1-MIT

// Package domain provides the core domain entities shared across the NFS-e emitter.
package domain

// Address represents an address that can be either Brazilian (national) or foreign.
// For national addresses, MunicipalityCode, State, and PostalCode are required.
// For foreign addresses, only CountryCode, Street, Number, and Neighborhood are required.
type Address struct {
	// Street is the street name (logradouro / xLgr).
	Street string `json:"street"`

	// Number is the building/house number (nro).
	Number string `json:"number"`

	// Complement is additional address information (apartment, suite, etc.) (xCpl).
	Complement string `json:"complement,omitempty"`

	// Neighborhood is the district or neighborhood (bairro / xBairro).
	Neighborhood string `json:"neighborhood"`

	// MunicipalityCode is the 7-digit IBGE municipality code (cMun).
	// Required for national addresses, empty for foreign addresses.
	MunicipalityCode string `json:"municipality_code,omitempty"`

	// State is the 2-letter state abbreviation (UF).
	// Required for national addresses, empty for foreign addresses.
	State string `json:"state,omitempty"`

	// PostalCode is the 8-digit CEP (postal code) without formatting.
	// Required for national addresses, empty for foreign addresses.
	PostalCode string `json:"postal_code,omitempty"`

	// CountryCode is the ISO 3166-1 alpha-2 country code (cPais).
	// Defaults to "BR" for national addresses.
	CountryCode string `json:"country_code,omitempty"`
}

// IsForeign returns true if the address is outside Brazil.
// An address is considered foreign if CountryCode is set and is not "BR".
func (a *Address) IsForeign() bool {
	if a == nil {
		return false
	}
	return a.CountryCode != "" && a.CountryCode != "BR"
}

// Values holds the monetary amounts of a service provision.
type Values struct {
	// ServiceValue is the gross value of the service (valor do servico).
	ServiceValue float64 `json:"service_value"`

	// UnconditionalDiscount is a discount applied regardless of payment conditions.
	UnconditionalDiscount float64 `json:"unconditional_discount,omitempty"`

	// ConditionalDiscount is a discount conditional on payment terms.
	ConditionalDiscount float64 `json:"conditional_discount,omitempty"`

	// Deductions are legally permitted deductions from the service value.
	Deductions float64 `json:"deductions,omitempty"`
}

// NetValue calculates the net value after discounts and deductions.
func (v *Values) NetValue() float64 {
	return v.ServiceValue - v.UnconditionalDiscount - v.ConditionalDiscount - v.Deductions
}

// Tax regime constants for service providers.
const (
	// TaxRegimeMEI represents the Microempreendedor Individual regime.
	TaxRegimeMEI = "mei"

	// TaxRegimeMEEPP represents the Microempresa ou Empresa de Pequeno Porte regime.
	TaxRegimeMEEPP = "me_epp"
)
