package brasilapi

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// contratoHabilitado gates the tests that talk to the real service. They are
// off by default: the suite must pass offline and in CI, and a public service
// that rate-limits is no place for a test that runs on every push.
//
//	NFSE_TESTE_CONTRATO=1 go test ./internal/infrastructure/brasilapi/ -run Contrato -v
func contratoHabilitado(t *testing.T) {
	t.Helper()
	if os.Getenv("NFSE_TESTE_CONTRATO") != "1" {
		t.Skip("contrato com o servico real: defina NFSE_TESTE_CONTRATO=1 para rodar")
	}
}

// The fields onboard reads were taken from the service's documentation, never
// from a real answer (issue #12). This holds the decoder to the live service.
//
// Banco do Brasil (00.000.000/0001-91) stands in for "a company everyone can
// look up": public, stable, headquartered in Brasília, and not a person. Its
// answer is checked field by field, so a renamed field or a code for the
// wrong municipality fails here instead of in someone's invoice.
func TestContratoConsultarCNPJ(t *testing.T) {
	contratoHabilitado(t)

	empresa := consultar(t, "00000000000191")

	if strings.TrimSpace(empresa.RazaoSocial) == "" {
		t.Error("razao_social veio vazia")
	}
	if !empresa.Ativa() {
		t.Errorf("descricao_situacao_cadastral = %q, esperava ATIVA", empresa.DescricaoSituacaoCadastral)
	}
	// The dangerous one: it must be the IBGE code, not the four-digit TOM
	// code in codigo_municipio, and it must be Brasília's.
	codigo, err := empresa.MunicipioIBGE()
	if err != nil {
		t.Errorf("codigo_municipio_ibge = %q: %v", empresa.CodigoMunicipioIBGE, err)
	}
	if codigo != "5300108" || !strings.EqualFold(empresa.UF, "DF") {
		t.Errorf("municipio = %s/%s, esperava 5300108/DF", codigo, empresa.UF)
	}
	if empresa.CNAEFiscal == "" {
		t.Error("cnae_fiscal veio vazio")
	}
	// A bank can opt for neither regime. Whether the registry says so with
	// false or with null is what this log records; true would mean the fields
	// are being read from the wrong place.
	if opcao(empresa.OpcaoPeloMEI) == "true" || opcao(empresa.OpcaoPeloSimples) == "true" {
		t.Errorf("opcao_pelo_mei=%s opcao_pelo_simples=%s para um banco",
			opcao(empresa.OpcaoPeloMEI), opcao(empresa.OpcaoPeloSimples))
	}

	t.Logf("municipio=%q uf=%q codigo_municipio_ibge=%s situacao=%q "+
		"opcao_pelo_mei=%s opcao_pelo_simples=%s cnae_fiscal=%s",
		empresa.Municipio, empresa.UF, codigo, empresa.DescricaoSituacaoCadastral,
		opcao(empresa.OpcaoPeloMEI), opcao(empresa.OpcaoPeloSimples), empresa.CNAEFiscal)
}

// The MEI flags decide regApTribSN, and a bank cannot show them set.
// NFSE_TESTE_CNPJ_MEI names an active MEI the person running the test
// chooses. A MEI's registered name is its owner's full name, often followed by
// the owner's CPF, so nothing from that answer but the flags is logged — and
// the variable is never set in CI, where logs are public.
func TestContratoOpcoesDeUmMEI(t *testing.T) {
	contratoHabilitado(t)

	cnpj := os.Getenv("NFSE_TESTE_CNPJ_MEI")
	if cnpj == "" {
		t.Skip("defina NFSE_TESTE_CNPJ_MEI com o CNPJ de um MEI ativo para conferir as opcoes")
	}

	empresa := consultar(t, cnpj)
	if opcao(empresa.OpcaoPeloMEI) != "true" || opcao(empresa.OpcaoPeloSimples) != "true" {
		t.Errorf("opcao_pelo_mei=%s opcao_pelo_simples=%s; um MEI ativo tem as duas",
			opcao(empresa.OpcaoPeloMEI), opcao(empresa.OpcaoPeloSimples))
	}
	if _, err := empresa.MunicipioIBGE(); err != nil {
		t.Errorf("codigo_municipio_ibge: %v", err)
	}
}

func consultar(t *testing.T, cnpj string) *Empresa {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	empresa, err := New(Config{UserAgent: "nfse-emissor-go/teste-de-contrato"}).ConsultarCNPJ(ctx, cnpj)
	if err != nil {
		t.Fatalf("consulta falhou: %v", err)
	}
	return empresa
}

// A CNPJ with valid check digits that the Receita never issued must come back
// as "not found", not as an empty company.
//
// 11.111.111/0001-91, the number test suites everywhere use, turned out to be
// a real registration, and a person's. The base 99.999.999 is far past the
// numeric bases issued so far. If it ever exists, the test says so without
// printing whose it is: a CNPJ's registered name can be a person's.
func TestContratoCNPJInexistente(t *testing.T) {
	contratoHabilitado(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	const cnpj = "99999999000191"
	_, err := New(Config{}).ConsultarCNPJ(ctx, cnpj)
	if err == nil {
		t.Fatalf("%s existe no cadastro; escolha outro numero para este teste", cnpj)
	}
	if !strings.Contains(err.Error(), "nao encontrado") {
		t.Errorf("erro = %v; esperava a mensagem de CNPJ nao encontrado", err)
	}
}

func opcao(v *bool) string {
	switch {
	case v == nil:
		return "null"
	case *v:
		return "true"
	}
	return "false"
}
