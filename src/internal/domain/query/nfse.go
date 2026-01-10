// Package query provides domain logic for NFS-e query operations.
package query

import (
	"encoding/xml"
	"fmt"
	"strings"
	"time"
)

// NFSeData represents parsed data from an NFS-e XML document.
// This structure contains the essential information extracted from
// the signed NFS-e XML returned by the government system.
type NFSeData struct {
	// ChaveAcesso is the 50-character NFS-e access key.
	ChaveAcesso string

	// Numero is the official NFS-e number assigned by the government.
	Numero string

	// DataEmissao is the emission date/time.
	DataEmissao time.Time

	// Status indicates the NFS-e status (e.g., "100" for normal/active).
	Status string

	// Prestador contains the service provider information.
	Prestador PrestadorData

	// Tomador contains the service taker information.
	// May be nil for anonymous consumer services.
	Tomador *TomadorData

	// Servico contains the service details.
	Servico ServicoData

	// Valores contains the monetary values.
	Valores ValoresData
}

// PrestadorData contains service provider information parsed from NFS-e XML.
type PrestadorData struct {
	// Documento is the provider's tax identification (CNPJ).
	Documento string

	// Nome is the provider's legal name (razao social).
	Nome string

	// Municipio is the provider's municipality name.
	Municipio string

	// MunicipioCodigo is the 7-digit IBGE municipality code.
	MunicipioCodigo string
}

// TomadorData contains service taker information parsed from NFS-e XML.
type TomadorData struct {
	// Documento is the taker's identification (CNPJ, CPF, or NIF).
	Documento string

	// TipoDocumento indicates the document type: "cnpj", "cpf", or "nif".
	TipoDocumento string

	// Nome is the taker's name.
	Nome string
}

// ServicoData contains service information parsed from NFS-e XML.
type ServicoData struct {
	// CodigoNacional is the 6-digit national service code (cTribNac).
	CodigoNacional string

	// Descricao is the service description.
	Descricao string

	// LocalPrestacao is the location where the service was provided.
	LocalPrestacao string

	// MunicipioCodigo is the 7-digit IBGE code of the service location.
	MunicipioCodigo string
}

// ValoresData contains monetary values parsed from NFS-e XML.
type ValoresData struct {
	// ValorServico is the gross service value.
	ValorServico float64

	// BaseCalculo is the ISS tax calculation base.
	BaseCalculo float64

	// Aliquota is the ISS tax rate as a percentage (e.g., 5.00 for 5%).
	Aliquota float64

	// ValorISSQN is the calculated ISS tax amount.
	ValorISSQN float64

	// ValorLiquido is the net value after taxes and deductions.
	ValorLiquido float64
}

// ================================================================================
// XML Parsing Structures
// ================================================================================

// nfseXML represents the root NFS-e XML element.
type nfseXML struct {
	XMLName xml.Name   `xml:"NFSe"`
	InfNFSe infNFSeXML `xml:"infNFSe"`
}

// infNFSeXML represents the infNFSe element containing NFS-e data.
type infNFSeXML struct {
	// ID is the XML element ID attribute containing the access key.
	ID string `xml:"Id,attr"`

	// Numero is the NFS-e number.
	NNFSe string `xml:"nNFSe"`

	// DataHoraEmissao is the emission timestamp.
	DhEmi string `xml:"dhEmi"`

	// ChaveAcesso is the NFS-e access key.
	ChNFSe string `xml:"chNFSe"`

	// Situacao is the NFS-e status code.
	Sit string `xml:"sit"`

	// Emit contains the provider (emitente) information.
	Emit emitXML `xml:"emit"`

	// Toma contains the taker (tomador) information.
	Toma *tomaXML `xml:"toma,omitempty"`

	// Serv contains the service information.
	Serv servXML `xml:"serv"`

	// Valores contains the monetary values.
	Valores valoresXML `xml:"valores"`
}

// emitXML represents the provider (emitente) element.
type emitXML struct {
	// CNPJ is the provider's CNPJ.
	CNPJ string `xml:"CNPJ"`

	// Nome is the provider's name.
	XNome string `xml:"xNome"`

	// Endereco contains the provider's address.
	Ender enderXML `xml:"ender"`
}

// tomaXML represents the taker (tomador) element.
type tomaXML struct {
	// CNPJ is the taker's CNPJ (optional, mutually exclusive with CPF and NIF).
	CNPJ string `xml:"CNPJ,omitempty"`

	// CPF is the taker's CPF (optional, mutually exclusive with CNPJ and NIF).
	CPF string `xml:"CPF,omitempty"`

	// NIF is the taker's foreign identification (optional).
	NIF string `xml:"NIF,omitempty"`

	// Nome is the taker's name.
	XNome string `xml:"xNome"`
}

// enderXML represents an address element.
type enderXML struct {
	// CodigoMunicipio is the 7-digit IBGE municipality code.
	CMun string `xml:"cMun"`

	// NomeMunicipio is the municipality name.
	XMun string `xml:"xMun"`

	// UF is the state abbreviation.
	UF string `xml:"UF"`
}

// servXML represents the service element.
type servXML struct {
	// CodigoTributacaoNacional is the 6-digit national service code.
	CTribNac string `xml:"cTribNac"`

	// Descricao is the service description.
	XDescServ string `xml:"xDescServ"`

	// LocalPrestacao contains the service location.
	LocalPrest localPrestXML `xml:"localPrest"`
}

// localPrestXML represents the service location element.
type localPrestXML struct {
	// CodigoMunicipio is the 7-digit IBGE municipality code.
	CMun string `xml:"cMun"`

	// NomeMunicipio is the municipality name.
	XMun string `xml:"xMun"`

	// UF is the state abbreviation.
	UF string `xml:"UF"`
}

// valoresXML represents the values element.
type valoresXML struct {
	// ValorServico is the gross service value.
	VServico float64 `xml:"vServico"`

	// BaseCalculo is the ISS calculation base.
	VBC float64 `xml:"vBC"`

	// Aliquota is the ISS rate.
	PAliq float64 `xml:"pAliq"`

	// ValorISS is the ISS value.
	VISS float64 `xml:"vISS"`

	// ValorLiquido is the net value.
	VLiq float64 `xml:"vLiq"`
}

// ================================================================================
// XML Parsing Functions
// ================================================================================

// ParseNFSeXML parses an NFS-e XML document and extracts structured data.
//
// The function handles the standard NFS-e XML structure as defined by the
// Sistema Nacional NFS-e XSD schema (NFSe_v1.00.xsd).
//
// Example:
//
//	data, err := ParseNFSeXML(xmlContent)
//	if err != nil {
//	    return fmt.Errorf("failed to parse NFS-e XML: %w", err)
//	}
//	fmt.Printf("NFS-e Number: %s\n", data.Numero)
//
// Returns an error if the XML is malformed or missing required elements.
func ParseNFSeXML(xmlContent string) (*NFSeData, error) {
	if xmlContent == "" {
		return nil, fmt.Errorf("XML content cannot be empty")
	}

	// Remove BOM if present
	xmlContent = strings.TrimPrefix(xmlContent, "\xef\xbb\xbf")

	// Parse XML
	var nfse nfseXML
	if err := xml.Unmarshal([]byte(xmlContent), &nfse); err != nil {
		return nil, fmt.Errorf("failed to unmarshal NFS-e XML: %w", err)
	}

	info := nfse.InfNFSe

	// Parse emission date
	dataEmissao, err := parseDateTime(info.DhEmi)
	if err != nil {
		// Try alternative formats
		dataEmissao = time.Time{} // Use zero time if parsing fails
	}

	// Extract access key from ID attribute or chNFSe element
	chaveAcesso := info.ChNFSe
	if chaveAcesso == "" && info.ID != "" {
		// Remove common prefixes from ID
		chaveAcesso = strings.TrimPrefix(info.ID, "NFSe")
		if len(chaveAcesso) < 50 && len(info.ID) >= 50 {
			chaveAcesso = info.ID
		}
	}

	// Build result
	result := &NFSeData{
		ChaveAcesso: chaveAcesso,
		Numero:      info.NNFSe,
		DataEmissao: dataEmissao,
		Status:      mapStatus(info.Sit),
		Prestador: PrestadorData{
			Documento:       info.Emit.CNPJ,
			Nome:            info.Emit.XNome,
			Municipio:       info.Emit.Ender.XMun,
			MunicipioCodigo: info.Emit.Ender.CMun,
		},
		Servico: ServicoData{
			CodigoNacional:  info.Serv.CTribNac,
			Descricao:       info.Serv.XDescServ,
			LocalPrestacao:  formatLocalPrestacao(info.Serv.LocalPrest),
			MunicipioCodigo: info.Serv.LocalPrest.CMun,
		},
		Valores: ValoresData{
			ValorServico: info.Valores.VServico,
			BaseCalculo:  info.Valores.VBC,
			Aliquota:     info.Valores.PAliq,
			ValorISSQN:   info.Valores.VISS,
			ValorLiquido: info.Valores.VLiq,
		},
	}

	// Add taker if present
	if info.Toma != nil && (info.Toma.CNPJ != "" || info.Toma.CPF != "" || info.Toma.NIF != "") {
		result.Tomador = &TomadorData{
			Nome: info.Toma.XNome,
		}

		// Determine document type
		if info.Toma.CNPJ != "" {
			result.Tomador.Documento = info.Toma.CNPJ
			result.Tomador.TipoDocumento = "cnpj"
		} else if info.Toma.CPF != "" {
			result.Tomador.Documento = info.Toma.CPF
			result.Tomador.TipoDocumento = "cpf"
		} else if info.Toma.NIF != "" {
			result.Tomador.Documento = info.Toma.NIF
			result.Tomador.TipoDocumento = "nif"
		}
	}

	return result, nil
}

// parseDateTime parses a date-time string in various formats.
func parseDateTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, fmt.Errorf("empty date string")
	}

	// Try common formats
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05-07:00",
		"2006-01-02T15:04:05",
		"2006-01-02T15:04:05Z",
		"2006-01-02",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, s); err == nil {
			return t, nil
		}
	}

	return time.Time{}, fmt.Errorf("unable to parse date: %s", s)
}

// mapStatus maps the XML status code to a human-readable status.
func mapStatus(sit string) string {
	switch sit {
	case "1", "100":
		return NFSeStatusActive
	case "2", "101":
		return NFSeStatusCancelled
	case "3", "102":
		return NFSeStatusSubstituted
	default:
		// Return the original value if not recognized
		if sit != "" {
			return sit
		}
		return NFSeStatusActive
	}
}

// formatLocalPrestacao formats the service location for display.
func formatLocalPrestacao(loc localPrestXML) string {
	if loc.XMun != "" && loc.UF != "" {
		return fmt.Sprintf("%s - %s", loc.XMun, loc.UF)
	}
	if loc.XMun != "" {
		return loc.XMun
	}
	return ""
}

// ================================================================================
// Conversion Helpers
// ================================================================================

// ToQueryResponse converts NFSeData to an NFSeQueryResponse DTO.
// The XML parameter is the original XML content to include in the response.
func (n *NFSeData) ToQueryResponse(xml string) *NFSeQueryResponse {
	response := &NFSeQueryResponse{
		ChaveAcesso: n.ChaveAcesso,
		Numero:      n.Numero,
		DataEmissao: formatDateTimeISO(n.DataEmissao),
		Status:      n.Status,
		Prestador: PrestadorInfo{
			Documento: n.Prestador.Documento,
			Nome:      n.Prestador.Nome,
			Municipio: n.Prestador.Municipio,
		},
		Servico: ServicoInfo{
			CodigoNacional: n.Servico.CodigoNacional,
			Descricao:      n.Servico.Descricao,
			LocalPrestacao: n.Servico.LocalPrestacao,
		},
		Valores: ValoresInfo{
			ValorServico: n.Valores.ValorServico,
			BaseCalculo:  n.Valores.BaseCalculo,
			ValorLiquido: n.Valores.ValorLiquido,
		},
		XML: xml,
	}

	// Set optional tax values
	if n.Valores.Aliquota > 0 {
		response.Valores.SetAliquota(n.Valores.Aliquota)
	}
	if n.Valores.ValorISSQN > 0 {
		response.Valores.SetValorISSQN(n.Valores.ValorISSQN)
	}

	// Add taker if present
	if n.Tomador != nil {
		response.Tomador = NewTomadorInfo(n.Tomador.Nome)
		if n.Tomador.Documento != "" {
			response.Tomador.SetDocumento(n.Tomador.Documento)
		}
	}

	return response
}

// formatDateTimeISO formats a time.Time to ISO 8601 string with timezone offset.
func formatDateTimeISO(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	// Use Brazil timezone offset (-03:00) by default
	return t.Format("2006-01-02T15:04:05-03:00")
}
