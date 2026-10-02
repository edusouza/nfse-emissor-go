package parametrizacao

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/edusouza/nfse-emissor-go/internal/infrastructure/mtls"
	"github.com/edusouza/nfse-emissor-go/pkg/codmun"
)

// DefaultTimeout bounds one lookup. The answers are a few hundred bytes.
const DefaultTimeout = 30 * time.Second

// maxResponseSize caps how much of a response is read. A rate history is a
// handful of periods.
const maxResponseSize = 1 << 20

// defaultRetryDelays are the pauses between attempts of a lookup that got no
// answer, so a lookup is tried len+1 times. Downloading the swagger with a
// valid certificate, the first handshake failed often and the second passed.
var defaultRetryDelays = []time.Duration{2 * time.Second, 5 * time.Second}

// codigoServico is the only spelling of a service code the service accepts
// (see servico.CodigoCompleto).
var codigoServico = regexp.MustCompile(`^\d{2}\.\d{2}\.\d{2}\.\d{3}$`)

// Config configures a Client.
type Config struct {
	// Ambiente is AmbienteProducao or AmbienteProducaoRestrita. Empty means
	// the restricted one, the same default as the rest of the emitter.
	Ambiente string

	// Certificate is the A1 certificate. The service refuses the connection
	// without one, still in the handshake: even its documentation page does.
	Certificate *tls.Certificate

	// Timeout bounds each request. Zero means DefaultTimeout.
	Timeout time.Duration

	// BaseURL overrides the environment's URL. Used by tests.
	BaseURL string

	// HTTPClient overrides the constructed client. Used by tests.
	HTTPClient *http.Client

	// OnRetry, when set, is called before a lookup is tried again, with the
	// pause about to be taken and the failure that caused it.
	OnRetry func(wait time.Duration, err error)
}

// Client queries the ADN municipal parameters API.
type Client struct {
	httpClient  *http.Client
	baseURL     string
	retryDelays []time.Duration
	onRetry     func(time.Duration, error)
}

// New builds a client for the configured environment.
func New(cfg Config) (*Client, error) {
	baseURL := cfg.BaseURL
	if baseURL == "" {
		switch cfg.Ambiente {
		case AmbienteProducao:
			baseURL = ProducaoBaseURL
		case AmbienteProducaoRestrita, "":
			baseURL = ProducaoRestritaBaseURL
		default:
			return nil, fmt.Errorf("ambiente %q invalido (use %q ou %q)",
				cfg.Ambiente, AmbienteProducao, AmbienteProducaoRestrita)
		}
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		timeout := cfg.Timeout
		if timeout == 0 {
			timeout = DefaultTimeout
		}
		var err error
		if httpClient, err = mtls.NewHTTPClient(cfg.Certificate, timeout); err != nil {
			return nil, err
		}
	}

	return &Client{
		httpClient:  httpClient,
		baseURL:     strings.TrimSuffix(baseURL, "/"),
		retryDelays: defaultRetryDelays,
		onRetry:     cfg.OnRetry,
	}, nil
}

// BaseURL reports where the lookups go, so a caller can say it before asking.
func (c *Client) BaseURL() string { return c.baseURL }

// Convenio reports whether a municipality's convênio is active.
//
// An inactive convênio is an answer, not an error: the service says it with a
// 404 in its own envelope, and that comes back as Ativo false with the
// service's reason. A 404 without the envelope did not come from the service's
// rules and is returned as an error, so that a wrong path or a proxy never
// reads as "inactive".
func (c *Client) Convenio(ctx context.Context, municipio string) (*Convenio, error) {
	if err := codmun.Validar(municipio); err != nil {
		return nil, err
	}

	status, body, err := c.get(ctx, "/"+url.PathEscape(municipio)+rotaConvenio)
	if err != nil {
		return nil, err
	}

	r, ok := decodificar[respostaConvenio](body)
	switch {
	case status == http.StatusOK:
		if !ok || r.ParametrosConvenio == nil {
			return nil, fmt.Errorf("%w: o convenio de %s veio sem parametrosConvenio", ErrRespostaInesperada, municipio)
		}
		p := r.ParametrosConvenio
		return &Convenio{
			Ativo:                                 true,
			Mensagem:                              strings.TrimSpace(r.Mensagem),
			AderenteAmbienteNacional:              p.AderenteAmbienteNacional,
			AderenteEmissorNacional:               p.AderenteEmissorNacional,
			SituacaoEmissaoPadraoContribuintesRFB: p.SituacaoEmissaoPadraoContribuintesRFB,
			AderenteMAN:                           p.AderenteMAN,
			PermiteAproveitamentoDeCreditos:       p.PermiteAproveitametoDeCreditos,
		}, nil

	case status == http.StatusNotFound && ok && r.ParametrosConvenio == nil && strings.TrimSpace(r.Mensagem) != "":
		return &Convenio{Ativo: false, Mensagem: strings.TrimSpace(r.Mensagem)}, nil

	default:
		return nil, novoRespostaError(status, body)
	}
}

// Aliquota returns the ISSQN rate in force for a service on a competence date.
//
// codigo is the full service code, "01.07.01.000" (servico.CodigoCompleto
// builds it). The service answers a rate that does not exist with 404, which
// comes back matching ErrNaoEncontrado.
func (c *Client) Aliquota(ctx context.Context, municipio, codigo string, competencia time.Time) ([]Aliquota, error) {
	if err := validarConsulta(municipio, codigo); err != nil {
		return nil, err
	}
	if competencia.IsZero() {
		return nil, errors.New("competencia obrigatoria: a aliquota depende da data")
	}

	caminho := "/" + url.PathEscape(municipio) + "/" + url.PathEscape(codigo) + "/" +
		competencia.Format(LayoutCompetencia) + rotaAliquota
	return c.aliquotas(ctx, caminho, codigo)
}

// HistoricoAliquotas returns every rate period a municipality set for a
// service, oldest first.
func (c *Client) HistoricoAliquotas(ctx context.Context, municipio, codigo string) ([]Aliquota, error) {
	if err := validarConsulta(municipio, codigo); err != nil {
		return nil, err
	}

	caminho := "/" + url.PathEscape(municipio) + "/" + url.PathEscape(codigo) + rotaHistoricoAliquotas
	return c.aliquotas(ctx, caminho, codigo)
}

func validarConsulta(municipio, codigo string) error {
	if err := codmun.Validar(municipio); err != nil {
		return err
	}
	if !codigoServico.MatchString(codigo) {
		return fmt.Errorf("codigo de servico %q fora do formato que o ADN aceita (II.SS.DD.CCC, como 01.07.01.000)", codigo)
	}
	return nil
}

func (c *Client) aliquotas(ctx context.Context, caminho, codigo string) ([]Aliquota, error) {
	status, body, err := c.get(ctx, caminho)
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, novoRespostaError(status, body)
	}

	r, ok := decodificar[respostaAliquotas](body)
	if !ok {
		return nil, fmt.Errorf("%w: as aliquotas de %s nao vieram no JSON esperado", ErrRespostaInesperada, codigo)
	}

	// The map is keyed by the code that was asked for. An answer about some
	// other code is not an answer to this question, whatever it holds.
	periodos, ok := r.Aliquotas[codigo]
	if !ok {
		return nil, fmt.Errorf("%w: pedi as aliquotas de %s e vieram as de %s",
			ErrRespostaInesperada, codigo, chaves(r.Aliquotas))
	}

	lista := make([]Aliquota, 0, len(periodos))
	for _, p := range periodos {
		if p.DtIni.IsZero() {
			return nil, fmt.Errorf("%w: uma aliquota de %s veio sem DtIni", ErrRespostaInesperada, codigo)
		}
		a := Aliquota{Incidencia: strings.TrimSpace(p.Incidencia), Percentual: p.Aliq, Inicio: p.DtIni.Time}
		if p.DtFim != nil && !p.DtFim.IsZero() {
			fim := p.DtFim.Time
			a.Fim = &fim
		}
		lista = append(lista, a)
	}
	sort.SliceStable(lista, func(i, j int) bool { return lista[i].Inicio.Before(lista[j].Inicio) })
	return lista, nil
}

// get performs one lookup and returns the status and body of whatever the
// service answered.
//
// A lookup that got no answer is tried again. Unlike an emission, a GET is safe
// to repeat whatever happened to it, and the handshake with the ADN was seen to
// fail once and pass on the next try. A timeout is not retried: waiting out the
// same timeout three times would only triple the wait.
func (c *Client) get(ctx context.Context, caminho string) (int, []byte, error) {
	for tentativa := 1; ; tentativa++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+caminho, nil)
		if err != nil {
			return 0, nil, fmt.Errorf("nao foi possivel montar a consulta: %w", err)
		}
		req.Header.Set("Accept", "application/json")

		resp, err := c.httpClient.Do(req)
		if err == nil {
			body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize))
			resp.Body.Close()
			if err != nil {
				return 0, nil, fmt.Errorf("%w: a resposta foi interrompida: %w", ErrInacessivel, err)
			}
			return resp.StatusCode, body, nil
		}

		if tentativa > len(c.retryDelays) || ctx.Err() != nil || tempoEsgotado(err) {
			return 0, nil, falhaDeComunicacao(err, tentativa)
		}

		espera := c.retryDelays[tentativa-1]
		if c.onRetry != nil {
			c.onRetry(espera, err)
		}
		select {
		case <-ctx.Done():
			return 0, nil, falhaDeComunicacao(err, tentativa)
		case <-time.After(espera):
		}
	}
}

func tempoEsgotado(err error) bool {
	var netErr net.Error
	return errors.As(err, &netErr) && netErr.Timeout() && !mtls.IsDialError(err)
}

func falhaDeComunicacao(err error, tentativas int) error {
	depois := ""
	if tentativas > 1 {
		depois = fmt.Sprintf(" depois de %d tentativas", tentativas)
	}
	return fmt.Errorf("%w%s: %w", ErrInacessivel, depois, err)
}

// decodificar reads a JSON object, reporting whether it was one.
func decodificar[T any](body []byte) (T, bool) {
	var v T
	if err := json.Unmarshal(body, &v); err != nil {
		return v, false
	}
	return v, true
}

func chaves[V any](m map[string]V) string {
	if len(m) == 0 {
		return "nenhuma"
	}
	lista := make([]string, 0, len(m))
	for k := range m {
		lista = append(lista, k)
	}
	sort.Strings(lista)
	return strings.Join(lista, ", ")
}
