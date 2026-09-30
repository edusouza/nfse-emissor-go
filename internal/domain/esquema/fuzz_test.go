package esquema

import "testing"

// The validator reads documents a user hands over, and XML the government
// sends back; no input may panic it.
func FuzzValidar(f *testing.F) {
	f.Add([]byte(`<DPS xmlns="http://www.sped.fazenda.gov.br/nfse" versao="1.00"><infDPS Id="x"/></DPS>`))
	f.Add([]byte(`<DPS`))
	f.Add([]byte(``))
	// A real DPS, so that mutations start from a document that reaches
	// every layer of the schema.
	f.Add([]byte(dpsDoEmissor(f, nil)))

	e, err := DPS()
	if err != nil {
		f.Fatal(err)
	}
	f.Fuzz(func(t *testing.T, doc []byte) {
		for _, erro := range e.Validar(doc) {
			if erro.Caminho == "" || erro.Mensagem == "" {
				t.Errorf("erro sem caminho ou mensagem: %+v", erro)
			}
		}
	})
}

func BenchmarkCarregar(b *testing.B) {
	for i := 0; i < b.N; i++ {
		if _, err := Carregar(arquivos, ArquivoDPS); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkValidar(b *testing.B) {
	e, err := DPS()
	if err != nil {
		b.Fatal(err)
	}
	doc := []byte(dpsDoEmissor(b, nil))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.Validar(doc)
	}
}
