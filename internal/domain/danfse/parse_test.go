package danfse

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func lerExemplo(t *testing.T) []byte {
	t.Helper()

	conteudo, err := os.ReadFile(filepath.Join("testdata", "nfse-exemplo.xml"))
	if err != nil {
		t.Fatalf("nao consegui ler a NFS-e de exemplo: %v", err)
	}
	return conteudo
}

func TestParse_Identificacao(t *testing.T) {
	doc, err := Parse(lerExemplo(t), nil)
	if err != nil {
		t.Fatalf("Parse devolveu erro: %v", err)
	}

	casos := []struct {
		campo    string
		obtido   string
		esperado string
	}{
		{"chave de acesso", doc.Identificacao.ChaveAcesso, "41069022212345678000195000000000000126081234567890"},
		{"numero", doc.Identificacao.Numero, "126"},
		{"competencia", doc.Identificacao.Competencia, "12/08/2026"},
		{"emissao da NFS-e", doc.Identificacao.EmissaoNFSe, "12/08/2026 14:31:05"},
		{"numero da DPS", doc.Identificacao.NumeroDPS, "126"},
		{"serie da DPS", doc.Identificacao.SerieDPS, "00001"},
		{"emissao da DPS", doc.Identificacao.EmissaoDPS, "12/08/2026 14:30:00"},
		{"emitente", doc.Identificacao.Emitente, "Prestador"},
		{"situacao", doc.Identificacao.Situacao, "NFS-e Gerada"},
	}

	for _, caso := range casos {
		if caso.obtido != caso.esperado {
			t.Errorf("%s: esperava %q, veio %q", caso.campo, caso.esperado, caso.obtido)
		}
	}
}

// A v1.00 invoice has no IBS/CBS group, so finNFSe has nowhere to come from.
// Note 12 of NT 008 says the field still prints, as a dash.
func TestParse_FinalidadeAusenteViraTraco(t *testing.T) {
	doc, err := Parse(lerExemplo(t), nil)
	if err != nil {
		t.Fatalf("Parse devolveu erro: %v", err)
	}

	if doc.Identificacao.Finalidade != Traco {
		t.Errorf("esperava %q na finalidade, veio %q", Traco, doc.Identificacao.Finalidade)
	}
}

func TestParse_Cabecalho(t *testing.T) {
	doc, err := Parse(lerExemplo(t), nil)
	if err != nil {
		t.Fatalf("Parse devolveu erro: %v", err)
	}

	if doc.Cabecalho.Municipio != "Curitiba / PR" {
		t.Errorf("municipio: esperava %q, veio %q", "Curitiba / PR", doc.Cabecalho.Municipio)
	}
	if doc.Cabecalho.AmbienteGerador != "Sistema Nacional da NFS-e" {
		t.Errorf("ambiente gerador: veio %q", doc.Cabecalho.AmbienteGerador)
	}
	if doc.Cabecalho.TipoAmbiente != "Homologação" {
		t.Errorf("tipo de ambiente: veio %q", doc.Cabecalho.TipoAmbiente)
	}
	if !doc.Cabecalho.SemValidadeJuridica {
		t.Error("tpAmb = 2 tem de marcar a NFS-e como sem validade juridica")
	}

	const esperado = "https://www.nfse.gov.br/ConsultaPublica/?tpc=1&chave=" +
		"41069022212345678000195000000000000126081234567890"
	if doc.Cabecalho.QRCode != esperado {
		t.Errorf("QR Code:\n esperava %q\n veio     %q", esperado, doc.Cabecalho.QRCode)
	}
}

// Production invoices carry no banner: the red warning exists to keep a test
// document from passing for a real one, and printing it on a real one would be
// the same mistake backwards.
func TestParse_ProducaoNaoLevaTarja(t *testing.T) {
	conteudo := []byte(trocar(t, string(lerExemplo(t)), "<tpAmb>2</tpAmb>", "<tpAmb>1</tpAmb>"))

	doc, err := Parse(conteudo, nil)
	if err != nil {
		t.Fatalf("Parse devolveu erro: %v", err)
	}

	if doc.Cabecalho.SemValidadeJuridica {
		t.Error("tpAmb = 1 nao pode marcar a NFS-e como sem validade juridica")
	}
	if doc.Cabecalho.TipoAmbiente != "Produção" {
		t.Errorf("tipo de ambiente: veio %q", doc.Cabecalho.TipoAmbiente)
	}
}

// Item 99 of the national list is for services outside it, which have no
// municipality to name — NT 008 asks for the header field to be left out.
func TestParse_ItemNoventaENoveOmiteOMunicipio(t *testing.T) {
	conteudo := []byte(trocar(t, string(lerExemplo(t)),
		"<cTribNac>010701</cTribNac>", "<cTribNac>990101</cTribNac>"))

	doc, err := Parse(conteudo, nil)
	if err != nil {
		t.Fatalf("Parse devolveu erro: %v", err)
	}

	if doc.Cabecalho.Municipio != "" {
		t.Errorf("esperava municipio vazio, veio %q", doc.Cabecalho.Municipio)
	}
}

func TestParse_Recusas(t *testing.T) {
	casos := []struct {
		nome     string
		conteudo string
		esperado error
	}{
		{
			nome:     "XML malformado",
			conteudo: "<NFSe><infNFSe>",
			esperado: ErrXMLInvalido,
		},
		{
			nome:     "DPS em vez de NFS-e",
			conteudo: `<?xml version="1.0"?><DPS><infDPS Id="DPS1"/></DPS>`,
			esperado: ErrSemNFSe,
		},
		{
			nome:     "sem atributo Id",
			conteudo: `<?xml version="1.0"?><NFSe><infNFSe/></NFSe>`,
			esperado: ErrSemChave,
		},
		{
			nome:     "chave curta",
			conteudo: `<?xml version="1.0"?><NFSe><infNFSe Id="NFS123"/></NFSe>`,
			esperado: ErrSemChave,
		},
		{
			nome:     "chave com letra",
			conteudo: `<?xml version="1.0"?><NFSe><infNFSe Id="NFS4106902221234567800019500000000000012608123456789X"/></NFSe>`,
			esperado: ErrSemChave,
		},
	}

	for _, caso := range casos {
		t.Run(caso.nome, func(t *testing.T) {
			_, err := Parse([]byte(caso.conteudo), nil)
			if !errors.Is(err, caso.esperado) {
				t.Fatalf("esperava %v, veio %v", caso.esperado, err)
			}
		})
	}
}

// trocar replaces the first occurrence of velho with novo, failing loudly when
// the fixture no longer contains it — a test that silently stops testing what
// it says it tests is worse than one that breaks.
func trocar(t *testing.T, texto, velho, novo string) string {
	t.Helper()

	if !strings.Contains(texto, velho) {
		t.Fatalf("a NFS-e de exemplo nao contem mais %q", velho)
	}
	return strings.Replace(texto, velho, novo, 1)
}
