// Package anexos reads the business-rule annexes of the Sistema Nacional
// NFS-e, versioned in docs/anexos: every rule the Sefin applies to what it
// receives, with the code it answers when one fails.
//
//   - ANEXO I covers the DPS: what nfse emitir and nfse enviar send.
//   - ANEXO II covers the event requests: what nfse cancelar sends.
//
// Nothing in the binary imports it. The site generator renders the codes as
// pages, and the generator of internal/domain/rejeicao turns them into the
// tables the emitter carries; both read the annexes through here, so the site
// and the terminal cannot disagree about what a code means.
//
// The .xlsx is read with archive/zip and encoding/xml, like the other readers
// of government annexes in this repository: a spreadsheet library would land
// in go.mod, next to the code that handles a private key.
package anexos

import (
	"archive/zip"
	"fmt"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
)

const (
	// ArquivoI is ANEXO I, the rules of the DPS and the NFS-e, relative to
	// the repository root.
	ArquivoI = "docs/anexos/anexo_i-sefin_adn-dps_nfse-snnfse-v1-01-20260209.xlsx"

	// ArquivoII is ANEXO II, the rules of the event requests (pedRegEvento)
	// and the events.
	ArquivoII = "docs/anexos/anexo_ii-sefin_adn-pedregevt_evt-snnfse-v1-01-20260122.xlsx"
)

const (
	// receptionSheetI is RN_RECEPCAO_DPS: the rules checked on the
	// transmission itself, the certificate, before the document is read.
	receptionSheetI = "xl/worksheets/sheet3.xml"

	// rulesSheetI is "RN DPS_NFS-e": one rule per field of the NFS-e and of
	// the DPS it carries.
	rulesSheetI = "xl/worksheets/sheet5.xml"

	// rulesSheetII is "RN EVENTO_PED.REG.EVENTO": one rule per field of the
	// event and of the request it carries. Its columns are those of
	// rulesSheetI. The other sheets of the annex list the event types and
	// which event may follow which; neither carries a code.
	rulesSheetII = "xl/worksheets/sheet4.xml"
)

// PadraoCodigo is the shape of a rejection code: E and four digits.
var PadraoCodigo = regexp.MustCompile(`^E\d{4}$`)

// Rejeicao is one code and every rule that answers with it. A code normally
// belongs to one rule, but the annexes reuse a few (E1570 is two unrelated
// rules, with two messages), and choosing one would hide the other.
type Rejeicao struct {
	Codigo string
	Regras []Regra
}

// Regra is one rule of an annex, as the annex words it.
type Regra struct {
	Mensagem string // what the Sefin answers
	Campo    string // XML path, from the root the annex uses; empty for the reception rules
	Texto    string // the rule itself
	Nivel    string // "1", "2" or "3"; empty when the annex gives none
	Notas    string // the annex's business notes, if any
}

// Caminho is where an annex is, given its path from the repository root,
// wherever the caller runs from. It only makes sense in a checkout of the
// repository, which is the only place this package runs.
func Caminho(arquivo string) string {
	_, este, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(este), "..", "..", filepath.FromSlash(arquivo))
}

// LerI reads the codes the Sefin can answer when it receives a DPS, sorted by
// code.
//
// The rules the annex applies only to invoices that municipalities share with
// the national repository are left out: an emitter never meets them.
func LerI(arquivo string) ([]Rejeicao, error) {
	zr, err := zip.OpenReader(arquivo)
	if err != nil {
		return nil, fmt.Errorf("abrir o ANEXO I: %w", err)
	}
	defer zr.Close()

	shared, err := readSharedStrings(&zr.Reader)
	if err != nil {
		return nil, err
	}
	reception, err := readSheet(&zr.Reader, shared, receptionSheetI)
	if err != nil {
		return nil, err
	}
	rules, err := readSheet(&zr.Reader, shared, rulesSheetI)
	if err != nil {
		return nil, err
	}

	err = checkHeaders(reception, receptionSheetI, []header{
		{"B1", "REGRAS DE NEGÓCIO"},
		{"F1", "CÓD. ERRO"},
		{"G1", "MSG. ERRO"},
		{"H1", "NOTAS EXPLICATIVAS"},
	})
	if err != nil {
		return nil, err
	}

	var c collector
	for row := 2; row <= reception.lastRow; row++ {
		code, ok, err := codeAt(reception, "F", row, receptionSheetI)
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
			return nil, fmt.Errorf("%s linha %d: codigo %s sem mensagem", receptionSheetI, row, code)
		}
		c.add(code, r)
	}

	if err := readRules(rules, rulesSheetI, &c); err != nil {
		return nil, err
	}
	return c.sorted("ANEXO I")
}

// LerII reads the codes the Sefin can answer when it receives an event
// request — a cancellation, among others — sorted by code.
//
// As in ANEXO I, the rules applied only to events that municipalities share
// with the national repository are left out.
func LerII(arquivo string) ([]Rejeicao, error) {
	zr, err := zip.OpenReader(arquivo)
	if err != nil {
		return nil, fmt.Errorf("abrir o ANEXO II: %w", err)
	}
	defer zr.Close()

	shared, err := readSharedStrings(&zr.Reader)
	if err != nil {
		return nil, err
	}
	rules, err := readSheet(&zr.Reader, shared, rulesSheetII)
	if err != nil {
		return nil, err
	}

	var c collector
	if err := readRules(rules, rulesSheetII, &c); err != nil {
		return nil, err
	}
	return c.sorted("ANEXO II")
}

// readRules reads a sheet laid out as "RN DPS_NFS-e": headings in rows 1 to
// 3, one rule per row from 4 on.
func readRules(s sheet, name string, c *collector) error {
	err := checkHeaders(s, name, []header{
		{"B1", "CAMINHO NO XML"},
		{"C1", "CAMPO"},
		{"J1", "NÍVEL DA REGRA"},
		{"K1", "EMISSORES PÚBLICOS NACIONAIS"},
		{"O1", "OBSERVAÇÕES DE NEGÓCIO"},
		{"D3", "REGRAS DE NEGÓCIO"},
		{"H3", "CÓD. ERRO"},
		{"I3", "MSG. ERRO"},
	})
	if err != nil {
		return err
	}

	for row := 4; row <= s.lastRow; row++ {
		code, ok, err := codeAt(s, "H", row, name)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		// Column K says whether the rule runs when the Sefin receives a
		// document from the provider. The ones marked X only run on documents
		// that a municipality shares with the national repository.
		switch k := s.cell("K", row); k {
		case "X":
			continue
		case "V":
		default:
			return fmt.Errorf("%s linha %d: codigo %s sem V ou X na coluna K (traz %q)", name, row, code, k)
		}
		r := Regra{
			Mensagem: s.cell("I", row),
			Campo:    s.cell("B", row) + s.cell("C", row),
			Texto:    s.cell("D", row),
			Nivel:    meaningful(s.cell("J", row)),
			Notas:    meaningful(s.cell("O", row)),
		}
		if r.Mensagem == "" {
			return fmt.Errorf("%s linha %d: codigo %s sem mensagem", name, row, code)
		}
		c.add(code, r)
	}
	return nil
}

type header struct {
	cell string
	want string
}

// checkHeaders refuses a sheet whose columns moved. The columns are fixed,
// and a future annex that moves one would put a message where a rule belongs
// without any error.
func checkHeaders(s sheet, name string, headers []header) error {
	for _, h := range headers {
		// Headers wrap inside their cells ("NÍVEL\nDA REGRA").
		got := strings.Join(strings.Fields(s.cells[h.cell]), " ")
		if !strings.HasPrefix(got, h.want) {
			return fmt.Errorf("%s: a celula %s deveria comecar com %q e traz %q; o leiaute do anexo mudou", name, h.cell, h.want, got)
		}
	}
	return nil
}

// collector gathers the rules of each code in the order the annex gives them.
type collector struct {
	byCode map[string]*Rejeicao
}

func (c *collector) add(code string, r Regra) {
	if c.byCode == nil {
		c.byCode = map[string]*Rejeicao{}
	}
	if c.byCode[code] == nil {
		c.byCode[code] = &Rejeicao{Codigo: code}
	}
	c.byCode[code].Regras = append(c.byCode[code].Regras, r)
}

func (c *collector) sorted(annex string) ([]Rejeicao, error) {
	if len(c.byCode) == 0 {
		return nil, fmt.Errorf("nenhum codigo de rejeicao encontrado no %s", annex)
	}
	codes := make([]string, 0, len(c.byCode))
	for code := range c.byCode {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	out := make([]Rejeicao, len(codes))
	for i, code := range codes {
		out[i] = *c.byCode[code]
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

// meaningful drops the "-" the annexes write in an empty column.
func meaningful(s string) string {
	if s == "-" {
		return ""
	}
	return s
}
