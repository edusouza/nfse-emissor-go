package cli

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/edusouza/nfse-emissor-go/internal/infrastructure/parametrizacao"
)

// Bodies as the live service returned them (docs/convenio-municipal.md).
const (
	admConvenioAtivo = `{"parametrosConvenio":{"aderenteAmbienteNacional":1,"aderenteEmissorNacional":1,` +
		`"situacaoEmissaoPadraoContribuintesRFB":1,"aderenteMAN":0,"permiteAproveitametoDeCreditos":true},` +
		`"mensagem":"Parâmetros do convênio recuperados com sucesso."}`
	admConvenioInativo = `{"parametrosConvenio":null,"mensagem":"O convênio do município <São Caetano do Sul/SP> ainda não está ativo…"}`
	admAliquota        = `{"aliquotas":{"01.07.01.000":[{"Incidencia":"SIM","Aliq":5.00,"DtIni":"2023-01-02T00:00:00","DtFim":null}]},"mensagem":"ok"}`
	admHistorico       = `{"aliquotas":{"01.07.01.000":[` +
		`{"Incidencia":"SIM","Aliq":5.00,"DtIni":"2020-01-01T00:00:00","DtFim":"2022-12-31T00:00:00"},` +
		`{"Incidencia":"SIM","Aliq":2.00,"DtIni":"2023-01-01T00:00:00","DtFim":"2023-01-01T00:00:00"},` +
		`{"Incidencia":"SIM","Aliq":5.00,"DtIni":"2023-01-02T00:00:00","DtFim":null}]},"mensagem":"ok"}`
)

type admResposta struct {
	status int
	body   string
}

// stubADN starts a fake ADN answering each path from rotas, 404 for anything
// else, and records the paths it was asked for.
func stubADN(t *testing.T, rotas map[string]admResposta) *[]string {
	t.Helper()

	var mu sync.Mutex
	var caminhos []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		caminhos = append(caminhos, r.URL.Path)
		mu.Unlock()

		resposta, ok := rotas[r.URL.Path]
		if !ok {
			t.Errorf("caminho inesperado: %s", r.URL.Path)
			resposta = admResposta{http.StatusNotFound, ""}
		}
		w.WriteHeader(resposta.status)
		w.Write([]byte(resposta.body))
	}))
	t.Cleanup(srv.Close)

	original := newParametrizacaoClient
	newParametrizacaoClient = func(cfg parametrizacao.Config) (*parametrizacao.Client, error) {
		cfg.BaseURL = srv.URL
		cfg.HTTPClient = srv.Client()
		return parametrizacao.New(cfg)
	}
	t.Cleanup(func() { newParametrizacaoClient = original })

	return &caminhos
}

func executarParametros(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()
	t.Setenv(envCertPassword, testCertPassword)
	// Municipality names come from the user's cache; a test must neither read
	// nor write the real one.
	t.Setenv("XDG_CACHE_HOME", t.TempDir())

	var out bytes.Buffer
	root := NewRootCommand()
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(append([]string{"parametros", "--config", filepath.Join(dir, "nfse.yaml")}, args...))

	err := root.Execute()
	return out.String(), err
}

func assertContem(t *testing.T, out string, trechos ...string) {
	t.Helper()
	for _, trecho := range trechos {
		if !strings.Contains(out, trecho) {
			t.Errorf("saida sem %q:\n%s", trecho, out)
		}
	}
}

func TestParametros_AliquotaNaCompetencia(t *testing.T) {
	caminhos := stubADN(t, map[string]admResposta{
		"/4106902/convenio":                         {http.StatusOK, admConvenioAtivo},
		"/4106902/01.07.01.000/2026-09-01/aliquota": {http.StatusOK, admAliquota},
	})

	out, err := executarParametros(t, workspace(t), "4106902", "010701", "--competencia", "2026-09-01")
	if err != nil {
		t.Fatalf("consulta falhou: %v\n%s", err, out)
	}

	if got := *caminhos; len(got) != 2 {
		t.Errorf("caminhos = %v, esperava convenio e aliquota", got)
	}
	assertContem(t, out,
		"IBGE 4106902",
		"ativo (emissor nacional: sim",
		"01.07.01.000",
		"01/09/2026",
		"5,00%",
		"de 02/01/2023 em diante",
		"E0635",
		// The workspace is producao-restrita, whose data is made up.
		"dados sao de teste",
	)
}

func TestParametros_HistoricoSemCompetencia(t *testing.T) {
	stubADN(t, map[string]admResposta{
		"/4106902/convenio":                        {http.StatusOK, admConvenioAtivo},
		"/4106902/01.07.01.000/historicoaliquotas": {http.StatusOK, admHistorico},
	})

	out, err := executarParametros(t, workspace(t), "4106902", "01.07.01")
	if err != nil {
		t.Fatalf("consulta falhou: %v\n%s", err, out)
	}

	assertContem(t, out,
		"Historico de aliquotas",
		"de 01/01/2020 a 31/12/2022",
		"2,00%",
		"de 02/01/2023 em diante",
	)
	if strings.Contains(out, "Competencia") {
		t.Errorf("sem --competencia, a saida nao deveria mostrar uma:\n%s", out)
	}
}

// Without an active convênio the ADN has no rate to give, so none is asked
// for, and the advice flips to E0640.
func TestParametros_ConvenioInativo(t *testing.T) {
	caminhos := stubADN(t, map[string]admResposta{
		"/3548807/convenio": {http.StatusNotFound, admConvenioInativo},
	})

	out, err := executarParametros(t, workspace(t), "3548807", "010701", "--competencia", "2026-09-01")
	if err != nil {
		t.Fatalf("convenio inativo e uma resposta, nao um erro: %v\n%s", err, out)
	}

	if got := *caminhos; len(got) != 1 {
		t.Errorf("caminhos = %v, esperava so o convenio", got)
	}
	assertContem(t, out, "INATIVO", "ainda não está ativo", "E0640")
}

func TestParametros_Complemento(t *testing.T) {
	caminhos := stubADN(t, map[string]admResposta{
		"/4106902/convenio":                         {http.StatusOK, admConvenioAtivo},
		"/4106902/01.07.01.001/2026-09-01/aliquota": {http.StatusOK, strings.ReplaceAll(admAliquota, ".000", ".001")},
	})

	out, err := executarParametros(t, workspace(t), "4106902", "010701", "--complemento", "001", "--competencia", "2026-09-01")
	if err != nil {
		t.Fatalf("consulta falhou: %v\n%s", err, out)
	}
	if got := *caminhos; len(got) != 2 || !strings.Contains(got[1], "01.07.01.001") {
		t.Errorf("caminhos = %v, esperava o complemento 001", got)
	}
}

// A rate the municipality never set is explained: the complement is the
// likeliest reason, and the user can do something about it.
func TestParametros_SemAliquota(t *testing.T) {
	stubADN(t, map[string]admResposta{
		"/4106902/convenio": {http.StatusOK, admConvenioAtivo},
		"/4106902/01.07.01.000/2026-09-01/aliquota": {http.StatusNotFound,
			`{"aliquotas":null,"mensagem":"Nenhuma alíquota encontrada."}`},
	})

	_, err := executarParametros(t, workspace(t), "4106902", "010701", "--competencia", "2026-09-01")
	if err == nil {
		t.Fatal("esperava erro")
	}
	assertContem(t, err.Error(), "Nenhuma alíquota encontrada.", "--complemento", "01/09/2026")
}

func TestParametros_CertificadoRecusado(t *testing.T) {
	stubADN(t, map[string]admResposta{
		"/4106902/convenio": {http.StatusForbidden, ""},
	})

	_, err := executarParametros(t, workspace(t), "4106902", "010701")
	if err == nil {
		t.Fatal("esperava erro")
	}
	assertContem(t, err.Error(), "ICP-Brasil")
}

// Every value checked offline is refused before the ADN is asked anything.
func TestParametros_EntradaInvalida(t *testing.T) {
	casos := map[string][]string{
		"codigo com letra":       {"4106902", "01070a"},
		"fora da lista nacional": {"4106902", "019999"},
		"complemento curto":      {"4106902", "010701", "--complemento", "1"},
		"competencia dia-mes":    {"4106902", "010701", "--competencia", "01-09-2026"},
		"municipio sem uf":       {"Curitiba", "010701"},
		"dv do municipio":        {"4106901", "010701"},
		"sem codigo":             {"4106902"},
	}

	for nome, args := range casos {
		t.Run(nome, func(t *testing.T) {
			caminhos := stubADN(t, map[string]admResposta{})

			if _, err := executarParametros(t, workspace(t), args...); err == nil {
				t.Fatal("esperava erro")
			}
			if len(*caminhos) != 0 {
				t.Errorf("o ADN foi consultado com entrada invalida: %v", *caminhos)
			}
		})
	}
}
