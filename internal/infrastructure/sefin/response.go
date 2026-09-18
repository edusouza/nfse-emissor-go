package sefin

import (
	"encoding/json"
	"fmt"
	"time"
)

// envelope holds the fields every Sefin Nacional response carries.
type envelope struct {
	// TipoAmbiente is an integer: 1 for production, 2 for restricted
	// production. It is worth reading back rather than assuming, because it is
	// the government's own statement of whether a document has fiscal value.
	TipoAmbiente          int       `json:"tipoAmbiente"`
	VersaoAplicativo      string    `json:"versaoAplicativo"`
	DataHoraProcessamento time.Time `json:"dataHoraProcessamento"`

	// Erros is the plural form, used by NFSePostResponseErro.
	Erros []Message `json:"erros"`

	// Erro is the singular form, used by ResponseErro on the lookup and event
	// endpoints. The API uses both shapes; a client that reads only one
	// silently loses the reason for half its failures.
	Erro *Message `json:"erro"`

	Alertas []Message `json:"alertas"`
}

// rejections normalises the singular and plural error forms into one slice.
func (e envelope) rejections() []Message {
	if len(e.Erros) > 0 {
		return e.Erros
	}
	if e.Erro != nil {
		return []Message{*e.Erro}
	}
	return nil
}

// EnvironmentName renders tipoAmbiente for a person.
func EnvironmentName(code int) string {
	switch code {
	case EnvCodeProduction:
		return "producao"
	case EnvCodeRestrictedProduction:
		return "producao-restrita"
	default:
		return fmt.Sprintf("desconhecido(%d)", code)
	}
}

// emissionResponse is NFSePostResponseSucesso.
type emissionResponse struct {
	envelope

	ChaveAcesso string `json:"chaveAcesso"`
	IDDPS       string `json:"idDps"`
	NFSeGZipB64 string `json:"nfseXmlGZipB64"`
}

// EmissionResult is a successful emission, as the caller sees it.
type EmissionResult struct {
	// AccessKey is the 50-character chaveAcesso of the issued NFS-e.
	AccessKey string

	// DPSID is the identifier of the declaration that produced it.
	DPSID string

	// NFSeXML is the authorised NFS-e, already decompressed.
	NFSeXML []byte

	// EnvironmentCode is the tipoAmbiente the government reports.
	EnvironmentCode int

	// AppVersion identifies the government application that processed it.
	AppVersion string

	// ProcessedAt is when the government processed the request.
	ProcessedAt time.Time

	// Warnings are non-fatal notices returned alongside a successful emission.
	Warnings []Message
}

// HasFiscalValue reports whether the issued invoice is a real one.
func (r *EmissionResult) HasFiscalValue() bool {
	return r.EnvironmentCode == EnvCodeProduction
}

// parseEmission turns a successful HTTP body into a result, or into the
// rejection it describes.
func parseEmission(body []byte) (*EmissionResult, error) {
	var resp emissionResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("resposta da Sefin nao e um JSON reconhecido: %w", err)
	}

	// A 2xx carrying errors is possible: the request was understood and the
	// document refused.
	if rejections := resp.rejections(); len(rejections) > 0 {
		return nil, &RejectionError{Rejections: rejections, Warnings: resp.Alertas}
	}

	if resp.NFSeGZipB64 == "" {
		return nil, fmt.Errorf("a Sefin respondeu sem erros e sem o campo %q", fieldAuthorizedNFSe)
	}

	nfseXML, err := decodeGzipBase64(resp.NFSeGZipB64)
	if err != nil {
		return nil, fmt.Errorf("nao foi possivel ler a NFS-e retornada: %w", err)
	}

	return &EmissionResult{
		AccessKey:       resp.ChaveAcesso,
		DPSID:           resp.IDDPS,
		NFSeXML:         nfseXML,
		EnvironmentCode: resp.TipoAmbiente,
		AppVersion:      resp.VersaoAplicativo,
		ProcessedAt:     resp.DataHoraProcessamento,
		Warnings:        resp.Alertas,
	}, nil
}

// parseErrorBody extracts the rejection envelope from a non-2xx response,
// falling back to an HTTPError when the body is not one.
func parseErrorBody(status int, body []byte) error {
	var env envelope
	if err := json.Unmarshal(body, &env); err == nil {
		if rejections := env.rejections(); len(rejections) > 0 {
			return &RejectionError{Rejections: rejections, Warnings: env.Alertas}
		}
	}
	return &HTTPError{StatusCode: status, Body: string(body)}
}

// nfseResponse is NFSeGetResponseSucesso.
type nfseResponse struct {
	envelope

	ChaveAcesso string `json:"chaveAcesso"`
	NFSeGZipB64 string `json:"nfseXmlGZipB64"`
}

// NFSeResult is an invoice fetched by its access key.
type NFSeResult struct {
	AccessKey       string
	NFSeXML         []byte
	EnvironmentCode int
	AppVersion      string
	ProcessedAt     time.Time
}

// dpsResponse is DpsGetResponse.
type dpsResponse struct {
	envelope

	IDDPS       string `json:"idDps"`
	ChaveAcesso string `json:"chaveAcesso"`
}

// DPSLookup is the access key of the invoice generated from a declaration.
type DPSLookup struct {
	DPSID           string
	AccessKey       string
	EnvironmentCode int
	AppVersion      string
	ProcessedAt     time.Time
}

func parseNFSe(body []byte) (*NFSeResult, error) {
	var resp nfseResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("resposta da Sefin nao e um JSON reconhecido: %w", err)
	}
	if rejections := resp.rejections(); len(rejections) > 0 {
		return nil, &RejectionError{Rejections: rejections, Warnings: resp.Alertas}
	}
	if resp.NFSeGZipB64 == "" {
		return nil, fmt.Errorf("a Sefin respondeu sem erros e sem o campo %q", fieldAuthorizedNFSe)
	}

	nfseXML, err := decodeGzipBase64(resp.NFSeGZipB64)
	if err != nil {
		return nil, fmt.Errorf("nao foi possivel ler a NFS-e retornada: %w", err)
	}

	return &NFSeResult{
		AccessKey:       resp.ChaveAcesso,
		NFSeXML:         nfseXML,
		EnvironmentCode: resp.TipoAmbiente,
		AppVersion:      resp.VersaoAplicativo,
		ProcessedAt:     resp.DataHoraProcessamento,
	}, nil
}

func parseDPSLookup(body []byte) (*DPSLookup, error) {
	var resp dpsResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("resposta da Sefin nao e um JSON reconhecido: %w", err)
	}
	if rejections := resp.rejections(); len(rejections) > 0 {
		return nil, &RejectionError{Rejections: rejections, Warnings: resp.Alertas}
	}

	return &DPSLookup{
		DPSID:           resp.IDDPS,
		AccessKey:       resp.ChaveAcesso,
		EnvironmentCode: resp.TipoAmbiente,
		AppVersion:      resp.VersaoAplicativo,
		ProcessedAt:     resp.DataHoraProcessamento,
	}, nil
}

// asRejection returns the rejection described by a body, or nil when the body
// is not a Sefin error envelope.
func asRejection(body []byte) *RejectionError {
	var env envelope
	if err := json.Unmarshal(body, &env); err != nil {
		return nil
	}
	rejections := env.rejections()
	if len(rejections) == 0 {
		return nil
	}
	return &RejectionError{Rejections: rejections, Warnings: env.Alertas}
}

// looksLikeSefinBody reports whether a body is a response from the national
// system, as opposed to something an intermediary produced.
//
// Every documented response carries versaoAplicativo and dataHoraProcessamento,
// so their presence is a reliable signal that the answer really came from the
// government.
func looksLikeSefinBody(body []byte) bool {
	var env envelope
	if err := json.Unmarshal(body, &env); err != nil {
		return false
	}
	return env.VersaoAplicativo != "" || !env.DataHoraProcessamento.IsZero() || env.TipoAmbiente != 0
}

// eventResponse is EventosPostResponseSucesso.
type eventResponse struct {
	envelope

	EventoGZipB64 string `json:"eventoXmlGZipB64"`
}

func parseEvent(body []byte) (*EventResult, error) {
	var resp eventResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("resposta da Sefin nao e um JSON reconhecido: %w", err)
	}
	if rejections := resp.rejections(); len(rejections) > 0 {
		return nil, &RejectionError{Rejections: rejections, Warnings: resp.Alertas}
	}
	if resp.EventoGZipB64 == "" {
		return nil, fmt.Errorf("a Sefin respondeu sem erros e sem o campo %q", "eventoXmlGZipB64")
	}

	eventXML, err := decodeGzipBase64(resp.EventoGZipB64)
	if err != nil {
		return nil, fmt.Errorf("nao foi possivel ler o evento retornado: %w", err)
	}

	return &EventResult{
		EventXML:        eventXML,
		EnvironmentCode: resp.TipoAmbiente,
		AppVersion:      resp.VersaoAplicativo,
		ProcessedAt:     resp.DataHoraProcessamento,
	}, nil
}
