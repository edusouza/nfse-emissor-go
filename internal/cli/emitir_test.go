package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/beevik/etree"

	"github.com/edusouza/nfse-emissor-go/internal/config"
	"github.com/edusouza/nfse-emissor-go/internal/infrastructure/xmlsigner"
)

const testCertPassword = "senha-de-teste"

// testCertCNPJ is the CNPJ in writeTestPFX's common name. The workspace uses it
// so that the emitter's certificate-versus-provider check passes by default;
// the tests that exercise a mismatch ask for a different one.
const testCertCNPJ = "12345678000195"

// workspace prepares a directory containing a valid nfse.yaml and a throwaway
// certificate, and returns its path. The provider is a MEI, which is the common
// case in these tests.
func workspace(t *testing.T) string {
	t.Helper()
	return workspaceRegime(t, config.RegimeMEI)
}

// workspaceRegime builds a workspace for a given tax regime. The regime decides
// whether an ISS rate may be declared at all, so tests that exercise pAliq need
// to pick one explicitly.
func workspaceRegime(t *testing.T, regime string) string {
	t.Helper()
	return workspaceFor(t, regime, testCertCNPJ)
}

// workspaceCNPJ builds a workspace whose provider is someone other than the
// test certificate's holder, for the checks that compare the two.
func workspaceCNPJ(t *testing.T, cnpj string) string {
	t.Helper()
	return workspaceFor(t, config.RegimeMEI, cnpj)
}

func workspaceFor(t *testing.T, regime, cnpj string) string {
	t.Helper()

	dir := t.TempDir()
	certPath := writeTestPFX(t, testCertPassword, time.Now().Add(300*24*time.Hour))

	data, err := os.ReadFile(certPath)
	if err != nil {
		t.Fatal(err)
	}
	localCert := filepath.Join(dir, "certificado.pfx")
	if err := os.WriteFile(localCert, data, 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := `
ambiente: producao-restrita
certificado:
  arquivo: ` + localCert + `
prestador:
  cnpj: "` + cnpj + `"
  nome: EMPRESA EXEMPLO LTDA
  regime_tributario: ` + regime + `
  inscricao_municipal: "1234567"
  municipio: "4106902"
dps:
  serie: "00001"
padroes:
  servico:
    codigo_tributacao_nacional: "010101"
    descricao: Desenvolvimento de software
  valores:
    iss_aliquota: 0
  tomador:
    nao_identificado: true
saida:
  diretorio: ` + filepath.Join(dir, "notas") + `
`
	if err := os.WriteFile(filepath.Join(dir, "nfse.yaml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

// runEmit executes `nfse emitir` with the given extra arguments.
func runEmit(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()
	t.Setenv(envCertPassword, testCertPassword)

	var out bytes.Buffer
	root := NewRootCommand()
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(append([]string{"emitir", "--config", filepath.Join(dir, "nfse.yaml")}, args...))

	err := root.Execute()
	return out.String(), err
}

// onlyXML returns the single XML file written under the workspace.
func onlyXML(t *testing.T, dir string) string {
	t.Helper()

	matches, err := filepath.Glob(filepath.Join(dir, "notas", "*.xml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Fatalf("esperava exatamente 1 XML gerado, encontrei %d", len(matches))
	}
	data, err := os.ReadFile(matches[0])
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func TestEmitir_MinimalInvoice(t *testing.T) {
	dir := workspace(t)

	out, err := runEmit(t, dir, "--numero", "42", "--valor", "1500.00",
		"--descricao", "Consultoria tecnica")
	if err != nil {
		t.Fatalf("emissao falhou: %v\n%s", err, out)
	}

	xmlStr := onlyXML(t, dir)
	doc := etree.NewDocument()
	if err := doc.ReadFromString(xmlStr); err != nil {
		t.Fatalf("XML gerado nao e parseavel: %v", err)
	}

	checks := map[string]string{
		"DPS/infDPS/tpAmb":                      "2",
		"DPS/infDPS/serie":                      "00001",
		"DPS/infDPS/nDPS":                       "42",
		"DPS/infDPS/cLocEmi":                    "4106902",
		"DPS/infDPS/prest/CNPJ":                 "12345678000195",
		"DPS/infDPS/serv/cServ/cTribNac":        "010101",
		"DPS/infDPS/serv/cServ/xDescServ":       "Consultoria tecnica",
		"DPS/infDPS/valores/vServPrest/vServ":   "1500.00",
		"DPS/infDPS/valores/trib/tribMun/pAliq": "",
	}
	for p, want := range checks {
		el := doc.FindElement(p)
		if want == "" {
			if el != nil {
				t.Errorf("%s deveria estar ausente, contem %q", p, el.Text())
			}
			continue
		}
		if el == nil {
			t.Errorf("%s ausente", p)
			continue
		}
		if el.Text() != want {
			t.Errorf("%s = %q, esperava %q", p, el.Text(), want)
		}
	}

	// The config default marks the taker as unidentified, so <toma> is omitted.
	if doc.FindElement("DPS/infDPS/toma") != nil {
		t.Error("<toma> nao deveria existir para tomador nao identificado")
	}
}

func TestEmitir_SignatureIsValid(t *testing.T) {
	dir := workspace(t)

	if out, err := runEmit(t, dir, "--numero", "1", "--valor", "100", "--descricao", "Servico"); err != nil {
		t.Fatalf("emissao falhou: %v\n%s", err, out)
	}

	result, err := xmlsigner.NewXMLVerifier().VerifyDPSSignature(onlyXML(t, dir))
	if err != nil {
		t.Fatalf("verificacao da assinatura falhou: %v", err)
	}
	if !result.Valid {
		t.Errorf("assinatura invalida: %v", result.Errors)
	}
}

func TestEmitir_FlagsOverrideConfigDefaults(t *testing.T) {
	// A ME/EPP with withheld ISSQN is the one case where declaring a rate is
	// allowed, so it is the only one where the override is observable.
	dir := workspaceRegime(t, config.RegimeMEEPP)

	// The config default sets iss_aliquota to 0; the flag must win.
	if out, err := runEmit(t, dir, "--numero", "7", "--valor", "1000",
		"--descricao", "Servico", "--retencao", "tomador", "--iss-aliquota", "2.5"); err != nil {
		t.Fatalf("emissao falhou: %v\n%s", err, out)
	}

	doc := etree.NewDocument()
	if err := doc.ReadFromString(onlyXML(t, dir)); err != nil {
		t.Fatal(err)
	}
	el := doc.FindElement("DPS/infDPS/valores/trib/tribMun/pAliq")
	if el == nil {
		t.Fatal("pAliq ausente apesar de --iss-aliquota")
	}
	if el.Text() != "2.50" {
		t.Errorf("pAliq = %q, esperava 2.50", el.Text())
	}
}

func TestEmitir_RetencaoAppearsInXML(t *testing.T) {
	dir := workspaceRegime(t, config.RegimeMEEPP)

	if out, err := runEmit(t, dir, "--numero", "8", "--valor", "1000",
		"--descricao", "Servico", "--retencao", "tomador", "--iss-aliquota", "3"); err != nil {
		t.Fatalf("emissao falhou: %v\n%s", err, out)
	}

	doc := etree.NewDocument()
	if err := doc.ReadFromString(onlyXML(t, dir)); err != nil {
		t.Fatal(err)
	}
	el := doc.FindElement("DPS/infDPS/valores/trib/tribMun/tpRetISSQN")
	if el == nil {
		t.Fatal("tpRetISSQN ausente")
	}
	if el.Text() != "2" {
		t.Errorf("tpRetISSQN = %q, esperava 2 (retido pelo tomador)", el.Text())
	}
}

// The rules below are the ones the Sefin applies on receipt. Catching them here
// is the difference between a message on the terminal and a rejected invoice.

func TestEmitir_RejectsISSRateFromMEI(t *testing.T) {
	dir := workspace(t)

	out, err := runEmit(t, dir, "--numero", "11", "--valor", "1000",
		"--descricao", "Servico", "--iss-aliquota", "2.5")
	if err == nil {
		t.Fatalf("esperava recusa: MEI nao informa aliquota\n%s", out)
	}
	if !strings.Contains(err.Error(), "E0600") {
		t.Errorf("a mensagem deveria citar a regra E0600: %v", err)
	}
}

func TestEmitir_RejectsISSRateWithoutRetencao(t *testing.T) {
	dir := workspaceRegime(t, config.RegimeMEEPP)

	out, err := runEmit(t, dir, "--numero", "12", "--valor", "1000",
		"--descricao", "Servico", "--iss-aliquota", "3")
	if err == nil {
		t.Fatalf("esperava recusa: ME/EPP sem retencao nao informa aliquota\n%s", out)
	}
	if !strings.Contains(err.Error(), "E0625") {
		t.Errorf("a mensagem deveria citar a regra E0625: %v", err)
	}
}

func TestEmitir_RejectsRetencaoWithoutISSRate(t *testing.T) {
	dir := workspaceRegime(t, config.RegimeMEEPP)

	out, err := runEmit(t, dir, "--numero", "13", "--valor", "1000",
		"--descricao", "Servico", "--retencao", "tomador")
	if err == nil {
		t.Fatalf("esperava recusa: com retencao a aliquota e obrigatoria\n%s", out)
	}
	if !strings.Contains(err.Error(), "E0621") {
		t.Errorf("a mensagem deveria citar a regra E0621: %v", err)
	}
}

func TestEmitir_RejectsUnknownRetencao(t *testing.T) {
	dir := workspaceRegime(t, config.RegimeMEEPP)

	out, err := runEmit(t, dir, "--numero", "14", "--valor", "1000",
		"--descricao", "Servico", "--retencao", "parcial")
	if err == nil {
		t.Fatalf("esperava recusa de um valor invalido de --retencao\n%s", out)
	}
	if !strings.Contains(err.Error(), "retencao_issqn") {
		t.Errorf("a mensagem deveria apontar o campo: %v", err)
	}
}

func TestEmitir_IdentifiedTakerReplacesDefault(t *testing.T) {
	dir := workspace(t)

	// A taker given on the command line must fully replace the config's
	// "nao_identificado" default, not merge into it.
	if out, err := runEmit(t, dir, "--numero", "8", "--valor", "1000", "--descricao", "Servico",
		"--tomador-cnpj", "98765432000198", "--tomador-nome", "CLIENTE EXEMPLO SA"); err != nil {
		t.Fatalf("emissao falhou: %v\n%s", err, out)
	}

	doc := etree.NewDocument()
	if err := doc.ReadFromString(onlyXML(t, dir)); err != nil {
		t.Fatal(err)
	}
	if el := doc.FindElement("DPS/infDPS/toma/CNPJ"); el == nil || el.Text() != "98765432000198" {
		t.Errorf("CNPJ do tomador ausente ou incorreto: %v", el)
	}
}

func TestEmitir_NotaFileLayersOverDefaults(t *testing.T) {
	dir := workspace(t)

	notaPath := filepath.Join(dir, "nota.yaml")
	nota := `
numero: "43"
competencia: "2026-08-01"
servico:
  descricao: Manutencao de sistema
valores:
  valor_servico: 2400.50
  desconto_incondicionado: 200.00
`
	if err := os.WriteFile(notaPath, []byte(nota), 0o644); err != nil {
		t.Fatal(err)
	}

	if out, err := runEmit(t, dir, "--yaml", notaPath); err != nil {
		t.Fatalf("emissao falhou: %v\n%s", err, out)
	}

	doc := etree.NewDocument()
	if err := doc.ReadFromString(onlyXML(t, dir)); err != nil {
		t.Fatal(err)
	}
	for p, want := range map[string]string{
		"DPS/infDPS/nDPS":    "43",
		"DPS/infDPS/dCompet": "2026-08-01",
		"DPS/infDPS/valores/vDescCondIncond/vDescIncond": "200.00",
		// cTribNac is not in the nota file; it comes from the config defaults.
		"DPS/infDPS/serv/cServ/cTribNac": "010101",
	} {
		el := doc.FindElement(p)
		if el == nil {
			t.Errorf("%s ausente", p)
			continue
		}
		if el.Text() != want {
			t.Errorf("%s = %q, esperava %q", p, el.Text(), want)
		}
	}
}

func TestEmitir_SemAssinar(t *testing.T) {
	dir := workspace(t)

	if out, err := runEmit(t, dir, "--numero", "9", "--valor", "100",
		"--descricao", "Servico", "--sem-assinar"); err != nil {
		t.Fatalf("emissao falhou: %v\n%s", err, out)
	}

	matches, _ := filepath.Glob(filepath.Join(dir, "notas", "*-sem-assinatura.xml"))
	if len(matches) != 1 {
		t.Fatalf("esperava um arquivo marcado como sem assinatura, encontrei %d", len(matches))
	}
	if strings.Contains(onlyXML(t, dir), "<Signature") {
		t.Error("o XML nao deveria conter assinatura")
	}
}

func TestEmitir_ReportsMissingFields(t *testing.T) {
	dir := workspace(t)

	out, err := runEmit(t, dir)
	if err == nil {
		t.Fatalf("esperava erro por falta de dados obrigatorios\n%s", out)
	}

	// The message names what is actually missing. The DPS number is not among
	// them — it comes from the series counter — and neither is the description,
	// which this workspace supplies through the config defaults.
	if !strings.Contains(err.Error(), "valor_servico") {
		t.Errorf("a mensagem de erro nao menciona o campo que falta: %v", err)
	}
	if strings.Contains(err.Error(), "numero") {
		t.Errorf("o numero deveria vir do contador, nao ser cobrado: %v", err)
	}
}

func TestEmitir_RejectsUnknownConfigField(t *testing.T) {
	dir := t.TempDir()
	cfg := "prestador:\n  cnpj: \"123\"\n  aliquota_iss: 2\n"
	if err := os.WriteFile(filepath.Join(dir, "nfse.yaml"), []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	// A typo in a fiscal config must fail loudly rather than be ignored.
	_, err := runEmit(t, dir, "--numero", "1", "--valor", "10", "--descricao", "x")
	if err == nil {
		t.Fatal("esperava erro para campo desconhecido na configuracao")
	}
	if !strings.Contains(err.Error(), "aliquota_iss") {
		t.Errorf("o erro deveria apontar o campo desconhecido: %v", err)
	}
}

// TestEmitir_RefusesSilentOverwrite pins that a repeated DPS number cannot
// destroy an earlier document. The file name comes from the DPS identifier,
// which repeats whenever a series and number are reused; overwriting silently
// would throw away a signed declaration — or, after transmission, the only
// local copy of an invoice that exists at the government.
func TestEmitir_RefusesSilentOverwrite(t *testing.T) {
	dir := workspace(t)

	if out, err := runEmit(t, dir, "--numero", "1", "--valor", "100", "--descricao", "Primeira"); err != nil {
		t.Fatalf("primeira emissao falhou: %v\n%s", err, out)
	}

	_, err := runEmit(t, dir, "--numero", "1", "--valor", "999", "--descricao", "Segunda")
	if err == nil {
		t.Fatal("a segunda emissao com o mesmo numero sobrescreveu a primeira em silencio")
	}
	if !strings.Contains(err.Error(), "ja existe") {
		t.Errorf("a mensagem deveria explicar a colisao: %v", err)
	}

	// The original must be untouched.
	xmlStr := onlyXML(t, dir)
	if !strings.Contains(xmlStr, "Primeira") {
		t.Error("o arquivo original foi alterado")
	}

	// And --sobrescrever must still allow it deliberately.
	if out, err := runEmit(t, dir, "--numero", "1", "--valor", "999",
		"--descricao", "Segunda", "--sobrescrever"); err != nil {
		t.Fatalf("--sobrescrever deveria permitir a substituicao: %v\n%s", err, out)
	}
	if !strings.Contains(onlyXML(t, dir), "Segunda") {
		t.Error("--sobrescrever nao substituiu o arquivo")
	}
}

// TestEmitir_NumeracaoAutomatica covers the counter that removes "which number
// am I on?" from the user's head. Repeating a number is not merely
// inconvenient: it collides on the DPS identifier, and the government rejects
// the second one.
func TestEmitir_NumeracaoAutomatica(t *testing.T) {
	dir := workspace(t)

	numeroDe := func(t *testing.T, saida string) string {
		t.Helper()
		// The DPS identifier ends with the 15-digit, zero-padded number.
		for _, linha := range strings.Split(saida, "\n") {
			if strings.HasPrefix(linha, "DPS DPS") {
				id := strings.TrimPrefix(linha, "DPS ")
				return strings.TrimLeft(id[len(id)-15:], "0")
			}
		}
		t.Fatalf("nao encontrei o identificador na saida:\n%s", saida)
		return ""
	}

	primeira, err := runEmit(t, dir, "--valor", "100", "--descricao", "Primeira")
	if err != nil {
		t.Fatalf("primeira emissao falhou: %v\n%s", err, primeira)
	}
	if got := numeroDe(t, primeira); got != "1" {
		t.Errorf("primeiro numero = %q, esperava 1", got)
	}

	segunda, err := runEmit(t, dir, "--valor", "200", "--descricao", "Segunda")
	if err != nil {
		t.Fatalf("segunda emissao falhou: %v\n%s", err, segunda)
	}
	if got := numeroDe(t, segunda); got != "2" {
		t.Errorf("segundo numero = %q, esperava 2", got)
	}

	// An explicit number is still honoured, and advances the counter past it.
	explicita, err := runEmit(t, dir, "--numero", "10", "--valor", "300", "--descricao", "Explicita")
	if err != nil {
		t.Fatalf("emissao explicita falhou: %v\n%s", err, explicita)
	}

	seguinte, err := runEmit(t, dir, "--valor", "400", "--descricao", "Seguinte")
	if err != nil {
		t.Fatalf("emissao seguinte falhou: %v\n%s", err, seguinte)
	}
	if got := numeroDe(t, seguinte); got != "11" {
		t.Errorf("numero apos o explicito = %q, esperava 11", got)
	}

	// Filling a gap below the counter must not drag it backwards, or the next
	// automatic number would collide with one already used.
	if _, err := runEmit(t, dir, "--numero", "5", "--valor", "500", "--descricao", "Lacuna"); err != nil {
		t.Fatalf("emissao de lacuna falhou: %v", err)
	}
	depois, err := runEmit(t, dir, "--valor", "600", "--descricao", "Depois da lacuna")
	if err != nil {
		t.Fatalf("emissao apos lacuna falhou: %v\n%s", err, depois)
	}
	if got := numeroDe(t, depois); got != "12" {
		t.Errorf("numero apos preencher lacuna = %q, esperava 12", got)
	}
}

// TestEmitir_ContadorNaoAvancaSemArquivo pins that a failed write leaves the
// counter alone: burning a number on a document that does not exist would
// create a permanent gap in the numbering for no reason.
func TestEmitir_ContadorNaoAvancaSemArquivo(t *testing.T) {
	dir := workspace(t)

	if _, err := runEmit(t, dir, "--numero", "1", "--valor", "100", "--descricao", "Primeira"); err != nil {
		t.Fatal(err)
	}
	// Colliding on purpose: the write is refused.
	if _, err := runEmit(t, dir, "--numero", "1", "--valor", "100", "--descricao", "Colide"); err == nil {
		t.Fatal("esperava erro de colisao")
	}

	proxima, err := runEmit(t, dir, "--valor", "100", "--descricao", "Proxima")
	if err != nil {
		t.Fatalf("emissao seguinte falhou: %v\n%s", err, proxima)
	}
	if !strings.Contains(proxima, strings.Repeat("0", 14)+"2") {
		t.Errorf("o contador deveria estar em 2:\n%s", proxima)
	}
}

// A substituição não é um evento: é uma DPS nova que aponta para a nota que
// ela troca. O elemento tem de sair no XML com a chave e o código do motivo.
func TestEmitir_SubstituicaoSaiNoXML(t *testing.T) {
	dir := workspace(t)
	const substituida = "41069022212345678000195000000000000126081234567890"

	if out, err := runEmit(t, dir, "--numero", "31", "--valor", "1000",
		"--descricao", "Servico", "--substitui", substituida,
		"--motivo", "saiu-do-simples", "--motivo-texto", "Desenquadramento em agosto"); err != nil {
		t.Fatalf("emissao falhou: %v\n%s", err, out)
	}

	doc := etree.NewDocument()
	if err := doc.ReadFromString(onlyXML(t, dir)); err != nil {
		t.Fatal(err)
	}

	casos := map[string]string{
		"DPS/infDPS/subst/chSubstda": substituida,
		"DPS/infDPS/subst/cMotivo":   "01",
		"DPS/infDPS/subst/xMotivo":   "Desenquadramento em agosto",
	}
	for caminho, querido := range casos {
		el := doc.FindElement(caminho)
		if el == nil {
			t.Fatalf("%s ausente no XML gerado", caminho)
		}
		if el.Text() != querido {
			t.Errorf("%s = %q, esperava %q", caminho, el.Text(), querido)
		}
	}
}

// Uma emissão comum não pode carregar subst: o elemento é opcional no schema e
// a presença dele diz ao governo que outra nota está sendo trocada.
func TestEmitir_SemSubstituicaoNaoEmiteOElemento(t *testing.T) {
	dir := workspace(t)

	if out, err := runEmit(t, dir, "--numero", "32", "--valor", "1000",
		"--descricao", "Servico"); err != nil {
		t.Fatalf("emissao falhou: %v\n%s", err, out)
	}

	doc := etree.NewDocument()
	if err := doc.ReadFromString(onlyXML(t, dir)); err != nil {
		t.Fatal(err)
	}
	if el := doc.FindElement("DPS/infDPS/subst"); el != nil {
		t.Errorf("subst nao deveria existir numa emissao comum")
	}
}

func TestEmitir_SubstituicaoRecusas(t *testing.T) {
	const chaveValida = "41069022212345678000195000000000000126081234567890"

	tests := []struct {
		name     string
		args     []string
		wantText string
	}{
		{
			name:     "chave sem motivo",
			args:     []string{"--substitui", chaveValida},
			wantText: "informe --motivo junto com --substitui",
		},
		{
			name:     "motivo sem chave",
			args:     []string{"--motivo", "outros"},
			wantText: "--motivo so vale com --substitui",
		},
		{
			// O motivo do cancelamento usa outro conjunto de codigos; mandar
			// um deles aqui produziria uma DPS que o schema recusa.
			name:     "motivo do cancelamento",
			args:     []string{"--substitui", chaveValida, "--motivo", "erro-emissao"},
			wantText: "nao e um motivo de substituicao",
		},
		{
			name:     "chave curta",
			args:     []string{"--substitui", "123", "--motivo", "outros"},
			wantText: "exatamente 50 digitos",
		},
		{
			name:     "chave com letra",
			args:     []string{"--substitui", "4106902221234567800019500000000000012608123456789X", "--motivo", "outros"},
			wantText: "apenas digitos",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := workspace(t)
			args := append([]string{"--numero", "40", "--valor", "1000", "--descricao", "Servico"}, tt.args...)

			out, err := runEmit(t, dir, args...)
			if err == nil {
				t.Fatalf("esperava erro\n%s", out)
			}
			if !strings.Contains(err.Error(), tt.wantText) {
				t.Errorf("erro = %q, esperava conter %q", err, tt.wantText)
			}
		})
	}
}
