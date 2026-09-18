package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/beevik/etree"

	"github.com/edusouza/nfse-emissor-go/internal/infrastructure/xmlsigner"
)

const testCertPassword = "senha-de-teste"

// workspace prepares a directory containing a valid nfse.yaml and a throwaway
// certificate, and returns its path.
func workspace(t *testing.T) string {
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
  cnpj: "12345678000195"
  nome: EMPRESA EXEMPLO LTDA
  regime_tributario: mei
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
	dir := workspace(t)

	// The config default sets iss_aliquota to 0; the flag must win.
	if out, err := runEmit(t, dir, "--numero", "7", "--valor", "1000",
		"--descricao", "Servico", "--iss-aliquota", "2.5"); err != nil {
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

	// The message must name every missing field at once, so the user is not
	// forced to rerun the command once per problem.
	for _, want := range []string{"numero", "valor_servico"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("a mensagem de erro nao menciona %q: %v", want, err)
		}
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
