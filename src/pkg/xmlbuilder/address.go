// Package xmlbuilder provides utilities for building NFS-e XML documents
// according to Brazilian government specifications.
package xmlbuilder

import (
	"fmt"
	"strings"

	"github.com/beevik/etree"
	"github.com/eduardo/nfse-nacional/internal/domain"
)

// AddressConfig contains address information for XML generation.
type AddressConfig struct {
	// Street is the street name (xLgr).
	Street string

	// Number is the building/house number (nro).
	Number string

	// Complement is additional address information (xCpl).
	Complement string

	// Neighborhood is the district or neighborhood (xBairro).
	Neighborhood string

	// MunicipalityCode is the 7-digit IBGE municipality code (cMun).
	// Required for national addresses.
	MunicipalityCode string

	// State is the 2-letter state abbreviation (UF).
	// Required for national addresses.
	State string

	// PostalCode is the 8-digit CEP without formatting.
	// Required for national addresses.
	PostalCode string

	// CountryCode is the ISO 3166-1 alpha-2 country code (cPais).
	// Defaults to "BR" for national addresses.
	CountryCode string
}

// IsForeign returns true if this address configuration represents a foreign address.
func (c *AddressConfig) IsForeign() bool {
	return c.CountryCode != "" && c.CountryCode != "BR"
}

// BuildAddressXML generates the <end> element for an address.
// It automatically determines whether to use national or foreign format
// based on the country code.
func BuildAddressXML(config *AddressConfig) (*etree.Element, error) {
	if config == nil {
		return nil, fmt.Errorf("address config cannot be nil")
	}

	if config.IsForeign() {
		return BuildForeignAddressXML(config)
	}
	return BuildNationalAddressXML(config)
}

// BuildNationalAddressXML generates the <end> element for a Brazilian address.
// National addresses require: street, number, neighborhood, municipality code,
// state (UF), and postal code (CEP).
//
// XML structure:
//
//	<end>
//	  <xLgr>Rua Example</xLgr>
//	  <nro>123</nro>
//	  <xCpl>Sala 101</xCpl>
//	  <xBairro>Centro</xBairro>
//	  <cMun>3550308</cMun>
//	  <UF>SP</UF>
//	  <CEP>01310100</CEP>
//	  <cPais>BR</cPais>
//	</end>
func BuildNationalAddressXML(config *AddressConfig) (*etree.Element, error) {
	if config == nil {
		return nil, fmt.Errorf("address config cannot be nil")
	}

	// Validate required fields for national address
	if err := validateNationalAddressConfig(config); err != nil {
		return nil, err
	}

	end := etree.NewElement("end")

	// xLgr - Street (required)
	end.CreateElement("xLgr").SetText(sanitizeXMLText(config.Street))

	// nro - Number (required)
	end.CreateElement("nro").SetText(sanitizeXMLText(config.Number))

	// xCpl - Complement (optional)
	if config.Complement != "" {
		end.CreateElement("xCpl").SetText(sanitizeXMLText(config.Complement))
	}

	// xBairro - Neighborhood (required)
	end.CreateElement("xBairro").SetText(sanitizeXMLText(config.Neighborhood))

	// cMun - Municipality IBGE code (required for national)
	end.CreateElement("cMun").SetText(config.MunicipalityCode)

	// UF - State (required for national)
	end.CreateElement("UF").SetText(strings.ToUpper(config.State))

	// CEP - Postal code (required for national)
	end.CreateElement("CEP").SetText(cleanPostalCode(config.PostalCode))

	// cPais - Country code (defaults to BR)
	countryCode := config.CountryCode
	if countryCode == "" {
		countryCode = "BR"
	}
	end.CreateElement("cPais").SetText(strings.ToUpper(countryCode))

	return end, nil
}

// BuildForeignAddressXML generates the <end> element for a foreign (non-Brazilian) address.
// Foreign addresses have minimal required fields: street, number, neighborhood, and country code.
// Municipality, state, and CEP are omitted.
//
// XML structure:
//
//	<end>
//	  <xLgr>Foreign Street</xLgr>
//	  <nro>456</nro>
//	  <xBairro>Foreign District</xBairro>
//	  <cPais>ES</cPais>
//	</end>
func BuildForeignAddressXML(config *AddressConfig) (*etree.Element, error) {
	if config == nil {
		return nil, fmt.Errorf("address config cannot be nil")
	}

	// Validate required fields for foreign address
	if err := validateForeignAddressConfig(config); err != nil {
		return nil, err
	}

	end := etree.NewElement("end")

	// xLgr - Street (required)
	end.CreateElement("xLgr").SetText(sanitizeXMLText(config.Street))

	// nro - Number (required)
	end.CreateElement("nro").SetText(sanitizeXMLText(config.Number))

	// xCpl - Complement (optional)
	if config.Complement != "" {
		end.CreateElement("xCpl").SetText(sanitizeXMLText(config.Complement))
	}

	// xBairro - Neighborhood (required)
	end.CreateElement("xBairro").SetText(sanitizeXMLText(config.Neighborhood))

	// cPais - Country code (required, must NOT be BR)
	end.CreateElement("cPais").SetText(strings.ToUpper(config.CountryCode))

	return end, nil
}

// AddressFromDomain converts a domain Address to AddressConfig.
func AddressFromDomain(addr *domain.Address) *AddressConfig {
	if addr == nil {
		return nil
	}
	return &AddressConfig{
		Street:           addr.Street,
		Number:           addr.Number,
		Complement:       addr.Complement,
		Neighborhood:     addr.Neighborhood,
		MunicipalityCode: addr.MunicipalityCode,
		State:            addr.State,
		PostalCode:       addr.PostalCode,
		CountryCode:      addr.CountryCode,
	}
}

// validateNationalAddressConfig validates required fields for a national address.
func validateNationalAddressConfig(config *AddressConfig) error {
	if config.Street == "" {
		return fmt.Errorf("street (xLgr) is required for address")
	}
	if config.Number == "" {
		return fmt.Errorf("number (nro) is required for address")
	}
	if config.Neighborhood == "" {
		return fmt.Errorf("neighborhood (xBairro) is required for address")
	}
	if config.MunicipalityCode == "" {
		return fmt.Errorf("municipality code (cMun) is required for national address")
	}
	if config.State == "" {
		return fmt.Errorf("state (UF) is required for national address")
	}
	if config.PostalCode == "" {
		return fmt.Errorf("postal code (CEP) is required for national address")
	}
	return nil
}

// validateForeignAddressConfig validates required fields for a foreign address.
func validateForeignAddressConfig(config *AddressConfig) error {
	if config.Street == "" {
		return fmt.Errorf("street (xLgr) is required for address")
	}
	if config.Number == "" {
		return fmt.Errorf("number (nro) is required for address")
	}
	if config.Neighborhood == "" {
		return fmt.Errorf("neighborhood (xBairro) is required for address")
	}
	if config.CountryCode == "" {
		return fmt.Errorf("country code (cPais) is required for foreign address")
	}
	if config.CountryCode == "BR" {
		return fmt.Errorf("country code cannot be 'BR' for foreign address")
	}
	return nil
}

// sanitizeXMLText removes or escapes characters that could cause XML issues.
func sanitizeXMLText(text string) string {
	// Remove leading/trailing whitespace
	text = strings.TrimSpace(text)

	// Replace multiple spaces with single space
	for strings.Contains(text, "  ") {
		text = strings.ReplaceAll(text, "  ", " ")
	}

	return text
}

// Note: cleanPostalCode is defined in dps.go and shared across the package.
