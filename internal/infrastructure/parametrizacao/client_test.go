package parametrizacao

import (
	"context"
	"crypto/tls"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// The bodies below are the ones the live service returned on 30/09/2026, as
// recorded in docs/convenio-municipal.md. The swagger declares types but not
// formats, so the fixtures come from the service, not from the document.
const (
	convenioAtivo = `{"parametrosConvenio":{"aderenteAmbienteNacional":1,"aderenteEmissorNacional":1,` +
		`"situacaoEmissaoPadraoContribuintesRFB":1,"aderenteMAN":0,"permiteAproveitametoDeCreditos":true},` +
		`"mensagem":"Parâmetros do convênio recuperados com sucesso."}`

	// The recorded message is cut short in the docs; the start is verbatim.
	convenioInativo = `{"parametrosConvenio":null,"mensagem":"O convênio do município <São Caetano do Sul/SP> ainda não está ativo…"}`

	aliquotaVigente = `{"aliquotas":{"01.07.01.000":[{"Incidencia":"SIM","Aliq":5.00,"DtIni":"2023-01-02T00:00:00","DtFim":null}]},` +
		`"mensagem":"Alíquotas recuperadas com sucesso."}`

	// Curitiba's test history: 5% until 31/12/2022, 2% on 01/01/2023 only,
	// 5% from 02/01/2023. Deliberately out of order, as nothing promises one.
	historicoCuritiba = `{"aliquotas":{"01.07.01.000":[` +
		`{"Incidencia":"SIM","Aliq":5.00,"DtIni":"2023-01-02T00:00:00","DtFim":null},` +
		`{"Incidencia":"SIM","Aliq":5.00,"DtIni":"2020-01-01T00:00:00","DtFim":"2022-12-31T00:00:00"},` +
		`{"Incidencia":"SIM","Aliq":2.00,"DtIni":"2023-01-01T00:00:00","DtFim":"2023-01-01T00:00:00"}]},` +
		`"mensagem":"Alíquotas recuperadas com sucesso."}`

	codigoMalFormado = `{"aliquotas":null,"mensagem":"Chamada mal formada. O código do serviço deve ser composto por nove dígitos."}`

	// The text of this 404 was not recorded; the shape is the envelope every
	// refusal of the service shares.
	semAliquota = `{"aliquotas":null,"mensagem":"Nenhuma alíquota encontrada."}`

	competenciaInvalida = `{"type":"https://tools.ietf.org/html/rfc9110#section-15.5.1",` +
		`"title":"One or more validation errors occurred.","status":400,` +
		`"errors":{"competencia":["The value '15-09-2026' is not valid."]},"traceId":"00-abc-def-00"}`
)

const curitiba = "4106902"

// serve starts a stub ADN answering every request with status and body, and
// returns a client pointed at it together with the paths it was asked for.
func serve(t *testing.T, status int, body string) (*Client, *[]string) {
	t.Helper()

	var caminhos []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		caminhos = append(caminhos, r.URL.EscapedPath())
		if r.Method != http.MethodGet {
			t.Errorf("metodo %s; o cliente so faz GET", r.Method)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	client, err := New(Config{BaseURL: srv.URL, HTTPClient: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	return client, &caminhos
}

func competencia(t *testing.T, valor string) time.Time {
	t.Helper()
	c, err := time.Parse(LayoutCompetencia, valor)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func dia(valor string) time.Time {
	d, _ := time.Parse(LayoutCompetencia, valor)
	return d
}

func TestConvenioAtivo(t *testing.T) {
	client, caminhos := serve(t, http.StatusOK, convenioAtivo)

	c, err := client.Convenio(context.Background(), curitiba)
	if err != nil {
		t.Fatal(err)
	}

	if got := *caminhos; len(got) != 1 || got[0] != "/4106902/convenio" {
		t.Errorf("caminhos = %v, esperava [/4106902/convenio]", got)
	}
	if !c.Ativo {
		t.Error("convenio respondido com parametros deveria estar ativo")
	}
	if c.AderenteAmbienteNacional != 1 || c.AderenteEmissorNacional != 1 ||
		c.SituacaoEmissaoPadraoContribuintesRFB != 1 || c.AderenteMAN != 0 {
		t.Errorf("parametros lidos errado: %+v", c)
	}
	if c.PermiteAproveitamentoDeCreditos == nil || !*c.PermiteAproveitamentoDeCreditos {
		t.Error("permiteAproveitametoDeCreditos (com a grafia da API) nao foi lido")
	}
	if !strings.Contains(c.Mensagem, "recuperados com sucesso") {
		t.Errorf("mensagem = %q", c.Mensagem)
	}
}

// An inactive convênio is the answer E0635/E0640 turn on, not a failure.
func TestConvenioInativo(t *testing.T) {
	client, _ := serve(t, http.StatusNotFound, convenioInativo)

	c, err := client.Convenio(context.Background(), "3548807")
	if err != nil {
		t.Fatalf("convenio inativo deveria ser resposta, nao erro: %v", err)
	}
	if c.Ativo {
		t.Error("404 com o envelope do servico deveria ser convenio inativo")
	}
	if !strings.Contains(c.Mensagem, "ainda não está ativo") {
		t.Errorf("o motivo do servico se perdeu: %q", c.Mensagem)
	}
}

// A 404 that did not come from the service's rules — a wrong path, a proxy —
// must never read as "inactive": that would demand a rate the Sefin forbids.
func TestConvenio404SemEnvelopeEErro(t *testing.T) {
	for nome, body := range map[string]string{
		"vazio":          "",
		"html":           "<html><body>Not Found</body></html>",
		"sem mensagem":   `{"parametrosConvenio":null}`,
		"outro endpoint": `{"aliquotas":null,"mensagem":""}`,
	} {
		t.Run(nome, func(t *testing.T) {
			client, _ := serve(t, http.StatusNotFound, body)

			c, err := client.Convenio(context.Background(), curitiba)
			if err == nil {
				t.Fatalf("esperava erro, veio %+v", c)
			}
			if !errors.Is(err, ErrNaoEncontrado) {
				t.Errorf("erro = %v, esperava ErrNaoEncontrado", err)
			}
		})
	}
}

func TestConvenio200SemParametrosEErro(t *testing.T) {
	client, _ := serve(t, http.StatusOK, `{"parametrosConvenio":null,"mensagem":"ok"}`)

	if _, err := client.Convenio(context.Background(), curitiba); !errors.Is(err, ErrRespostaInesperada) {
		t.Errorf("erro = %v, esperava ErrRespostaInesperada", err)
	}
}

func TestAliquota(t *testing.T) {
	client, caminhos := serve(t, http.StatusOK, aliquotaVigente)

	lista, err := client.Aliquota(context.Background(), curitiba, "01.07.01.000", competencia(t, "2026-09-01"))
	if err != nil {
		t.Fatal(err)
	}

	if got := *caminhos; len(got) != 1 || got[0] != "/4106902/01.07.01.000/2026-09-01/aliquota" {
		t.Errorf("caminhos = %v", got)
	}
	if len(lista) != 1 {
		t.Fatalf("%d aliquotas, esperava 1", len(lista))
	}
	a := lista[0]
	if a.Percentual == nil || *a.Percentual != 5 {
		t.Errorf("percentual = %v, esperava 5", a.Percentual)
	}
	if a.Incidencia != "SIM" {
		t.Errorf("incidencia = %q", a.Incidencia)
	}
	if !a.Inicio.Equal(dia("2023-01-02")) {
		t.Errorf("inicio = %s, esperava 2023-01-02", a.Inicio)
	}
	if a.Fim != nil {
		t.Errorf("fim = %s, esperava vigencia aberta", a.Fim)
	}
}

// The competence date travels as the calendar day it names, whatever zone the
// caller's time is in. Late evening in São Paulo is already the next day in UTC.
func TestAliquotaCompetenciaNoFusoDoChamador(t *testing.T) {
	client, caminhos := serve(t, http.StatusOK, aliquotaVigente)

	saoPaulo := time.FixedZone("BRT", -3*3600)
	noite := time.Date(2026, 9, 30, 23, 30, 0, 0, saoPaulo)
	if _, err := client.Aliquota(context.Background(), curitiba, "01.07.01.000", noite); err != nil {
		t.Fatal(err)
	}
	if got := (*caminhos)[0]; !strings.Contains(got, "/2026-09-30/") {
		t.Errorf("caminho = %q, esperava a competencia 2026-09-30", got)
	}
}

func TestHistoricoAliquotas(t *testing.T) {
	client, caminhos := serve(t, http.StatusOK, historicoCuritiba)

	lista, err := client.HistoricoAliquotas(context.Background(), curitiba, "01.07.01.000")
	if err != nil {
		t.Fatal(err)
	}

	if got := *caminhos; len(got) != 1 || got[0] != "/4106902/01.07.01.000/historicoaliquotas" {
		t.Errorf("caminhos = %v", got)
	}
	if len(lista) != 3 {
		t.Fatalf("%d periodos, esperava 3", len(lista))
	}

	inicios := []string{"2020-01-01", "2023-01-01", "2023-01-02"}
	for i, quer := range inicios {
		if !lista[i].Inicio.Equal(dia(quer)) {
			t.Errorf("periodo %d comeca em %s, esperava %s (do mais antigo ao mais novo)", i, lista[i].Inicio, quer)
		}
	}
	if lista[0].Fim == nil || !lista[0].Fim.Equal(dia("2022-12-31")) {
		t.Errorf("fim do primeiro periodo = %v, esperava 2022-12-31", lista[0].Fim)
	}
	if *lista[1].Percentual != 2 {
		t.Errorf("percentual do segundo periodo = %v, esperava 2", *lista[1].Percentual)
	}
}

func TestAliquotaRecusas(t *testing.T) {
	casos := []struct {
		nome     string
		status   int
		body     string
		sentinel error
		trecho   string
	}{
		{"codigo mal formado", http.StatusBadRequest, codigoMalFormado, ErrConsultaInvalida, "nove dígitos"},
		{"competencia ilegivel", http.StatusBadRequest, competenciaInvalida, ErrConsultaInvalida,
			"competencia: The value '15-09-2026' is not valid."},
		{"sem aliquota", http.StatusNotFound, semAliquota, ErrNaoEncontrado, "Nenhuma alíquota"},
		{"certificado recusado", http.StatusForbidden, "", ErrAcessoNegado, "HTTP 403"},
		{"fora do ar", http.StatusServiceUnavailable, "<html>", ErrIndisponivel, "HTTP 503"},
		{"outro status", http.StatusTeapot, "", ErrRespostaInesperada, "HTTP 418"},
	}

	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			client, _ := serve(t, c.status, c.body)

			_, err := client.Aliquota(context.Background(), curitiba, "01.07.01.000", competencia(t, "2026-09-01"))
			if !errors.Is(err, c.sentinel) {
				t.Fatalf("erro = %v, esperava %v", err, c.sentinel)
			}
			var resposta *RespostaError
			if !errors.As(err, &resposta) || resposta.Status != c.status {
				t.Errorf("erro = %#v, esperava RespostaError com status %d", err, c.status)
			}
			if !strings.Contains(err.Error(), c.trecho) {
				t.Errorf("mensagem %q nao traz %q", err, c.trecho)
			}
		})
	}
}

// An answer about a code other than the one asked for is not an answer.
func TestAliquotaDeOutroCodigoEErro(t *testing.T) {
	client, _ := serve(t, http.StatusOK, strings.ReplaceAll(aliquotaVigente, "01.07.01.000", "01.07.01.001"))

	_, err := client.Aliquota(context.Background(), curitiba, "01.07.01.000", competencia(t, "2026-09-01"))
	if !errors.Is(err, ErrRespostaInesperada) || !strings.Contains(err.Error(), "01.07.01.001") {
		t.Errorf("erro = %v, esperava ErrRespostaInesperada citando o codigo que veio", err)
	}
}

func TestAliquotaSemDtIniEErro(t *testing.T) {
	client, _ := serve(t, http.StatusOK, `{"aliquotas":{"01.07.01.000":[{"Incidencia":"SIM","Aliq":5.0,"DtIni":null}]}}`)

	_, err := client.Aliquota(context.Background(), curitiba, "01.07.01.000", competencia(t, "2026-09-01"))
	if !errors.Is(err, ErrRespostaInesperada) {
		t.Errorf("erro = %v, esperava ErrRespostaInesperada", err)
	}
}

// Values the service would refuse are refused here, before any round trip.
func TestValidacaoAntesDaConsulta(t *testing.T) {
	client, caminhos := serve(t, http.StatusOK, aliquotaVigente)
	ctx := context.Background()
	quando := competencia(t, "2026-09-01")

	casos := map[string]func() error{
		"municipio curto": func() error { _, err := client.Convenio(ctx, "410690"); return err },
		"municipio com dv errado": func() error {
			_, err := client.Aliquota(ctx, "4106901", "01.07.01.000", quando)
			return err
		},
		"cTribNac sem complemento": func() error {
			_, err := client.Aliquota(ctx, curitiba, "010701", quando)
			return err
		},
		"nove digitos sem pontos": func() error {
			_, err := client.HistoricoAliquotas(ctx, curitiba, "010701000")
			return err
		},
		"sem competencia": func() error {
			_, err := client.Aliquota(ctx, curitiba, "01.07.01.000", time.Time{})
			return err
		},
	}
	for nome, consulta := range casos {
		if err := consulta(); err == nil {
			t.Errorf("%s: esperava erro", nome)
		}
	}
	if len(*caminhos) != 0 {
		t.Errorf("valores invalidos chegaram ao servico: %v", *caminhos)
	}
}

func TestNew(t *testing.T) {
	cert := &tls.Certificate{}

	for ambiente, quer := range map[string]string{
		"":                       ProducaoRestritaBaseURL,
		AmbienteProducaoRestrita: ProducaoRestritaBaseURL,
		AmbienteProducao:         ProducaoBaseURL,
	} {
		client, err := New(Config{Ambiente: ambiente, Certificate: cert})
		if err != nil {
			t.Fatalf("%q: %v", ambiente, err)
		}
		if client.BaseURL() != quer {
			t.Errorf("ambiente %q: URL = %q, esperava %q", ambiente, client.BaseURL(), quer)
		}
	}

	if _, err := New(Config{Ambiente: "homologacao", Certificate: cert}); err == nil {
		t.Error("ambiente desconhecido deveria ser recusado")
	}
	if _, err := New(Config{}); err == nil {
		t.Error("sem certificado deveria ser recusado: o ADN exige TLS mutuo")
	}
}

// falhaTransporte fails the first n round trips with err, then delegates.
type falhaTransporte struct {
	n          int32
	err        error
	tentativas atomic.Int32
	proximo    http.RoundTripper
}

func (f *falhaTransporte) RoundTrip(r *http.Request) (*http.Response, error) {
	if f.tentativas.Add(1) <= f.n {
		return nil, f.err
	}
	return f.proximo.RoundTrip(r)
}

type tempoEsgotadoErr struct{}

func (tempoEsgotadoErr) Error() string   { return "i/o timeout" }
func (tempoEsgotadoErr) Timeout() bool   { return true }
func (tempoEsgotadoErr) Temporary() bool { return true }

func serveFalhando(t *testing.T, falhas int32, err error) (*Client, *falhaTransporte) {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(convenioAtivo))
	}))
	t.Cleanup(srv.Close)

	transporte := &falhaTransporte{n: falhas, err: err, proximo: srv.Client().Transport}
	client, errNew := New(Config{BaseURL: srv.URL, HTTPClient: &http.Client{Transport: transporte}})
	if errNew != nil {
		t.Fatal(errNew)
	}
	client.retryDelays = make([]time.Duration, len(client.retryDelays))
	return client, transporte
}

// A failed handshake is what downloading the swagger ran into, and the second
// attempt passed. A GET is safe to repeat, so it is.
func TestHandshakeQueFalhaETentadoDeNovo(t *testing.T) {
	client, transporte := serveFalhando(t, 2, errors.New("remote error: tls: handshake failure"))

	var esperas int
	client.onRetry = func(time.Duration, error) { esperas++ }

	c, err := client.Convenio(context.Background(), curitiba)
	if err != nil {
		t.Fatalf("a terceira tentativa deveria passar: %v", err)
	}
	if !c.Ativo {
		t.Error("resposta errada depois das tentativas")
	}
	if got := transporte.tentativas.Load(); got != 3 {
		t.Errorf("%d tentativas, esperava 3", got)
	}
	if esperas != 2 {
		t.Errorf("OnRetry chamado %d vezes, esperava 2", esperas)
	}
}

func TestSemRespostaDepoisDasTentativas(t *testing.T) {
	client, transporte := serveFalhando(t, 99, errors.New("connection reset by peer"))

	_, err := client.Convenio(context.Background(), curitiba)
	if !errors.Is(err, ErrInacessivel) {
		t.Fatalf("erro = %v, esperava ErrInacessivel", err)
	}
	if !strings.Contains(err.Error(), "3 tentativas") {
		t.Errorf("mensagem %q nao diz quantas vezes tentou", err)
	}
	if got := transporte.tentativas.Load(); got != 3 {
		t.Errorf("%d tentativas, esperava 3", got)
	}
}

// Waiting out the same timeout three times would triple the wait for nothing.
func TestTempoEsgotadoNaoETentadoDeNovo(t *testing.T) {
	client, transporte := serveFalhando(t, 99, tempoEsgotadoErr{})

	if _, err := client.Convenio(context.Background(), curitiba); !errors.Is(err, ErrInacessivel) {
		t.Fatalf("erro = %v, esperava ErrInacessivel", err)
	}
	if got := transporte.tentativas.Load(); got != 1 {
		t.Errorf("%d tentativas, esperava 1", got)
	}
}

func TestDatas(t *testing.T) {
	for _, texto := range []string{
		`"2023-01-02T00:00:00"`,
		`"2023-01-02T13:45:10.1234567"`,
		`"2023-01-02T00:00:00-03:00"`,
		`"2023-01-02"`,
	} {
		var d data
		if err := d.UnmarshalJSON([]byte(texto)); err != nil {
			t.Errorf("%s: %v", texto, err)
			continue
		}
		if !d.Equal(dia("2023-01-02")) {
			t.Errorf("%s virou %s, esperava o dia 2023-01-02", texto, d.Time)
		}
	}

	var d data
	if err := d.UnmarshalJSON([]byte(`"02/01/2023"`)); err == nil {
		t.Error("data fora do formato deveria ser recusada")
	}
}

func TestSimNao(t *testing.T) {
	for valor, quer := range map[SimNao]string{1: "sim", 0: "nao", -1: "-1 (valor sem significado publicado)"} {
		if got := valor.String(); got != quer {
			t.Errorf("SimNao(%d) = %q, esperava %q", int(valor), got, quer)
		}
	}
}
