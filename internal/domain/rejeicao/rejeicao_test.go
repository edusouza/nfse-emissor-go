package rejeicao

import (
	"reflect"
	"testing"

	"github.com/edusouza/nfse-emissor-go/internal/anexos"
)

// TestTabelasEstaoSincronizadasComOsAnexos reads the annexes and compares
// them with the committed tables.
//
// Everything else here tests the tables against themselves, and would go on
// passing if someone had fixed a message by hand or an annex had been
// replaced without regenerating. Only the annex can say whether a table is
// right, and both are versioned in this repository.
func TestTabelasEstaoSincronizadasComOsAnexos(t *testing.T) {
	for _, tt := range []struct {
		arquivo string
		ler     func(string) ([]anexos.Rejeicao, error)
		tabela  map[string][]Regra
		gerado  string
	}{
		{anexos.ArquivoI, anexos.LerI, anexoI, "anexo_i.go"},
		{anexos.ArquivoII, anexos.LerII, anexoII, "anexo_ii.go"},
	} {
		lidas, err := tt.ler(anexos.Caminho(tt.arquivo))
		if err != nil {
			t.Fatal(err)
		}

		esperado := make(map[string][]Regra, len(lidas))
		for _, r := range lidas {
			for _, regra := range r.Regras {
				esperado[r.Codigo] = append(esperado[r.Codigo], Regra{
					Mensagem: regra.Mensagem,
					Campo:    regra.Campo,
					Texto:    regra.Texto,
					Nivel:    regra.Nivel,
				})
			}
		}

		if reflect.DeepEqual(esperado, tt.tabela) {
			continue
		}
		for codigo := range esperado {
			if !reflect.DeepEqual(esperado[codigo], tt.tabela[codigo]) {
				t.Errorf("%s: %s difere do anexo", tt.gerado, codigo)
			}
		}
		for codigo := range tt.tabela {
			if _, ok := esperado[codigo]; !ok {
				t.Errorf("%s: %s nao esta no anexo", tt.gerado, codigo)
			}
		}
		t.Errorf("%s nao e mais o que o anexo produz.\n"+
			"Se o anexo mudou, rode 'go generate ./internal/domain/rejeicao' e "+
			"registre a mudanca no CHANGELOG; o arquivo nao se edita a mao.", tt.gerado)
	}
}

func TestBuscarDPS(t *testing.T) {
	regras := BuscarDPS("E0600", "")
	if len(regras) != 1 {
		t.Fatalf("E0600: %d regras, quer 1", len(regras))
	}
	r := regras[0]
	if r.CampoNoArquivo() != "DPS/infDPS/valores/trib/tribMun/pAliq" {
		t.Errorf("E0600: campo na DPS %q", r.CampoNoArquivo())
	}
	if r.DependeDoMunicipio() {
		t.Error("E0600 e regra geral, nivel 2")
	}
	if !r.AcrescentaAMensagem() {
		t.Error("a regra do E0600 diz opSimpNac = 2, que a mensagem nao diz")
	}

	// Lower case and stray spaces are still the same code.
	if len(BuscarDPS(" e0600 ", "")) != 1 {
		t.Error("o codigo deveria ser encontrado sem depender de caixa e espacos")
	}

	if BuscarDPS("E2240", "") != nil {
		t.Error("E2240 e do ADN (ANEXO IV), nao do ANEXO I")
	}
	if BuscarDPS("", "") != nil {
		t.Error("codigo vazio nao deveria encontrar nada")
	}
}

func TestBuscarNivel3(t *testing.T) {
	regras := BuscarDPS("E0635", "")
	if len(regras) != 1 || !regras[0].DependeDoMunicipio() {
		t.Fatalf("E0635 depende do convenio do municipio: %+v", regras)
	}
}

// The annex gives E1570 to two unrelated rules. The Sefin's sentence says
// which one was broken.
func TestBuscarCodigoRepetido(t *testing.T) {
	todas := BuscarDPS("E1570", "")
	if len(todas) != 2 {
		t.Fatalf("E1570: %d regras, quer 2", len(todas))
	}

	// Wrapped, in another case and without the final period, as an answer
	// may come: still the same sentence.
	got := BuscarDPS("E1570", "valor do diferimento  para a CBS\nnão deve ser informado")
	if len(got) != 1 {
		t.Fatalf("E1570 com a mensagem da CBS: %d regras, quer 1", len(got))
	}
	if got[0].CampoNoArquivo() != "NFSe/infNFSe/IBSCBS/totCIBS/gCBS/vDifCBS" {
		t.Errorf("um campo que a Sefin calcula, fora da DPS, deveria manter o caminho inteiro: %q", got[0].CampoNoArquivo())
	}

	if got := BuscarDPS("E1570", "uma mensagem que o anexo nao tem"); len(got) != 2 {
		t.Errorf("sem saber qual regra, as duas deveriam vir: %d", len(got))
	}
}

func TestRecepcaoNaoTemCampo(t *testing.T) {
	regras := BuscarDPS("E1200", "")
	if len(regras) == 0 {
		t.Fatal("E1200 ausente")
	}
	if regras[0].CampoNoArquivo() != "" || regras[0].DependeDoMunicipio() {
		t.Errorf("regra da recepcao nao tem campo nem nivel: %+v", regras[0])
	}
}

func TestBuscarEvento(t *testing.T) {
	regras := BuscarEvento("E0822", "")
	if len(regras) != 1 {
		t.Fatalf("E0822: %d regras, quer 1", len(regras))
	}
	r := regras[0]
	// The request the provider signs is pedRegEvento; the event wraps it.
	if r.CampoNoArquivo() != "pedRegEvento/infPedReg/chNFSe" {
		t.Errorf("E0822: campo no arquivo %q", r.CampoNoArquivo())
	}
	if !r.DependeDoMunicipio() {
		t.Error("o prazo de cancelamento e o que o municipio parametrizou: nivel 3")
	}

	// A code of ANEXO I is not looked up in ANEXO II, nor the other way.
	if BuscarEvento("E0600", "") != nil {
		t.Error("E0600 e da DPS, nao de evento")
	}
	if BuscarDPS("E0822", "") != nil {
		t.Error("E0822 e de evento, nao da DPS")
	}

	// E1260 is in both, about a different document in each.
	dps, evento := BuscarDPS("E1260", ""), BuscarEvento("E1260", "")
	if len(dps) == 0 || len(evento) == 0 || dps[0].Campo == evento[0].Campo {
		t.Fatalf("E1260 deveria existir nos dois anexos, com campos diferentes: %+v %+v", dps, evento)
	}
	// The event's own version field is outside the request the user sent.
	if evento[0].CampoNoArquivo() != "evento/versao" {
		t.Errorf("E1260 do evento: campo %q", evento[0].CampoNoArquivo())
	}
}
