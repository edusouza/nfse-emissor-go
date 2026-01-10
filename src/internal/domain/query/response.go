// Package query provides DTOs and validation logic for NFS-e query operations.
package query

import (
	"time"
)

// ================================================================================
// NFS-e Query Response (GET /v1/nfse/{chaveAcesso})
// ================================================================================

// NFSeQueryResponse represents the response for querying an NFS-e by access key.
// This response contains the full NFS-e document details including provider,
// taker, service, and value information.
//
// All required fields per OpenAPI spec:
//   - chave_acesso: 50-character access key
//   - numero: NFS-e number assigned by government
//   - data_emissao: ISO 8601 datetime
//   - status: NFS-e status code
//   - prestador: service provider information
//   - servico: service details
//   - valores: monetary values
//   - xml: complete signed XML
type NFSeQueryResponse struct {
	// ChaveAcesso is the 50-character NFS-e access key.
	ChaveAcesso string `json:"chave_acesso"`

	// Numero is the official NFS-e number assigned by the government.
	Numero string `json:"numero"`

	// DataEmissao is the emission date in ISO 8601 format (YYYY-MM-DDTHH:MM:SS-03:00).
	DataEmissao string `json:"data_emissao"`

	// Status indicates the current NFS-e status code (e.g., "100" for normal).
	Status string `json:"status"`

	// Prestador contains the service provider information.
	Prestador PrestadorInfo `json:"prestador"`

	// Tomador contains the service taker (customer) information.
	// May be nil for anonymous consumer services.
	Tomador *TomadorInfo `json:"tomador,omitempty"`

	// Servico contains the service details.
	Servico ServicoInfo `json:"servico"`

	// Valores contains the monetary values and tax calculations.
	Valores ValoresInfo `json:"valores"`

	// XML contains the complete signed NFS-e XML document.
	XML string `json:"xml"`
}

// NFSeStatus constants define the possible statuses of an NFS-e.
const (
	// NFSeStatusActive indicates the NFS-e is valid and active.
	NFSeStatusActive = "active"

	// NFSeStatusCancelled indicates the NFS-e was cancelled.
	NFSeStatusCancelled = "cancelled"

	// NFSeStatusSubstituted indicates the NFS-e was replaced by another.
	NFSeStatusSubstituted = "substituted"
)

// ================================================================================
// Participant Information (Prestador / Tomador)
// ================================================================================

// PrestadorInfo contains information about the service provider (prestador).
// Required fields per OpenAPI spec: documento, nome, municipio.
type PrestadorInfo struct {
	// Documento is the provider's tax identification (CNPJ or CPF without formatting).
	Documento string `json:"documento"`

	// Nome is the provider's name (razao social or full name).
	Nome string `json:"nome"`

	// Municipio is the provider's municipality name.
	Municipio string `json:"municipio"`
}

// TomadorInfo contains information about the service taker (tomador/customer).
// All fields are optional when taker identification is not required (B2C anonymous).
type TomadorInfo struct {
	// Documento is the taker's identification number (CNPJ or CPF).
	// Optional for anonymous consumer services.
	Documento *string `json:"documento,omitempty"`

	// Nome is the taker's name (razao social for companies, full name for individuals).
	Nome string `json:"nome"`
}

// TipoIdentificacao constants for taker identification types.
const (
	// TipoIdentificacaoCNPJ indicates the taker is a company (CNPJ).
	TipoIdentificacaoCNPJ = "cnpj"

	// TipoIdentificacaoCPF indicates the taker is an individual (CPF).
	TipoIdentificacaoCPF = "cpf"

	// TipoIdentificacaoNIF indicates the taker is a foreign entity (NIF).
	TipoIdentificacaoNIF = "nif"
)

// ================================================================================
// Service Information
// ================================================================================

// ServicoInfo contains information about the service provided.
// Required fields per OpenAPI spec: codigo_nacional, descricao, local_prestacao.
type ServicoInfo struct {
	// CodigoNacional is the 6-digit national service code (cTribNac per LC 116/2003).
	CodigoNacional string `json:"codigo_nacional"`

	// Descricao is the detailed service description.
	Descricao string `json:"descricao"`

	// LocalPrestacao is the location where the service was provided.
	LocalPrestacao string `json:"local_prestacao"`
}

// ================================================================================
// Value Information
// ================================================================================

// ValoresInfo contains monetary values and tax calculations.
// Required fields per OpenAPI spec: valor_servico, base_calculo, valor_liquido.
//
// Note: The OpenAPI spec uses number/double for monetary values. While float64
// is used here for JSON serialization compatibility, for precise monetary
// calculations in business logic, convert to decimal.Decimal before performing
// arithmetic operations to avoid floating-point precision issues.
type ValoresInfo struct {
	// ValorServico is the gross service value.
	ValorServico float64 `json:"valor_servico"`

	// BaseCalculo is the ISS tax base (valor_servico - deductions - discounts).
	BaseCalculo float64 `json:"base_calculo"`

	// Aliquota is the ISS tax rate as a percentage (e.g., 5.00 for 5%).
	// Optional when tax is exempt or not applicable.
	Aliquota *float64 `json:"aliquota,omitempty"`

	// ValorISSQN is the calculated ISS tax amount.
	// Optional when tax is exempt or not applicable.
	ValorISSQN *float64 `json:"valor_issqn,omitempty"`

	// ValorLiquido is the net value after taxes and deductions.
	ValorLiquido float64 `json:"valor_liquido"`
}

// ================================================================================
// DPS Lookup Response (GET /v1/dps/{id})
// ================================================================================

// DPSLookupResponse represents the response for looking up an NFS-e by DPS ID.
// This provides a mapping from the DPS identifier to the corresponding NFS-e.
//
// Required fields per OpenAPI spec: dps_id, chave_acesso, nfse_url.
type DPSLookupResponse struct {
	// DPSID is the 42-character DPS identifier that was queried.
	DPSID string `json:"dps_id"`

	// ChaveAcesso is the 50-character access key of the corresponding NFS-e.
	ChaveAcesso string `json:"chave_acesso"`

	// NFSeURL is the URL to retrieve the full NFS-e details.
	NFSeURL string `json:"nfse_url"`
}

// DPSStatus constants define the possible statuses of a DPS lookup.
const (
	// DPSStatusProcessed indicates the DPS was successfully processed into an NFS-e.
	DPSStatusProcessed = "processed"

	// DPSStatusPending indicates the DPS is still being processed.
	DPSStatusPending = "pending"

	// DPSStatusRejected indicates the DPS was rejected by the government.
	DPSStatusRejected = "rejected"
)

// ================================================================================
// Events Query Response (GET /v1/nfse/{chaveAcesso}/eventos)
// ================================================================================

// EventsQueryResponse represents the response for querying NFS-e events.
// Events include cancellations, substitutions, and other lifecycle changes.
//
// Required fields per OpenAPI spec: chave_acesso, total, eventos.
type EventsQueryResponse struct {
	// ChaveAcesso is the 50-character access key of the NFS-e.
	ChaveAcesso string `json:"chave_acesso"`

	// Total is the total number of events for this NFS-e.
	Total int `json:"total"`

	// Eventos is the list of events in chronological order.
	Eventos []EventInfo `json:"eventos"`
}

// EventInfo contains information about a single NFS-e event.
//
// Required fields per OpenAPI spec: tipo, descricao, sequencia, data, xml.
type EventInfo struct {
	// Tipo is the event type code (e.g., "e101101" for cancellation).
	Tipo string `json:"tipo"`

	// Descricao is a human-readable description of the event.
	Descricao string `json:"descricao"`

	// Sequencia is the sequential number of this event for the NFS-e.
	Sequencia int `json:"sequencia"`

	// Data is the event timestamp in ISO 8601 format.
	Data string `json:"data"`

	// XML contains the complete signed event XML document.
	XML string `json:"xml"`
}

// EventType constants define the possible event type codes.
const (
	// EventTypeEmission indicates the NFS-e was emitted.
	EventTypeEmission = "EMISSAO"

	// EventTypeCancellation indicates the NFS-e was cancelled.
	EventTypeCancellation = "CANCELAMENTO"

	// EventTypeCancellationCode is the government event code for cancellation.
	EventTypeCancellationCode = "e101101"

	// EventTypeSubstitution indicates the NFS-e was substituted.
	EventTypeSubstitution = "SUBSTITUICAO"

	// EventTypeCorrection indicates a correction event (carta de correcao).
	EventTypeCorrection = "CORRECAO"

	// EventTypeLockout indicates the NFS-e was locked by the tax authority.
	EventTypeLockout = "BLOQUEIO"
)

// EventTypeDescriptions maps event types to human-readable descriptions.
var EventTypeDescriptions = map[string]string{
	EventTypeEmission:         "NFS-e emitida",
	EventTypeCancellation:     "NFS-e cancelada",
	EventTypeCancellationCode: "Cancelamento de NFS-e",
	EventTypeSubstitution:     "NFS-e substituida",
	EventTypeCorrection:       "Carta de correcao registrada",
	EventTypeLockout:          "NFS-e bloqueada pelo fisco",
}

// ================================================================================
// Helper Functions
// ================================================================================

// NewNFSeQueryResponse creates a new NFSeQueryResponse with required fields.
// The caller must populate Prestador, Servico, Valores, and XML separately.
func NewNFSeQueryResponse(chaveAcesso, numero, dataEmissao, status string) *NFSeQueryResponse {
	return &NFSeQueryResponse{
		ChaveAcesso: chaveAcesso,
		Numero:      numero,
		DataEmissao: dataEmissao,
		Status:      status,
	}
}

// NewPrestadorInfo creates a new PrestadorInfo with all required fields.
func NewPrestadorInfo(documento, nome, municipio string) PrestadorInfo {
	return PrestadorInfo{
		Documento: documento,
		Nome:      nome,
		Municipio: municipio,
	}
}

// NewTomadorInfo creates a new TomadorInfo with a name.
// Use SetDocumento to set the optional documento field.
func NewTomadorInfo(nome string) *TomadorInfo {
	return &TomadorInfo{
		Nome: nome,
	}
}

// SetDocumento sets the documento field on a TomadorInfo.
// Returns the TomadorInfo for method chaining.
func (t *TomadorInfo) SetDocumento(documento string) *TomadorInfo {
	t.Documento = &documento
	return t
}

// NewServicoInfo creates a new ServicoInfo with all required fields.
func NewServicoInfo(codigoNacional, descricao, localPrestacao string) ServicoInfo {
	return ServicoInfo{
		CodigoNacional: codigoNacional,
		Descricao:      descricao,
		LocalPrestacao: localPrestacao,
	}
}

// NewValoresInfo creates a new ValoresInfo with required fields.
// Use SetAliquota and SetValorISSQN to set optional tax fields.
func NewValoresInfo(valorServico, baseCalculo, valorLiquido float64) ValoresInfo {
	return ValoresInfo{
		ValorServico: valorServico,
		BaseCalculo:  baseCalculo,
		ValorLiquido: valorLiquido,
	}
}

// SetAliquota sets the optional aliquota field.
// Returns a pointer to ValoresInfo for method chaining.
func (v *ValoresInfo) SetAliquota(aliquota float64) *ValoresInfo {
	v.Aliquota = &aliquota
	return v
}

// SetValorISSQN sets the optional valor_issqn field.
// Returns a pointer to ValoresInfo for method chaining.
func (v *ValoresInfo) SetValorISSQN(valorISSQN float64) *ValoresInfo {
	v.ValorISSQN = &valorISSQN
	return v
}

// NewDPSLookupResponse creates a new DPSLookupResponse with required fields.
func NewDPSLookupResponse(dpsID, chaveAcesso, nfseURL string) *DPSLookupResponse {
	return &DPSLookupResponse{
		DPSID:       dpsID,
		ChaveAcesso: chaveAcesso,
		NFSeURL:     nfseURL,
	}
}

// NewEventsQueryResponse creates a new EventsQueryResponse.
// The Total field is automatically set based on the number of eventos.
func NewEventsQueryResponse(chaveAcesso string, eventos []EventInfo) *EventsQueryResponse {
	return &EventsQueryResponse{
		ChaveAcesso: chaveAcesso,
		Total:       len(eventos),
		Eventos:     eventos,
	}
}

// NewEventInfo creates a new EventInfo with required fields.
// Descricao is automatically populated from EventTypeDescriptions if the tipo is known.
func NewEventInfo(tipo string, sequencia int, data, xml string) *EventInfo {
	descricao, ok := EventTypeDescriptions[tipo]
	if !ok {
		descricao = tipo
	}
	return &EventInfo{
		Tipo:      tipo,
		Descricao: descricao,
		Sequencia: sequencia,
		Data:      data,
		XML:       xml,
	}
}

// FormatDateTime formats a time.Time to ISO 8601 string format.
func FormatDateTime(t time.Time) string {
	return t.UTC().Format(time.RFC3339)
}

// FormatDate formats a time.Time to date-only string (YYYY-MM-DD).
func FormatDate(t time.Time) string {
	return t.Format("2006-01-02")
}
