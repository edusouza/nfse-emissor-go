package anexoi

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// sheet is a worksheet as plain text, addressed by cell reference ("B505").
type sheet struct {
	cells   map[string]string
	lastRow int
}

// cell returns the text of a cell, empty when the cell does not exist.
func (s sheet) cell(col string, row int) string {
	return s.cells[col+strconv.Itoa(row)]
}

// readSheet decodes one worksheet of an open workbook.
//
// Merged cells are spread over the whole range they cover. The annex merges
// a field's path across every rule that applies to that field, so only the
// first of those rules carries it in the file; the others would come out
// with no field at all.
func readSheet(zr *zip.Reader, shared []string, name string) (sheet, error) {
	data, err := readFile(zr, name)
	if err != nil {
		return sheet{}, err
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
		Merges []struct {
			Ref string `xml:"ref,attr"`
		} `xml:"mergeCells>mergeCell"`
	}
	if err := xml.Unmarshal(data, &doc); err != nil {
		return sheet{}, fmt.Errorf("%s: %w", name, err)
	}

	s := sheet{cells: map[string]string{}}
	for _, r := range doc.Rows {
		for _, c := range r.Cells {
			text := c.Value
			switch c.Type {
			case "s":
				index, err := strconv.Atoi(strings.TrimSpace(c.Value))
				if err != nil || index < 0 || index >= len(shared) {
					return sheet{}, fmt.Errorf("%s: celula %s aponta para a string %q, que nao existe", name, c.Ref, c.Value)
				}
				text = shared[index]
			case "inlineStr":
				text = strings.Join(c.Inline, "")
			}
			_, row, err := splitRef(c.Ref)
			if err != nil {
				return sheet{}, fmt.Errorf("%s: %w", name, err)
			}
			s.cells[c.Ref] = strings.TrimSpace(text)
			s.lastRow = max(s.lastRow, row)
		}
	}

	for _, m := range doc.Merges {
		if err := s.spread(m.Ref); err != nil {
			return sheet{}, fmt.Errorf("%s: %w", name, err)
		}
	}
	return s, nil
}

// spread copies the top-left cell of a merged range ("B504:C505") to the
// rest of it.
func (s sheet) spread(ref string) error {
	from, to, ok := strings.Cut(ref, ":")
	if !ok {
		return nil
	}
	c1, r1, err := splitRef(from)
	if err != nil {
		return err
	}
	c2, r2, err := splitRef(to)
	if err != nil {
		return err
	}
	value := s.cells[from]
	for row := r1; row <= r2; row++ {
		for col := colIndex(c1); col <= colIndex(c2); col++ {
			key := colName(col) + strconv.Itoa(row)
			if s.cells[key] == "" {
				s.cells[key] = value
			}
		}
	}
	return nil
}

// splitRef separates "AB12" into "AB" and 12.
func splitRef(ref string) (string, int, error) {
	i := strings.IndexAny(ref, "0123456789")
	if i <= 0 {
		return "", 0, fmt.Errorf("referencia de celula invalida: %q", ref)
	}
	row, err := strconv.Atoi(ref[i:])
	if err != nil {
		return "", 0, fmt.Errorf("referencia de celula invalida: %q", ref)
	}
	return ref[:i], row, nil
}

// colIndex turns a column name into its 1-based position: "A" is 1, "AB" 28.
func colIndex(name string) int {
	n := 0
	for _, r := range name {
		n = n*26 + int(r-'A') + 1
	}
	return n
}

// colName is the inverse of colIndex.
func colName(index int) string {
	var b []byte
	for index > 0 {
		index--
		b = append([]byte{byte('A' + index%26)}, b...)
		index /= 26
	}
	return string(b)
}

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
	// stretch of different formatting. Reading only the first shape loses
	// the text of every cell the annex happens to have styled.
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

func readFile(zr *zip.Reader, name string) ([]byte, error) {
	f, err := zr.Open(name)
	if err != nil {
		return nil, fmt.Errorf("%s nao esta na planilha: %w", name, err)
	}
	defer f.Close()
	return io.ReadAll(f)
}
