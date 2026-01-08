// Package xmlbuilder provides utilities for building NFS-e XML documents
// according to Brazilian government specifications.
package xmlbuilder

import (
	"encoding/xml"
	"fmt"
	"strings"
	"time"
)

// DPSConfig contains all parameters needed to build a DPS XML document.
type DPSConfig struct {
	// Environment: 1 = production, 2 = homologation
	Environment int

	// EmissionDateTime is the date/time of emission (defaults to now if zero)
	EmissionDateTime time.Time

	// ApplicationVersion identifies the emitting application
	ApplicationVersion string

	// Series is the 5-digit DPS series
	Series string

	// Number is the DPS number (1-15 digits)
	Number string

	// CompetenceDate is the date of service competence
	CompetenceDate time.Time

	// EmitterType: 1 = service provider, 2 = service taker
	EmitterType int

	// MunicipalityCode is the 7-digit IBGE code where the DPS is emitted
	MunicipalityCode string

	// Substitution: 1 = yes, 2 = no
	Substitution int

	// Provider information
	Provider DPSProvider

	// Taker information (optional)
	Taker *DPSTaker

	// Service information
	Service DPSService

	// Monetary values
	Values DPSValues
}

// DPSProvider contains provider information for the DPS.
type DPSProvider struct {
	CNPJ                  string
	Name                  string
	TaxRegime             string // "mei" or "me_epp"
	MunicipalRegistration string
}

// DPSTaker contains taker information for the DPS.
type DPSTaker struct {
	// Identification (mutually exclusive)
	CNPJ string
	CPF  string
	NIF  string

	// Basic info
	Name  string
	Phone string
	Email string

	// Address (optional, but recommended for B2B)
	Address *AddressConfig
}

// DPSService contains service information for the DPS.
type DPSService struct {
	NationalCode     string // cTribNac - 6 digits
	Description      string
	MunicipalityCode string // IBGE code where service was provided
}

// DPSValues contains monetary values for the DPS.
// These values are used to calculate the tax base according to Brazilian NFS-e rules:
// Tax Base (vBCCalc) = ServiceValue - UnconditionalDiscount - Deductions
// Note: ConditionalDiscount does NOT affect the tax base.
type DPSValues struct {
	// ServiceValue is the gross value of the service (vServ).
	ServiceValue float64

	// UnconditionalDiscount is a discount applied regardless of payment conditions (vDescIncond).
	// Reduces the tax base.
	UnconditionalDiscount float64

	// ConditionalDiscount is a discount conditional on payment terms (vDescCond).
	// Does NOT reduce the tax base.
	ConditionalDiscount float64

	// Deductions are legally permitted deductions from the service value (vDR).
	// Reduces the tax base.
	Deductions float64

	// DeductionPercentage is the deduction as a percentage of service value (pDR).
	// Calculated as (Deductions / ServiceValue) * 100.
	// If set to 0 and Deductions > 0, it will be calculated automatically.
	DeductionPercentage float64

	// TaxBase is the calculated tax base for ISS (vBCCalc).
	// If set to 0, it will be calculated automatically.
	TaxBase float64

	// ISSRate is the ISS tax rate percentage (pAliq).
	// Can be 0 for SIMPLES NACIONAL MEI providers.
	ISSRate float64

	// ISSAmount is the calculated ISS tax amount (vISS).
	// If set to 0 and ISSRate > 0, it will be calculated automatically.
	ISSAmount float64
}

// DPSBuildResult contains the result of building a DPS XML.
type DPSBuildResult struct {
	// DPSID is the generated DPS identification string
	DPSID string

	// XML is the complete DPS XML document as a string
	XML string

	// XMLBytes is the raw XML bytes
	XMLBytes []byte
}

// DPSBuilder builds DPS XML documents according to the Sistema Nacional NFS-e specification.
type DPSBuilder struct {
	config DPSConfig
}

// NewDPSBuilder creates a new DPS builder with the given configuration.
func NewDPSBuilder(config DPSConfig) *DPSBuilder {
	return &DPSBuilder{config: config}
}

// Build generates the complete DPS XML document.
func (b *DPSBuilder) Build() (*DPSBuildResult, error) {
	// Set defaults
	if b.config.EmissionDateTime.IsZero() {
		b.config.EmissionDateTime = time.Now()
	}
	if b.config.CompetenceDate.IsZero() {
		b.config.CompetenceDate = b.config.EmissionDateTime
	}
	if b.config.ApplicationVersion == "" {
		b.config.ApplicationVersion = "1.0.0"
	}
	if b.config.EmitterType == 0 {
		b.config.EmitterType = 1 // Default to provider
	}
	if b.config.Substitution == 0 {
		b.config.Substitution = 2 // Default to no substitution
	}

	// Generate DPS ID
	dpsID, err := GenerateDPSID(DPSIDConfig{
		MunicipalityCode:    b.config.MunicipalityCode,
		RegistrationType:    RegistrationTypeCNPJ,
		FederalRegistration: b.config.Provider.CNPJ,
		Series:              b.config.Series,
		Number:              b.config.Number,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to generate DPS ID: %w", err)
	}

	// Build XML structure
	dps := &dpsXML{
		XMLNs:  "http://www.sped.fazenda.gov.br/nfse",
		Versao: "1.00",
		InfDPS: infDPSXML{
			ID:        dpsID,
			TpAmb:     b.config.Environment,
			DhEmi:     formatDateTime(b.config.EmissionDateTime),
			VerAplic:  b.config.ApplicationVersion,
			Serie:     b.config.Series,
			NDPS:      b.config.Number,
			DCompet:   formatDate(b.config.CompetenceDate),
			TpEmit:    b.config.EmitterType,
			CLocEmi:   b.config.MunicipalityCode,
			Subst:     b.config.Substitution,
			Prest:     b.buildProvider(),
			Toma:      b.buildTaker(),
			Serv:      b.buildService(),
			Valores:   b.buildValues(),
		},
	}

	// Marshal to XML
	xmlBytes, err := xml.MarshalIndent(dps, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("failed to marshal DPS XML: %w", err)
	}

	// Add XML declaration
	xmlStr := xml.Header + string(xmlBytes)

	return &DPSBuildResult{
		DPSID:    dpsID,
		XML:      xmlStr,
		XMLBytes: []byte(xmlStr),
	}, nil
}

// buildProvider creates the provider (prestador) XML element.
func (b *DPSBuilder) buildProvider() prestXML {
	// Convert tax regime to opSimpNac value
	opSimpNac := 2 // MEI
	if b.config.Provider.TaxRegime == "me_epp" {
		opSimpNac = 3 // ME/EPP
	}

	prest := prestXML{
		CNPJ:  cleanTaxID(b.config.Provider.CNPJ),
		XNome: b.config.Provider.Name,
		RegTrib: regTribXML{
			OpSimpNac: opSimpNac,
		},
	}

	if b.config.Provider.MunicipalRegistration != "" {
		prest.IM = b.config.Provider.MunicipalRegistration
	}

	return prest
}

// buildTaker creates the taker (tomador) XML element.
func (b *DPSBuilder) buildTaker() *tomaXML {
	if b.config.Taker == nil {
		return nil
	}

	toma := &tomaXML{
		XNome: b.config.Taker.Name,
	}

	// Set identification (only one should be set)
	if b.config.Taker.CNPJ != "" {
		toma.CNPJ = cleanTaxID(b.config.Taker.CNPJ)
	} else if b.config.Taker.CPF != "" {
		toma.CPF = cleanTaxID(b.config.Taker.CPF)
	} else if b.config.Taker.NIF != "" {
		toma.NIF = b.config.Taker.NIF
	}

	// Set address if provided
	if b.config.Taker.Address != nil {
		toma.End = b.buildTakerAddress(b.config.Taker.Address)
	}

	// Set phone if provided
	if b.config.Taker.Phone != "" {
		toma.Fone = cleanPhoneNumber(b.config.Taker.Phone)
	}

	// Set email if provided
	if b.config.Taker.Email != "" {
		toma.Email = b.config.Taker.Email
	}

	return toma
}

// buildTakerAddress creates the address (end) XML element for the taker.
func (b *DPSBuilder) buildTakerAddress(addr *AddressConfig) *endXML {
	if addr == nil {
		return nil
	}

	end := &endXML{
		XLgr:    addr.Street,
		Nro:     addr.Number,
		XBairro: addr.Neighborhood,
	}

	// Set complement if provided
	if addr.Complement != "" {
		end.XCpl = addr.Complement
	}

	// Set fields based on whether this is a national or foreign address
	if addr.IsForeign() {
		// Foreign address: only country code required
		end.CPais = strings.ToUpper(addr.CountryCode)
	} else {
		// National address: include all fields
		if addr.MunicipalityCode != "" {
			end.CMun = addr.MunicipalityCode
		}
		if addr.State != "" {
			end.UF = strings.ToUpper(addr.State)
		}
		if addr.PostalCode != "" {
			end.CEP = cleanPostalCode(addr.PostalCode)
		}
		// Default to BR for national addresses
		countryCode := addr.CountryCode
		if countryCode == "" {
			countryCode = "BR"
		}
		end.CPais = strings.ToUpper(countryCode)
	}

	return end
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

// cleanPostalCode removes formatting characters from postal codes.
func cleanPostalCode(postal string) string {
	postal = strings.ReplaceAll(postal, "-", "")
	postal = strings.ReplaceAll(postal, ".", "")
	postal = strings.ReplaceAll(postal, " ", "")
	return postal
}

// buildService creates the service (serv) XML element.
func (b *DPSBuilder) buildService() servXML {
	return servXML{
		LocPrest: locPrestXML{
			CLocPrestacao: b.config.Service.MunicipalityCode,
		},
		CServ: cServXML{
			CTribNac: b.config.Service.NationalCode,
		},
		XDescServ: b.config.Service.Description,
	}
}

// buildValues creates the values (valores) XML element with complete discount,
// deduction, and tax calculation sections according to Brazilian NFS-e rules.
func (b *DPSBuilder) buildValues() valoresXML {
	valores := valoresXML{
		VServPrest: b.buildServiceValues(),
		Trib:       b.buildTaxSection(),
		TotTrib:    b.buildTotalTaxSection(),
	}

	// Add deduction section if deductions are present
	if b.config.Values.Deductions > 0 {
		valores.VDedRed = b.buildDeductionSection()
	}

	return valores
}

// buildServiceValues creates the service values (vServPrest) section.
func (b *DPSBuilder) buildServiceValues() vServPrestXML {
	vServPrest := vServPrestXML{
		VServ: formatMoney(b.config.Values.ServiceValue),
	}

	// Add unconditional discount if present
	if b.config.Values.UnconditionalDiscount > 0 {
		vServPrest.VDescIncond = formatMoney(b.config.Values.UnconditionalDiscount)
	}

	// Add conditional discount if present
	if b.config.Values.ConditionalDiscount > 0 {
		vServPrest.VDescCond = formatMoney(b.config.Values.ConditionalDiscount)
	}

	return vServPrest
}

// buildDeductionSection creates the deduction (vDedRed) section.
func (b *DPSBuilder) buildDeductionSection() *vDedRedXML {
	if b.config.Values.Deductions <= 0 {
		return nil
	}

	// Calculate deduction percentage if not provided
	deductionPercentage := b.config.Values.DeductionPercentage
	if deductionPercentage == 0 && b.config.Values.ServiceValue > 0 {
		deductionPercentage = (b.config.Values.Deductions / b.config.Values.ServiceValue) * 100
	}

	return &vDedRedXML{
		VDR: formatMoney(b.config.Values.Deductions),
		PDR: formatMoney(deductionPercentage),
	}
}

// buildTaxSection creates the tax (trib) section with municipal tax (ISSQN) details.
func (b *DPSBuilder) buildTaxSection() tribXML {
	// Calculate tax base if not provided
	taxBase := b.config.Values.TaxBase
	if taxBase == 0 {
		taxBase = b.config.Values.ServiceValue -
			b.config.Values.UnconditionalDiscount -
			b.config.Values.Deductions
	}

	// Ensure tax base is not negative
	if taxBase < 0 {
		taxBase = 0
	}

	// Calculate ISS amount if not provided
	issAmount := b.config.Values.ISSAmount
	if issAmount == 0 && b.config.Values.ISSRate > 0 {
		issAmount = taxBase * b.config.Values.ISSRate / 100
	}

	// Determine tribISSQN value based on tax regime
	// tribISSQN: 1 = Operação tributável
	// For SIMPLES NACIONAL MEI, ISS is typically not charged (use tribISSQN = 1 anyway)
	tribISSQN := 1

	return tribXML{
		TribMun: tribMunXML{
			TribISSQN:   tribISSQN,
			CPaisResult: "BR", // Service result country code
			BM: bmXML{
				VBCCalc: formatMoney(taxBase),
				PAliq:   formatMoney(b.config.Values.ISSRate),
				VISS:    formatMoney(issAmount),
			},
		},
	}
}

// buildTotalTaxSection creates the total tax (totTrib) section.
// For SIMPLES NACIONAL providers (MEI/ME/EPP), total taxes are typically 0.
func (b *DPSBuilder) buildTotalTaxSection() totTribXML {
	return totTribXML{
		IndTotTrib: 0, // 0 = Not informed
		PTotTrib: pTotTribXML{
			PTotTribFed: formatMoney(0),
			PTotTribEst: formatMoney(0),
			PTotTribMun: formatMoney(0),
		},
	}
}

// XML structure types for marshaling

type dpsXML struct {
	XMLName xml.Name  `xml:"DPS"`
	XMLNs   string    `xml:"xmlns,attr"`
	Versao  string    `xml:"versao,attr"`
	InfDPS  infDPSXML `xml:"infDPS"`
}

type infDPSXML struct {
	ID       string     `xml:"Id,attr"`
	TpAmb    int        `xml:"tpAmb"`
	DhEmi    string     `xml:"dhEmi"`
	VerAplic string     `xml:"verAplic"`
	Serie    string     `xml:"serie"`
	NDPS     string     `xml:"nDPS"`
	DCompet  string     `xml:"dCompet"`
	TpEmit   int        `xml:"tpEmit"`
	CLocEmi  string     `xml:"cLocEmi"`
	Subst    int        `xml:"subst"`
	Prest    prestXML   `xml:"prest"`
	Toma     *tomaXML   `xml:"toma,omitempty"`
	Serv     servXML    `xml:"serv"`
	Valores  valoresXML `xml:"valores"`
}

type prestXML struct {
	CNPJ    string     `xml:"CNPJ"`
	IM      string     `xml:"IM,omitempty"`
	XNome   string     `xml:"xNome"`
	RegTrib regTribXML `xml:"regTrib"`
}

type regTribXML struct {
	OpSimpNac int `xml:"opSimpNac"`
}

type tomaXML struct {
	CNPJ  string  `xml:"CNPJ,omitempty"`
	CPF   string  `xml:"CPF,omitempty"`
	NIF   string  `xml:"NIF,omitempty"`
	XNome string  `xml:"xNome"`
	End   *endXML `xml:"end,omitempty"`
	Fone  string  `xml:"fone,omitempty"`
	Email string  `xml:"email,omitempty"`
}

type endXML struct {
	XLgr    string `xml:"xLgr"`
	Nro     string `xml:"nro"`
	XCpl    string `xml:"xCpl,omitempty"`
	XBairro string `xml:"xBairro"`
	CMun    string `xml:"cMun,omitempty"`
	UF      string `xml:"UF,omitempty"`
	CEP     string `xml:"CEP,omitempty"`
	CPais   string `xml:"cPais"`
}

type servXML struct {
	LocPrest  locPrestXML `xml:"locPrest"`
	CServ     cServXML    `xml:"cServ"`
	XDescServ string      `xml:"xDescServ"`
}

type locPrestXML struct {
	CLocPrestacao string `xml:"cLocPrestacao"`
}

type cServXML struct {
	CTribNac string `xml:"cTribNac"`
}

type valoresXML struct {
	VServPrest vServPrestXML `xml:"vServPrest"`
	VDedRed    *vDedRedXML   `xml:"vDedRed,omitempty"`
	Trib       tribXML       `xml:"trib"`
	TotTrib    totTribXML    `xml:"totTrib"`
}

type vServPrestXML struct {
	VServ        string `xml:"vServ"`
	VDescIncond  string `xml:"vDescIncond,omitempty"`
	VDescCond    string `xml:"vDescCond,omitempty"`
}

// vDedRedXML represents the deduction section in the valores element.
type vDedRedXML struct {
	VDR string `xml:"vDR"`
	PDR string `xml:"pDR"`
}

// tribXML represents the tax section in the valores element.
type tribXML struct {
	TribMun tribMunXML `xml:"tribMun"`
}

// tribMunXML represents the municipal tax (ISSQN) details.
type tribMunXML struct {
	TribISSQN   int    `xml:"tribISSQN"`
	CPaisResult string `xml:"cPaisResult"`
	BM          bmXML  `xml:"BM"`
}

// bmXML represents the tax base calculation details.
type bmXML struct {
	VBCCalc string `xml:"vBCCalc"`
	PAliq   string `xml:"pAliq"`
	VISS    string `xml:"vISS"`
}

// totTribXML represents the total tax information section.
type totTribXML struct {
	IndTotTrib int         `xml:"indTotTrib"`
	PTotTrib   pTotTribXML `xml:"pTotTrib"`
}

// pTotTribXML represents the total tax percentages by jurisdiction.
type pTotTribXML struct {
	PTotTribFed string `xml:"pTotTribFed"`
	PTotTribEst string `xml:"pTotTribEst"`
	PTotTribMun string `xml:"pTotTribMun"`
}

// Helper functions

// formatDateTime formats a time.Time to the required ISO 8601 format with timezone.
func formatDateTime(t time.Time) string {
	// Use Brazil timezone offset (-03:00)
	loc, err := time.LoadLocation("America/Sao_Paulo")
	if err != nil {
		// Fallback to fixed offset if timezone not available
		loc = time.FixedZone("BRT", -3*60*60)
	}
	return t.In(loc).Format("2006-01-02T15:04:05-07:00")
}

// formatDate formats a time.Time to YYYY-MM-DD format.
func formatDate(t time.Time) string {
	return t.Format("2006-01-02")
}

// formatMoney formats a float64 as a decimal string with 2 decimal places.
func formatMoney(value float64) string {
	return fmt.Sprintf("%.2f", value)
}

// cleanTaxID removes formatting characters from a tax ID (CNPJ/CPF).
func cleanTaxID(taxID string) string {
	taxID = strings.ReplaceAll(taxID, ".", "")
	taxID = strings.ReplaceAll(taxID, "-", "")
	taxID = strings.ReplaceAll(taxID, "/", "")
	return taxID
}
