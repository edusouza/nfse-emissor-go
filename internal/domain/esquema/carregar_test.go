package esquema

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
)

func TestDPSCarrega(t *testing.T) {
	e, err := DPS()
	if err != nil {
		t.Fatalf("o schema da DPS nao carregou: %v", err)
	}
	if _, ok := e.raizes[nome{ns: "http://www.sped.fazenda.gov.br/nfse", local: "DPS"}]; !ok {
		t.Error("o elemento DPS nao esta entre as raizes")
	}
	assinatura := nome{ns: "http://www.w3.org/2000/09/xmldsig#", local: "Signature"}
	if _, ok := e.globais[assinatura]; !ok {
		t.Error("ds:Signature nao foi importado")
	}
	// A DPS schema validates a DPS; a bare signature is not one.
	if _, ok := e.raizes[assinatura]; ok {
		t.Error("ds:Signature aceito como raiz de uma DPS")
	}
}

// The embedded schemas must be the official ones, byte for byte. A copy that
// drifts from docs/schemas would be the same kind of divergence this package
// exists to end.
func TestSchemasEmbutidosSaoOsOficiais(t *testing.T) {
	const oficial = "../../../docs/schemas/nfse-esquemas_xsd-v1-01-20260209/Schemas/1.01"

	entradas, err := arquivos.ReadDir("xsd")
	if err != nil {
		t.Fatal(err)
	}
	if len(entradas) == 0 {
		t.Fatal("nenhum schema embutido")
	}
	for _, e := range entradas {
		embutido, err := arquivos.ReadFile("xsd/" + e.Name())
		if err != nil {
			t.Fatal(err)
		}
		original, err := os.ReadFile(filepath.Join(oficial, e.Name()))
		if err != nil {
			t.Fatalf("%s nao existe no pacote oficial: %v", e.Name(), err)
		}
		if !bytes.Equal(embutido, original) {
			t.Errorf("%s difere da copia oficial em docs/schemas", e.Name())
		}
	}
}
