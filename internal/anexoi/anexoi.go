// Package anexoi reads the business-rules annex of the DPS and the NFS-e,
// ANEXO I, versioned in docs/anexos: every rule the Sefin applies to a
// declaration, with the code it answers when one fails.
//
// Nothing in the binary imports it. The site generator renders the codes as a
// page, and the generator of internal/domain/rejeicao turns them into the
// table the emitter carries; both read the annex through here, so the page and
// the terminal cannot disagree about what a code means.
//
// The .xlsx is read with archive/zip and encoding/xml, like the other readers
// of government annexes in this repository: a spreadsheet library would land
// in go.mod, next to the code that handles a private key.
package anexoi

import (
	"archive/zip"
	"fmt"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
)

// Arquivo is the annex, relative to the repository root.
const Arquivo = "docs/anexos/anexo_i-sefin_adn-dps_nfse-snnfse-v1-01-20260209.xlsx"

const (
	// receptionSheet is RN_RECEPCAO_DPS: the rules checked on the
	// transmission itself, the certificate, before the document is read.
	receptionSheet = "xl/worksheets/sheet3.xml"

	// rulesSheet is "RN DPS_NFS-e": one rule per field of the NFS-e and of
	// the DPS it carries.
	rulesSheet = "xl/worksheets/sheet5.xml"
)

// PadraoCodigo is the shape of a rejection code in the annex: E and four
// digits.
var PadraoCodigo = regexp.MustCompile(`^E\d{4}$`)

// Rejeicao is one code and every rule that answers with it. A code normally
// belongs to one rule, but the annex reuses a few (E1570 is two unrelated
// rules, with two messages), and choosing one would hide the other.
type Rejeicao struct {
	Codigo string
	Regras []Regra
}

// Regra is one rule of the annex, as the annex words it.
type Regra struct {
	Mensagem string // what the Sefin answers
	Campo    string // XML path, from NFSe/; empty for the reception rules
	Texto    string // the rule itself
	Nivel    string // "1", "2" or "3"; empty for the reception rules
	Notas    string // the annex's business notes, if any
}

// Caminho is where the annex is, wherever the caller runs from. It only
// makes sense in a checkout of the repository, which is the only place this
// package runs.
func Caminho() string {
	_, arquivo, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(arquivo), "..", "..", filepath.FromSlash(Arquivo))
}

// Ler reads the codes the Sefin can answer when it receives a DPS, sorted by
// code.
//
// The rules the annex applies only to invoices that municipalities share with
// the national repository are left out: an emitter never meets them.
func Ler(arquivo string) ([]Rejeicao, error) {
	zr, err := zip.OpenReader(arquivo)
	if err != nil {
		return nil, fmt.Errorf("abrir o ANEXO I: %w", err)
	}
	defer zr.Close()

	shared, err := readSharedStrings(&zr.Reader)
	if err != nil {
		return nil, err
	}
	reception, err := readSheet(&zr.Reader, shared, receptionSheet)
	if err != nil {
		return nil, err
	}
	rules, err := readSheet(&zr.Reader, shared, rulesSheet)
	if err != nil {
		return nil, err
	}

	// The columns are fixed, and a future annex that moves one would put a
	// message where a rule belongs without any error. Checking the headers
	// turns that into a refusal to read.
	headers := []struct {
		sheet sheet
		name  string
		cell  string
		want  string
	}{
		{reception, receptionSheet, "B1", "REGRAS DE NEGÓCIO"},
		{reception, receptionSheet, "F1", "CÓD. ERRO"},
		{reception, receptionSheet, "G1", "MSG. ERRO"},
		{reception, receptionSheet, "H1", "NOTAS EXPLICATIVAS"},
		{rules, rulesSheet, "B1", "CAMINHO NO XML"},
		{rules, rulesSheet, "C1", "CAMPO"},
		{rules, rulesSheet, "J1", "NÍVEL DA REGRA"},
		{rules, rulesSheet, "K1", "EMISSORES PÚBLICOS NACIONAIS"},
		{rules, rulesSheet, "O1", "OBSERVAÇÕES DE NEGÓCIO"},
		{rules, rulesSheet, "D3", "REGRAS DE NEGÓCIO"},
		{rules, rulesSheet, "H3", "CÓD. ERRO"},
		{rules, rulesSheet, "I3", "MSG. ERRO"},
	}
	for _, h := range headers {
		// Headers wrap inside their cells ("NÍVEL\nDA REGRA").
		got := strings.Join(strings.Fields(h.sheet.cells[h.cell]), " ")
		if !strings.HasPrefix(got, h.want) {
			return nil, fmt.Errorf("%s: a celula %s deveria comecar com %q e traz %q; o leiaute do anexo mudou", h.name, h.cell, h.want, got)
		}
	}

	byCode := map[string]*Rejeicao{}
	var order []string
	add := func(code string, r Regra) {
		if byCode[code] == nil {
			byCode[code] = &Rejeicao{Codigo: code}
			order = append(order, code)
		}
		byCode[code].Regras = append(byCode[code].Regras, r)
	}

	for row := 2; row <= reception.lastRow; row++ {
		code, ok, err := codeAt(reception, "F", row, receptionSheet)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		r := Regra{
			Mensagem: reception.cell("G", row),
			Texto:    reception.cell("B", row),
			Notas:    meaningful(reception.cell("H", row)),
		}
		if r.Mensagem == "" {
			return nil, fmt.Errorf("%s linha %d: codigo %s sem mensagem", receptionSheet, row, code)
		}
		add(code, r)
	}

	for row := 4; row <= rules.lastRow; row++ {
		code, ok, err := codeAt(rules, "H", row, rulesSheet)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		// Column K says whether the rule runs when the Sefin receives a DPS
		// from the provider. The ones marked X only run on invoices that a
		// municipality shares with the national repository.
		switch k := rules.cell("K", row); k {
		case "X":
			continue
		case "V":
		default:
			return nil, fmt.Errorf("%s linha %d: codigo %s sem V ou X na coluna K (traz %q)", rulesSheet, row, code, k)
		}
		r := Regra{
			Mensagem: rules.cell("I", row),
			Campo:    rules.cell("B", row) + rules.cell("C", row),
			Texto:    rules.cell("D", row),
			Nivel:    rules.cell("J", row),
			Notas:    meaningful(rules.cell("O", row)),
		}
		if r.Mensagem == "" {
			return nil, fmt.Errorf("%s linha %d: codigo %s sem mensagem", rulesSheet, row, code)
		}
		add(code, r)
	}

	if len(order) == 0 {
		return nil, fmt.Errorf("nenhum codigo de rejeicao encontrado no ANEXO I")
	}
	sort.Strings(order)
	out := make([]Rejeicao, len(order))
	for i, code := range order {
		out[i] = *byCode[code]
	}
	return out, nil
}

// codeAt reads the rejection code of a row. Rows without one — group
// headings, and rules that only warn — hold "-" or nothing; anything else
// that is not a code means the annex changed shape.
func codeAt(s sheet, col string, row int, name string) (string, bool, error) {
	v := s.cell(col, row)
	switch {
	case v == "" || v == "-":
		return "", false, nil
	case PadraoCodigo.MatchString(v):
		return v, true, nil
	}
	return "", false, fmt.Errorf("%s linha %d: %q nao e um codigo de rejeicao", name, row, v)
}

// meaningful drops the "-" the annex writes in an empty column.
func meaningful(s string) string {
	if s == "-" {
		return ""
	}
	return s
}
