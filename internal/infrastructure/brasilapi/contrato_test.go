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
// look up": it is public, stable and not a person. Its answer says nothing
// about the MEI flags of an actual MEI; NFSE_TESTE_CNPJ_MEI checks those
// against a CNPJ the person running the test chooses, and is never committed.
func TestContratoConsultarCNPJ(t *testing.T) {
	contratoHabilitado(t)

	casos := map[string]string{"banco do brasil": "00000000000191"}
	if mei := os.Getenv("NFSE_TESTE_CNPJ_MEI"); mei != "" {
		casos["MEI informado"] = mei
	}

	client := New(Config{UserAgent: "nfse-emissor-go/teste-de-contrato"})
	for nome, cnpj := range casos {
		t.Run(nome, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			empresa, err := client.ConsultarCNPJ(ctx, cnpj)
			if err != nil {
				t.Fatalf("consulta falhou: %v", err)
			}

			if strings.TrimSpace(empresa.RazaoSocial) == "" {
				t.Error("razao_social veio vazia")
			}
			if empresa.DescricaoSituacaoCadastral == "" {
				t.Error("descricao_situacao_cadastral veio vazia")
			}
			// The dangerous one: a wrong code sends invoices to the wrong
			// municipality. It must be the IBGE code, not the TOM code, and
			// agree with the state the same answer names.
			codigo, err := empresa.MunicipioIBGE()
			if err != nil {
				t.Errorf("codigo_municipio_ibge = %q: %v", empresa.CodigoMunicipioIBGE, err)
			}

			t.Logf("razao_social=%q municipio=%q uf=%q codigo_municipio_ibge=%s situacao=%q "+
				"opcao_pelo_mei=%s opcao_pelo_simples=%s cnae_fiscal=%s",
				empresa.RazaoSocial, empresa.Municipio, empresa.UF, codigo,
				empresa.DescricaoSituacaoCadastral,
				opcao(empresa.OpcaoPeloMEI), opcao(empresa.OpcaoPeloSimples), empresa.CNAEFiscal)
		})
	}
}

// A CNPJ with valid check digits that the Receita never issued must come back
// as "not found", not as an empty company.
func TestContratoCNPJInexistente(t *testing.T) {
	contratoHabilitado(t)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 11.111.111/0001-91 is the number test suites everywhere use, precisely
	// because the Receita never issued it.
	_, err := New(Config{}).ConsultarCNPJ(ctx, "11111111000191")
	if err == nil {
		t.Fatal("esperava erro para um CNPJ que nao existe")
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
