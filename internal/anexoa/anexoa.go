// Package anexoa reads the official municipality table of the Sistema
// Nacional NFS-e, ANEXO_A, versioned in docs/anexos.
//
// It exists for tests. The binary does not carry the table (ADR 0012), but
// the rules that stand in for it — the IBGE check digit, the state prefixes,
// the way names are folded for lookup — are only trustworthy if they hold for
// every municipality in it, and that is what the tests use this for. Nothing
// outside a _test.go file imports it.
//
// The .xlsx is read with archive/zip and encoding/xml, like the generators in
// this repository: a spreadsheet library would be a dependency in a binary
// that handles a private key.
package anexoa

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"io"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
)

// Arquivo is the annex, relative to the repository root.
const Arquivo = "docs/anexos/ANEXO_A-MUNICIPIO_IBGE-PAISES_ISO2-v1.00-SNNFSe-20251210.xlsx"

// Municipio is one row of the TAB.MUN_IBGE sheet.
type Municipio struct {
	NomeUF string // column A, the state's full name
	Nome   string // column C
	Codigo string // column D, seven digits
}

// Ler reads every municipality of the annex.
func Ler() ([]Municipio, error) {
	zr, err := zip.OpenReader(caminho())
	if err != nil {
		return nil, fmt.Errorf("abrir o ANEXO_A: %w", err)
	}
	defer zr.Close()

	var shared struct {
		Items []struct {
			Text []string `xml:"t"`
			Runs []struct {
				Text string `xml:"t"`
			} `xml:"r"`
		} `xml:"si"`
	}
	if err := lerXML(&zr.Reader, "xl/sharedStrings.xml", &shared); err != nil {
		return nil, err
	}

	// A string arrives either as a single <t> or split into <r> runs; reading
	// only the first shape loses the text of every styled cell.
	strs := make([]string, len(shared.Items))
	for i, item := range shared.Items {
		var sb strings.Builder
		for _, t := range item.Text {
			sb.WriteString(t)
		}
		for _, r := range item.Runs {
			sb.WriteString(r.Text)
		}
		strs[i] = sb.String()
	}

	// sheet1 is TAB.MUN_IBGE; the others hold countries and a single
	// "águas marítimas" row.
	var sheet struct {
		Rows []struct {
			Cells []struct {
				Ref   string `xml:"r,attr"`
				Type  string `xml:"t,attr"`
				Value string `xml:"v"`
			} `xml:"c"`
		} `xml:"sheetData>row"`
	}
	if err := lerXML(&zr.Reader, "xl/worksheets/sheet1.xml", &sheet); err != nil {
		return nil, err
	}

	var out []Municipio
	for _, row := range sheet.Rows {
		var m Municipio
		for _, c := range row.Cells {
			valor := c.Value
			if c.Type == "s" {
				i, err := strconv.Atoi(valor)
				if err != nil || i < 0 || i >= len(strs) {
					return nil, fmt.Errorf("celula %s aponta para a string %q, que nao existe", c.Ref, valor)
				}
				valor = strs[i]
			}
			valor = strings.TrimSpace(valor)
			switch coluna(c.Ref) {
			case "A":
				m.NomeUF = valor
			case "C":
				m.Nome = valor
			case "D":
				m.Codigo = valor
			}
		}
		// The header row has no seven-digit code.
		if len(m.Codigo) == 7 && strings.Trim(m.Codigo, "0123456789") == "" {
			out = append(out, m)
		}
	}
	return out, nil
}

// caminho finds the annex from this source file, so that a test in any
// package reads the same file regardless of its working directory.
func caminho() string {
	_, arquivo, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(arquivo), "..", "..", Arquivo)
}

func coluna(ref string) string {
	return strings.TrimRight(ref, "0123456789")
}

func lerXML(zr *zip.Reader, nome string, v any) error {
	f, err := zr.Open(nome)
	if err != nil {
		return fmt.Errorf("%s: %w", nome, err)
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return fmt.Errorf("%s: %w", nome, err)
	}
	if err := xml.Unmarshal(data, v); err != nil {
		return fmt.Errorf("%s: %w", nome, err)
	}
	return nil
}
