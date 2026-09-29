// Package xmlbuilder provides utilities for building NFS-e XML documents
// according to Brazilian government specifications.
package xmlbuilder

// AddressConfig contains address information for XML generation.
//
// The DPS builder writes it as TCEndereco; see buildTakerAddress.
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

	// State is the state, province or region. A foreign address carries it
	// in xEstProvReg; a national one does not carry it at all, because the
	// municipality code already says which state it is in.
	State string

	// City is the city of a foreign address (xCidade). A national address
	// is located by MunicipalityCode instead.
	City string

	// PostalCode is the 8-digit CEP of a national address, or the postal
	// code of a foreign one (cEndPost).
	PostalCode string

	// CountryCode is the ISO 3166-1 alpha-2 country code (cPais). Empty or
	// "BR" makes the address national.
	CountryCode string
}

// IsForeign returns true if this address configuration represents a foreign address.
func (c *AddressConfig) IsForeign() bool {
	return c.CountryCode != "" && c.CountryCode != "BR"
}
