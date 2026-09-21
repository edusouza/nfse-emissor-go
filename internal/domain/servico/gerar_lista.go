//go:build ignore

// Command gerar_lista rebuilds lista.csv from the government spreadsheet.
//
//	go run gerar_lista.go [-saida arquivo.csv]
//
// The source of truth is ANEXO_B, versioned in docs/anexos/. Nothing here runs
// during a build: the generated CSV is committed, and this program exists so
// that the next version of the annex can be applied by rerunning it instead of
// by editing 335 lines by hand.
//
// The .xlsx is read with archive/zip and encoding/xml — an OOXML workbook is a
// zip of XML documents. A spreadsheet library would be a cleaner read, but a
// dependency in a binary that handles a private key is surface, and this file
// is the only place in the project that needs one.
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
	annex         = "../../../docs/anexos/anexo_b-nbs2-lista_servico_nacional-snnfse-v1-01-20260122.xlsx"
	defaultOutput = "lista.csv"

	// sheet holds the national service list. The workbook's second sheet is
	// the NBS list, a different taxonomy that the DPS does not carry.
	sheet = "xl/worksheets/sheet1.xml"
)

func main() {
	// The output path is a flag so that a test can regenerate the list into a
	// scratch file and compare: the committed CSV is only trustworthy while
	// rerunning this program reproduces it byte for byte.
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

// row is one spreadsheet row, indexed by column letter.
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
	// text of every cell the annex happens to have styled — silently, as an
	// empty description.
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

// readSheet decodes the worksheet into rows of plain text.
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

// column strips the row number off a cell reference: "AB12" becomes "AB".
func column(ref string) string {
	for i, r := range ref {
		if r >= '0' && r <= '9' {
			return ref[:i]
		}
	}
	return ref
}

// record is one line of the generated file.
type record struct {
	codigo    string
	grupo     string
	descricao string
}

// build turns the sheet into the leaf codes plus the heading each one sits
// under.
//
// The sheet is a tree flattened into rows: an item heading has subitem and
// desdobro zero, a subitem heading has desdobro zero, and only the leaves — the
// rows that carry a desdobro — have a code. Only leaves can go on a DPS; the
// headings are kept as the group name, because "Serviços de Informática e
// congêneres." is how someone recognizes their own trade in a list of 335
// entries.
func build(rows []row) ([]record, error) {
	const (
		colCodigo    = "A"
		colItem      = "B"
		colSubitem   = "C"
		colDesdobro  = "D"
		colDescricao = "E"
	)

	if len(rows) == 0 || rows[0][colCodigo] == "" {
		return nil, fmt.Errorf("a planilha nao comeca com o cabecalho esperado")
	}

	grupos := map[int]string{}
	var out []record

	for i, r := range rows[1:] {
		linha := i + 2

		item, err := strconv.Atoi(r[colItem])
		if err != nil {
			return nil, fmt.Errorf("linha %d: item %q nao e um numero", linha, r[colItem])
		}
		subitem, err := strconv.Atoi(r[colSubitem])
		if err != nil {
			return nil, fmt.Errorf("linha %d: subitem %q nao e um numero", linha, r[colSubitem])
		}
		desdobro, err := strconv.Atoi(r[colDesdobro])
		if err != nil {
			return nil, fmt.Errorf("linha %d: desdobro %q nao e um numero", linha, r[colDesdobro])
		}

		if subitem == 0 && desdobro == 0 {
			grupos[item] = r[colDescricao]
			continue
		}
		if desdobro == 0 {
			continue
		}

		// The code is the concatenation of the three numbers, two digits
		// each. Checking it against the column rather than trusting either one
		// is what would catch a shifted row in a future annex.
		codigo := fmt.Sprintf("%02d%02d%02d", item, subitem, desdobro)
		if got := fmt.Sprintf("%06s", r[colCodigo]); got != codigo {
			return nil, fmt.Errorf("linha %d: codigo %q nao bate com item %d subitem %d desdobro %d (%s)",
				linha, r[colCodigo], item, subitem, desdobro, codigo)
		}

		descricao := r[colDescricao]
		if descricao == "" {
			return nil, fmt.Errorf("linha %d: codigo %s sem descricao", linha, codigo)
		}

		out = append(out, record{codigo: codigo, grupo: grupos[item], descricao: descricao})
	}

	for i, rec := range out {
		if rec.grupo == "" {
			return nil, fmt.Errorf("codigo %s nao tem cabecalho de item", out[i].codigo)
		}
	}

	sort.Slice(out, func(i, j int) bool { return out[i].codigo < out[j].codigo })

	seen := map[string]bool{}
	for _, rec := range out {
		if seen[rec.codigo] {
			return nil, fmt.Errorf("codigo %s aparece duas vezes", rec.codigo)
		}
		seen[rec.codigo] = true
	}
	return out, nil
}

func write(output string, records []record) error {
	var buf strings.Builder
	fmt.Fprintf(&buf, "# Lista de servicos nacional (LC 116/2003), gerada por gerar_lista.go.\n")
	fmt.Fprintf(&buf, "# Fonte: docs/anexos/%s\n", filepath.Base(annex))
	fmt.Fprintf(&buf, "# Nao edite a mao: rode 'go generate ./internal/domain/servico'.\n")

	w := csv.NewWriter(&buf)
	if err := w.Write([]string{"codigo", "grupo", "descricao"}); err != nil {
		return err
	}
	for _, rec := range records {
		if err := w.Write([]string{rec.codigo, rec.grupo, rec.descricao}); err != nil {
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
	fmt.Printf("%s: %d codigos\n", output, len(records))
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
