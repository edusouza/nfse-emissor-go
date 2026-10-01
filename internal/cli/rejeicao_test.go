package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/edusouza/nfse-emissor-go/internal/infrastructure/sefin"
)

func TestExplicarRejeicao(t *testing.T) {
	err := explicarRejeicao(&sefin.RejectionError{Rejections: []sefin.Message{
		{Codigo: "E0600", Descricao: "Não é permitido informar a alíquota para prestador de serviço optante do simples nacional do tipo MEI."},
		{Codigo: "E0635", Descricao: "Não é permitido informar alíquota quando o convênio do município de incidência do ISSQN está ativo."},
		{Codigo: "E0714", Descricao: "Arquivo enviado com erro na assinatura."},
		{Codigo: "E9999", Descricao: "Um codigo que o anexo nao tem."},
	}})
	msg := err.Error()

	exigirTrechos(t, msg,
		"documento rejeitado pela Sefin Nacional",
		"[E0600] Não é permitido informar a alíquota",
		// The path as the user's own file has it, not as the annex roots it.
		"Campo   DPS/infDPS/valores/trib/tribMun/pAliq",
		// The rule says what the message does not: which opSimpNac is MEI.
		"opSimpNac = 2",
		paginaRejeicoes+"#e0600",
		paginaRejeicoes+"#e0714",
		"Campo   DPS/Signature",
		// A code the annex does not have is shown as the Sefin sent it.
		"[E9999] Um codigo que o anexo nao tem.",
	)
	if strings.Contains(msg, "NFSe/infNFSe/DPS") {
		t.Errorf("o caminho deveria comecar em DPS, como no arquivo do usuario:\n%s", msg)
	}
	if strings.Contains(msg, "#e9999") {
		t.Errorf("um codigo fora do anexo nao tem pagina:\n%s", msg)
	}

	// Only the level 3 rule depends on the municipality.
	if n := strings.Count(msg, "Depende do municipio"); n != 1 {
		t.Errorf("%d avisos de regra municipal, esperava 1 (o E0635):\n%s", n, msg)
	}
	e0635 := msg[strings.Index(msg, "[E0635]"):strings.Index(msg, "[E0714]")]
	if !strings.Contains(e0635, "Depende do municipio") {
		t.Errorf("o aviso deveria estar sob o E0635:\n%s", e0635)
	}

	// The rule of E0006 is the message, word for word; it is not repeated.
	e0006 := explicarRejeicao(&sefin.RejectionError{Rejections: []sefin.Message{{Codigo: "E0006"}}}).Error()
	if strings.Contains(e0006, "Regra") {
		t.Errorf("a regra do E0006 so repete a mensagem e nao deveria aparecer:\n%s", e0006)
	}

	// Still the error callers branch on.
	if !errors.Is(err, sefin.ErrRejected) {
		t.Error("errors.Is(err, ErrRejected) deixou de valer")
	}
	var rej *sefin.RejectionError
	if !errors.As(err, &rej) || len(rej.Codes()) != 4 {
		t.Error("errors.As(err, *RejectionError) deixou de valer")
	}
}

// A rule the annex states over several lines keeps its lines, each lined up
// under the first.
func TestExplicarRejeicaoRegraEmVariasLinhas(t *testing.T) {
	msg := explicarRejeicao(&sefin.RejectionError{Rejections: []sefin.Message{
		{Codigo: "E0635"},
	}}).Error()

	// No sentence from the Sefin: the annex supplies it.
	exigirTrechos(t, msg, "[E0635] Não é permitido informar alíquota quando o convênio")

	linhas := strings.Split(msg, "\n")
	var regra int
	for i, l := range linhas {
		if strings.Contains(l, "Regra   ") {
			regra = i
		}
	}
	if regra == 0 || regra+1 >= len(linhas) {
		t.Fatalf("regra do E0635 ausente:\n%s", msg)
	}
	continuacao := linhas[regra+1]
	if continuacao != "" && !strings.HasPrefix(continuacao, recuoRegra+"        ") {
		t.Errorf("a continuacao da regra deveria vir alinhada sob ela: %q", continuacao)
	}
	for _, l := range linhas {
		if l != strings.TrimRight(l, " ") {
			t.Errorf("linha com espacos sobrando: %q", l)
		}
	}
}

func TestExplicarRejeicaoDeixaOutrosErros(t *testing.T) {
	if explicarRejeicao(nil) != nil {
		t.Error("sem erro, sem explicacao")
	}
	if err := explicarRejeicao(sefin.ErrUnreachable); err != sefin.ErrUnreachable {
		t.Errorf("um erro que nao e rejeicao deveria passar intacto: %v", err)
	}
}

// What the user sees when the Sefin refuses: from the HTTP answer to the
// message on the terminal.
func TestEnviar_RejeicaoExplicadaPeloAnexo(t *testing.T) {
	dir := workspace(t)
	if out, err := runEmit(t, dir, "--numero", "30", "--valor", "100", "--descricao", "Servico"); err != nil {
		t.Fatalf("emissao falhou: %v\n%s", err, out)
	}

	stubQuery(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tipoAmbiente":          2,
			"versaoAplicativo":      "1.0",
			"dataHoraProcessamento": "2026-09-18T12:00:00Z",
			"erros": []map[string]string{{
				"codigo":      "E0310",
				"descricao":   "O código de tributação nacional informado não existe conforme a lista de serviços nacional do Sistema Nacional NFS-e.",
				"complemento": "serv/cServ/cTribNac",
			}},
		})
	})

	out, err := execEnviar(t, dir, onlyXMLPath(t, dir))
	if !errors.Is(err, sefin.ErrRejected) {
		t.Fatalf("erro = %v, esperava a rejeicao\n%s", err, out)
	}
	exigirTrechos(t, err.Error(),
		"[E0310] O código de tributação nacional informado não existe",
		"(serv/cServ/cTribNac)",
		"Campo   DPS/infDPS/serv/cServ/cTribNac",
		paginaRejeicoes+"#e0310",
	)
}

// The link points at the site the repository publishes, at the page the site
// generator writes. Either moving would leave the terminal pointing at nothing.
func TestPaginaRejeicoesEhADoSite(t *testing.T) {
	mkdocs, err := os.ReadFile(filepath.Join("..", "..", "site", "mkdocs.yml"))
	if err != nil {
		t.Fatal(err)
	}
	var siteURL string
	for _, l := range strings.Split(string(mkdocs), "\n") {
		if v, ok := strings.CutPrefix(l, "site_url:"); ok {
			siteURL = strings.TrimSpace(v)
		}
	}
	if siteURL == "" {
		t.Fatal("site_url ausente de site/mkdocs.yml")
	}
	if want := strings.TrimSuffix(siteURL, "/") + "/referencia/rejeicoes/"; paginaRejeicoes != want {
		t.Errorf("paginaRejeicoes = %q, o site publica %q", paginaRejeicoes, want)
	}

	gerador, err := os.ReadFile(filepath.Join("..", "sitegen", "rejections.go"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(gerador), fmt.Sprintf("%q", "referencia/rejeicoes.md")) {
		t.Error("o sitegen nao grava mais referencia/rejeicoes.md; atualize paginaRejeicoes")
	}
}

func exigirTrechos(t *testing.T, texto string, trechos ...string) {
	t.Helper()
	for _, s := range trechos {
		if !strings.Contains(texto, s) {
			t.Errorf("faltou %q em:\n%s", s, texto)
		}
	}
}
