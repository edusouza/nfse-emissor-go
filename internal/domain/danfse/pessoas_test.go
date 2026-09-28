package danfse

import (
	"strings"
	"testing"
)

// municipiosFalsos answers from a table, and records what was asked, so a test
// can tell a lookup that happened from one that did not.
type municipiosFalsos struct {
	tabela     map[string][2]string
	perguntas  []string
	responderA bool
}

func (m *municipiosFalsos) Nome(codigo string) (string, string, bool) {
	m.perguntas = append(m.perguntas, codigo)
	if !m.responderA {
		return "", "", false
	}
	if dados, ok := m.tabela[codigo]; ok {
		return dados[0], dados[1], true
	}
	return "", "", false
}

func municipios() *municipiosFalsos {
	return &municipiosFalsos{
		responderA: true,
		tabela: map[string][2]string{
			"3550308": {"São Paulo", "SP"},
			"4106902": {"Curitiba", "PR"},
		},
	}
}

func TestParse_Tomador(t *testing.T) {
	doc, err := Parse(lerExemplo(t), municipios())
	if err != nil {
		t.Fatalf("Parse devolveu erro: %v", err)
	}

	casos := []struct{ campo, obtido, esperado string }{
		{"documento", doc.Tomador.Documento, "98.765.432/0001-98"},
		{"nome", doc.Tomador.Nome, "CLIENTE EXEMPLO COMERCIO LTDA"},
		{"municipio", doc.Tomador.Municipio, "São Paulo / SP"},
		{"codigo e CEP", doc.Tomador.CodigoCEP, "3550308 / 01.310-100"},
		{"endereco", doc.Tomador.Endereco, "Avenida Paulista, 1000, Bela Vista"},
		{"email", doc.Tomador.Email, "financeiro@cliente.com.br"},
		{"inscricao municipal ausente", doc.Tomador.InscricaoMunicipal, Traco},
	}
	for _, caso := range casos {
		if caso.obtido != caso.esperado {
			t.Errorf("%s: esperava %q, veio %q", caso.campo, caso.esperado, caso.obtido)
		}
	}
}

// Without a translation the code still prints: it points at the right
// municipality, which a blank would not.
func TestParse_MunicipioSemTraducaoImprimeOCodigo(t *testing.T) {
	semResposta := municipios()
	semResposta.responderA = false

	doc, err := Parse(lerExemplo(t), semResposta)
	if err != nil {
		t.Fatalf("Parse devolveu erro: %v", err)
	}

	if doc.Tomador.Municipio != "3550308" {
		t.Errorf("esperava o codigo do IBGE, veio %q", doc.Tomador.Municipio)
	}
	if len(semResposta.perguntas) == 0 {
		t.Error("o codigo nem chegou a ser perguntado")
	}
}

// A nil lookup is the --sem-rede case with an empty cache: nothing is asked,
// and the document still prints.
func TestParse_SemConsultaDeMunicipios(t *testing.T) {
	doc, err := Parse(lerExemplo(t), nil)
	if err != nil {
		t.Fatalf("Parse devolveu erro: %v", err)
	}

	if doc.Tomador.Municipio != "3550308" {
		t.Errorf("esperava o codigo do IBGE, veio %q", doc.Tomador.Municipio)
	}
}

func TestParse_Prestador(t *testing.T) {
	doc, err := Parse(lerExemplo(t), municipios())
	if err != nil {
		t.Fatalf("Parse devolveu erro: %v", err)
	}

	if doc.Prestador.Documento != "12.345.678/0001-95" {
		t.Errorf("documento: veio %q", doc.Prestador.Documento)
	}
	if doc.Prestador.InscricaoMunicipal != "1234567" {
		t.Errorf("inscricao municipal: veio %q", doc.Prestador.InscricaoMunicipal)
	}
	// The example is an MEI, and the description is longer than the 40
	// characters the field takes.
	if !strings.HasPrefix(doc.Prestador.SimplesNacional, "Optante - Microempreendedor") {
		t.Errorf("simples nacional: veio %q", doc.Prestador.SimplesNacional)
	}
	if !strings.HasSuffix(doc.Prestador.SimplesNacional, "...") {
		t.Errorf("a descricao deveria ter sido truncada com reticencias: %q", doc.Prestador.SimplesNacional)
	}
	if doc.Prestador.RegimeApuracao != "Regime de apuração dos tributos federais e municipal pelo Simples Nacional" {
		t.Errorf("regime de apuracao: veio %q", doc.Prestador.RegimeApuracao)
	}
}

// The provider who emits has only the CNPJ in the DPS — rule E0121 rejects the
// name there — and the rest of the block comes from infNFSe/emit. Taking the
// mapping of NT 008 to the letter printed a column of dashes.
func TestParse_PrestadorEmitenteVemDoGrupoEmit(t *testing.T) {
	// No lookup at all: the emitter's municipality is named by the invoice.
	doc, err := Parse(lerExemplo(t), nil)
	if err != nil {
		t.Fatalf("Parse devolveu erro: %v", err)
	}

	casos := []struct{ campo, obtido, esperado string }{
		{"nome", doc.Prestador.Nome, "EMPRESA EXEMPLO DE SERVICOS LTDA"},
		{"telefone", doc.Prestador.Telefone, "4133334444"},
		{"municipio", doc.Prestador.Municipio, "Curitiba / PR"},
		{"codigo e CEP", doc.Prestador.CodigoCEP, "4106902 / 80.010-000"},
		{"endereco", doc.Prestador.Endereco, "Rua das Flores, 100, Sala 21, Centro"},
		{"email", doc.Prestador.Email, "contato@exemplo.com.br"},
	}
	for _, caso := range casos {
		if caso.obtido != caso.esperado {
			t.Errorf("%s: esperava %q, veio %q", caso.campo, caso.esperado, caso.obtido)
		}
	}
}

// What the DPS declared is what prints; emit only fills the gaps.
func TestParse_DPSPrevaleceSobreEmit(t *testing.T) {
	conteudo := trocar(t, string(lerExemplo(t)),
		"<IM>1234567</IM>\n          <regTrib>",
		"<IM>1234567</IM>\n          <fone>4199998888</fone>\n          <regTrib>")

	doc, err := Parse([]byte(conteudo), nil)
	if err != nil {
		t.Fatalf("Parse devolveu erro: %v", err)
	}

	if doc.Prestador.Telefone != "4199998888" {
		t.Errorf("o telefone da DPS deveria prevalecer, veio %q", doc.Prestador.Telefone)
	}
	if doc.Prestador.Nome != "EMPRESA EXEMPLO DE SERVICOS LTDA" {
		t.Errorf("o nome ausente da DPS deveria vir do emit, veio %q", doc.Prestador.Nome)
	}
}

// emit describes whoever tpEmit names. When the buyer emits, it is the buyer's
// block that gets completed, never the provider's.
func TestParse_EmitCompletaQuemEmitiu(t *testing.T) {
	conteudo := trocar(t, string(lerExemplo(t)), "<tpEmit>1</tpEmit>", "<tpEmit>2</tpEmit>")

	doc, err := Parse([]byte(conteudo), municipios())
	if err != nil {
		t.Fatalf("Parse devolveu erro: %v", err)
	}

	if doc.Prestador.Nome != Traco {
		t.Errorf("com tpEmit = 2, o emit nao descreve o prestador; veio %q", doc.Prestador.Nome)
	}
	if doc.Tomador.Nome != "CLIENTE EXEMPLO COMERCIO LTDA" {
		t.Errorf("o nome que o tomador declarou deveria prevalecer, veio %q", doc.Tomador.Nome)
	}
	if doc.Tomador.Telefone != "4133334444" {
		t.Errorf("o telefone ausente do tomador deveria vir do emit, veio %q", doc.Tomador.Telefone)
	}
}

// An invoice without the emit group — not one the schema allows, but the
// document still prints — falls back to the dashes of note 12.
func TestParse_SemGrupoEmit(t *testing.T) {
	conteudo := string(lerExemplo(t))
	inicio := strings.Index(conteudo, "<emit>")
	fim := strings.Index(conteudo, "</emit>") + len("</emit>")
	if inicio < 0 || fim <= inicio {
		t.Fatal("a NFS-e de exemplo nao tem mais o grupo emit")
	}

	doc, err := Parse([]byte(conteudo[:inicio]+conteudo[fim:]), nil)
	if err != nil {
		t.Fatalf("Parse devolveu erro: %v", err)
	}

	if doc.Prestador.Nome != Traco || doc.Prestador.Endereco != Traco {
		t.Errorf("sem emit, os campos deveriam ser tracos: %q e %q",
			doc.Prestador.Nome, doc.Prestador.Endereco)
	}
	if doc.Prestador.Documento != "12.345.678/0001-95" {
		t.Errorf("o documento da DPS nao depende do emit, veio %q", doc.Prestador.Documento)
	}
}

// Note 2: a block with nobody in it says so, in the sentence the nota técnica
// dictates, instead of showing empty fields.
func TestParse_BlocosVaziosViramFrase(t *testing.T) {
	doc, err := Parse(lerExemplo(t), municipios())
	if err != nil {
		t.Fatalf("Parse devolveu erro: %v", err)
	}

	if doc.Intermediario.Mensagem != MensagemSemIntermediario {
		t.Errorf("intermediario: veio %q", doc.Intermediario.Mensagem)
	}
	// Layout v1.00 has no recipient group at all, so the invoice really does
	// not identify one.
	if doc.Destinatario.Mensagem != MensagemSemDestinatario {
		t.Errorf("destinatario: veio %q", doc.Destinatario.Mensagem)
	}
	if doc.Tomador.Mensagem != "" {
		t.Errorf("o tomador esta na nota e nao deveria virar frase: %q", doc.Tomador.Mensagem)
	}
}

func TestParse_TomadorAusente(t *testing.T) {
	conteudo := string(lerExemplo(t))
	inicio := strings.Index(conteudo, "<toma>")
	fim := strings.Index(conteudo, "</toma>") + len("</toma>")
	if inicio < 0 || fim <= inicio {
		t.Fatal("a NFS-e de exemplo nao tem mais o bloco do tomador")
	}

	doc, err := Parse([]byte(conteudo[:inicio]+conteudo[fim:]), municipios())
	if err != nil {
		t.Fatalf("Parse devolveu erro: %v", err)
	}

	if doc.Tomador.Mensagem != MensagemSemTomador {
		t.Errorf("tomador: veio %q", doc.Tomador.Mensagem)
	}
}

// Note 3: when the invoice says the recipient is the buyer — indDest = 0 — the
// block says so instead of repeating the same person twice.
func TestParse_DestinatarioEhOProprioTomador(t *testing.T) {
	const bloco = `<IBSCBS>
          <finNFSe>0</finNFSe>
          <cIndOp>000001</cIndOp>
          <indDest>0</indDest>
        </IBSCBS>`

	conteudo := []byte(trocar(t, string(lerExemplo(t)), "</infDPS>", bloco+"\n      </infDPS>"))

	doc, err := Parse(conteudo, municipios())
	if err != nil {
		t.Fatalf("Parse devolveu erro: %v", err)
	}

	if doc.Destinatario.Mensagem != MensagemDestinatarioEhTomador {
		t.Errorf("destinatario: veio %q", doc.Destinatario.Mensagem)
	}
	// And the finalidade, which lives in the same group, now has a source.
	if doc.Identificacao.Finalidade != "NFS-e regular" {
		t.Errorf("finalidade: veio %q", doc.Identificacao.Finalidade)
	}
}

func TestFormatarCEP(t *testing.T) {
	casos := []struct{ entrada, esperado string }{
		{"80010000", "80.010-000"},
		{"01310100", "01.310-100"},
		{"8001000", "8001000"},   // curto demais: sai como veio
		{"abcdefgh", "abcdefgh"}, // nao sao digitos
		{"", ""},
	}

	for _, caso := range casos {
		if obtido := formatarCEP(caso.entrada); obtido != caso.esperado {
			t.Errorf("formatarCEP(%q) = %q, esperava %q", caso.entrada, obtido, caso.esperado)
		}
	}
}
