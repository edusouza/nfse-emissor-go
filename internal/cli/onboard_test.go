package cli

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const (
	onboardCNPJ     = "12345678000195"
	onboardSubject  = "EMPRESA TESTE LTDA:" + onboardCNPJ
	onboardPassword = "senha-de-teste"
)

// registroFake serves the CNPJ registry answer the command expects.
func registroFake(t *testing.T, status int, body string) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

const respostaMEI = `{
	"cnpj": "12345678000195",
	"razao_social": "EMPRESA TESTE LTDA",
	"municipio": "SAO PAULO",
	"uf": "SP",
	"codigo_municipio_ibge": 3550308,
	"descricao_situacao_cadastral": "ATIVA",
	"opcao_pelo_mei": true,
	"opcao_pelo_simples": true
}`

// runOnboard executes the command and returns its combined output.
func runOnboard(t *testing.T, args ...string) (string, error) {
	t.Helper()

	var out bytes.Buffer
	root := NewRootCommand()
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(append([]string{"onboard"}, args...))

	err := root.Execute()
	return out.String(), err
}

func TestOnboardCertificadoMaisConsulta(t *testing.T) {
	srv := registroFake(t, http.StatusOK, respostaMEI)
	cert := writeTestPFXSubject(t, onboardSubject, onboardPassword, time.Now().Add(300*24*time.Hour))
	path := filepath.Join(t.TempDir(), "nfse.yaml")

	out, err := runOnboard(t,
		"--certificado", cert, "--senha", onboardPassword,
		"--fonte", srv.URL, "--arquivo", path)
	if err != nil {
		t.Fatalf("onboard falhou: %v\n%s", err, out)
	}

	// The CNPJ leaves the machine, so the command says so before it happens.
	if !strings.Contains(out, "Consultando o CNPJ 12.345.678/0001-95") {
		t.Errorf("a saida nao anuncia a consulta:\n%s", out)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("arquivo nao foi criado: %v", err)
	}
	gerado := string(data)

	for _, want := range []string{
		`cnpj: "12345678000195"`,
		`nome: "EMPRESA TESTE LTDA"`,
		`regime_tributario: "mei"`,
		`municipio: "3550308"`,
		`arquivo: "` + cert + `"`,
	} {
		if !strings.Contains(gerado, want) {
			t.Errorf("faltou %q no arquivo gerado:\n%s", want, gerado)
		}
	}

	// The whole point of the command is that the next one already passes.
	var check bytes.Buffer
	root := NewRootCommand()
	root.SetOut(&check)
	root.SetErr(&check)
	root.SetArgs([]string{"config", "check", "--arquivo", path})
	if err := root.Execute(); err != nil {
		t.Fatalf("config check recusou o arquivo gerado: %v\n%s", err, check.String())
	}
}

// Without the lookup the certificate alone still answers two of the four
// fields, and the command says which ones are left.
func TestOnboardSemRede(t *testing.T) {
	cert := writeTestPFXSubject(t, onboardSubject, onboardPassword, time.Now().Add(300*24*time.Hour))
	path := filepath.Join(t.TempDir(), "nfse.yaml")

	out, err := runOnboard(t,
		"--certificado", cert, "--senha", onboardPassword,
		"--sem-rede", "--arquivo", path)
	if err != nil {
		t.Fatalf("onboard falhou: %v\n%s", err, out)
	}

	if strings.Contains(out, "Consultando") {
		t.Errorf("--sem-rede nao deveria consultar:\n%s", out)
	}

	gerado, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("arquivo nao foi criado: %v", err)
	}
	if !strings.Contains(string(gerado), `cnpj: "12345678000195"`) {
		t.Errorf("o CNPJ do certificado nao foi aproveitado:\n%s", gerado)
	}
	for _, want := range []string{"prestador.municipio", "prestador.regime_tributario"} {
		if !strings.Contains(out, want) {
			t.Errorf("a saida nao lista %q como pendente:\n%s", want, out)
		}
	}
}

// A registry outage is not a reason to send the user away empty-handed.
func TestOnboardConsultaIndisponivelAindaGeraArquivo(t *testing.T) {
	srv := registroFake(t, http.StatusBadGateway, "")
	cert := writeTestPFXSubject(t, onboardSubject, onboardPassword, time.Now().Add(300*24*time.Hour))
	path := filepath.Join(t.TempDir(), "nfse.yaml")

	out, err := runOnboard(t,
		"--certificado", cert, "--senha", onboardPassword,
		"--fonte", srv.URL, "--arquivo", path)
	if err != nil {
		t.Fatalf("onboard deveria continuar apos a falha da consulta: %v\n%s", err, out)
	}
	if !strings.Contains(out, "aviso: a consulta falhou") {
		t.Errorf("a falha da consulta nao foi reportada:\n%s", out)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("arquivo nao foi criado: %v", err)
	}
}

// The registry answers "not a MEI" and says nothing about the Simples: the
// regime stays blank instead of being guessed.
func TestOnboardRegimeDesconhecido(t *testing.T) {
	srv := registroFake(t, http.StatusOK, `{
		"razao_social": "EMPRESA TESTE LTDA",
		"codigo_municipio_ibge": 3550308,
		"descricao_situacao_cadastral": "ATIVA",
		"opcao_pelo_mei": false,
		"opcao_pelo_simples": null
	}`)
	path := filepath.Join(t.TempDir(), "nfse.yaml")

	out, err := runOnboard(t, "--cnpj", onboardCNPJ, "--fonte", srv.URL, "--arquivo", path)
	if err != nil {
		t.Fatalf("onboard falhou: %v\n%s", err, out)
	}

	gerado, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(gerado), `regime_tributario: ""`) {
		t.Errorf("o regime nao deveria ser adivinhado:\n%s", gerado)
	}
	if !strings.Contains(out, "opcao pelo Simples Nacional") {
		t.Errorf("a saida nao explica por que o regime ficou vazio:\n%s", out)
	}
	if !strings.Contains(out, "certificado.arquivo") {
		t.Errorf("sem --certificado, o caminho do arquivo deveria ficar pendente:\n%s", out)
	}
}

func TestOnboardSituacaoCadastralInativa(t *testing.T) {
	srv := registroFake(t, http.StatusOK, `{
		"razao_social": "EMPRESA TESTE LTDA",
		"codigo_municipio_ibge": 3550308,
		"descricao_situacao_cadastral": "BAIXADA",
		"opcao_pelo_mei": true
	}`)
	path := filepath.Join(t.TempDir(), "nfse.yaml")

	out, err := runOnboard(t, "--cnpj", onboardCNPJ, "--fonte", srv.URL, "--arquivo", path)
	if err != nil {
		t.Fatalf("onboard falhou: %v\n%s", err, out)
	}
	if !strings.Contains(out, "nao esta ATIVA") {
		t.Errorf("a situacao cadastral nao foi sinalizada:\n%s", out)
	}
}

func TestOnboardRecusas(t *testing.T) {
	cert := writeTestPFXSubject(t, onboardSubject, onboardPassword, time.Now().Add(300*24*time.Hour))
	dir := t.TempDir()

	existente := filepath.Join(dir, "existe.yaml")
	if err := os.WriteFile(existente, []byte("ambiente: producao\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		args     []string
		wantText string
	}{
		{
			name:     "sem certificado e sem cnpj",
			args:     []string{"--arquivo", filepath.Join(dir, "a.yaml"), "--sem-rede"},
			wantText: "informe --certificado",
		},
		{
			name: "cnpj diferente do certificado",
			args: []string{
				"--certificado", cert, "--senha", onboardPassword,
				"--cnpj", "19131243000197", "--sem-rede",
				"--arquivo", filepath.Join(dir, "b.yaml"),
			},
			wantText: "a Sefin so aceita a DPS assinada pelo certificado",
		},
		{
			name:     "cnpj invalido",
			args:     []string{"--cnpj", "12345678000100", "--sem-rede", "--arquivo", filepath.Join(dir, "c.yaml")},
			wantText: "digitos verificadores invalidos",
		},
		{
			name:     "ambiente inexistente",
			args:     []string{"--cnpj", onboardCNPJ, "--sem-rede", "--ambiente", "homologacao", "--arquivo", filepath.Join(dir, "e.yaml")},
			wantText: "--ambiente \"homologacao\" e invalido",
		},
		{
			name:     "arquivo ja existe",
			args:     []string{"--cnpj", onboardCNPJ, "--sem-rede", "--arquivo", existente},
			wantText: "use --forcar",
		},
		{
			name: "certificado sem CNPJ no titular",
			args: []string{
				"--certificado", writeTestPFXSubject(t, "EMPRESA SEM CNPJ", onboardPassword, time.Now().Add(300*24*time.Hour)),
				"--senha", onboardPassword, "--sem-rede",
				"--arquivo", filepath.Join(dir, "d.yaml"),
			},
			wantText: "informe --cnpj",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out, err := runOnboard(t, tt.args...)
			if err == nil {
				t.Fatalf("esperava erro\n%s", out)
			}
			if !strings.Contains(err.Error(), tt.wantText) {
				t.Errorf("erro = %q, esperava conter %q", err, tt.wantText)
			}
		})
	}

	// The refused file must be the one that was already there.
	data, err := os.ReadFile(existente)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "ambiente: producao\n" {
		t.Errorf("o arquivo existente foi sobrescrito: %q", data)
	}
}

func TestOnboardForcarSobrescreve(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nfse.yaml")
	if err := os.WriteFile(path, []byte("ambiente: producao\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := runOnboard(t, "--cnpj", onboardCNPJ, "--sem-rede", "--forcar", "--arquivo", path)
	if err != nil {
		t.Fatalf("onboard falhou: %v\n%s", err, out)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `cnpj: "12345678000195"`) {
		t.Errorf("o arquivo nao foi regravado:\n%s", data)
	}
}

func TestOnboardSerieInvalida(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nfse.yaml")

	out, err := runOnboard(t, "--cnpj", onboardCNPJ, "--sem-rede", "--serie", "1", "--arquivo", path)
	if err == nil {
		t.Fatalf("esperava erro\n%s", out)
	}
	if !strings.Contains(err.Error(), "5 digitos") {
		t.Errorf("erro = %q", err)
	}
	if _, statErr := os.Stat(path); statErr == nil {
		t.Error("nao deveria ter gravado o arquivo")
	}
}

// The certificate and the registry both answer prestador.nome; the header must
// credit the value that ended up in the file, once.
func TestOnboardProcedenciaNaoDuplica(t *testing.T) {
	srv := registroFake(t, http.StatusOK, respostaMEI)
	cert := writeTestPFXSubject(t, onboardSubject, onboardPassword, time.Now().Add(300*24*time.Hour))
	path := filepath.Join(t.TempDir(), "nfse.yaml")

	if out, err := runOnboard(t,
		"--certificado", cert, "--senha", onboardPassword,
		"--fonte", srv.URL, "--arquivo", path); err != nil {
		t.Fatalf("onboard falhou: %v\n%s", err, out)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(string(data), "#   prestador.nome:"); n != 1 {
		t.Errorf("prestador.nome aparece %d vezes no cabecalho, esperava 1:\n%s", n, data)
	}
}

// respostaTI is the registry answer for a software provider: the CNAE is the
// only thing it says about what the company does.
const respostaTI = `{
	"cnpj": "12345678000195",
	"razao_social": "EMPRESA TESTE LTDA",
	"municipio": "CURITIBA",
	"uf": "PR",
	"codigo_municipio_ibge": 4106902,
	"descricao_situacao_cadastral": "ATIVA",
	"cnae_fiscal": 6209100,
	"cnae_fiscal_descricao": "Suporte técnico, manutenção e outros serviços em tecnologia da informação",
	"opcao_pelo_mei": true,
	"opcao_pelo_simples": true
}`

// The CNAE cannot decide the service code, but it can rank candidates. They go
// into the file commented out, and the field stays empty: a wrong cTribNac
// would ride on every invoice.
func TestOnboardSugereServicoPeloCNAE(t *testing.T) {
	srv := registroFake(t, http.StatusOK, respostaTI)
	path := filepath.Join(t.TempDir(), "nfse.yaml")

	out, err := runOnboard(t, "--cnpj", onboardCNPJ, "--fonte", srv.URL, "--arquivo", path)
	if err != nil {
		t.Fatalf("onboard falhou: %v\n%s", err, out)
	}

	if !strings.Contains(out, "010701") {
		t.Errorf("a saida nao sugere 010701 para o CNAE 6209-1/00:\n%s", out)
	}
	if !strings.Contains(out, "nao um mapeamento oficial") {
		t.Errorf("a saida apresenta a sugestao sem dizer que e palpite:\n%s", out)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	gerado := string(data)

	if !strings.Contains(gerado, `codigo_tributacao_nacional: ""`) {
		t.Errorf("a sugestao nao pode preencher o campo:\n%s", gerado)
	}
	for _, want := range []string{"#   010701", "CNAE 6209-1/00", "nfse servico buscar"} {
		if !strings.Contains(gerado, want) {
			t.Errorf("faltou %q no arquivo gerado:\n%s", want, gerado)
		}
	}
}

// Knowing the code is the normal case once the user has looked it up, and then
// nothing is left to guess.
func TestOnboardComServicoInformado(t *testing.T) {
	srv := registroFake(t, http.StatusOK, respostaTI)
	path := filepath.Join(t.TempDir(), "nfse.yaml")

	// Without the leading zero, which is how a spreadsheet shows it.
	out, err := runOnboard(t, "--cnpj", onboardCNPJ, "--servico", "10701",
		"--fonte", srv.URL, "--arquivo", path)
	if err != nil {
		t.Fatalf("onboard falhou: %v\n%s", err, out)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	gerado := string(data)

	if !strings.Contains(gerado, `codigo_tributacao_nacional: "010701"`) {
		t.Errorf("o codigo informado nao foi gravado:\n%s", gerado)
	}
	if !strings.Contains(gerado, "Suporte") {
		t.Errorf("o arquivo nao diz o que o codigo significa:\n%s", gerado)
	}
	if !strings.Contains(gerado, "padroes.servico.codigo_tributacao_nacional: --servico") {
		t.Errorf("a origem do codigo nao foi registrada no cabecalho:\n%s", gerado)
	}
	if strings.Contains(out, "Falta preencher") &&
		strings.Contains(out, "codigo_tributacao_nacional — 6 digitos") {
		t.Errorf("o codigo informado nao deveria continuar pendente:\n%s", out)
	}
}

func TestOnboardRecusaServicoForaDaLista(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nfse.yaml")

	out, err := runOnboard(t, "--cnpj", onboardCNPJ, "--sem-rede",
		"--servico", "999999", "--arquivo", path)
	if err == nil {
		t.Fatalf("esperava erro para um codigo fora da lista\n%s", out)
	}
	if !strings.Contains(err.Error(), "nao esta na lista nacional") {
		t.Errorf("erro = %q", err)
	}
	if _, statErr := os.Stat(path); statErr == nil {
		t.Error("o arquivo nao deveria ter sido criado")
	}
}

// Without the lookup there is no CNAE, so there is nothing to rank — and the
// user gets the command that searches instead of a silence.
func TestOnboardSemRedeApontaABusca(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nfse.yaml")

	out, err := runOnboard(t, "--cnpj", onboardCNPJ, "--sem-rede", "--arquivo", path)
	if err != nil {
		t.Fatalf("onboard falhou: %v\n%s", err, out)
	}
	if !strings.Contains(out, "nfse servico buscar") {
		t.Errorf("a saida nao diz como achar o codigo:\n%s", out)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "Candidatos a partir do CNAE") {
		t.Errorf("sem consulta nao ha CNAE para sugerir de:\n%s", data)
	}
}
