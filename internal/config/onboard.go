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

	// Origens are the "campo: fonte" lines written into the file's header, so
	// that whoever reads it later knows which values were looked up and which
	// were typed.
	Origens []string
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
// several IBGE municipality codes — from being read back as numbers.
func quoteYAML(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `"`, `\"`, "\n", " ", "\r", " ", "\t", " ")
	return `"` + replacer.Replace(value) + `"`
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
