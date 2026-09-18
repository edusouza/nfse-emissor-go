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

	// EmitterType is tpEmit: who is sending the declaration. It decides
	// whether the provider's name travels with it — see buildProvider.
	EmitterType int

	// MunicipalityCode is the 7-digit IBGE code where the DPS is emitted
	MunicipalityCode string

	// Substitution, when set, marks this DPS as replacing an existing NFS-e.
	// It is omitted from the XML for an ordinary emission: the schema models
	// subst as a structure carrying the replaced key, not as a yes/no flag.
	Substitution *DPSSubstitution

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

	// SpecialTaxRegime is regEspTrib, required by the schema.
	// 0 = none, 1 = cooperative act, 2 = estimate, 3 = municipal micro-company,
	// 4 = notary, 5 = self-employed professional, 6 = professional society.
	SpecialTaxRegime int

	// SimplesApuracao is regApTribSN, which the schema expects when the
	// provider is an ME/EPP opting into Simples Nacional. Defaults to 1.
	SimplesApuracao int
}

// DPSSubstitution identifies the NFS-e being replaced by this DPS.
type DPSSubstitution struct {
	// AccessKey is the 50-character key of the NFS-e being replaced.
	AccessKey string

	// ReasonCode is the substitution justification code (cMotivo).
	ReasonCode string

	// ReasonText is the free-text justification (xMotivo), optional.
	ReasonText string
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

	// AmountReceived is the amount actually received (vReceb), optional.
	AmountReceived float64

	// UnconditionalDiscount is a discount applied regardless of payment
	// conditions (vDescIncond). It reduces the tax base.
	UnconditionalDiscount float64

	// ConditionalDiscount is a discount conditional on payment terms
	// (vDescCond). It does NOT reduce the tax base.
	ConditionalDiscount float64

	// Deductions are legally permitted deductions from the service value (vDR).
	Deductions float64

	// DeductionPercentage is the deduction as a percentage of service value
	// (pDR). Computed from Deductions when left at zero.
	DeductionPercentage float64

	// ISSRate is the ISS tax rate percentage (pAliq). Zero is valid and is the
	// normal case for an MEI, who pays ISS through the DAS.
	ISSRate float64

	// ISSQNTaxation is tribISSQN: 1 = taxable operation, 2 = service export,
	// 3 = non-incidence, 4 = immunity. Defaults to 1.
	ISSQNTaxation int

	// ISSRetention is tpRetISSQN: 1 = not withheld, 2 = withheld by the taker,
	// 3 = withheld by the intermediary. Defaults to 1.
	ISSRetention int

	// TotalTaxPercentSN is pTotTribSN, the Simples Nacional total tax
	// percentage. When zero, the DPS declares indTotTrib = 0 ("not informed")
	// instead, which the schema allows.
	TotalTaxPercentSN float64
}

// Note: the tax base (vBCCalc) and the ISS amount (vISS) are deliberately
// absent. They are not DPS fields — the government computes them and returns
// them on the resulting NFS-e. Sending them was a defect.

// DPSBuildResult contains the result of building a DPS XML.
type DPSBuildResult struct {
	// DPSID is the generated DPS identification string
	DPSID string

	// XML is the complete DPS XML document as a string
	XML string

	// XMLBytes is the raw XML bytes
	XMLBytes []byte
}

// Simples Nacional status, as opSimpNac carries it in the DPS.
const (
	// SimplesNaoOptante is 1: the provider is not in the Simples Nacional.
	SimplesNaoOptante = 1

	// SimplesMEI is 2.
	SimplesMEI = 2

	// SimplesMEEPP is 3.
	SimplesMEEPP = 3
)

// Emitter types, as tpEmit carries them in the DPS.
const (
	// EmitterTypeProvider is 1: the provider of the service sends the
	// declaration. The only case this CLI emits.
	EmitterTypeProvider = 1

	// EmitterTypeTaker is 2: the taker of the service sends it.
	EmitterTypeTaker = 2

	// EmitterTypeIntermediary is 3: the intermediary sends it.
	EmitterTypeIntermediary = 3
)

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
		b.config.EmitterType = EmitterTypeProvider
	}
	// A nil Substitution means an ordinary emission and omits <subst> entirely.

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
		XMLNs:  NFSeNamespace,
		Versao: "1.00",
		InfDPS: infDPSXML{
			ID:       dpsID,
			TpAmb:    b.config.Environment,
			DhEmi:    formatDateTime(b.config.EmissionDateTime),
			VerAplic: b.config.ApplicationVersion,
			Serie:    b.config.Series,
			NDPS:     b.config.Number,
			DCompet:  formatDate(b.config.CompetenceDate),
			TpEmit:   b.config.EmitterType,
			CLocEmi:  b.config.MunicipalityCode,
			Subst:    b.buildSubstitution(),
			Prest:    b.buildProvider(),
			Toma:     b.buildTaker(),
			Serv:     b.buildService(),
			Valores:  b.buildValues(),
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

// simplesStatus returns opSimpNac for the configured tax regime.
//
// Two sections of the DPS depend on it — the provider block and the total-tax
// choice — and they must not disagree, so both read it from here.
func (b *DPSBuilder) simplesStatus() int {
	if b.config.Provider.TaxRegime == "me_epp" {
		return SimplesMEEPP
	}
	return SimplesMEI
}

// buildProvider creates the provider (prestador) XML element.
func (b *DPSBuilder) buildProvider() prestXML {
	opSimpNac := b.simplesStatus()
	regApTribSN := 0
	if opSimpNac == SimplesMEEPP {
		// regApTribSN says which taxes are still assessed under the Simples.
		// It decides whether pAliq may be declared, so the caller's choice
		// matters; 1 (everything under the Simples) is the common case.
		regApTribSN = b.config.Provider.SimplesApuracao
		if regApTribSN == 0 {
			regApTribSN = 1
		}
	}

	prest := prestXML{
		CNPJ: cleanTaxID(b.config.Provider.CNPJ),
		RegTrib: regTribXML{
			OpSimpNac:   opSimpNac,
			RegApTribSN: regApTribSN,
			// regEspTrib is required by the schema; 0 means "no special regime".
			RegEspTrib: b.config.Provider.SpecialTaxRegime,
		},
	}

	if b.config.Provider.MunicipalRegistration != "" {
		prest.IM = b.config.Provider.MunicipalRegistration
	}

	// The name is the government's to fill in when it already knows who is
	// emitting. Rules E0121 and E0122 of the business-rules spreadsheet:
	//
	//	tpEmit = 1 (the provider emits)  → xNome must NOT be informed
	//	tpEmit = 2 or 3                  → xNome MUST be informed
	//
	// Sending it as the provider is a rejection, not a redundancy, so the
	// condition belongs here rather than in a validation: a document that
	// cannot be built wrong needs nothing checking it afterwards.
	if b.config.EmitterType != EmitterTypeProvider {
		prest.XNome = b.config.Provider.Name
	}

	return prest
}

// buildSubstitution creates the <subst> element, or nil for a normal emission.
func (b *DPSBuilder) buildSubstitution() *substXML {
	sub := b.config.Substitution
	if sub == nil {
		return nil
	}
	return &substXML{
		ChSubstda: sub.AccessKey,
		CMotivo:   sub.ReasonCode,
		XMotivo:   sub.ReasonText,
	}
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
		// locPrest is a choice: a municipality code for a domestic service, or
		// a country code for one provided abroad.
		LocPrest: locPrestXML{
			CLocPrestacao: b.config.Service.MunicipalityCode,
		},
		// xDescServ belongs inside cServ, not beside it.
		CServ: cServXML{
			CTribNac:  b.config.Service.NationalCode,
			XDescServ: b.config.Service.Description,
		},
	}
}

// buildValues creates the <valores> element.
//
// The layout follows TCInfoValores in tiposComplexos_v1.00.xsd:
//
//	valores
//	  vServPrest       (vReceb?, vServ)
//	  vDescCondIncond? (vDescIncond?, vDescCond?)
//	  vDedRed?
//	  trib             (tribMun, tribFed?, totTrib)
func (b *DPSBuilder) buildValues() valoresXML {
	valores := valoresXML{
		VServPrest: b.buildServiceValues(),
		Trib:       b.buildTaxSection(),
	}

	// Discounts live in their own element, as siblings of vServPrest.
	if b.config.Values.UnconditionalDiscount > 0 || b.config.Values.ConditionalDiscount > 0 {
		valores.VDescCondIncond = b.buildDiscountSection()
	}

	if b.config.Values.Deductions > 0 {
		valores.VDedRed = b.buildDeductionSection()
	}

	return valores
}

// buildServiceValues creates the vServPrest section.
func (b *DPSBuilder) buildServiceValues() vServPrestXML {
	v := vServPrestXML{
		VServ: formatMoney(b.config.Values.ServiceValue),
	}
	if b.config.Values.AmountReceived > 0 {
		v.VReceb = formatMoney(b.config.Values.AmountReceived)
	}
	return v
}

// buildDiscountSection creates the vDescCondIncond section.
func (b *DPSBuilder) buildDiscountSection() *vDescCondIncondXML {
	d := &vDescCondIncondXML{}
	if b.config.Values.UnconditionalDiscount > 0 {
		d.VDescIncond = formatMoney(b.config.Values.UnconditionalDiscount)
	}
	if b.config.Values.ConditionalDiscount > 0 {
		d.VDescCond = formatMoney(b.config.Values.ConditionalDiscount)
	}
	return d
}

// buildDeductionSection creates the vDedRed section.
func (b *DPSBuilder) buildDeductionSection() *vDedRedXML {
	if b.config.Values.Deductions <= 0 {
		return nil
	}

	percentage := b.config.Values.DeductionPercentage
	if percentage == 0 && b.config.Values.ServiceValue > 0 {
		percentage = (b.config.Values.Deductions / b.config.Values.ServiceValue) * 100
	}

	return &vDedRedXML{
		VDR: formatMoney(b.config.Values.Deductions),
		PDR: formatMoney(percentage),
	}
}

// buildTaxSection creates the <trib> element.
//
// It carries no tax base and no ISS amount: the schema has no place for them
// in a DPS. The government computes vBCCalc and vISS and returns them on the
// NFS-e.
func (b *DPSBuilder) buildTaxSection() tribXML {
	taxation := b.config.Values.ISSQNTaxation
	if taxation == 0 {
		taxation = 1 // taxable operation
	}

	retention := b.config.Values.ISSRetention
	if retention == 0 {
		retention = 1 // not withheld
	}

	tribMun := tribMunXML{
		TribISSQN:  taxation,
		TpRetISSQN: retention,
	}
	if b.config.Values.ISSRate > 0 {
		tribMun.PAliq = formatMoney(b.config.Values.ISSRate)
	}

	return tribXML{
		TribMun: tribMun,
		TotTrib: b.buildTotalTaxSection(),
	}
}

// buildTotalTaxSection creates the <totTrib> element.
//
// The schema models totTrib as a choice of exactly one of vTotTrib, pTotTrib,
// indTotTrib or pTotTribSN, and which one is allowed depends on the provider's
// standing in the Simples Nacional, not on what the caller happens to know:
//
//	E0710 — for a MEI, pTotTribSN may never be informed;
//	E0712 — for a ME/EPP, indTotTrib may never be informed.
//
// Choosing by the configured value instead of by the regime was wrong in both
// directions: a ME/EPP without a rate declared indTotTrib, and a MEI with one
// declared pTotTribSN.
//
// A ME/EPP that does not know its rate declares pTotTribSN = 0, which the
// schema's own pattern for TSDec2V2 admits. It has no indTotTrib to opt out
// with, so zero is the honest way to say nothing is being estimated.
func (b *DPSBuilder) buildTotalTaxSection() totTribXML {
	if b.simplesStatus() == SimplesMEEPP {
		return totTribXML{PTotTribSN: formatMoney(b.config.Values.TotalTaxPercentSN)}
	}

	// A MEI pays through the DAS and estimates nothing here.
	notInformed := 0
	return totTribXML{IndTotTrib: &notInformed}
}

// XML structure types for marshaling

type dpsXML struct {
	XMLName xml.Name  `xml:"DPS"`
	XMLNs   string    `xml:"xmlns,attr"`
	Versao  string    `xml:"versao,attr"`
	InfDPS  infDPSXML `xml:"infDPS"`
}

// infDPSXML mirrors TCInfDPS. Element order is significant: the schema declares
// a sequence, so the fields must stay in this order.
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
	Subst    *substXML  `xml:"subst,omitempty"`
	Prest    prestXML   `xml:"prest"`
	Toma     *tomaXML   `xml:"toma,omitempty"`
	Serv     servXML    `xml:"serv"`
	Valores  valoresXML `xml:"valores"`
}

// substXML mirrors TCSubstituicao.
type substXML struct {
	ChSubstda string `xml:"chSubstda"`
	CMotivo   string `xml:"cMotivo"`
	XMotivo   string `xml:"xMotivo,omitempty"`
}

// prestXML mirrors TCInfoPrestador.
type prestXML struct {
	CNPJ    string     `xml:"CNPJ"`
	IM      string     `xml:"IM,omitempty"`
	XNome   string     `xml:"xNome,omitempty"`
	RegTrib regTribXML `xml:"regTrib"`
}

// regTribXML mirrors TCRegTrib.
//
// RegEspTrib carries no omitempty: 0 is the valid encoding for "no special
// regime", and the element is mandatory.
type regTribXML struct {
	OpSimpNac   int `xml:"opSimpNac"`
	RegApTribSN int `xml:"regApTribSN,omitempty"`
	RegEspTrib  int `xml:"regEspTrib"`
}

// tomaXML mirrors TCInfoPessoa for the taker.
type tomaXML struct {
	CNPJ  string  `xml:"CNPJ,omitempty"`
	CPF   string  `xml:"CPF,omitempty"`
	NIF   string  `xml:"NIF,omitempty"`
	XNome string  `xml:"xNome"`
	End   *endXML `xml:"end,omitempty"`
	Fone  string  `xml:"fone,omitempty"`
	Email string  `xml:"email,omitempty"`
}

// endXML mirrors TCEndereco.
type endXML struct {
	XLgr    string `xml:"xLgr"`
	Nro     string `xml:"nro"`
	XCpl    string `xml:"xCpl,omitempty"`
	XBairro string `xml:"xBairro"`
	CMun    string `xml:"cMun,omitempty"`
	UF      string `xml:"UF,omitempty"`
	CEP     string `xml:"CEP,omitempty"`
	CPais   string `xml:"cPais,omitempty"`
}

// servXML mirrors TCServ.
type servXML struct {
	LocPrest locPrestXML `xml:"locPrest"`
	CServ    cServXML    `xml:"cServ"`
}

// locPrestXML mirrors TCLocPrest, which is a choice: exactly one of the two.
type locPrestXML struct {
	CLocPrestacao  string `xml:"cLocPrestacao,omitempty"`
	CPaisPrestacao string `xml:"cPaisPrestacao,omitempty"`
}

// cServXML mirrors TCCServ.
type cServXML struct {
	CTribNac  string `xml:"cTribNac"`
	XDescServ string `xml:"xDescServ"`
}

// valoresXML mirrors TCInfoValores.
type valoresXML struct {
	VServPrest      vServPrestXML       `xml:"vServPrest"`
	VDescCondIncond *vDescCondIncondXML `xml:"vDescCondIncond,omitempty"`
	VDedRed         *vDedRedXML         `xml:"vDedRed,omitempty"`
	Trib            tribXML             `xml:"trib"`
}

// vServPrestXML mirrors TCVServPrest.
type vServPrestXML struct {
	VReceb string `xml:"vReceb,omitempty"`
	VServ  string `xml:"vServ"`
}

// vDescCondIncondXML mirrors TCVDescCondIncond. Discounts belong here, as a
// sibling of vServPrest, not nested inside it.
type vDescCondIncondXML struct {
	VDescIncond string `xml:"vDescIncond,omitempty"`
	VDescCond   string `xml:"vDescCond,omitempty"`
}

// vDedRedXML mirrors TCInfoDedRed.
type vDedRedXML struct {
	VDR string `xml:"vDR"`
	PDR string `xml:"pDR"`
}

// tribXML mirrors TCInfoTributacao.
type tribXML struct {
	TribMun tribMunXML `xml:"tribMun"`
	TotTrib totTribXML `xml:"totTrib"`
}

// tribMunXML mirrors TCTribMunicipal.
type tribMunXML struct {
	TribISSQN  int    `xml:"tribISSQN"`
	TpRetISSQN int    `xml:"tpRetISSQN"`
	PAliq      string `xml:"pAliq,omitempty"`
}

// totTribXML mirrors TCTribTotal, a choice of exactly one child. IndTotTrib is
// a pointer so that the valid value 0 is still emitted.
type totTribXML struct {
	IndTotTrib *int   `xml:"indTotTrib,omitempty"`
	PTotTribSN string `xml:"pTotTribSN,omitempty"`
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
