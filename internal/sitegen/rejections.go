package main

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/edusouza/nfse-emissor-go/internal/anexos"
)

// annexI is the business-rules annex of the DPS and the NFS-e.
const annexI = anexos.ArquivoI

// annexII is the business-rules annex of the event requests.
const annexII = anexos.ArquivoII

// Where the pages land, relative to the site content. The emitter links to
// both, by these paths (internal/cli/rejeicao.go).
const (
	rejectionsPage      = "referencia/rejeicoes.md"
	eventRejectionsPage = "referencia/rejeicoes-de-eventos.md"
)

// rejection is one code of an annex, and whether nfse emitir checks it
// before signing.
type rejection struct {
	anexos.Rejeicao
	local bool
}

func writeRejections(root, out string) error {
	read, err := anexos.LerI(filepath.Join(root, filepath.FromSlash(annexI)))
	if err != nil {
		return err
	}
	rejections := make([]rejection, len(read))
	for i, r := range read {
		rejections[i] = rejection{Rejeicao: r}
	}

	local, err := localCodes(filepath.Join(root, "internal", "domain", "validation"))
	if err != nil {
		return err
	}
	byCode := map[string]*rejection{}
	for i := range rejections {
		byCode[rejections[i].Codigo] = &rejections[i]
	}
	for code, source := range local {
		r, ok := byCode[code]
		if !ok {
			return fmt.Errorf("%s cita o codigo %s, que nao esta no ANEXO I: corrija a mensagem ou atualize o anexo", source, code)
		}
		r.local = true
	}

	if err := writePage(out, rejectionsPage, dpsHeader(len(rejections)), rejections); err != nil {
		return err
	}

	read, err = anexos.LerII(filepath.Join(root, filepath.FromSlash(annexII)))
	if err != nil {
		return err
	}
	events := make([]rejection, len(read))
	for i, r := range read {
		events[i] = rejection{Rejeicao: r}
	}
	return writePage(out, eventRejectionsPage, eventHeader(len(events)), events)
}

func writePage(out, page, header string, rejections []rejection) error {
	dst := filepath.Join(out, filepath.FromSlash(page))
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(dst, []byte(header+renderRejections(rejections)), 0o644); err != nil {
		return err
	}
	fmt.Printf("%s: %d codigos\n", dst, len(rejections))
	return nil
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

func dpsHeader(count int) string {
	return fmt.Sprintf(`---
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

`, generatedMarker, annexI, count, githubBlob(annexI), path.Base(annexI))
}

func eventHeader(count int) string {
	return fmt.Sprintf(`---
hide:
  - toc
---

%s a partir de %s. Não edite: as mudanças somem na próxima publicação. -->

# Códigos de rejeição de eventos

Um cancelamento é um pedido de registro de evento, e a Sefin Nacional o recusa
com códigos próprios, diferentes dos da DPS. Esta página lista os %d códigos
que ela pode devolver na recepção de um pedido de evento, com a regra que cada
um aplica. Os da emissão estão em [códigos de rejeição](rejeicoes.md).

O texto é o do governo, tirado do [ANEXO II](%s) do Sistema Nacional NFS-e
(`+"`%s`"+`) toda vez que o site é publicado. Em caso de dúvida, vale a planilha.

!!! tip "Procurando um código?"
    Use a busca do site ou o Ctrl+F do navegador com o código — por exemplo, `+"`E0822`"+`.

O anexo cobre todos os eventos, e o `+"`nfse`"+` só envia o cancelamento: as regras
dos outros eventos (manifestação, análise fiscal, bloqueio) estão aqui porque
estão no anexo, mas um cancelamento nunca as encontra. Ficam de fora as regras
que o anexo aplica só aos eventos que os municípios compartilham com o Ambiente
de Dados Nacional. As regras de **nível 3** dependem do que o município
parametrizou no Sistema Nacional, como o prazo e o valor máximo para cancelar.

`, generatedMarker, annexII, count, githubBlob(annexII), path.Base(annexII))
}

func renderRejections(rejections []rejection) string {
	var b strings.Builder
	for _, r := range rejections {
		fmt.Fprintf(&b, "## %s\n\n", r.Codigo)
		if r.local {
			b.WriteString("!!! info \"Conferido pelo `nfse` antes de assinar\"\n" +
				"    O `nfse emitir` recusa a DPS por esta regra antes de usar o certificado, com a mesma explicação.\n\n")
		}
		for _, ru := range r.Regras {
			// A message is one sentence that the annex sometimes wraps; kept
			// wrapped, the bold around it would depend on where the break fell.
			fmt.Fprintf(&b, "**%s**\n\n", inline(oneLine(ru.Mensagem)))
			var meta []string
			if ru.Campo != "" {
				meta = append(meta, "Campo `"+oneLine(ru.Campo)+"`")
			}
			if l, ok := levels[ru.Nivel]; ok {
				meta = append(meta, l)
			}
			if len(meta) > 0 {
				b.WriteString(strings.Join(meta, " · ") + "\n\n")
			}
			if ru.Texto != "" && oneLine(ru.Texto) != oneLine(ru.Mensagem) {
				b.WriteString(prose(ru.Texto) + "\n\n")
			}
			if ru.Notas != "" {
				b.WriteString("*Observação do anexo:* " + prose(ru.Notas) + "\n\n")
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
