package cli

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/edusouza/nfse-emissor-go/internal/anexoa"
	"github.com/edusouza/nfse-emissor-go/internal/infrastructure/ibge"
)

// ibgeFake serves the state list the IBGE documents, counting the requests so
// that a test can tell a cache answer from a network one.
func ibgeFake(t *testing.T, status int) (*httptest.Server, *atomic.Int32) {
	t.Helper()

	var chamadas atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		chamadas.Add(1)
		if r.URL.Path != "/estados/PR/municipios" {
			http.NotFound(w, r)
			return
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`[
			{"id": 4106902, "nome": "Curitiba"},
			{"id": 4105805, "nome": "Colombo"},
			{"id": 4100103, "nome": "Abatiá"}
		]`))
	}))
	t.Cleanup(srv.Close)
	return srv, &chamadas
}

// registroContador is a registry that must not be asked, counting if it is.
func registroContador(t *testing.T) (*httptest.Server, *atomic.Int32) {
	t.Helper()

	var chamadas atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		chamadas.Add(1)
		_, _ = w.Write([]byte(respostaMEI))
	}))
	t.Cleanup(srv.Close)
	return srv, &chamadas
}

// The municipality tests only need a certificate to exist; generating an RSA
// key for each of them was most of their running time.
var (
	pfxMunicipioOnce sync.Once
	pfxMunicipio     []byte
)

func certificadoDeTeste(t *testing.T) string {
	t.Helper()

	pfxMunicipioOnce.Do(func() {
		caminho := writeTestPFXSubject(t, onboardSubject, onboardPassword, time.Now().Add(300*24*time.Hour))
		var err error
		if pfxMunicipio, err = os.ReadFile(caminho); err != nil {
			t.Fatal(err)
		}
	})
	caminho := filepath.Join(t.TempDir(), "teste.pfx")
	if err := os.WriteFile(caminho, pfxMunicipio, 0o600); err != nil {
		t.Fatal(err)
	}
	return caminho
}

func onboardComMunicipio(t *testing.T, cache string, extra ...string) (string, string, error) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "nfse.yaml")
	args := append([]string{
		"--certificado", certificadoDeTeste(t), "--senha", onboardPassword,
		"--arquivo", path, "--cache-municipios", cache,
	}, extra...)

	out, err := runOnboard(t, args...)
	gerado, _ := os.ReadFile(path)
	return out, string(gerado), err
}

func cacheCom(t *testing.T, municipios ...ibge.Municipio) string {
	t.Helper()

	caminho := filepath.Join(t.TempDir(), "municipios.json")
	cache := ibge.NovoCache(caminho)
	for _, m := range municipios {
		cache.Guardar(m)
	}
	if err := cache.Gravar(); err != nil {
		t.Fatal(err)
	}
	return caminho
}

var curitiba = ibge.Municipio{Codigo: "4106902", Nome: "Curitiba", UF: "PR"}

func TestOnboardMunicipioPeloCodigoSemRede(t *testing.T) {
	cache := filepath.Join(t.TempDir(), "municipios.json")

	out, gerado, err := onboardComMunicipio(t, cache, "--sem-rede", "--municipio", "4106902")
	if err != nil {
		t.Fatalf("onboard falhou: %v\n%s", err, out)
	}
	if !strings.Contains(gerado, `municipio: "4106902"`) {
		t.Errorf("o municipio nao foi gravado:\n%s", gerado)
	}
	if strings.Contains(out, "prestador.municipio —") {
		t.Errorf("o municipio continua listado como pendente:\n%s", out)
	}
	if _, err := os.Stat(cache); err == nil {
		t.Error("um codigo informado nao deveria criar o cache")
	}
}

// A bare code the cache knows is shown with its name.
func TestOnboardMunicipioPeloCodigoMostraONomeDoCache(t *testing.T) {
	out, _, err := onboardComMunicipio(t, cacheCom(t, curitiba), "--sem-rede", "--municipio", "4106902")
	if err != nil {
		t.Fatalf("onboard falhou: %v\n%s", err, out)
	}
	if !strings.Contains(out, "Curitiba/PR (IBGE 4106902)") {
		t.Errorf("a saida nao traz o nome do cache:\n%s", out)
	}
}

func TestOnboardMunicipioRecusas(t *testing.T) {
	cache := filepath.Join(t.TempDir(), "municipios.json")

	for _, c := range []struct {
		nome, valor, quer string
	}{
		{"digito verificador", "4106903", "digito verificador"},
		{"codigo TOM", "7535", "7 digitos"},
		{"nome sem UF", "Bom Jesus", "nao encontrei a UF"},
		{"UF que nao existe", "Curitiba/XX", "nao encontrei a UF"},
		{"so a UF", "/PR", "falta o nome"},
		{"nome fora do cache, sem rede", "Curitiba/PR", "sem rede"},
	} {
		t.Run(c.nome, func(t *testing.T) {
			out, gerado, err := onboardComMunicipio(t, cache, "--sem-rede", "--municipio", c.valor)
			if err == nil || !strings.Contains(err.Error(), c.quer) {
				t.Errorf("esperava erro com %q; veio %v\n%s", c.quer, err, out)
			}
			if gerado != "" {
				t.Error("o arquivo foi gravado apesar do erro")
			}
		})
	}
}

// A name is resolved once against the state's list and remembered, so the
// second run works without the network.
func TestOnboardMunicipioPeloNome(t *testing.T) {
	srv, chamadas := ibgeFake(t, http.StatusOK)
	cache := filepath.Join(t.TempDir(), "municipios.json")

	out, gerado, err := onboardComMunicipio(t, cache,
		"--fonte", "http://127.0.0.1:1", "--fonte-municipios", srv.URL, "--municipio", "curitiba / pr")
	if err != nil {
		t.Fatalf("onboard falhou: %v\n%s", err, out)
	}
	if !strings.Contains(gerado, `municipio: "4106902"`) {
		t.Errorf("o municipio nao foi gravado:\n%s", gerado)
	}
	for _, want := range []string{"Consultando os municipios de PR", "Curitiba/PR (IBGE 4106902)"} {
		if !strings.Contains(out, want) {
			t.Errorf("a saida nao traz %q:\n%s", want, out)
		}
	}
	if !strings.Contains(gerado, "prestador.municipio: --municipio, consultado em") {
		t.Errorf("a origem do municipio nao esta no cabecalho:\n%s", gerado)
	}
	if chamadas.Load() != 1 {
		t.Errorf("%d consultas ao IBGE, esperava 1", chamadas.Load())
	}

	// Any other municipality of the same state is in the cache now too, and
	// the folded name matches: "abatia" is Abatiá.
	for valor, codigo := range map[string]string{"COLOMBO, PR": "4105805", "abatia - pr": "4100103", "Abatiá-PR": "4100103"} {
		out, gerado, err = onboardComMunicipio(t, cache, "--sem-rede", "--municipio", valor)
		if err != nil {
			t.Fatalf("%s: onboard com cache falhou: %v\n%s", valor, err, out)
		}
		if !strings.Contains(gerado, `municipio: "`+codigo+`"`) {
			t.Errorf("%s: o cache nao resolveu o nome:\n%s", valor, gerado)
		}
	}
	if chamadas.Load() != 1 {
		t.Errorf("as execucoes seguintes consultaram o IBGE (%d consultas)", chamadas.Load())
	}
}

// With the network on, a name the cache knows is still answered from the
// cache: nothing leaves the machine that does not have to.
func TestOnboardMunicipioCacheAntesDaRede(t *testing.T) {
	srv, chamadas := ibgeFake(t, http.StatusOK)

	out, gerado, err := onboardComMunicipio(t, cacheCom(t, curitiba),
		"--fonte", "http://127.0.0.1:1", "--fonte-municipios", srv.URL, "--municipio", "Curitiba/PR")
	if err != nil {
		t.Fatalf("onboard falhou: %v\n%s", err, out)
	}
	if chamadas.Load() != 0 {
		t.Errorf("consultou o IBGE %d vezes com o nome no cache", chamadas.Load())
	}
	if !strings.Contains(gerado, `municipio: "4106902"`) || strings.Contains(gerado, "consultado em") {
		t.Errorf("o arquivo nao reflete o cache:\n%s", gerado)
	}
}

// A poisoned cache — a name paired with another state's code — is ignored,
// and the list is asked for again.
func TestOnboardMunicipioCacheEnvenenado(t *testing.T) {
	srv, chamadas := ibgeFake(t, http.StatusOK)
	cache := filepath.Join(t.TempDir(), "municipios.json")
	if err := os.WriteFile(cache, []byte(`[{"codigo":"3550308","nome":"Curitiba","uf":"PR"}]`), 0o644); err != nil {
		t.Fatal(err)
	}

	out, gerado, err := onboardComMunicipio(t, cache,
		"--fonte", "http://127.0.0.1:1", "--fonte-municipios", srv.URL, "--municipio", "Curitiba/PR")
	if err != nil {
		t.Fatalf("onboard falhou: %v\n%s", err, out)
	}
	if !strings.Contains(gerado, `municipio: "4106902"`) {
		t.Errorf("o cache envenenado decidiu o municipio:\n%s", gerado)
	}
	if chamadas.Load() != 1 {
		t.Errorf("%d consultas ao IBGE, esperava 1", chamadas.Load())
	}
}

// A misspelled name is refused before the CNPJ is sent anywhere, with the
// closest names.
func TestOnboardMunicipioNomeErradoSugere(t *testing.T) {
	srv, _ := ibgeFake(t, http.StatusOK)
	registro, consultas := registroContador(t)
	cache := filepath.Join(t.TempDir(), "municipios.json")

	_, gerado, err := onboardComMunicipio(t, cache,
		"--fonte", registro.URL, "--fonte-municipios", srv.URL, "--municipio", "Curitib/PR")
	if err == nil || !strings.Contains(err.Error(), "4106902  Curitiba/PR") {
		t.Errorf("esperava a sugestao de Curitiba; veio %v", err)
	}
	if gerado != "" {
		t.Error("o arquivo foi gravado apesar do erro")
	}
	if consultas.Load() != 0 {
		t.Errorf("o CNPJ foi consultado %d vezes antes de o municipio ser resolvido", consultas.Load())
	}
}

func TestOnboardMunicipioConsultaFalha(t *testing.T) {
	srv, _ := ibgeFake(t, http.StatusInternalServerError)
	cache := filepath.Join(t.TempDir(), "municipios.json")

	_, gerado, err := onboardComMunicipio(t, cache,
		"--fonte", "http://127.0.0.1:1", "--fonte-municipios", srv.URL, "--municipio", "Curitiba/PR")
	if err == nil || !strings.Contains(err.Error(), "a consulta falhou") || !strings.Contains(err.Error(), "500") {
		t.Errorf("esperava a falha da consulta; veio %v", err)
	}
	if gerado != "" {
		t.Error("o arquivo foi gravado apesar do erro")
	}
	if _, err := os.Stat(cache); err == nil {
		t.Error("uma consulta que falhou criou o cache")
	}
}

// The flag outranks the registry, and a disagreement is said out loud.
func TestOnboardMunicipioDivergeDoCadastro(t *testing.T) {
	registro := registroFake(t, http.StatusOK, respostaMEI) // 3550308, São Paulo
	cache := filepath.Join(t.TempDir(), "municipios.json")

	out, gerado, err := onboardComMunicipio(t, cache, "--fonte", registro.URL, "--municipio", "4106902")
	if err != nil {
		t.Fatalf("onboard falhou: %v\n%s", err, out)
	}
	if !strings.Contains(gerado, `municipio: "4106902"`) {
		t.Errorf("o cadastro sobrescreveu o --municipio:\n%s", gerado)
	}
	if !strings.Contains(out, "vale o --municipio 4106902") {
		t.Errorf("a divergencia nao foi avisada:\n%s", out)
	}
	if !strings.Contains(gerado, "prestador.municipio: --municipio") {
		t.Errorf("a origem ficou errada:\n%s", gerado)
	}
}

// When the two agree there is nothing to say, and the origin stays the flag.
func TestOnboardMunicipioConcordaComOCadastro(t *testing.T) {
	registro := registroFake(t, http.StatusOK, respostaMEI) // 3550308, São Paulo
	cache := filepath.Join(t.TempDir(), "municipios.json")

	out, gerado, err := onboardComMunicipio(t, cache, "--fonte", registro.URL, "--municipio", "3550308")
	if err != nil {
		t.Fatalf("onboard falhou: %v\n%s", err, out)
	}
	if strings.Contains(out, "aviso") {
		t.Errorf("avisou sem motivo:\n%s", out)
	}
	if !strings.Contains(gerado, "prestador.municipio: --municipio\n") {
		t.Errorf("a origem deveria continuar sendo o --municipio:\n%s", gerado)
	}
}

func TestSepararUF(t *testing.T) {
	casos := []struct {
		valor, nome, uf string
		ok              bool
	}{
		{"Curitiba/PR", "Curitiba", "PR", true},
		{"curitiba/pr", "curitiba", "PR", true},
		{"Curitiba/ pr ", "Curitiba", "PR", true},
		{"Curitiba,PR", "Curitiba", "PR", true},
		{"Curitiba-PR", "Curitiba", "PR", true},
		{"Embu-Guaçu - SP", "Embu-Guaçu", "SP", true},
		{"Embu-Guaçu-SP", "Embu-Guaçu", "SP", true},
		{"Santa Bárbara d'Oeste/SP", "Santa Bárbara d'Oeste", "SP", true},
		{"Curitiba/Paraná", "", "", false},
		{"São Paulo/SP/", "", "", false},
		{"Curitiba", "", "", false},
		{"Embu-Guaçu", "", "", false},
	}
	for _, c := range casos {
		nome, uf, ok := separarUF(c.valor)
		if ok != c.ok || nome != c.nome || uf != c.uf {
			t.Errorf("separarUF(%q) = %q, %q, %v; esperava %q, %q, %v", c.valor, nome, uf, ok, c.nome, c.uf, c.ok)
		}
	}
}

// No official name may be mistaken for "name + state" on its own: a name that
// ended in "-SP" or "/SP" would lose its last part.
func TestSepararUFNaoCortaNomesOficiais(t *testing.T) {
	municipios, err := anexoa.Ler()
	if err != nil {
		t.Fatal(err)
	}
	for _, m := range municipios {
		if nome, uf, ok := separarUF(m.Nome); ok {
			t.Errorf("%q seria lido como %q/%s", m.Nome, nome, uf)
		}
	}
}

func TestParecidos(t *testing.T) {
	lista := []ibge.Municipio{
		{Codigo: "1", Nome: "Santa Cruz do Sul", UF: "RS"},
		{Codigo: "2", Nome: "Cruzeiro", UF: "SP"},
		{Codigo: "3", Nome: "Cruz Alta", UF: "RS"},
		{Codigo: "4", Nome: "Vera Cruz", UF: "RS"},
		{Codigo: "5", Nome: "Cruzaltense", UF: "RS"},
		{Codigo: "6", Nome: "Nova Cruz", UF: "RN"},
		{Codigo: "7", Nome: "Porto Alegre", UF: "RS"},
	}

	got := parecidos(lista, "cruz", 5)
	want := []string{
		"3  Cruz Alta/RS", "5  Cruzaltense/RS", "2  Cruzeiro/SP", // start with it, by name
		"6  Nova Cruz/RN", "1  Santa Cruz do Sul/RS", // contain it, by name
	}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Errorf("parecidos = %q\nesperava    %q", got, want)
	}

	// An extra letter at the end still finds the name.
	if got := parecidos(lista, "portoalegree", 5); len(got) != 1 || got[0] != "7  Porto Alegre/RS" {
		t.Errorf("parecidos(portoalegree) = %q", got)
	}
}
