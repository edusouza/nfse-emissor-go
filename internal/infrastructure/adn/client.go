// Package adn talks to the Ambiente de Dados Nacional, which serves the
// DANFSe.
//
// The DANFSe — Documento Auxiliar da NFS-e — is the printable PDF of an
// invoice. The emitter does not draw it: the government generates it from the
// XML it already holds, so this package is a download, not a renderer. That is
// why there is no PDF library in go.mod.
//
// # What is verified and what is not
//
// The endpoint and its meaning come from the government's own manual, which is
// versioned in this repository:
//
//	docs/markdown/04-api-manual-municipios-adn.md, section 1.5 "API DANFSe"
//	a) GET – /danfse/{chaveAcesso}
//
// Two things about it are *not* verified, and the ADR says so plainly. First,
// that manual is the one written for municipalities; the contributors' manual
// does not describe the DANFSe API at all, and the ADN swagger this repository
// carries covers only /DFe and /NFSe/{chave}/Eventos. Whether a provider's A1
// is accepted here is unknown until someone tries. Second, the production host
// is inferred from the restricted one by the same naming the Sefin uses.
//
// The client is built for that uncertainty: it never writes a file it cannot
// recognize as a PDF, and it reports what actually came back instead.
package adn

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/edusouza/nfse-emissor-go/internal/domain/query"
)

// Base URLs of the Ambiente de Dados Nacional.
const (
	// RestrictedProductionBaseURL is the government's testing environment. It
	// is the host the Sefin swagger names when it answers 501 for /DANFSe:
	// "Este serviço foi movido para
	// https://adn.producaorestrita.nfse.gov.br/danfse/docs".
	RestrictedProductionBaseURL = "https://adn.producaorestrita.nfse.gov.br/danfse"

	// ProductionBaseURL is **inferred**, not read from a specification: the
	// Sefin publishes sefin.nfse.gov.br and sefin.producaorestrita.nfse.gov.br,
	// and this applies the same shape to the ADN. Use --url if it is wrong; the
	// error message says so.
	ProductionBaseURL = "https://adn.nfse.gov.br/danfse"
)

// Environment codes, matching the Sefin's tipoAmbiente.
const (
	EnvCodeProduction           = 1
	EnvCodeRestrictedProduction = 2
)

// DefaultTimeout bounds the request. It is longer than the Sefin client's
// because the service renders a document rather than looking a record up.
const DefaultTimeout = 60 * time.Second

// maxPDFSize caps how much of a response is read. A DANFSe is a page or two.
const maxPDFSize = 20 << 20

// pdfMagic is what every PDF starts with. It is the only way this client can
// tell a document from an error page, since it cannot rely on a Content-Type
// it has never seen the service send.
const pdfMagic = "%PDF-"

// Config configures a Client.
type Config struct {
	// BaseURL overrides the environment's URL.
	BaseURL string

	// Environment selects the base URL when BaseURL is empty.
	Environment int

	// Certificate is the A1 used for mutual TLS.
	Certificate *tls.Certificate

	// Timeout bounds the request. Zero means DefaultTimeout.
	Timeout time.Duration

	// HTTPClient overrides the constructed client. Used by tests.
	HTTPClient *http.Client

	// UserAgent identifies the caller.
	UserAgent string
}

// Client downloads DANFSe documents.
type Client struct {
	httpClient *http.Client
	baseURL    string
	userAgent  string
}

// New builds a client.
func New(cfg Config) *Client {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		if cfg.Environment == EnvCodeProduction {
			baseURL = ProductionBaseURL
		} else {
			baseURL = RestrictedProductionBaseURL
		}
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		timeout := cfg.Timeout
		if timeout == 0 {
			timeout = DefaultTimeout
		}

		transport := &http.Transport{}
		if cfg.Certificate != nil {
			transport.TLSClientConfig = &tls.Config{
				Certificates: []tls.Certificate{*cfg.Certificate},
				MinVersion:   tls.VersionTLS12,

				// Same reason as the Sefin client: the government's servers ask
				// for the client certificate by renegotiating after the
				// handshake, and Go refuses that by default. Without this the
				// first request dies as "local error: tls: no renegotiation".
				Renegotiation: tls.RenegotiateFreelyAsClient,
			}
		}
		httpClient = &http.Client{Timeout: timeout, Transport: transport}
	}

	return &Client{
		httpClient: httpClient,
		baseURL:    strings.TrimSuffix(baseURL, "/"),
		userAgent:  cfg.UserAgent,
	}
}

// BaseURL reports the service URL in use, so a caller can show where it went.
func (c *Client) BaseURL() string { return c.baseURL }

// BaixarDANFSe fetches the PDF of an NFS-e by its access key.
func (c *Client) BaixarDANFSe(ctx context.Context, chaveAcesso string) ([]byte, error) {
	chave := strings.TrimSpace(chaveAcesso)
	if err := query.ValidateAccessKey(chave); err != nil {
		return nil, err
	}

	url := fmt.Sprintf("%s/%s", c.baseURL, chave)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("nao foi possivel montar a requisicao: %w", err)
	}
	req.Header.Set("Accept", "application/pdf")
	if c.userAgent != "" {
		req.Header.Set("User-Agent", c.userAgent)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("falha na comunicacao com o ADN (%s): %w", c.baseURL, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxPDFSize))
	if err != nil {
		return nil, fmt.Errorf("falha ao ler a resposta do ADN: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, c.statusError(resp.StatusCode, chave, body)
	}

	// The status says OK, which is not the same as the body being a document.
	// Writing whatever arrived to a .pdf would hand the user a file that no
	// reader opens and no message explains.
	if !strings.HasPrefix(string(body), pdfMagic) {
		return nil, fmt.Errorf("o ADN respondeu 200 mas o conteudo nao e um PDF "+
			"(content-type %q, %d bytes): %s",
			resp.Header.Get("Content-Type"), len(body), resumo(body))
	}

	return body, nil
}

// statusError turns an HTTP status into something the user can act on.
func (c *Client) statusError(status int, chave string, body []byte) error {
	switch status {
	case http.StatusNotFound:
		// A 404 here has two causes that the status alone cannot tell apart,
		// and guessing the wrong one sends the user hunting in the wrong
		// place. The endpoint's own documentation page — the address the Sefin
		// names when it answers 501 — is itself 404 today, so "the service is
		// not at this address" is at least as likely as "the invoice is not
		// there".
		return fmt.Errorf("o ADN respondeu 404 em %s/%s.\n"+
			"Isso pode significar duas coisas:\n"+
			"  1. o servico nao esta nesse endereco — a pagina de documentacao\n"+
			"     que a Sefin indica (%s/docs/index.html) tambem responde 404,\n"+
			"     entao o caminho pode ter mudado. Tente --url, por exemplo\n"+
			"     %s/contribuintes/danfse ou %s/municipios/danfse;\n"+
			"  2. a NFS-e nao esta no Ambiente de Dados Nacional — confira se\n"+
			"     ela existe com 'nfse consultar <chave>'.\n"+
			"Resposta: %s",
			c.baseURL, chave, c.baseURL, hostDe(c.baseURL), hostDe(c.baseURL), resumo(body))

	case http.StatusForbidden, http.StatusUnauthorized:
		// This is the failure the ADR predicts: the DANFSe API is documented in
		// the municipalities' manual, and whether a provider's certificate is
		// accepted was never verified.
		return fmt.Errorf("acesso negado pelo ADN (%d).\n"+
			"A API DANFSe e descrita no manual dos municipios, e nao no dos\n"+
			"contribuintes — pode ser que um certificado de prestador nao seja\n"+
			"aceito aqui. Resposta: %s", status, resumo(body))

	case http.StatusNotImplemented:
		return fmt.Errorf("o ADN respondeu 501 em %s.\n"+
			"O endereco do servico provavelmente mudou; informe o novo com --url.\n"+
			"Resposta: %s", c.baseURL, resumo(body))

	case http.StatusServiceUnavailable:
		// 503 is not the same news as 404, and the difference matters enough to
		// separate: a 404 says nothing is routed at this path, while a 503 says
		// something is — the request reached a route and the backend behind it
		// did not answer. That can be a passing outage, or a service that is
		// simply not published in this environment. Telling the user to "try
		// again later" would be wrong in the second case.
		return fmt.Errorf("o ADN respondeu 503 em %s/%s.\n"+
			"O endereco existe e esta roteado, mas o servico atras dele nao\n"+
			"respondeu. Pode ser indisponibilidade passageira — tente de novo —\n"+
			"ou o servico pode nao estar publicado neste ambiente.\n"+
			"Resposta: %s", c.baseURL, chave, resumo(body))
	}

	if status >= 500 {
		return fmt.Errorf("o ADN respondeu %d; o servico esta indisponivel no momento", status)
	}
	return fmt.Errorf("o ADN respondeu %d: %s", status, resumo(body))
}

// hostDe strips the path off a base URL, so a suggested alternative address
// can be built from whatever --url the caller actually used.
func hostDe(baseURL string) string {
	rest := baseURL
	prefixo := ""
	if i := strings.Index(rest, "://"); i >= 0 {
		prefixo, rest = rest[:i+3], rest[i+3:]
	}
	if i := strings.Index(rest, "/"); i >= 0 {
		rest = rest[:i]
	}
	return prefixo + rest
}

// resumo shortens a response body for an error message.
func resumo(body []byte) string {
	const limit = 300

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
