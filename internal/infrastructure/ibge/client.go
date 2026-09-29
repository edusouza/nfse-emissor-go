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
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
	"unicode"

	"github.com/edusouza/nfse-emissor-go/pkg/codmun"
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

// coerente reports whether a municipality can be trusted as a translation:
// a valid IBGE code, the state that code belongs to, and a printable name.
//
// Every entry that reaches the cache or leaves this package passes through
// it. The cache is shared by the DANFSe and by onboard, and onboard writes
// what it finds into nfse.yaml: an entry that pairs a name with another
// state's code — from a lying server, a proxy, or a hand-edited file — would
// otherwise put every invoice in the wrong municipality. A name with control
// characters would reach the terminal as escape sequences.
func (m Municipio) coerente() bool {
	if codmun.Validar(m.Codigo) != nil || m.UF != codmun.UF(m.Codigo) {
		return false
	}
	if strings.TrimSpace(m.Nome) == "" {
		return false
	}
	for _, r := range m.Nome {
		if unicode.IsControl(r) {
			return false
		}
	}
	return true
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
// codes are about to be sent. A base URL that does not parse is shown whole:
// the announcement is about being honest, and the full string is honest.
func (c *Client) Host() string {
	endereco, err := url.Parse(c.baseURL)
	if err != nil || endereco.Host == "" {
		return c.baseURL
	}
	return endereco.Host
}

// ErrInacessivel marks a lookup that never reached the service: no route, a
// refused connection, a timeout. The municipality may well exist; there was
// just nobody to ask, and asking again during the same run would only wait
// out the same timeout.
var ErrInacessivel = errors.New("servico de municipios inacessivel")

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

	body, err := c.get(ctx, "/municipios/"+codigo, maxResponseSize, "o municipio "+codigo)
	if err != nil {
		return nil, err
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

	// The state comes from the code, which is checked; the answer's own UF
	// only has to agree with it.
	m := &Municipio{Codigo: codigo, Nome: strings.TrimSpace(dados.Nome), UF: codmun.UF(codigo)}
	if uf := dados.uf(); uf != "" && !strings.EqualFold(uf, m.UF) {
		return nil, fmt.Errorf("%s respondeu a UF %s para o municipio %s, que e de %s", c.Host(), uf, codigo, m.UF)
	}
	if !m.coerente() {
		return nil, fmt.Errorf("%s respondeu um municipio %s que nao confere; a resposta foi descartada", c.Host(), codigo)
	}
	return m, nil
}

// maxListaSize caps the answer for a whole state. Minas Gerais has 853
// municipalities, each a few hundred bytes in the service's nested format:
// about 1 MB. A larger body is refused rather than decoded, because the
// decoder would hold all of it, several times over, before any check ran.
const maxListaSize = 2 << 20

// Municipios fetches every municipality of a state, which is how a name typed
// by a person is turned into a code without carrying the table in the binary.
//
// The answer is checked, not trusted: every code must be a valid IBGE code of
// the state that was asked for. One that is not means the service is not
// answering what this client thinks it is, and a list that might put a name on
// the wrong code is refused whole.
func (c *Client) Municipios(ctx context.Context, uf string) ([]Municipio, error) {
	uf = strings.ToUpper(strings.TrimSpace(uf))
	if !codmun.UFExiste(uf) {
		return nil, fmt.Errorf("%q nao e a sigla de uma UF", uf)
	}

	body, err := c.get(ctx, "/estados/"+uf+"/municipios", maxListaSize+1, "os municipios de "+uf)
	if err != nil {
		return nil, err
	}
	if len(body) > maxListaSize {
		return nil, fmt.Errorf("%s devolveu mais de %d MB para os municipios de %s; a resposta foi descartada",
			c.Host(), maxListaSize>>20, uf)
	}

	var dados []resposta
	if err := json.Unmarshal(body, &dados); err != nil {
		return nil, fmt.Errorf("resposta de %s nao e o JSON esperado: %w", c.Host(), err)
	}
	if len(dados) == 0 {
		return nil, fmt.Errorf("%s nao devolveu municipios para %s", c.Host(), uf)
	}

	municipios := make([]Municipio, 0, len(dados))
	for _, d := range dados {
		m := Municipio{Codigo: d.ID.String(), Nome: strings.TrimSpace(d.Nome), UF: uf}
		if !m.coerente() {
			return nil, fmt.Errorf("%s devolveu %q entre os municipios de %s, que nao confere; a resposta foi descartada",
				c.Host(), m.Codigo, uf)
		}
		municipios = append(municipios, m)
	}
	return municipios, nil
}

// get performs one GET against the service and returns the body of a 200.
// oQue names what was asked for, for the error messages.
func (c *Client) get(ctx context.Context, caminho string, limite int64, oQue string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+caminho, nil)
	if err != nil {
		return nil, fmt.Errorf("nao foi possivel montar a consulta: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("nao foi possivel consultar %s: %w: %w", c.Host(), ErrInacessivel, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, limite))
	if err != nil {
		return nil, fmt.Errorf("falha ao ler a resposta de %s: %w", c.Host(), err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%s respondeu %d para %s", c.Host(), resp.StatusCode, oQue)
	}
	return body, nil
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
