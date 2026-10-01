package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/edusouza/nfse-emissor-go/internal/domain/rejeicao"
	"github.com/edusouza/nfse-emissor-go/internal/infrastructure/sefin"
)

// paginaRejeicoes is the page of the documentation site that lists every code,
// generated from the same annex as the table (ADR 0016). Each code is a
// heading there, so "#e0600" lands on it.
const paginaRejeicoes = "https://edusouza.github.io/nfse-emissor-go/referencia/rejeicoes/"

// recuoRegra lines up the explanation under the code it belongs to.
const recuoRegra = "      "

// explicarRejeicao adds, under each code the Sefin answered with, what ANEXO I
// says about it: the field of the DPS, the rule that failed and, when it
// applies, that the rule depends on the municipality. The Sefin's sentence
// alone rarely says which field to fix.
//
// Anything that is not a rejection passes through untouched, and the result
// still matches sefin.ErrRejected and *sefin.RejectionError.
func explicarRejeicao(err error) error {
	var rej *sefin.RejectionError
	if !errors.As(err, &rej) {
		return err
	}
	return &rejeicaoExplicada{rej}
}

type rejeicaoExplicada struct {
	*sefin.RejectionError
}

func (e *rejeicaoExplicada) Unwrap() error { return e.RejectionError }

func (e *rejeicaoExplicada) Error() string {
	var b strings.Builder
	b.WriteString(sefin.ErrRejected.Error())
	for _, m := range e.Rejections {
		escreverRejeicao(&b, m)
	}
	return b.String()
}

func escreverRejeicao(b *strings.Builder, m sefin.Message) {
	mensagem := m.Descricao
	if mensagem == "" {
		mensagem = m.Mensagem
	}
	regras := rejeicao.Buscar(m.Codigo, mensagem)

	// An answer without a sentence still has one in the annex.
	if mensagem == "" && len(regras) == 1 {
		m.Descricao = regras[0].Mensagem
	}
	fmt.Fprintf(b, "\n  - %s", m)

	if len(regras) == 0 {
		return
	}
	for _, r := range regras {
		if campo := r.CampoNaDPS(); campo != "" {
			fmt.Fprintf(b, "\n%sCampo   %s", recuoRegra, campo)
		}
		if r.AcrescentaAMensagem() {
			fmt.Fprintf(b, "\n%sRegra   %s", recuoRegra, recuar(r.Texto, recuoRegra+"        "))
		}
		if r.DependeDoMunicipio() {
			fmt.Fprintf(b, "\n%sDepende do municipio: segue a parametrizacao que ele fez no Sistema Nacional.", recuoRegra)
		}
	}
	fmt.Fprintf(b, "\n%sMais    %s#%s", recuoRegra, paginaRejeicoes, strings.ToLower(strings.TrimSpace(m.Codigo)))
}

// recuar indents every line of a multi-line text but the first, which the
// caller has already placed. The annex breaks lines inside a cell to list
// conditions; run together they read wrong, so the breaks are kept.
func recuar(texto, recuo string) string {
	linhas := strings.Split(strings.ReplaceAll(strings.TrimSpace(texto), "\r\n", "\n"), "\n")
	var out []string
	branco := false
	for _, l := range linhas {
		l = strings.TrimSpace(l)
		if l == "" {
			// One blank line is a paragraph; more are the annex's padding.
			if !branco {
				out = append(out, "")
			}
			branco = true
			continue
		}
		branco = false
		if len(out) > 0 {
			l = recuo + l
		}
		out = append(out, l)
	}
	return strings.Join(out, "\n")
}
