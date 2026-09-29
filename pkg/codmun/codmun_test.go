package codmun

import (
	"strings"
	"testing"

	"github.com/edusouza/nfse-emissor-go/internal/anexoa"
)

func TestValidar(t *testing.T) {
	validos := []string{
		"4106902", // Curitiba/PR
		"3550308", // São Paulo/SP
		"5300108", // Brasília/DF
		"1100015", // Alta Floresta D'Oeste/RO, o primeiro da tabela
		"2201919", // Bom Princípio do Piauí/PI, sem dígito verificador válido
	}
	for _, codigo := range validos {
		if err := Validar(codigo); err != nil {
			t.Errorf("Validar(%q) = %v, esperava nil", codigo, err)
		}
	}

	// Each case names the rule that must refuse it, so that a rule removed
	// cannot hide behind another one rejecting the same code.
	invalidos := []struct {
		codigo, motivo, mensagem string
	}{
		{"", "vazio", "7 digitos"},
		{"7107", "codigo TOM da Receita, 4 digitos", "7 digitos"},
		{"41", "codigo da UF", "7 digitos"},
		{"41069020", "8 digitos", "7 digitos"},
		{"410690a", "letra", "7 digitos"},
		{" 4106902", "espaco", "7 digitos"},
		{"4106903", "digito verificador errado", "digito verificador"},
		{"4160902", "digitos transpostos", "digito verificador"},
		{"3400009", "prefixo 34 nao e UF, digito verificador certo", "codigo de uma UF"},
		{"9900002", "prefixo 99 nao e UF, digito verificador certo", "codigo de uma UF"},
		{"0000000", "zeros, digito verificador certo", "codigo de uma UF"},
	}
	for _, c := range invalidos {
		err := Validar(c.codigo)
		if err == nil {
			t.Errorf("Validar(%q) aceitou (%s)", c.codigo, c.motivo)
			continue
		}
		if !strings.Contains(err.Error(), c.mensagem) {
			t.Errorf("Validar(%q) = %v; esperava a regra %q (%s)", c.codigo, err, c.mensagem, c.motivo)
		}
	}
}

func TestUF(t *testing.T) {
	casos := map[string]string{
		"4106902": "PR",
		"3550308": "SP",
		"5300108": "DF",
		"1200013": "AC",
		"3450308": "",
		"410690":  "",
		"41x6902": "",
	}
	for codigo, esperado := range casos {
		if obtido := UF(codigo); obtido != esperado {
			t.Errorf("UF(%q) = %q, esperava %q", codigo, obtido, esperado)
		}
	}
}

// Every code in the official table must pass, and the check-digit exceptions
// must be exactly the codes the table has that fail the algorithm — a code
// listed as an exception but valid by the algorithm would hide a typo in the
// list.
func TestValidarContraOAnexoA(t *testing.T) {
	codigos := codigosDoAnexoA(t)
	if len(codigos) != 5570 {
		t.Fatalf("o ANEXO_A trouxe %d municipios, esperava 5570", len(codigos))
	}

	excecoes := map[string]bool{}
	for _, codigo := range codigos {
		if err := Validar(codigo); err != nil {
			t.Errorf("codigo oficial recusado: %v", err)
		}
		if digitoVerificador(codigo) != codigo[6] {
			excecoes[codigo] = true
		}
	}

	ufs := ufsDoAnexoA(t)
	for prefixo, nome := range ufs {
		if quer := siglaPorNome[nome]; quer == "" || ufPorPrefixo[prefixo] != quer {
			t.Errorf("prefixo %s e %q no ANEXO_A; a tabela diz %q", prefixo, nome, ufPorPrefixo[prefixo])
		}
	}
	if len(ufs) != len(ufPorPrefixo) {
		t.Errorf("o ANEXO_A tem %d UFs; a tabela tem %d", len(ufs), len(ufPorPrefixo))
	}

	for codigo := range semDigitoVerificador {
		if !excecoes[codigo] {
			t.Errorf("%s esta na lista de excecoes, mas nao e excecao no ANEXO_A", codigo)
		}
	}
	if len(excecoes) != len(semDigitoVerificador) {
		t.Errorf("o ANEXO_A tem %d codigos sem digito verificador valido; a lista tem %d",
			len(excecoes), len(semDigitoVerificador))
	}
}

// siglaPorNome is written from the state names, not from the annex's own
// "Sigla UF" column, which is empty for most rows.
var siglaPorNome = map[string]string{
	"Rondônia": "RO", "Acre": "AC", "Amazonas": "AM", "Roraima": "RR", "Pará": "PA",
	"Amapá": "AP", "Tocantins": "TO", "Maranhão": "MA", "Piauí": "PI", "Ceará": "CE",
	"Rio Grande do Norte": "RN", "Paraíba": "PB", "Pernambuco": "PE", "Alagoas": "AL",
	"Sergipe": "SE", "Bahia": "BA", "Minas Gerais": "MG", "Espírito Santo": "ES",
	"Rio de Janeiro": "RJ", "São Paulo": "SP", "Paraná": "PR", "Santa Catarina": "SC",
	"Rio Grande do Sul": "RS", "Mato Grosso do Sul": "MS", "Mato Grosso": "MT",
	"Goiás": "GO", "Distrito Federal": "DF",
}

func codigosDoAnexoA(t *testing.T) []string {
	t.Helper()

	var codigos []string
	for _, m := range lerAnexoA(t) {
		codigos = append(codigos, m.Codigo)
	}
	return codigos
}

// ufsDoAnexoA maps each code prefix to the state name the annex gives it, and
// fails if one prefix appears under two names.
func ufsDoAnexoA(t *testing.T) map[string]string {
	t.Helper()

	ufs := map[string]string{}
	for _, m := range lerAnexoA(t) {
		prefixo := m.Codigo[:2]
		if nome, ok := ufs[prefixo]; ok && nome != m.NomeUF {
			t.Fatalf("prefixo %s aparece como %q e %q no ANEXO_A", prefixo, nome, m.NomeUF)
		}
		ufs[prefixo] = m.NomeUF
	}
	return ufs
}

func lerAnexoA(t *testing.T) []anexoa.Municipio {
	t.Helper()

	municipios, err := anexoa.Ler()
	if err != nil {
		t.Fatal(err)
	}
	return municipios
}

func BenchmarkValidar(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = Validar("4106902")
	}
}
