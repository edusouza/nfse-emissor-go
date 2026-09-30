package config

import (
	_ "embed"
	"fmt"
	"strings"
	"text/template"
)

//go:embed onboard.yaml.tmpl
var onboardTemplate string

// Onboarded is the configuration assembled by `nfse onboard`.
//
// Every field may be empty: the command writes what it managed to discover and
// reports the rest, because a file with three fields filled in is still a head
// start over a blank template.
type Onboarded struct {
	Ambiente           string
	CertificadoArquivo string
	CNPJ               string
	Nome               string
	RegimeTributario   string
	Municipio          string
	Serie              string

	// Servico is cTribNac, and ServicoDescricao the official text of that
	// code, written as a comment so the file says what the six digits mean.
	Servico          string
	ServicoDescricao string

	// Descricao is padroes.servico.descricao, the text that goes in xDescServ
	// when --descricao is not given. Only the user can say it.
	Descricao string

	// SugestoesServico are candidate codes ranked from the provider's CNAE and
	// written commented out, for the user to uncomment one.
	//
	// They are never filled in. The CNAE is the Receita Federal's economic
	// activity classification and cTribNac is the service list of LC 116/2003:
	// two taxonomies with no official correspondence, so this is a ranking of
	// words. A wrong code here would go on every invoice and only surface at a
	// rejection — or not at all.
	SugestoesServico []SugestaoServico

	// CNAE and CNAEDescricao say what the suggestions were derived from, so
	// that a bad list can be recognized as a bad starting point rather than as
	// a bad law.
	CNAE          string
	CNAEDescricao string

	// Origens are the "campo: fonte" lines written into the file's header, so
	// that whoever reads it later knows which values were looked up and which
	// were typed.
	Origens []string
}

// SugestaoServico is one candidate cTribNac written into the generated file.
type SugestaoServico struct {
	Codigo    string
	Descricao string
}

// RenderOnboarded produces an annotated nfse.yaml with the discovered values
// already in place.
func RenderOnboarded(o Onboarded) (string, error) {
	if o.Ambiente == "" {
		o.Ambiente = EnvProducaoRestrita
	}
	if o.Serie == "" {
		o.Serie = "00001"
	}

	tmpl, err := template.New("nfse.yaml").Funcs(template.FuncMap{"q": quoteYAML}).Parse(onboardTemplate)
	if err != nil {
		return "", fmt.Errorf("modelo de configuracao invalido: %w", err)
	}

	var out strings.Builder
	if err := tmpl.Execute(&out, o); err != nil {
		return "", fmt.Errorf("nao foi possivel montar a configuracao: %w", err)
	}
	return out.String(), nil
}

// quoteYAML renders a value as a double-quoted YAML scalar.
//
// Quoting everything keeps codes that begin with a zero — the DPS series and
// several IBGE municipality codes — from being read back as numbers. The
// values are single-line, so line breaks and tabs become spaces; anything
// else YAML does not allow raw is escaped, because the razao social comes
// from a third party and a file config check cannot read looks like success.
func quoteYAML(value string) string {
	var b strings.Builder
	b.WriteByte('"')
	for _, r := range value { // an invalid byte arrives as U+FFFD
		switch {
		case r == '\\':
			b.WriteString(`\\`)
		case r == '"':
			b.WriteString(`\"`)
		case r == '\n' || r == '\r' || r == '\t':
			b.WriteByte(' ')
		case imprimivelEmYAML(r):
			b.WriteRune(r)
		case r <= 0xFFFF:
			fmt.Fprintf(&b, `\u%04X`, r)
		default:
			fmt.Fprintf(&b, `\U%08X`, r)
		}
	}
	b.WriteByte('"')
	return b.String()
}

// imprimivelEmYAML is YAML's c-printable, less NEL, which a double-quoted
// scalar would fold as a line break.
func imprimivelEmYAML(r rune) bool {
	return (r >= 0x20 && r <= 0x7E) || (r >= 0xA0 && r <= 0xD7FF) ||
		(r >= 0xE000 && r <= 0xFFFD) || (r >= 0x10000 && r <= 0x10FFFF)
}

// TaxRegimeFromSimples maps the registry's Simples flags onto
// prestador.regime_tributario, returning false when the answer is not known.
//
// MEI is checked first because an MEI is also a Simples opter: the flags are
// not mutually exclusive, and the narrower one is the right answer.
func TaxRegimeFromSimples(mei, simples *bool) (string, bool) {
	if mei != nil && *mei {
		return RegimeMEI, true
	}
	if simples != nil && *simples {
		return RegimeMEEPP, true
	}
	// A false MEI flag with an unknown Simples status says nothing: the
	// provider may still be an ME/EPP, or outside the Simples entirely, and
	// this emitter only supports the former.
	return "", false
}
