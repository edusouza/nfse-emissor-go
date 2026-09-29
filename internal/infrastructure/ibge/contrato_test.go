package ibge

import (
	"context"
	"os"
	"testing"
	"time"
)

// The DANFSe prints municipality names from this service, and the decoder was
// written from its documentation (ADR 0012). This holds it to the live
// service; it is off by default so the suite stays offline.
//
//	NFSE_TESTE_CONTRATO=1 go test ./internal/infrastructure/ibge/ -run Contrato -v
func TestContratoConsultar(t *testing.T) {
	if os.Getenv("NFSE_TESTE_CONTRATO") != "1" {
		t.Skip("contrato com o servico real: defina NFSE_TESTE_CONTRATO=1 para rodar")
	}

	casos := []Municipio{
		{Codigo: "4106902", Nome: "Curitiba", UF: "PR"},
		{Codigo: "3550308", Nome: "São Paulo", UF: "SP"},
		{Codigo: "5300108", Nome: "Brasília", UF: "DF"},
		// One of the nine codes whose check digit does not verify.
		{Codigo: "2201919", Nome: "Bom Princípio do Piauí", UF: "PI"},
	}

	client := New(Config{})
	for _, quer := range casos {
		t.Run(quer.Codigo, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			defer cancel()

			obtido, err := client.Consultar(ctx, quer.Codigo)
			if err != nil {
				t.Fatalf("consulta falhou: %v", err)
			}
			if *obtido != quer {
				t.Errorf("Consultar(%s) = %+v, esperava %+v", quer.Codigo, *obtido, quer)
			}
		})
	}
}
