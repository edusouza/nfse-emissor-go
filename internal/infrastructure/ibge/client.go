// Package ibge turns the seven-digit municipality code into a name and a state.
//
// NT 008 asks the DANFSe to print the name of the municipality ("Utilizar a
// descrição destes códigos"), and the NFS-e XML carries only the code for
// everyone but the issuer. The table that translates it has 5570 rows; rather
// than carry them inside the binary, the emitter asks the IBGE's public
// service for the ones a document actually needs, and remembers the answers.
//
// The lookup is never a requirement. Whatever fails — no network, a blocked
// proxy, a service that is down — the document is still generated, with the
// code where the name would be. A DANFSe that does not print is worse than one
// that prints "3550308".
package ibge

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// DefaultBaseURL is the IBGE's public localities service.
const DefaultBaseURL = "https://servicodados.ibge.gov.br/api/v1/localidades"

// DefaultTimeout bounds one lookup. It is short because the document does not
// depend on the answer: waiting longer only delays a page that would print
// anyway.
const DefaultTimeout = 10 * time.Second

// maxResponseSize caps how much of a response is read. One municipality is a
// couple of kilobytes.
const maxResponseSize = 1 << 20

// CodigoLength is the length of an IBGE municipality code.
const CodigoLength = 7

// Municipio is what the DANFSe needs out of the answer.
type Municipio struct {
	Codigo string `json:"codigo"`
	Nome   string `json:"nome"`
	UF     string `json:"uf"`
}

// Config configures a Client.
type Config struct {
	// BaseURL overrides the public service.
	BaseURL string

	// Timeout bounds each request. Zero means DefaultTimeout.
	Timeout time.Duration

	// HTTPClient overrides the constructed client. Used by tests.
	HTTPClient *http.Client

	// UserAgent identifies the caller to the public service.
	UserAgent string
}

// Client queries the IBGE localities service.
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
// codes are about to be sent.
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

// resposta mirrors the shape the service answers with.
//
// The state abbreviation hangs off two different branches of the response, and
// which one is filled has changed between the service's own revisions. Both
// are decoded and the first one with an answer wins, because a municipality
// without its UF is half a field on a fiscal document.
type resposta struct {
	ID   json.Number `json:"id"`
	Nome string      `json:"nome"`

	Microrregiao *struct {
		Mesorregiao *struct {
			UF *struct {
				Sigla string `json:"sigla"`
			} `json:"UF"`
		} `json:"mesorregiao"`
	} `json:"microrregiao"`

	RegiaoImediata *struct {
		RegiaoIntermediaria *struct {
			UF *struct {
				Sigla string `json:"sigla"`
			} `json:"UF"`
		} `json:"regiao-intermediaria"`
	} `json:"regiao-imediata"`
}

// uf digs the state abbreviation out of whichever branch carries it.
func (r *resposta) uf() string {
	if r.Microrregiao != nil && r.Microrregiao.Mesorregiao != nil && r.Microrregiao.Mesorregiao.UF != nil {
		if sigla := strings.TrimSpace(r.Microrregiao.Mesorregiao.UF.Sigla); sigla != "" {
			return sigla
		}
	}
	if r.RegiaoImediata != nil && r.RegiaoImediata.RegiaoIntermediaria != nil && r.RegiaoImediata.RegiaoIntermediaria.UF != nil {
		return strings.TrimSpace(r.RegiaoImediata.RegiaoIntermediaria.UF.Sigla)
	}
	return ""
}

// Consultar fetches one municipality by its IBGE code.
func (c *Client) Consultar(ctx context.Context, codigo string) (*Municipio, error) {
	codigo = strings.TrimSpace(codigo)
	if len(codigo) != CodigoLength || !somenteDigitos(codigo) {
		return nil, fmt.Errorf("codigo de municipio %q invalido: sao %d digitos", codigo, CodigoLength)
	}

	url := fmt.Sprintf("%s/municipios/%s", c.baseURL, codigo)
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
		return nil, fmt.Errorf("%s respondeu %d para o municipio %s", c.Host(), resp.StatusCode, codigo)
	}

	// The service answers an unknown code with an empty array instead of a
	// 404, so a successful status is not by itself an answer.
	if vazia(body) {
		return nil, fmt.Errorf("%s nao conhece o municipio %s", c.Host(), codigo)
	}

	var dados resposta
	if err := json.Unmarshal(body, &dados); err != nil {
		return nil, fmt.Errorf("resposta de %s nao e o JSON esperado: %w", c.Host(), err)
	}
	if strings.TrimSpace(dados.Nome) == "" {
		return nil, fmt.Errorf("resposta de %s nao trouxe o nome do municipio %s", c.Host(), codigo)
	}

	return &Municipio{Codigo: codigo, Nome: strings.TrimSpace(dados.Nome), UF: dados.uf()}, nil
}

// vazia reports whether the body is an empty JSON array, which is how the
// service says "no such municipality".
func vazia(body []byte) bool {
	trimmed := strings.TrimSpace(string(body))
	return trimmed == "" || trimmed == "[]" || trimmed == "null"
}

func somenteDigitos(valor string) bool {
	for _, r := range valor {
		if r < '0' || r > '9' {
			return false
		}
	}
	return valor != ""
}
