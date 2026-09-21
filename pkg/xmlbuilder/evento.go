// Copyright 2026 Eduardo Souza
// SPDX-License-Identifier: FSL-1.1-MIT

package xmlbuilder

import (
	"encoding/xml"
	"fmt"
	"strings"
	"time"
)

// NFSeNamespace is the target namespace of every document in the national
// system's schemas.
const NFSeNamespace = "http://www.sped.fazenda.gov.br/nfse"

// Event codes recognised by the national system, from TSCodigoEventoNFSe.
const (
	// EventCancellation is "Cancelamento de NFS-e" (e101101).
	EventCancellation = "101101"
)

// Cancellation reason codes, from TSCodJustCanc.
const (
	// CancelReasonIssuingError is "Erro na Emissão".
	CancelReasonIssuingError = "1"

	// CancelReasonServiceNotProvided is "Serviço não Prestado".
	CancelReasonServiceNotProvided = "2"

	// CancelReasonOther is "Outros".
	CancelReasonOther = "9"
)

// cancellationDescription is the only value TE101101/xDesc accepts. The schema
// declares it as an enumeration of exactly this string.
const cancellationDescription = "Cancelamento de NFS-e"

// Length limits from TSMotivo.
const (
	minReasonLength = 15
	maxReasonLength = 255
)

// CancellationConfig describes a request to cancel an issued invoice.
type CancellationConfig struct {
	// Environment is tpAmb: 1 = production, 2 = restricted production.
	Environment int

	// ApplicationVersion is verAplic, capped at 20 characters by the schema.
	ApplicationVersion string

	// EventDateTime defaults to now.
	EventDateTime time.Time

	// AuthorCNPJ or AuthorCPF identifies who is requesting the cancellation.
	// Exactly one must be set.
	AuthorCNPJ string
	AuthorCPF  string

	// AccessKey is the 50-digit key of the invoice being cancelled.
	AccessKey string

	// ReasonCode is cMotivo: one of the CancelReason constants.
	ReasonCode string

	// Reason is xMotivo, the free-text justification. The schema requires
	// between 15 and 255 characters — the government wants a real explanation,
	// not "erro".
	Reason string
}

// CancellationResult is a built cancellation request.
type CancellationResult struct {
	// RequestID is the Id attribute of infPedReg.
	RequestID string

	// XML is the complete document, ready to be signed.
	XML string
}

// BuildCancellation generates the pedRegEvento XML for cancelling an invoice.
func BuildCancellation(cfg CancellationConfig) (*CancellationResult, error) {
	if err := cfg.validate(); err != nil {
		return nil, err
	}

	when := cfg.EventDateTime
	if when.IsZero() {
		when = time.Now()
	}

	// TSIdPedRegEvt: "PRE" followed by 56 digits — the access key and the event
	// code. The three-digit sequence number that appears in the resulting
	// event's own identifier (TSIdEvento, "EVT" + 59 digits) is assigned by the
	// government and is not part of the request.
	requestID := "PRE" + cfg.AccessKey + EventCancellation

	doc := &pedRegEventoXML{
		XMLNs:  NFSeNamespace,
		Versao: "1.00",
		InfPedReg: infPedRegXML{
			ID:        requestID,
			TpAmb:     cfg.Environment,
			VerAplic:  cfg.ApplicationVersion,
			DhEvento:  formatDateTime(when),
			CNPJAutor: cleanTaxID(cfg.AuthorCNPJ),
			CPFAutor:  cleanTaxID(cfg.AuthorCPF),
			ChNFSe:    cfg.AccessKey,
			E101101: &e101101XML{
				XDesc:   cancellationDescription,
				CMotivo: cfg.ReasonCode,
				XMotivo: cfg.Reason,
			},
		},
	}

	out, err := xml.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("falha ao montar o XML do pedido de cancelamento: %w", err)
	}

	return &CancellationResult{
		RequestID: requestID,
		XML:       xml.Header + string(out),
	}, nil
}

func (c CancellationConfig) validate() error {
	var problems []string

	if c.Environment != 1 && c.Environment != 2 {
		problems = append(problems, fmt.Sprintf("ambiente %d invalido (use 1 ou 2)", c.Environment))
	}

	if len(c.AccessKey) != 50 {
		problems = append(problems, fmt.Sprintf("chave de acesso deve ter 50 digitos, tem %d", len(c.AccessKey)))
	}

	hasCNPJ, hasCPF := c.AuthorCNPJ != "", c.AuthorCPF != ""
	switch {
	case hasCNPJ && hasCPF:
		problems = append(problems, "informe CNPJ ou CPF do autor, nunca os dois")
	case !hasCNPJ && !hasCPF:
		problems = append(problems, "informe o CNPJ ou o CPF do autor do cancelamento")
	}

	switch c.ReasonCode {
	case CancelReasonIssuingError, CancelReasonServiceNotProvided, CancelReasonOther:
	case "":
		problems = append(problems, "codigo do motivo obrigatorio (1, 2 ou 9)")
	default:
		problems = append(problems, fmt.Sprintf("codigo do motivo %q invalido (use 1, 2 ou 9)", c.ReasonCode))
	}

	// The schema's 15-character floor is a deliberate nudge: the justification
	// goes on the fiscal record, and "erro" explains nothing to a later reader.
	switch n := len([]rune(strings.TrimSpace(c.Reason))); {
	case n == 0:
		problems = append(problems, "descricao do motivo obrigatoria")
	case n < minReasonLength:
		problems = append(problems, fmt.Sprintf(
			"descricao do motivo precisa de ao menos %d caracteres, tem %d", minReasonLength, n))
	case n > maxReasonLength:
		problems = append(problems, fmt.Sprintf(
			"descricao do motivo excede %d caracteres, tem %d", maxReasonLength, n))
	}

	if len(problems) > 0 {
		return fmt.Errorf("pedido de cancelamento invalido:\n  - %s", strings.Join(problems, "\n  - "))
	}
	return nil
}

// pedRegEventoXML mirrors TCPedRegEvt.
type pedRegEventoXML struct {
	XMLName   xml.Name     `xml:"pedRegEvento"`
	XMLNs     string       `xml:"xmlns,attr"`
	Versao    string       `xml:"versao,attr"`
	InfPedReg infPedRegXML `xml:"infPedReg"`
}

// infPedRegXML mirrors TCInfPedReg. Element order follows the schema's
// sequence, and CNPJAutor/CPFAutor are a choice: exactly one is emitted.
type infPedRegXML struct {
	ID        string      `xml:"Id,attr"`
	TpAmb     int         `xml:"tpAmb"`
	VerAplic  string      `xml:"verAplic"`
	DhEvento  string      `xml:"dhEvento"`
	CNPJAutor string      `xml:"CNPJAutor,omitempty"`
	CPFAutor  string      `xml:"CPFAutor,omitempty"`
	ChNFSe    string      `xml:"chNFSe"`
	E101101   *e101101XML `xml:"e101101,omitempty"`
}

// e101101XML mirrors TE101101, the cancellation event.
type e101101XML struct {
	XDesc   string `xml:"xDesc"`
	CMotivo string `xml:"cMotivo"`
	XMotivo string `xml:"xMotivo"`
}
