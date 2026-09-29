package codmun

import (
	"archive/zip"
	"encoding/xml"
	"io"
	"strconv"
	"strings"
	"testing"
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

	invalidos := map[string]string{
		"":         "vazio",
		"7107":     "codigo TOM da Receita, 4 digitos",
		"41":       "codigo da UF",
		"41069020": "8 digitos",
		"410690a":  "letra",
		"4106903":  "digito verificador errado",
		"4160902":  "digitos transpostos",
		"3450308":  "prefixo 34 nao e UF",
		"9999999":  "prefixo 99 nao e UF",
		" 4106902": "espaco",
	}
	for codigo, motivo := range invalidos {
		if err := Validar(codigo); err == nil {
			t.Errorf("Validar(%q) aceitou (%s)", codigo, motivo)
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

// anexoA is the official municipality table of the Sistema Nacional, versioned
// in docs/anexos. It is read by the test only; the binary does not carry it.
const anexoA = "../../docs/anexos/ANEXO_A-MUNICIPIO_IBGE-PAISES_ISO2-v1.00-SNNFSe-20251210.xlsx"

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

// codigosDoAnexoA reads column D of the first sheet, where the annex keeps the
// seven-digit codes, with archive/zip and encoding/xml like the generators in
// this repository do.
func codigosDoAnexoA(t *testing.T) []string {
	t.Helper()

	zr, err := zip.OpenReader(anexoA)
	if err != nil {
		t.Fatalf("abrir o ANEXO_A: %v", err)
	}
	defer zr.Close()

	shared := lerXML[struct {
		Items []struct {
			Text []string `xml:"t"`
			Runs []struct {
				Text string `xml:"t"`
			} `xml:"r"`
		} `xml:"si"`
	}](t, &zr.Reader, "xl/sharedStrings.xml")

	strs := make([]string, len(shared.Items))
	for i, item := range shared.Items {
		var sb strings.Builder
		for _, s := range item.Text {
			sb.WriteString(s)
		}
		for _, r := range item.Runs {
			sb.WriteString(r.Text)
		}
		strs[i] = sb.String()
	}

	sheet := lerXML[struct {
		Rows []struct {
			Cells []struct {
				Ref   string `xml:"r,attr"`
				Type  string `xml:"t,attr"`
				Value string `xml:"v"`
			} `xml:"c"`
		} `xml:"sheetData>row"`
	}](t, &zr.Reader, "xl/worksheets/sheet1.xml")

	var codigos []string
	for _, row := range sheet.Rows {
		for _, c := range row.Cells {
			// Column D; the header row fails the digit check below.
			if !strings.HasPrefix(c.Ref, "D") {
				continue
			}
			valor := c.Value
			if c.Type == "s" {
				i, err := strconv.Atoi(valor)
				if err != nil || i < 0 || i >= len(strs) {
					t.Fatalf("celula %s aponta para a string %q, que nao existe", c.Ref, valor)
				}
				valor = strs[i]
			}
			if valor = strings.TrimSpace(valor); digitos(valor) {
				codigos = append(codigos, valor)
			}
		}
	}
	return codigos
}

func lerXML[T any](t *testing.T, zr *zip.Reader, nome string) T {
	t.Helper()

	var v T
	f, err := zr.Open(nome)
	if err != nil {
		t.Fatalf("%s: %v", nome, err)
	}
	defer f.Close()
	data, err := io.ReadAll(f)
	if err != nil {
		t.Fatalf("%s: %v", nome, err)
	}
	if err := xml.Unmarshal(data, &v); err != nil {
		t.Fatalf("%s: %v", nome, err)
	}
	return v
}

func BenchmarkValidar(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = Validar("4106902")
	}
}
