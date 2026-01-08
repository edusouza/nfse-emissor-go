// Package emission provides DTOs and business logic for NFS-e emission operations.
package emission

import (
	"encoding/base64"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/beevik/etree"
)

// Pre-signed XML validation error types.
var (
	// ErrPreSignedInvalidXML indicates the XML could not be parsed.
	ErrPreSignedInvalidXML = errors.New("invalid XML document")

	// ErrPreSignedNotDPS indicates the XML is not a DPS document.
	ErrPreSignedNotDPS = errors.New("XML is not a DPS document")

	// ErrPreSignedMissingInfDPS indicates the infDPS element is missing.
	ErrPreSignedMissingInfDPS = errors.New("infDPS element not found")

	// ErrPreSignedMissingID indicates the infDPS Id attribute is missing.
	ErrPreSignedMissingID = errors.New("infDPS Id attribute not found")

	// ErrPreSignedMissingProvider indicates provider information is missing.
	ErrPreSignedMissingProvider = errors.New("provider (prest) information not found")

	// ErrPreSignedMissingProviderID indicates provider CNPJ/CPF is missing.
	ErrPreSignedMissingProviderID = errors.New("provider CNPJ or CPF not found")

	// ErrPreSignedInvalidBase64 indicates invalid base64 encoding.
	ErrPreSignedInvalidBase64 = errors.New("invalid base64 encoding")
)

// PreSignedXMLRequest represents a request with pre-signed XML.
// This is the JSON structure accepted by POST /v1/nfse/xml when Content-Type is application/json.
type PreSignedXMLRequest struct {
	// XML is the base64-encoded signed DPS XML document.
	XML string `json:"xml" binding:"required"`

	// WebhookURL is an optional override for the default webhook URL.
	WebhookURL string `json:"webhook_url,omitempty"`
}

// DecodeXML decodes the base64-encoded XML and returns the raw XML string.
func (r *PreSignedXMLRequest) DecodeXML() (string, error) {
	if r.XML == "" {
		return "", ErrPreSignedInvalidBase64
	}

	decoded, err := base64.StdEncoding.DecodeString(r.XML)
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrPreSignedInvalidBase64, err)
	}

	return string(decoded), nil
}

// PreSignedInfo contains extracted information from a pre-signed DPS XML.
// This information is extracted to store metadata and for audit purposes.
type PreSignedInfo struct {
	// DPSID is the Id attribute value from the infDPS element.
	// This is the unique identifier for the DPS document.
	DPSID string `json:"dps_id"`

	// ProviderCNPJ is the 14-digit CNPJ of the service provider.
	// Either ProviderCNPJ or ProviderCPF will be set, not both.
	ProviderCNPJ string `json:"provider_cnpj,omitempty"`

	// ProviderCPF is the 11-digit CPF of the service provider.
	// Either ProviderCNPJ or ProviderCPF will be set, not both.
	ProviderCPF string `json:"provider_cpf,omitempty"`

	// ProviderName is the name/razao social of the service provider.
	ProviderName string `json:"provider_name,omitempty"`

	// MunicipalityCode is the 7-digit IBGE code of the emission municipality.
	MunicipalityCode string `json:"municipality_code,omitempty"`

	// Series is the 5-digit DPS series.
	Series string `json:"series,omitempty"`

	// Number is the DPS number (1-15 digits).
	Number string `json:"number,omitempty"`

	// ServiceValue is the total service value.
	ServiceValue float64 `json:"service_value,omitempty"`

	// Environment is the environment type: 1 (production) or 2 (homologation).
	Environment int `json:"environment,omitempty"`

	// EmissionDate is the emission date/time from the DPS.
	EmissionDate time.Time `json:"emission_date,omitempty"`

	// NationalServiceCode is the 6-digit national service code (cTribNac).
	NationalServiceCode string `json:"national_service_code,omitempty"`

	// ServiceDescription is the service description from the XML.
	ServiceDescription string `json:"service_description,omitempty"`

	// ServiceMunicipalityCode is the municipality where the service was provided.
	ServiceMunicipalityCode string `json:"service_municipality_code,omitempty"`

	// HasSignature indicates whether the XML contains a Signature element.
	HasSignature bool `json:"has_signature"`
}

// ParsePreSignedXML extracts information from a pre-signed DPS XML document.
//
// This function parses the XML and extracts key information for:
//   - Creating the emission request record
//   - Audit logging
//   - Request validation
//
// Parameters:
//   - xmlContent: The raw DPS XML content (not base64 encoded)
//
// Returns:
//   - *PreSignedInfo: The extracted information
//   - error: Any parsing error encountered
func ParsePreSignedXML(xmlContent string) (*PreSignedInfo, error) {
	if xmlContent == "" {
		return nil, ErrPreSignedInvalidXML
	}

	doc := etree.NewDocument()
	if err := doc.ReadFromString(xmlContent); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrPreSignedInvalidXML, err)
	}

	info := &PreSignedInfo{}

	// Find DPS element
	dps := doc.FindElement("//DPS")
	if dps == nil {
		return nil, ErrPreSignedNotDPS
	}

	// Find infDPS element
	infDPS := dps.FindElement("infDPS")
	if infDPS == nil {
		return nil, ErrPreSignedMissingInfDPS
	}

	// Extract Id attribute
	idAttr := infDPS.SelectAttr("Id")
	if idAttr == nil {
		return nil, ErrPreSignedMissingID
	}
	info.DPSID = idAttr.Value

	// Check for Signature element
	signature := dps.FindElement("Signature")
	info.HasSignature = signature != nil

	// Extract environment (tpAmb)
	if tpAmb := infDPS.FindElement("tpAmb"); tpAmb != nil {
		if val, err := strconv.Atoi(strings.TrimSpace(tpAmb.Text())); err == nil {
			info.Environment = val
		}
	}

	// Extract emission date (dhEmi)
	if dhEmi := infDPS.FindElement("dhEmi"); dhEmi != nil {
		info.EmissionDate = parseDateTime(strings.TrimSpace(dhEmi.Text()))
	}

	// Extract series (serie)
	if serie := infDPS.FindElement("serie"); serie != nil {
		info.Series = strings.TrimSpace(serie.Text())
	}

	// Extract number (nDPS)
	if nDPS := infDPS.FindElement("nDPS"); nDPS != nil {
		info.Number = strings.TrimSpace(nDPS.Text())
	}

	// Extract municipality code (cLocEmi)
	if cLocEmi := infDPS.FindElement("cLocEmi"); cLocEmi != nil {
		info.MunicipalityCode = strings.TrimSpace(cLocEmi.Text())
	}

	// Extract provider information (prest)
	if err := extractProviderInfo(infDPS, info); err != nil {
		return nil, err
	}

	// Extract service information (serv)
	extractServiceInfo(infDPS, info)

	// Extract values (valores)
	extractValuesInfo(infDPS, info)

	return info, nil
}

// extractProviderInfo extracts provider information from the prest element.
func extractProviderInfo(infDPS *etree.Element, info *PreSignedInfo) error {
	prest := infDPS.FindElement("prest")
	if prest == nil {
		return ErrPreSignedMissingProvider
	}

	// Extract CNPJ
	if cnpj := prest.FindElement("CNPJ"); cnpj != nil {
		info.ProviderCNPJ = cleanNumericString(cnpj.Text())
	}

	// Extract CPF (if CNPJ not present)
	if info.ProviderCNPJ == "" {
		if cpf := prest.FindElement("CPF"); cpf != nil {
			info.ProviderCPF = cleanNumericString(cpf.Text())
		}
	}

	// Validate at least one ID is present
	if info.ProviderCNPJ == "" && info.ProviderCPF == "" {
		return ErrPreSignedMissingProviderID
	}

	// Extract provider name (xNome)
	if xNome := prest.FindElement("xNome"); xNome != nil {
		info.ProviderName = strings.TrimSpace(xNome.Text())
	}

	return nil
}

// extractServiceInfo extracts service information from the serv element.
func extractServiceInfo(infDPS *etree.Element, info *PreSignedInfo) {
	serv := infDPS.FindElement("serv")
	if serv == nil {
		return
	}

	// Extract national service code (cTribNac)
	if cTribNac := serv.FindElement("cTribNac"); cTribNac != nil {
		info.NationalServiceCode = cleanNumericString(cTribNac.Text())
	}

	// Extract service description (xDescServ)
	if xDescServ := serv.FindElement("xDescServ"); xDescServ != nil {
		info.ServiceDescription = strings.TrimSpace(xDescServ.Text())
	}

	// Extract service municipality code (cLocPrest)
	if cLocPrest := serv.FindElement("cLocPrest"); cLocPrest != nil {
		info.ServiceMunicipalityCode = cleanNumericString(cLocPrest.Text())
	}
}

// extractValuesInfo extracts value information from the valores element.
func extractValuesInfo(infDPS *etree.Element, info *PreSignedInfo) {
	valores := infDPS.FindElement("valores")
	if valores == nil {
		return
	}

	// Try vServPrest first, then vServ as fallback
	var vServElem *etree.Element
	vServElem = valores.FindElement("vServPrest")
	if vServElem == nil {
		vServElem = valores.FindElement("vServ")
	}

	if vServElem != nil {
		if val, err := parseDecimal(vServElem.Text()); err == nil {
			info.ServiceValue = val
		}
	}
}

// parseDateTime parses an ISO 8601 datetime string.
func parseDateTime(s string) time.Time {
	// Try various formats
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05-07:00",
		"2006-01-02T15:04:05Z",
		"2006-01-02T15:04:05.000-07:00",
		"2006-01-02T15:04:05.000Z",
		"2006-01-02T15:04:05",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return t
		}
	}

	return time.Time{}
}

// parseDecimal parses a decimal number string to float64.
func parseDecimal(s string) (float64, error) {
	s = strings.TrimSpace(s)
	// Replace comma with period for Brazilian decimal format
	s = strings.Replace(s, ",", ".", 1)
	return strconv.ParseFloat(s, 64)
}

// cleanNumericString removes all non-numeric characters from a string.
func cleanNumericString(s string) string {
	re := regexp.MustCompile(`[^0-9]`)
	return re.ReplaceAllString(s, "")
}

// GetEnvironmentString returns the environment string for the given code.
func (p *PreSignedInfo) GetEnvironmentString() string {
	switch p.Environment {
	case 1:
		return "producao"
	case 2:
		return "homologacao"
	default:
		return "homologacao" // Default to homologation if unknown
	}
}

// GetProviderID returns the provider identifier (CNPJ or CPF).
func (p *PreSignedInfo) GetProviderID() string {
	if p.ProviderCNPJ != "" {
		return p.ProviderCNPJ
	}
	return p.ProviderCPF
}

// Validate performs basic validation on the extracted information.
// Returns a list of validation errors.
func (p *PreSignedInfo) Validate() []string {
	var errors []string

	if p.DPSID == "" {
		errors = append(errors, "DPS ID is required")
	}

	if p.ProviderCNPJ == "" && p.ProviderCPF == "" {
		errors = append(errors, "Provider CNPJ or CPF is required")
	}

	if p.ProviderCNPJ != "" && len(p.ProviderCNPJ) != 14 {
		errors = append(errors, "Provider CNPJ must be 14 digits")
	}

	if p.ProviderCPF != "" && len(p.ProviderCPF) != 11 {
		errors = append(errors, "Provider CPF must be 11 digits")
	}

	if p.Environment != 1 && p.Environment != 2 {
		errors = append(errors, "Environment must be 1 (production) or 2 (homologation)")
	}

	if !p.HasSignature {
		errors = append(errors, "XML document is not signed (no Signature element found)")
	}

	return errors
}

// PreSignedXMLResponse is the response structure for pre-signed XML submission.
// It extends EmissionAccepted with additional information.
type PreSignedXMLResponse struct {
	// RequestID is the unique identifier for tracking this emission request.
	RequestID string `json:"request_id"`

	// Status indicates the current status of the request.
	Status string `json:"status"`

	// Message provides additional context about the request.
	Message string `json:"message"`

	// StatusURL is the URL to poll for status updates.
	StatusURL string `json:"status_url"`

	// DPSID is the Id from the submitted DPS XML.
	DPSID string `json:"dps_id,omitempty"`

	// Provider is the provider identifier from the XML.
	Provider string `json:"provider,omitempty"`
}

// Error codes specific to pre-signed XML submission.
const (
	// ErrorCodeSignatureInvalid indicates the XML signature verification failed.
	ErrorCodeSignatureInvalid = "SIGNATURE_INVALID"

	// ErrorCodeXMLNotSigned indicates the XML is not signed.
	ErrorCodeXMLNotSigned = "XML_NOT_SIGNED"

	// ErrorCodeXSDValidationFailed indicates XSD schema validation failed.
	ErrorCodeXSDValidationFailed = "XSD_VALIDATION_FAILED"

	// ErrorCodeInvalidXMLFormat indicates the XML format is invalid.
	ErrorCodeInvalidXMLFormat = "INVALID_XML_FORMAT"
)
