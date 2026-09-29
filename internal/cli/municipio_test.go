package cli

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// ibgeFake serves the state list the IBGE documents, counting the requests so
// that a test can tell a cache answer from a network one.
func ibgeFake(t *testing.T, status int) (*httptest.Server, *int32) {
	t.Helper()

	var chamadas int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&chamadas, 1)
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

func onboardComMunicipio(t *testing.T, cache string, extra ...string) (string, string, error) {
	t.Helper()

	cert := writeTestPFXSubject(t, onboardSubject, onboardPassword, time.Now().Add(300*24*time.Hour))
	path := filepath.Join(t.TempDir(), "nfse.yaml")
	args := append([]string{
		"--certificado", cert, "--senha", onboardPassword,
		"--arquivo", path, "--cache-municipios", cache,
	}, extra...)

	out, err := runOnboard(t, args...)
	gerado, _ := os.ReadFile(path)
	return out, string(gerado), err
}

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

func TestOnboardMunicipioRecusas(t *testing.T) {
	cache := filepath.Join(t.TempDir(), "municipios.json")

	for _, c := range []struct {
		nome, valor, quer string
	}{
		{"digito verificador", "4106903", "digito verificador"},
		{"codigo TOM", "7535", "7 digitos"},
		{"nome sem UF", "Bom Jesus", "informe a UF"},
		{"UF que nao existe", "Curitiba/XX", "informe a UF"},
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
		"--sem-rede=false", "--fonte", "http://127.0.0.1:1", "--fonte-municipios", srv.URL,
		"--municipio", "curitiba / pr")
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
	if *chamadas != 1 {
		t.Errorf("%d consultas ao IBGE, esperava 1", *chamadas)
	}

	// Any other municipality of the same state is in the cache now too.
	out, gerado, err = onboardComMunicipio(t, cache, "--sem-rede", "--municipio", "COLOMBO, PR")
	if err != nil {
		t.Fatalf("onboard com cache falhou: %v\n%s", err, out)
	}
	if !strings.Contains(gerado, `municipio: "4105805"`) {
		t.Errorf("o cache nao resolveu o nome:\n%s", gerado)
	}
	if *chamadas != 1 {
		t.Errorf("a segunda execucao consultou o IBGE (%d consultas)", *chamadas)
	}
}

func TestOnboardMunicipioNomeErradoSugere(t *testing.T) {
	srv, _ := ibgeFake(t, http.StatusOK)
	cache := filepath.Join(t.TempDir(), "municipios.json")

	_, gerado, err := onboardComMunicipio(t, cache,
		"--fonte", "http://127.0.0.1:1", "--fonte-municipios", srv.URL, "--municipio", "Curitib/PR")
	if err == nil || !strings.Contains(err.Error(), "4106902  Curitiba/PR") {
		t.Errorf("esperava a sugestao de Curitiba; veio %v", err)
	}
	if gerado != "" {
		t.Error("o arquivo foi gravado apesar do erro")
	}
}

func TestOnboardMunicipioConsultaFalha(t *testing.T) {
	srv, _ := ibgeFake(t, http.StatusInternalServerError)
	cache := filepath.Join(t.TempDir(), "municipios.json")

	_, _, err := onboardComMunicipio(t, cache,
		"--fonte", "http://127.0.0.1:1", "--fonte-municipios", srv.URL, "--municipio", "Curitiba/PR")
	if err == nil || !strings.Contains(err.Error(), "codigo IBGE de 7 digitos") {
		t.Errorf("esperava erro apontando o codigo como saida; veio %v", err)
	}
}

// The flag outranks the registry, and the disagreement is said out loud.
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
