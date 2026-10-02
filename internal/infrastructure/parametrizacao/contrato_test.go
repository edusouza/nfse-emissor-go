package parametrizacao

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/edusouza/nfse-emissor-go/internal/infrastructure/xmlsigner"
)

// The decoder was written from the swagger and from one round of calls against
// the live service (docs/convenio-municipal.md). This holds it to that service.
// It needs an ICP-Brasil A1 — the ADN refuses the handshake without one — and
// is off by default so the suite stays offline:
//
//	NFSE_TESTE_CONTRATO=1 NFSE_TESTE_CERT=certificado.pfx NFSE_CERT_SENHA=... \
//	  go test ./internal/infrastructure/parametrizacao/ -run Contrato -v
//
// Restricted production holds test data, so only formats are asserted, never
// values: a rate that changes there is not a broken contract.
func clienteDeContrato(t *testing.T) *Client {
	t.Helper()

	if os.Getenv("NFSE_TESTE_CONTRATO") != "1" {
		t.Skip("contrato com o servico real: defina NFSE_TESTE_CONTRATO=1 para rodar")
	}
	caminho := os.Getenv("NFSE_TESTE_CERT")
	if caminho == "" {
		t.Skip("o ADN exige certificado: defina NFSE_TESTE_CERT com o caminho do A1 e NFSE_CERT_SENHA")
	}

	pfx, err := os.ReadFile(caminho)
	if err != nil {
		t.Fatal(err)
	}
	info, err := xmlsigner.ParsePFX(pfx, os.Getenv("NFSE_CERT_SENHA"))
	if err != nil {
		t.Fatal(err)
	}
	cert, err := info.TLSCertificate()
	if err != nil {
		t.Fatal(err)
	}

	client, err := New(Config{Ambiente: AmbienteProducaoRestrita, Certificate: cert})
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func contexto(t *testing.T) context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	t.Cleanup(cancel)
	return ctx
}

func TestContratoConvenio(t *testing.T) {
	client := clienteDeContrato(t)

	ativo, err := client.Convenio(contexto(t), "4106902") // Curitiba
	if err != nil {
		t.Fatalf("Curitiba: %v", err)
	}
	if !ativo.Ativo || ativo.AderenteEmissorNacional != 1 {
		t.Errorf("Curitiba: %+v, esperava convenio ativo e aderente ao emissor", ativo)
	}

	inativo, err := client.Convenio(contexto(t), "3548807") // São Caetano do Sul
	if err != nil {
		t.Fatalf("São Caetano do Sul: %v", err)
	}
	if inativo.Ativo || inativo.Mensagem == "" {
		t.Errorf("São Caetano do Sul: %+v, esperava convenio inativo com o motivo", inativo)
	}
}

func TestContratoAliquota(t *testing.T) {
	client := clienteDeContrato(t)
	competencia := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)

	vigente, err := client.Aliquota(contexto(t), "4106902", "01.07.01.000", competencia)
	if err != nil {
		t.Fatal(err)
	}
	if len(vigente) == 0 || vigente[0].Percentual == nil || vigente[0].Inicio.IsZero() {
		t.Errorf("aliquota vigente fora do formato: %+v", vigente)
	}

	historico, err := client.HistoricoAliquotas(contexto(t), "4106902", "01.07.01.000")
	if err != nil {
		t.Fatal(err)
	}
	if len(historico) < len(vigente) {
		t.Errorf("historico com %d periodos, menos que os %d vigentes", len(historico), len(vigente))
	}

	// A complement the municipality never created: right format, no data.
	_, err = client.Aliquota(contexto(t), "4106902", "01.07.01.001", competencia)
	if !errors.Is(err, ErrNaoEncontrado) {
		t.Errorf("complemento inexistente: erro = %v, esperava ErrNaoEncontrado", err)
	}
}
