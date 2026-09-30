package cli

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/edusouza/nfse-emissor-go/internal/config"
)

// workspaceForaDoSimples builds an ME/EPP that assesses the ISSQN by the
// municipal rate, the one regime E0635 and E0640 apply to. A non-empty
// prestacao puts the service somewhere other than the provider's municipality.
func workspaceForaDoSimples(t *testing.T, apuracao, prestacao string) string {
	t.Helper()

	dir := workspaceRegime(t, config.RegimeMEEPP)
	caminho := filepath.Join(dir, "nfse.yaml")
	conteudo, err := os.ReadFile(caminho)
	if err != nil {
		t.Fatal(err)
	}

	yaml := strings.Replace(string(conteudo), "regime_tributario: me_epp\n",
		"regime_tributario: me_epp\n  regime_apuracao: "+apuracao+"\n", 1)
	if prestacao != "" {
		yaml = strings.Replace(yaml, "  servico:\n", "  servico:\n    municipio_prestacao: \""+prestacao+"\"\n", 1)
	}
	if yaml == string(conteudo) {
		t.Fatal("o nfse.yaml do teste mudou de forma; ajuste workspaceForaDoSimples")
	}
	if err := os.WriteFile(caminho, []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func emitirSimples(t *testing.T, dir string, extra ...string) (string, error) {
	t.Helper()
	args := append([]string{"--numero", "7", "--valor", "1000", "--descricao", "Consultoria"}, extra...)
	return runEmit(t, dir, args...)
}

func semXML(t *testing.T, dir string) {
	t.Helper()
	if matches, _ := filepath.Glob(filepath.Join(dir, "notas", "*.xml")); len(matches) != 0 {
		t.Errorf("uma DPS que a Sefin recusaria foi gravada: %v", matches)
	}
}

// E0635: with the convênio active, the Sefin applies the municipality's rate
// and refuses a declared one. The refusal comes before signing.
func TestEmitir_ConvenioAtivoRecusaAliquota(t *testing.T) {
	caminhos := stubADN(t, map[string]admResposta{
		"/4106902/convenio": {http.StatusOK, admConvenioAtivo},
	})
	dir := workspaceForaDoSimples(t, config.ApuracaoISSMunicipio, "")

	out, err := emitirSimples(t, dir, "--iss-aliquota", "3")
	if err == nil {
		t.Fatalf("esperava a recusa da E0635\n%s", out)
	}
	assertContem(t, err.Error(), "E0635", "iss_aliquota: 0", "consultado agora no ADN")
	if len(*caminhos) != 1 {
		t.Errorf("caminhos = %v, esperava so o convenio", *caminhos)
	}
	semXML(t, dir)
}

// E0640: with the convênio inactive there is no parameterised rate, and one
// has to be declared.
func TestEmitir_ConvenioInativoExigeAliquota(t *testing.T) {
	stubADN(t, map[string]admResposta{
		"/4106902/convenio": {http.StatusNotFound, admConvenioInativo},
	})
	dir := workspaceForaDoSimples(t, config.ApuracaoFora, "")

	out, err := emitirSimples(t, dir)
	if err == nil {
		t.Fatalf("esperava a recusa da E0640\n%s", out)
	}
	assertContem(t, err.Error(), "E0640", "--iss-aliquota")
	semXML(t, dir)
}

func TestEmitir_ConvenioConfereEPassa(t *testing.T) {
	casos := []struct {
		nome     string
		convenio admResposta
		extra    []string
		situacao string
	}{
		{"ativo, sem aliquota", admResposta{http.StatusOK, admConvenioAtivo}, nil, "ativo"},
		{"inativo, com aliquota", admResposta{http.StatusNotFound, admConvenioInativo}, []string{"--iss-aliquota", "3"}, "inativo"},
	}

	for _, c := range casos {
		t.Run(c.nome, func(t *testing.T) {
			stubADN(t, map[string]admResposta{"/4106902/convenio": c.convenio})
			dir := workspaceForaDoSimples(t, config.ApuracaoISSMunicipio, "")

			out, err := emitirSimples(t, dir, c.extra...)
			if err != nil {
				t.Fatalf("emissao falhou: %v\n%s", err, out)
			}
			assertContem(t, out, "Convenio do municipio 4106902: "+c.situacao)
			onlyXML(t, dir)
		})
	}
}

// A second emission the same day reuses the answer, and a refusal built on a
// remembered answer says so and how to ask again.
func TestEmitir_ConvenioDoCache(t *testing.T) {
	caminhos := stubADN(t, map[string]admResposta{
		"/4106902/convenio": {http.StatusOK, admConvenioAtivo},
	})
	dir := workspaceForaDoSimples(t, config.ApuracaoISSMunicipio, "")

	if out, err := emitirSimples(t, dir); err != nil {
		t.Fatalf("primeira emissao falhou: %v\n%s", err, out)
	}

	_, err := runEmit(t, dir, "--numero", "8", "--valor", "1000", "--descricao", "Consultoria", "--iss-aliquota", "3")
	if err == nil {
		t.Fatal("esperava a recusa da E0635")
	}
	assertContem(t, err.Error(), "resposta guardada", "nfse parametros 4106902 010101 --sem-cache")
	if len(*caminhos) != 1 {
		t.Errorf("%d consultas ao ADN em duas emissoes, esperava 1", len(*caminhos))
	}
}

// A lookup that fails leaves the rules to the Sefin, as before this check
// existed: a refusal here has to be certain.
func TestEmitir_ConvenioIndisponivelSegueComAviso(t *testing.T) {
	stubADN(t, map[string]admResposta{
		"/4106902/convenio": {http.StatusServiceUnavailable, "<html>fora do ar</html>"},
	})
	dir := workspaceForaDoSimples(t, config.ApuracaoISSMunicipio, "")

	out, err := emitirSimples(t, dir, "--iss-aliquota", "3")
	if err != nil {
		t.Fatalf("uma consulta que falha nao deveria impedir a emissao: %v\n%s", err, out)
	}
	assertContem(t, out, "E0635 e E0640", "nao foram conferidas", "indisponivel")
	onlyXML(t, dir)
}

// Where the ISSQN is due depends on the service (LC 116/2003, art. 3º). When
// the two candidates agree, it does not matter which; when they disagree, the
// check stays out of the way.
func TestEmitir_ConvenioComLocalDePrestacaoDiferente(t *testing.T) {
	t.Run("concordam", func(t *testing.T) {
		caminhos := stubADN(t, map[string]admResposta{
			"/4106902/convenio": {http.StatusOK, admConvenioAtivo},
			"/3550308/convenio": {http.StatusOK, admConvenioAtivo},
		})
		dir := workspaceForaDoSimples(t, config.ApuracaoISSMunicipio, "3550308")

		if _, err := emitirSimples(t, dir, "--iss-aliquota", "3"); err == nil || !strings.Contains(err.Error(), "E0635") {
			t.Fatalf("erro = %v, esperava E0635 com os dois convenios ativos", err)
		}
		if len(*caminhos) != 2 {
			t.Errorf("caminhos = %v, esperava os dois municipios", *caminhos)
		}
	})

	t.Run("discordam", func(t *testing.T) {
		stubADN(t, map[string]admResposta{
			"/4106902/convenio": {http.StatusOK, admConvenioAtivo},
			"/3550308/convenio": {http.StatusNotFound, admConvenioInativo},
		})
		dir := workspaceForaDoSimples(t, config.ApuracaoISSMunicipio, "3550308")

		out, err := emitirSimples(t, dir, "--iss-aliquota", "3")
		if err != nil {
			t.Fatalf("com os convenios discordando, o emissor nao deveria opinar: %v\n%s", err, out)
		}
		assertContem(t, out, "nao foram conferidas", "art. 3o")
	})
}

// Nobody outside the one regime pays the round trip.
func TestEmitir_ConvenioSoParaQuemDepende(t *testing.T) {
	caminhos := stubADN(t, map[string]admResposta{})

	for nome, dir := range map[string]string{
		"MEI":          workspace(t),
		"ME/EPP no SN": workspaceRegime(t, config.RegimeMEEPP),
	} {
		if out, err := emitirSimples(t, dir); err != nil {
			t.Fatalf("%s: emissao falhou: %v\n%s", nome, err, out)
		}
	}
	if len(*caminhos) != 0 {
		t.Errorf("o ADN foi consultado fora do regime que depende dele: %v", *caminhos)
	}
}

func TestEmitir_ConvenioSemAssinar(t *testing.T) {
	caminhos := stubADN(t, map[string]admResposta{})
	dir := workspaceForaDoSimples(t, config.ApuracaoISSMunicipio, "")

	out, err := emitirSimples(t, dir, "--sem-assinar", "--iss-aliquota", "3")
	if err != nil {
		t.Fatalf("emissao sem assinar falhou: %v\n%s", err, out)
	}
	assertContem(t, out, "nao foram conferidas", "sem assinar")
	if len(*caminhos) != 0 {
		t.Errorf("sem certificado nao ha como consultar o ADN: %v", *caminhos)
	}
}
