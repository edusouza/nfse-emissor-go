package anexoa

import "testing"

func TestLer(t *testing.T) {
	municipios, err := Ler()
	if err != nil {
		t.Fatal(err)
	}
	if len(municipios) != 5570 {
		t.Fatalf("o ANEXO_A trouxe %d municipios, esperava 5570", len(municipios))
	}

	achou := false
	for _, m := range municipios {
		if m.Codigo == "4106902" {
			achou = true
			if m.Nome != "Curitiba" || m.NomeUF != "Paraná" {
				t.Errorf("4106902 = %+v, esperava Curitiba/Paraná", m)
			}
		}
		if m.Nome == "" || m.NomeUF == "" {
			t.Errorf("linha incompleta: %+v", m)
		}
	}
	if !achou {
		t.Error("Curitiba (4106902) nao esta no ANEXO_A lido")
	}
}
