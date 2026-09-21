//go:build ignore

// Command gerar_lista rebuilds municipios.csv from the government spreadsheet.
//
//	go run gerar_lista.go [-saida arquivo.csv]
//
// The source of truth is ANEXO_A, versioned in docs/anexos/. Nothing here runs
// during a build: the generated CSV is committed, and this program exists so
// that the next version of the annex can be applied by rerunning it.
//
// The .xlsx is read with archive/zip and encoding/xml, for the same reason as
// the service list generator: a spreadsheet library would be a dependency in a
// binary that handles a private key.
package main

import (
	"archive/zip"
	"encoding/csv"
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

const (
	annex         = "../../../docs/anexos/ANEXO_A-MUNICIPIO_IBGE-PAISES_ISO2-v1.00-SNNFSe-20251210.xlsx"
	defaultOutput = "municipios.csv"

	// sheet holds TAB.MUN_IBGE. The other two sheets are the ISO-2 country
	// table and a single "águas marítimas" row, neither of which is a
	// municipality.
	sheet = "xl/worksheets/sheet1.xml"
)

// siglaPorPrefixo maps the first two digits of an IBGE municipality code to
// the state abbreviation.
//
// This table is written here rather than read from the annex because **the
// annex does not carry it**: the "Sigla UF" column is filled for 450 of the
// 5570 rows and empty for every state outside the North region — whoever built
// the spreadsheet stopped filling it after Tocantins. The UF *names* are
// complete, and so are the codes, so the abbreviation is the one thing that has
// to come from outside.
//
// It is not taken on faith. build() checks it three ways: every row's code
// prefix must be in this table, all rows of one UF name must share one prefix,
// and wherever the annex does state a sigla it must be the one below.
var siglaPorPrefixo = map[string]string{
	"11": "RO", "12": "AC", "13": "AM", "14": "RR", "15": "PA", "16": "AP",
	"17": "TO", "21": "MA", "22": "PI", "23": "CE", "24": "RN", "25": "PB",
	"26": "PE", "27": "AL", "28": "SE", "29": "BA", "31": "MG", "32": "ES",
	"33": "RJ", "35": "SP", "41": "PR", "42": "SC", "43": "RS", "50": "MS",
	"51": "MT", "52": "GO", "53": "DF",
}

func main() {
	output := flag.String("saida", defaultOutput, "arquivo CSV a gerar")
	flag.Parse()

	if err := run(*output); err != nil {
		log.Fatal(err)
	}
}

func run(output string) error {
	zr, err := zip.OpenReader(annex)
	if err != nil {
		return fmt.Errorf("abrir %s: %w", annex, err)
	}
	defer zr.Close()

	shared, err := readSharedStrings(&zr.Reader)
	if err != nil {
		return err
	}

	rows, err := readSheet(&zr.Reader, shared)
	if err != nil {
		return err
	}

	records, err := build(rows)
	if err != nil {
		return err
	}

	return write(output, records)
}

type row map[string]string

// readSharedStrings loads the string table an .xlsx keeps outside the sheets.
func readSharedStrings(zr *zip.Reader) ([]string, error) {
	data, err := readFile(zr, "xl/sharedStrings.xml")
	if err != nil {
		return nil, err
	}

	var table struct {
		Items []struct {
			Text []string `xml:"t"`
			Runs []struct {
				Text string `xml:"t"`
			} `xml:"r"`
		} `xml:"si"`
	}
	if err := xml.Unmarshal(data, &table); err != nil {
		return nil, fmt.Errorf("sharedStrings.xml: %w", err)
	}

	// A string arrives either as a single <t> or split into <r> runs, one per
	// stretch of different formatting. Reading only the first shape loses the
	// text of every styled cell, silently, as an empty string.
	out := make([]string, len(table.Items))
	for i, item := range table.Items {
		var sb strings.Builder
		for _, t := range item.Text {
			sb.WriteString(t)
		}
		for _, run := range item.Runs {
			sb.WriteString(run.Text)
		}
		out[i] = sb.String()
	}
	return out, nil
}

func readSheet(zr *zip.Reader, shared []string) ([]row, error) {
	data, err := readFile(zr, sheet)
	if err != nil {
		return nil, err
	}

	var doc struct {
		Rows []struct {
			Cells []struct {
				Ref    string   `xml:"r,attr"`
				Type   string   `xml:"t,attr"`
				Value  string   `xml:"v"`
				Inline []string `xml:"is>t"`
			} `xml:"c"`
		} `xml:"sheetData>row"`
	}
	if err := xml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("%s: %w", sheet, err)
	}

	var rows []row
	for _, r := range doc.Rows {
		cells := row{}
		for _, c := range r.Cells {
			text := c.Value
			switch c.Type {
			case "s":
				index, err := strconv.Atoi(strings.TrimSpace(c.Value))
				if err != nil || index < 0 || index >= len(shared) {
					return nil, fmt.Errorf("celula %s aponta para a string %q, que nao existe", c.Ref, c.Value)
				}
				text = shared[index]
			case "inlineStr":
				text = strings.Join(c.Inline, "")
			}
			cells[column(c.Ref)] = strings.TrimSpace(text)
		}
		rows = append(rows, cells)
	}
	return rows, nil
}

func column(ref string) string {
	for i, r := range ref {
		if r >= '0' && r <= '9' {
			return ref[:i]
		}
	}
	return ref
}

type record struct {
	codigo string
	nome   string
	uf     string
}

func build(rows []row) ([]record, error) {
	const (
		colNomeUF   = "A"
		colSiglaUF  = "B"
		colNome     = "C"
		colCodigo   = "D"
		esperadoQtd = 5570
	)

	if len(rows) == 0 || rows[0][colCodigo] == "" {
		return nil, fmt.Errorf("a planilha nao comeca com o cabecalho esperado")
	}

	prefixoPorUF := map[string]string{}
	var out []record

	for i, r := range rows[1:] {
		linha := i + 2

		codigo := r[colCodigo]
		if len(codigo) != 7 {
			return nil, fmt.Errorf("linha %d: codigo %q nao tem 7 digitos", linha, codigo)
		}
		if _, err := strconv.Atoi(codigo); err != nil {
			return nil, fmt.Errorf("linha %d: codigo %q nao e numerico", linha, codigo)
		}

		nome := r[colNome]
		if nome == "" {
			return nil, fmt.Errorf("linha %d: codigo %s sem nome de municipio", linha, codigo)
		}

		nomeUF := r[colNomeUF]
		if nomeUF == "" {
			return nil, fmt.Errorf("linha %d: codigo %s sem nome de UF", linha, codigo)
		}

		prefixo := codigo[:2]
		sigla, ok := siglaPorPrefixo[prefixo]
		if !ok {
			return nil, fmt.Errorf("linha %d: codigo %s comeca com %q, que nao e uma UF conhecida",
				linha, codigo, prefixo)
		}

		// One UF name must map to exactly one code prefix. A mismatch means
		// either the annex changed shape or a row landed under the wrong state.
		if anterior, visto := prefixoPorUF[nomeUF]; visto && anterior != prefixo {
			return nil, fmt.Errorf("linha %d: %q aparece com os prefixos %s e %s",
				linha, nomeUF, anterior, prefixo)
		}
		prefixoPorUF[nomeUF] = prefixo

		// Where the annex does state the abbreviation, it has to agree with the
		// table above. This is the only external check available on it.
		if informada := r[colSiglaUF]; informada != "" && informada != sigla {
			return nil, fmt.Errorf("linha %d: o anexo informa a sigla %q para o prefixo %s, a tabela diz %q",
				linha, informada, prefixo, sigla)
		}

		out = append(out, record{codigo: codigo, nome: nome, uf: sigla})
	}

	if len(out) != esperadoQtd {
		return nil, fmt.Errorf("li %d municipios, esperava %d", len(out), esperadoQtd)
	}
	if len(prefixoPorUF) != len(siglaPorPrefixo) {
		return nil, fmt.Errorf("a planilha traz %d UFs, a tabela de siglas tem %d",
			len(prefixoPorUF), len(siglaPorPrefixo))
	}

	sort.Slice(out, func(i, j int) bool { return out[i].codigo < out[j].codigo })

	vistos := map[string]bool{}
	for _, rec := range out {
		if vistos[rec.codigo] {
			return nil, fmt.Errorf("codigo %s aparece duas vezes", rec.codigo)
		}
		vistos[rec.codigo] = true
	}

	// Name plus UF has to identify exactly one municipality: it is the key the
	// CLI resolves. 232 names repeat across states, so the pair is what must be
	// unique, not the name.
	pares := map[string]string{}
	for _, rec := range out {
		chave := strings.ToLower(rec.nome) + "/" + rec.uf
		if anterior, repetido := pares[chave]; repetido {
			return nil, fmt.Errorf("%q identifica tanto %s quanto %s", chave, anterior, rec.codigo)
		}
		pares[chave] = rec.codigo
	}

	return out, nil
}

func write(output string, records []record) error {
	var buf strings.Builder
	fmt.Fprintf(&buf, "# Municipios brasileiros e seus codigos IBGE, gerado por gerar_lista.go.\n")
	fmt.Fprintf(&buf, "# Fonte: docs/anexos/%s\n", filepath.Base(annex))
	fmt.Fprintf(&buf, "# Nao edite a mao: rode 'go generate ./internal/domain/municipio'.\n")

	w := csv.NewWriter(&buf)
	if err := w.Write([]string{"codigo", "nome", "uf"}); err != nil {
		return err
	}
	for _, rec := range records {
		if err := w.Write([]string{rec.codigo, rec.nome, rec.uf}); err != nil {
			return err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return err
	}

	if err := os.WriteFile(output, []byte(buf.String()), 0o644); err != nil {
		return err
	}
	fmt.Printf("%s: %d municipios\n", output, len(records))
	return nil
}

func readFile(zr *zip.Reader, name string) ([]byte, error) {
	f, err := zr.Open(name)
	if err != nil {
		return nil, fmt.Errorf("%s nao esta na planilha: %w", name, err)
	}
	defer f.Close()
	return io.ReadAll(f)
}
