package main

import (
	"archive/zip"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// annexI is the business-rules annex of the DPS and the NFS-e: every rule the
// Sefin applies to a declaration, with the code it answers when one fails.
const annexI = "docs/anexos/anexo_i-sefin_adn-dps_nfse-snnfse-v1-01-20260209.xlsx"

const (
	// receptionSheet is RN_RECEPCAO_DPS: the rules checked on the
	// transmission itself, the certificate, before the document is read.
	receptionSheet = "xl/worksheets/sheet3.xml"

	// rulesSheet is "RN DPS_NFS-e": one rule per field of the NFS-e and of
	// the DPS it carries.
	rulesSheet = "xl/worksheets/sheet5.xml"
)

// rejectionsPage is where the page lands, relative to the site content.
const rejectionsPage = "referencia/rejeicoes.md"

var codePattern = regexp.MustCompile(`^E\d{4}$`)

// rejection is one code and every rule that answers with it. A code
// normally belongs to one rule, but the annex reuses a few (E1570 is two
// unrelated rules, with two messages), and choosing one would hide the other.
type rejection struct {
	code  string
	rules []rule
	local bool
}

type rule struct {
	message string
	field   string // XML path; empty for the reception rules
	text    string
	level   string // "1", "2" or "3"; empty for the reception rules
	notes   string
}

func writeRejections(root, out string) error {
	rejections, err := readRejections(filepath.Join(root, filepath.FromSlash(annexI)))
	if err != nil {
		return err
	}

	local, err := localCodes(filepath.Join(root, "internal", "domain", "validation"))
	if err != nil {
		return err
	}
	byCode := map[string]*rejection{}
	for i := range rejections {
		byCode[rejections[i].code] = &rejections[i]
	}
	for code, source := range local {
		r, ok := byCode[code]
		if !ok {
			return fmt.Errorf("%s cita o codigo %s, que nao esta no ANEXO I: corrija a mensagem ou atualize o anexo", source, code)
		}
		r.local = true
	}

	dst := filepath.Join(out, filepath.FromSlash(rejectionsPage))
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(dst, []byte(renderRejections(rejections)), 0o644); err != nil {
		return err
	}
	fmt.Printf("%s: %d codigos\n", dst, len(rejections))
	return nil
}

// readRejections reads the codes the Sefin can answer when it receives a DPS,
// sorted by code.
func readRejections(file string) ([]rejection, error) {
	zr, err := zip.OpenReader(file)
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
	// turns that into a refusal to generate.
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

	byCode := map[string]*rejection{}
	var order []string
	add := func(code string, r rule) {
		if byCode[code] == nil {
			byCode[code] = &rejection{code: code}
			order = append(order, code)
		}
		byCode[code].rules = append(byCode[code].rules, r)
	}

	for row := 2; row <= reception.lastRow; row++ {
		code, ok, err := codeAt(reception, "F", row, receptionSheet)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		r := rule{
			message: reception.cell("G", row),
			text:    reception.cell("B", row),
			notes:   meaningful(reception.cell("H", row)),
		}
		if r.message == "" {
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
		// municipality shares with the national repository, and an emitter
		// never meets them.
		switch k := rules.cell("K", row); k {
		case "X":
			continue
		case "V":
		default:
			return nil, fmt.Errorf("%s linha %d: codigo %s sem V ou X na coluna K (traz %q)", rulesSheet, row, code, k)
		}
		r := rule{
			message: rules.cell("I", row),
			field:   rules.cell("B", row) + rules.cell("C", row),
			text:    rules.cell("D", row),
			level:   rules.cell("J", row),
			notes:   meaningful(rules.cell("O", row)),
		}
		if r.message == "" {
			return nil, fmt.Errorf("%s linha %d: codigo %s sem mensagem", rulesSheet, row, code)
		}
		add(code, r)
	}

	if len(order) == 0 {
		return nil, fmt.Errorf("nenhum codigo de rejeicao encontrado no ANEXO I")
	}
	sort.Strings(order)
	out := make([]rejection, len(order))
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
	case codePattern.MatchString(v):
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

// localPattern matches how the validation package cites the rule it applies
// in the message the user reads.
var localPattern = regexp.MustCompile(`a Sefin rejeita: (E\d{4})`)

// localCodes finds the codes that nfse emitir refuses before signing, by the
// messages that cite them, so that the page follows the code instead of a
// list someone has to remember to update.
func localCodes(dir string) (map[string]string, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		return nil, err
	}
	out := map[string]string{}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		data, err := os.ReadFile(f)
		if err != nil {
			return nil, err
		}
		for _, m := range localPattern.FindAllStringSubmatch(string(data), -1) {
			out[m[1]] = filepath.ToSlash(f)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("nenhuma mensagem em %s cita um codigo da Sefin; o formato 'a Sefin rejeita: E0000' mudou?", dir)
	}
	return out, nil
}

var levels = map[string]string{
	"1": "Nível 1 — consistência do leiaute",
	"2": "Nível 2 — regra geral, a mesma em todos os municípios",
	"3": "Nível 3 — depende da legislação do município, parametrizada no Sistema Nacional",
}

func renderRejections(rejections []rejection) string {
	var b strings.Builder
	fmt.Fprintf(&b, `---
hide:
  - toc
---

%s a partir de %s. Não edite: as mudanças somem na próxima publicação. -->

# Códigos de rejeição

Quando recusa uma DPS, a Sefin Nacional responde com um código e uma mensagem.
Esta página lista os %d códigos que ela pode devolver na recepção de uma
declaração, com a regra que cada um aplica.

O texto é o do governo, tirado do [ANEXO I](%s) do Sistema Nacional NFS-e
(`+"`%s`"+`) toda vez que o site é publicado. Em caso de dúvida, vale a planilha.

!!! tip "Procurando um código?"
    Use a busca do site ou o Ctrl+F do navegador com o código — por exemplo, `+"`E0600`"+`.

Ficam de fora as regras que o anexo aplica só às notas que os municípios
compartilham com o Ambiente de Dados Nacional: um emissor nunca as encontra.
As regras de **nível 3** dependem da legislação do município; a página
[o que a validação local cobre](validacao.md) explica o que o `+"`nfse`"+` pode e
o que não pode conferir antes de enviar.

`, generatedMarker, annexI, len(rejections), githubBlob(annexI), path.Base(annexI))

	for _, r := range rejections {
		fmt.Fprintf(&b, "## %s\n\n", r.code)
		if r.local {
			b.WriteString("!!! info \"Conferido pelo `nfse` antes de assinar\"\n" +
				"    O `nfse emitir` recusa a DPS por esta regra antes de usar o certificado, com a mesma explicação.\n\n")
		}
		for _, ru := range r.rules {
			// A message is one sentence that the annex sometimes wraps; kept
			// wrapped, the bold around it would depend on where the break fell.
			fmt.Fprintf(&b, "**%s**\n\n", inline(oneLine(ru.message)))
			var meta []string
			if ru.field != "" {
				meta = append(meta, "Campo `"+oneLine(ru.field)+"`")
			}
			if l, ok := levels[ru.level]; ok {
				meta = append(meta, l)
			}
			if len(meta) > 0 {
				b.WriteString(strings.Join(meta, " · ") + "\n\n")
			}
			if ru.text != "" && oneLine(ru.text) != oneLine(ru.message) {
				b.WriteString(prose(ru.text) + "\n\n")
			}
			if ru.notes != "" {
				b.WriteString("*Observação do anexo:* " + prose(ru.notes) + "\n\n")
			}
		}
	}
	return b.String()
}

func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// prose renders the text of a spreadsheet cell as Markdown that shows what
// the cell shows. Blank lines separate paragraphs; a single line break is
// kept, because the annex uses it for enumerations and formulas that read
// wrong run together.
func prose(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	var paras []string
	for _, p := range strings.Split(s, "\n\n") {
		var lines []string
		for _, l := range strings.Split(p, "\n") {
			if l = strings.TrimSpace(l); l != "" {
				lines = append(lines, inline(l))
			}
		}
		if len(lines) > 0 {
			paras = append(paras, strings.Join(lines, "  \n"))
		}
	}
	return strings.Join(paras, "\n\n")
}

var (
	markdownSpecial = strings.NewReplacer(
		`\`, `\\`, "`", "\\`", `*`, `\*`, `_`, `\_`,
		`[`, `\[`, `]`, `\]`, `{`, `\{`, `}`, `\}`,
		`<`, `&lt;`,
	)
	// A line that starts like a list item, a heading, a quote or a numbered
	// item would become one.
	blockStart = regexp.MustCompile(`^([-+#>]|\d+\.)`)
)

// inline escapes one line of annex text so that Markdown prints it as is.
func inline(s string) string {
	s = markdownSpecial.Replace(s)
	if m := blockStart.FindStringIndex(s); m != nil {
		s = s[:m[1]-1] + `\` + s[m[1]-1:]
	}
	return s
}
