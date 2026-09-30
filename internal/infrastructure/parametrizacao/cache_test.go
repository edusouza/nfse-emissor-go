package parametrizacao

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// relogio is a clock the test moves by hand.
type relogio struct{ agora time.Time }

func (r *relogio) passar(d time.Duration) { r.agora = r.agora.Add(d) }

// adnContado starts a stub ADN that answers each path from rotas and counts
// the requests that reached it.
func adnContado(t *testing.T, rotas map[string]string) (*Client, *atomic.Int32) {
	t.Helper()

	var pedidos atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pedidos.Add(1)
		body, ok := rotas[r.URL.Path]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(semAliquota))
			return
		}
		if strings.Contains(body, "ainda não está ativo") {
			w.WriteHeader(http.StatusNotFound)
		}
		w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	client, err := New(Config{BaseURL: srv.URL, HTTPClient: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	return client, &pedidos
}

func cacheDeTeste(t *testing.T) (*Cache, *relogio, string) {
	t.Helper()
	caminho := filepath.Join(t.TempDir(), NomeArquivo)
	r := &relogio{agora: time.Date(2026, 9, 30, 9, 0, 0, 0, time.UTC)}
	cache := NovoCache(caminho)
	cache.agora = func() time.Time { return r.agora }
	return cache, r, caminho
}

func TestCacheRespondeDentroDaValidade(t *testing.T) {
	client, pedidos := adnContado(t, map[string]string{"/4106902/convenio": convenioAtivo})
	cache, r, _ := cacheDeTeste(t)
	consulta := NovaConsulta(client, cache)
	ctx := context.Background()

	primeira, err := consulta.Convenio(ctx, curitiba)
	if err != nil {
		t.Fatal(err)
	}
	if primeira.DoCache || !primeira.ConsultadoEm.Equal(r.agora) {
		t.Errorf("primeira resposta = %+v, esperava vinda do ADN agora", primeira)
	}

	r.passar(Validade - time.Minute)
	segunda, err := consulta.Convenio(ctx, curitiba)
	if err != nil {
		t.Fatal(err)
	}
	if !segunda.DoCache || !segunda.Valor.Ativo {
		t.Errorf("segunda resposta = %+v, esperava o convenio do cache", segunda)
	}
	if !segunda.ConsultadoEm.Equal(primeira.ConsultadoEm) {
		t.Errorf("o cache deveria dizer quando a resposta foi obtida: %s, esperava %s",
			segunda.ConsultadoEm, primeira.ConsultadoEm)
	}
	if got := pedidos.Load(); got != 1 {
		t.Errorf("%d consultas ao ADN, esperava 1", got)
	}
}

func TestCacheVenceEmUmDia(t *testing.T) {
	client, pedidos := adnContado(t, map[string]string{"/4106902/convenio": convenioAtivo})
	cache, r, _ := cacheDeTeste(t)
	consulta := NovaConsulta(client, cache)

	if _, err := consulta.Convenio(context.Background(), curitiba); err != nil {
		t.Fatal(err)
	}
	r.passar(Validade)
	resposta, err := consulta.Convenio(context.Background(), curitiba)
	if err != nil {
		t.Fatal(err)
	}
	if resposta.DoCache {
		t.Error("uma resposta de 24 horas atras nao deveria mais valer")
	}
	if got := pedidos.Load(); got != 2 {
		t.Errorf("%d consultas ao ADN, esperava 2", got)
	}
}

func TestCacheRenovar(t *testing.T) {
	client, pedidos := adnContado(t, map[string]string{"/4106902/convenio": convenioAtivo})
	cache, r, _ := cacheDeTeste(t)
	consulta := NovaConsulta(client, cache)

	if _, err := consulta.Convenio(context.Background(), curitiba); err != nil {
		t.Fatal(err)
	}
	r.passar(time.Hour)
	consulta.Renovar = true
	renovada, err := consulta.Convenio(context.Background(), curitiba)
	if err != nil {
		t.Fatal(err)
	}
	if renovada.DoCache || pedidos.Load() != 2 {
		t.Errorf("Renovar deveria consultar o ADN (%d consultas, DoCache %v)", pedidos.Load(), renovada.DoCache)
	}

	// What it learned is remembered, with the new date.
	consulta.Renovar = false
	lembrada, err := consulta.Convenio(context.Background(), curitiba)
	if err != nil {
		t.Fatal(err)
	}
	if !lembrada.DoCache || !lembrada.ConsultadoEm.Equal(r.agora) {
		t.Errorf("resposta = %+v, esperava a renovada, do cache", lembrada)
	}
}

// What is remembered survives the run: that is the point of the cache.
func TestCachePersisteEntreExecucoes(t *testing.T) {
	client, pedidos := adnContado(t, map[string]string{
		"/4106902/convenio":                         convenioAtivo,
		"/4106902/01.07.01.000/2026-09-01/aliquota": aliquotaVigente,
		"/4106902/01.07.01.000/historicoaliquotas":  historicoCuritiba,
	})
	cache, r, caminho := cacheDeTeste(t)
	consulta := NovaConsulta(client, cache)
	ctx := context.Background()
	competencia := dia("2026-09-01")

	convenio, _ := consulta.Convenio(ctx, curitiba)
	vigente, _ := consulta.Aliquota(ctx, curitiba, "01.07.01.000", competencia)
	historico, _ := consulta.HistoricoAliquotas(ctx, curitiba, "01.07.01.000")
	if err := cache.Gravar(); err != nil {
		t.Fatal(err)
	}

	outra := NovoCache(caminho)
	outra.agora = func() time.Time { return r.agora }
	depois := NovaConsulta(client, outra)

	c2, err := depois.Convenio(ctx, curitiba)
	if err != nil || !c2.DoCache || *c2.Valor.PermiteAproveitamentoDeCreditos != *convenio.Valor.PermiteAproveitamentoDeCreditos {
		t.Errorf("convenio = %+v (%v), esperava o gravado", c2, err)
	}
	v2, err := depois.Aliquota(ctx, curitiba, "01.07.01.000", competencia)
	if err != nil || !v2.DoCache || *v2.Valor[0].Percentual != *vigente.Valor[0].Percentual || v2.Valor[0].Fim != nil {
		t.Errorf("aliquota = %+v (%v), esperava a gravada", v2, err)
	}
	h2, err := depois.HistoricoAliquotas(ctx, curitiba, "01.07.01.000")
	if err != nil || !h2.DoCache || len(h2.Valor) != len(historico.Valor) || !h2.Valor[0].Fim.Equal(*historico.Valor[0].Fim) {
		t.Errorf("historico = %+v (%v), esperava o gravado", h2, err)
	}
	if got := pedidos.Load(); got != 3 {
		t.Errorf("%d consultas ao ADN, esperava 3 (so na primeira execucao)", got)
	}
}

// An inactive convênio is an answer, and is remembered like one; a refusal is
// not, and is asked again.
func TestCacheGuardaRespostasNaoRecusas(t *testing.T) {
	client, pedidos := adnContado(t, map[string]string{"/3548807/convenio": convenioInativo})
	cache, _, _ := cacheDeTeste(t)
	consulta := NovaConsulta(client, cache)
	ctx := context.Background()

	for range 2 {
		c, err := consulta.Convenio(ctx, "3548807")
		if err != nil || c.Valor.Ativo {
			t.Fatalf("convenio = %+v (%v), esperava inativo", c, err)
		}
	}
	if got := pedidos.Load(); got != 1 {
		t.Errorf("convenio inativo consultado %d vezes, esperava 1", got)
	}

	for range 2 {
		if _, err := consulta.Aliquota(ctx, curitiba, "01.07.01.000", dia("2026-09-01")); !errors.Is(err, ErrNaoEncontrado) {
			t.Fatalf("erro = %v, esperava ErrNaoEncontrado", err)
		}
	}
	if got := pedidos.Load(); got != 3 {
		t.Errorf("%d consultas ao ADN, esperava que a recusa fosse repetida", got)
	}
}

// Production and restricted production answer differently, and neither may
// ever answer for the other.
func TestCacheSeparaAmbientes(t *testing.T) {
	restrita, _ := adnContado(t, map[string]string{"/4106902/convenio": convenioAtivo})
	producao, pedidos := adnContado(t, map[string]string{"/4106902/convenio": convenioAtivo})
	cache, _, _ := cacheDeTeste(t)

	if _, err := NovaConsulta(restrita, cache).Convenio(context.Background(), curitiba); err != nil {
		t.Fatal(err)
	}
	resposta, err := NovaConsulta(producao, cache).Convenio(context.Background(), curitiba)
	if err != nil {
		t.Fatal(err)
	}
	if resposta.DoCache || pedidos.Load() != 1 {
		t.Error("a resposta de um ambiente foi usada no outro")
	}
}

func TestCacheListaVaziaEResposta(t *testing.T) {
	client, pedidos := adnContado(t, map[string]string{
		"/4106902/01.07.01.000/historicoaliquotas": `{"aliquotas":{"01.07.01.000":[]},"mensagem":"ok"}`,
	})
	cache, r, caminho := cacheDeTeste(t)
	if _, err := NovaConsulta(client, cache).HistoricoAliquotas(context.Background(), curitiba, "01.07.01.000"); err != nil {
		t.Fatal(err)
	}
	if err := cache.Gravar(); err != nil {
		t.Fatal(err)
	}

	outra := NovoCache(caminho)
	outra.agora = func() time.Time { return r.agora }
	resposta, err := NovaConsulta(client, outra).HistoricoAliquotas(context.Background(), curitiba, "01.07.01.000")
	if err != nil || !resposta.DoCache || len(resposta.Valor) != 0 {
		t.Errorf("resposta = %+v (%v), esperava a lista vazia do cache", resposta, err)
	}
	if pedidos.Load() != 1 {
		t.Errorf("%d consultas ao ADN, esperava 1", pedidos.Load())
	}
}

func TestCacheGravarDescartaVencidas(t *testing.T) {
	client, _ := adnContado(t, map[string]string{
		"/4106902/convenio": convenioAtivo,
		"/3550308/convenio": convenioAtivo,
	})
	cache, r, caminho := cacheDeTeste(t)
	consulta := NovaConsulta(client, cache)

	consulta.Convenio(context.Background(), curitiba)
	r.passar(Validade)
	consulta.Convenio(context.Background(), "3550308")
	if err := cache.Gravar(); err != nil {
		t.Fatal(err)
	}

	conteudo, err := os.ReadFile(caminho)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(conteudo), "4106902") || !strings.Contains(string(conteudo), "3550308") {
		t.Errorf("o arquivo deveria guardar so a resposta que ainda vale:\n%s", conteudo)
	}
}

// The file is plain JSON in the user's cache directory. What does not hold
// together is dropped on the way in and asked for again.
func TestCacheDescartaEntradasIncoerentes(t *testing.T) {
	cache, _, caminho := cacheDeTeste(t)
	const base = "https://adn.exemplo"
	arquivo := `{
  "` + base + `/4106902/convenio": {"consultado_em": "2026-09-30T08:00:00Z", "convenio": {"ativo": true}, "aliquotas": null},
  "` + base + `/3550308/convenio": {"consultado_em": "2026-09-30T08:00:00Z", "aliquotas": []},
  "` + base + `/4106902/01.07.01.000/historicoaliquotas": {"consultado_em": "2026-09-30T08:00:00Z", "aliquotas": null},
  "` + base + `/4106902/01.07.01.001/historicoaliquotas": {"consultado_em": "2026-09-30T08:00:00Z",
      "aliquotas": [{"inicio": "2023-01-02T00:00:00Z", "fim": "2022-01-01T00:00:00Z"}]},
  "` + base + `/4106902/01.07.01.002/historicoaliquotas": {"consultado_em": "2026-09-30T08:00:00Z",
      "aliquotas": [{"percentual": 5}]},
  "` + base + `/4106902/01.07.01.003/historicoaliquotas": {"aliquotas": []},
  "` + base + `/4106902/beneficio": {"consultado_em": "2026-09-30T08:00:00Z", "aliquotas": []},
  "` + base + `/5300108/convenio": {"consultado_em": "2026-10-30T08:00:00Z", "convenio": {"ativo": true}, "aliquotas": null}
}`
	if err := os.WriteFile(caminho, []byte(arquivo), 0o644); err != nil {
		t.Fatal(err)
	}

	var chaves []string
	cache.mu.Lock()
	cache.carregar()
	for chave := range cache.dados {
		chaves = append(chaves, chave)
	}
	cache.mu.Unlock()
	sort.Strings(chaves)

	quer := []string{base + "/4106902/convenio", base + "/5300108/convenio"}
	if strings.Join(chaves, " ") != strings.Join(quer, " ") {
		t.Errorf("entradas aceitas = %v, esperava %v", chaves, quer)
	}

	// Loaded is not the same as usable: an answer dated in the future cannot
	// say how old it is.
	if _, ok := cache.buscar(base + "/5300108/convenio"); ok {
		t.Error("uma resposta datada no futuro nao deveria ser usada")
	}
	if _, ok := cache.buscar(base + "/4106902/convenio"); !ok {
		t.Error("a entrada coerente e recente deveria ser usada")
	}
}

func TestCacheCorrompidoEIgnorado(t *testing.T) {
	client, pedidos := adnContado(t, map[string]string{"/4106902/convenio": convenioAtivo})
	cache, _, caminho := cacheDeTeste(t)
	if err := os.WriteFile(caminho, []byte("isto nao e json"), 0o644); err != nil {
		t.Fatal(err)
	}

	resposta, err := NovaConsulta(client, cache).Convenio(context.Background(), curitiba)
	if err != nil || resposta.DoCache || pedidos.Load() != 1 {
		t.Errorf("resposta = %+v (%v); um cache corrompido deveria so custar uma consulta", resposta, err)
	}
	if err := cache.Gravar(); err != nil {
		t.Fatalf("o cache corrompido deveria ser substituido: %v", err)
	}
}

func TestSemCacheConsultaSempre(t *testing.T) {
	client, pedidos := adnContado(t, map[string]string{"/4106902/convenio": convenioAtivo})
	consulta := NovaConsulta(client, nil)

	for range 2 {
		if _, err := consulta.Convenio(context.Background(), curitiba); err != nil {
			t.Fatal(err)
		}
	}
	if got := pedidos.Load(); got != 2 {
		t.Errorf("%d consultas, esperava 2", got)
	}
}
