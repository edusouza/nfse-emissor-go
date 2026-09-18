// Package brasilapi looks a CNPJ up in the public registry so that the emitter
// can fill its own configuration.
//
// The data comes from the Receita Federal's open CNPJ dataset, served by
// BrasilAPI (https://brasilapi.com.br). It is public information and no
// authentication is involved, but the query does send the CNPJ to a third
// party — which is why every caller of this package announces the lookup and
// can skip it.
//
// This is a convenience path, never a requirement: everything it returns can
// be typed by hand, so a failure here degrades into "fill it yourself" rather
// than into an error.
package brasilapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/edusouza/nfse-emissor-go/pkg/cnpjcpf"
)

// DefaultBaseURL is the public BrasilAPI endpoint.
const DefaultBaseURL = "https://brasilapi.com.br/api"

// DefaultTimeout bounds the lookup. It is short on purpose: the command works
// without the answer, so waiting is worse than giving up and letting the user
// type the three fields.
const DefaultTimeout = 15 * time.Second

// maxResponseSize caps how much of a response body is read. The registry
// record is a few kilobytes.
const maxResponseSize = 1 << 20

// Config configures a Client.
type Config struct {
	// BaseURL overrides the public endpoint. Useful for a self-hosted
	// minhareceita instance, and for tests.
	BaseURL string

	// Timeout bounds the request. Zero means DefaultTimeout.
	Timeout time.Duration

	// HTTPClient overrides the constructed client. Used by tests.
	HTTPClient *http.Client

	// UserAgent identifies the caller to the public service.
	UserAgent string
}

// Client queries the public CNPJ registry.
type Client struct {
	httpClient *http.Client
	baseURL    string
	userAgent  string
}

// New builds a client.
func New(cfg Config) *Client {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		timeout := cfg.Timeout
		if timeout == 0 {
			timeout = DefaultTimeout
		}
		httpClient = &http.Client{Timeout: timeout}
	}

	return &Client{
		httpClient: httpClient,
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		userAgent:  cfg.UserAgent,
	}
}

// Host returns the host the client talks to, so the caller can say where the
// CNPJ is about to be sent.
func (c *Client) Host() string {
	rest := c.baseURL
	if i := strings.Index(rest, "://"); i >= 0 {
		rest = rest[i+3:]
	}
	if i := strings.Index(rest, "/"); i >= 0 {
		rest = rest[:i]
	}
	return rest
}

// Empresa is the subset of the registry record the emitter uses.
//
// Only the fields that feed nfse.yaml — plus the ones needed to explain a
// surprising answer — are decoded. The record has around forty.
type Empresa struct {
	CNPJ                       string  `json:"cnpj"`
	RazaoSocial                string  `json:"razao_social"`
	NomeFantasia               string  `json:"nome_fantasia"`
	Municipio                  string  `json:"municipio"`
	UF                         string  `json:"uf"`
	DescricaoSituacaoCadastral string  `json:"descricao_situacao_cadastral"`
	CNAEFiscalDescricao        string  `json:"cnae_fiscal_descricao"`
	CodigoMunicipioIBGE        digitos `json:"codigo_municipio_ibge"`
	CNAEFiscal                 digitos `json:"cnae_fiscal"`

	// OpcaoPeloMEI and OpcaoPeloSimples are pointers because the registry
	// answers "no" and "we do not know" differently: false is an answer, null
	// is the absence of one, and guessing the tax regime from a null would put
	// a wrong regApTribSN in every invoice.
	OpcaoPeloMEI     *bool `json:"opcao_pelo_mei"`
	OpcaoPeloSimples *bool `json:"opcao_pelo_simples"`
}

// Ativa reports whether the registration is active. An inactive CNPJ cannot
// issue invoices, and the rejection would only show up at the Sefin.
func (e *Empresa) Ativa() bool {
	return strings.EqualFold(strings.TrimSpace(e.DescricaoSituacaoCadastral), "ATIVA")
}

// digitos is a numeric field that may arrive as a JSON number or as a string.
// BrasilAPI sends numbers, a self-hosted minhareceita may send either, and a
// code with a leading zero only survives as a string. Accepting both costs
// less than being wrong about it.
type digitos string

// UnmarshalJSON accepts a number, a string or null.
func (d *digitos) UnmarshalJSON(data []byte) error {
	text := strings.TrimSpace(string(data))
	if text == "null" || text == "" {
		*d = ""
		return nil
	}
	if strings.HasPrefix(text, `"`) {
		var s string
		if err := json.Unmarshal(data, &s); err != nil {
			return err
		}
		*d = digitos(strings.TrimSpace(s))
		return nil
	}
	*d = digitos(text)
	return nil
}

// String returns the value as text.
func (d digitos) String() string { return string(d) }

// ConsultarCNPJ fetches the registry record for a CNPJ.
func (c *Client) ConsultarCNPJ(ctx context.Context, cnpj string) (*Empresa, error) {
	clean := cnpjcpf.CleanCNPJ(cnpj)
	if !cnpjcpf.ValidateCNPJ(clean) {
		return nil, fmt.Errorf("CNPJ %q invalido: confira os digitos antes de consultar", cnpj)
	}

	url := fmt.Sprintf("%s/cnpj/v1/%s", c.baseURL, clean)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("nao foi possivel montar a consulta: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("nao foi possivel consultar %s: %w", c.Host(), err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
	if err != nil {
		return nil, fmt.Errorf("falha ao ler a resposta de %s: %w", c.Host(), err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, statusError(c.Host(), clean, resp.StatusCode, body)
	}

	var empresa Empresa
	if err := json.Unmarshal(body, &empresa); err != nil {
		return nil, fmt.Errorf("resposta de %s nao e o JSON esperado: %w", c.Host(), err)
	}
	if empresa.RazaoSocial == "" && empresa.CodigoMunicipioIBGE == "" {
		return nil, fmt.Errorf("resposta de %s nao trouxe razao social nem municipio", c.Host())
	}
	return &empresa, nil
}

// statusError turns an HTTP status into something the user can act on.
func statusError(host, cnpj string, status int, body []byte) error {
	switch status {
	case http.StatusNotFound:
		return fmt.Errorf("CNPJ %s nao encontrado na base da Receita Federal consultada em %s", cnpj, host)
	case http.StatusTooManyRequests:
		return fmt.Errorf("%s limitou as consultas (429); tente novamente em alguns minutos", host)
	}
	if status >= 500 {
		return fmt.Errorf("%s respondeu %d; o servico esta indisponivel no momento", host, status)
	}
	return fmt.Errorf("%s respondeu %d: %s", host, status, resumo(body))
}

// resumo shortens a response body for an error message.
func resumo(body []byte) string {
	const limit = 200

	text := strings.TrimSpace(string(body))
	text = strings.Join(strings.Fields(text), " ")
	if text == "" {
		return "(resposta vazia)"
	}
	if len(text) > limit {
		return text[:limit] + "..."
	}
	return text
}
