// Package config loads the emitter's settings and per-invoice data.
//
// Values come from three places, each overriding the previous one:
//
//  1. the "padroes" section of nfse.yaml, for anything that repeats across
//     every invoice (service code, municipality, ISS rate, an unidentified
//     taker);
//  2. an invoice file passed with --yaml;
//  3. command-line flags.
//
// The point of the layering is that a provider who always issues the same kind
// of service to an unidentified taker only has to type the amount.
package config

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/edusouza/nfse-emissor-go/pkg/cnpjcpf"
)

// Environment names accepted in the "ambiente" field.
const (
	// EnvProducao is the live environment. Invoices issued here are real.
	EnvProducao = "producao"

	// EnvProducaoRestrita is the government's testing environment.
	EnvProducaoRestrita = "producao-restrita"
)

// Tax regime names accepted in "prestador.regime_tributario".
const (
	RegimeMEI   = "mei"
	RegimeMEEPP = "me_epp"
)

// Config is the contents of nfse.yaml.
type Config struct {
	// Ambiente selects the government environment: "producao" or
	// "producao-restrita". Defaults to producao-restrita, so that a missing
	// or half-written config never issues a real invoice by accident.
	Ambiente string `yaml:"ambiente"`

	Certificado Certificado `yaml:"certificado"`
	Prestador   Prestador   `yaml:"prestador"`
	DPS         DPS         `yaml:"dps"`
	Saida       Saida       `yaml:"saida"`

	// Padroes holds the invoice fields that repeat, so they can be omitted
	// from the command line.
	Padroes Nota `yaml:"padroes"`
}

// Certificado points at the A1 certificate file.
type Certificado struct {
	Arquivo string `yaml:"arquivo"`

	// Senha is deliberately absent. Certificate passwords belong in the
	// NFSE_CERT_SENHA environment variable or in an interactive prompt, not
	// in a file that is easy to commit by mistake.
}

// Prestador is the service provider: you.
type Prestador struct {
	CNPJ               string `yaml:"cnpj"`
	Nome               string `yaml:"nome"`
	RegimeTributario   string `yaml:"regime_tributario"`
	InscricaoMunicipal string `yaml:"inscricao_municipal"`

	// Municipio is the 7-digit IBGE code of the municipality where the DPS is
	// issued.
	Municipio string `yaml:"municipio"`
}

// DPS holds the document series. The number is per-invoice.
type DPS struct {
	Serie string `yaml:"serie"`
}

// Saida controls where generated files land.
type Saida struct {
	Diretorio string `yaml:"diretorio"`
}

// Nota is a single invoice. It doubles as the "padroes" section of the config
// and as the file given to --yaml, so that both accept the same shape.
type Nota struct {
	Numero      string   `yaml:"numero"`
	Competencia string   `yaml:"competencia"`
	Servico     Servico  `yaml:"servico"`
	Valores     Valores  `yaml:"valores"`
	Tomador     *Tomador `yaml:"tomador"`
}

// Servico describes what was provided.
type Servico struct {
	// CodigoTributacaoNacional is cTribNac: 6 digits from the national service
	// list (LC 116/2003).
	CodigoTributacaoNacional string `yaml:"codigo_tributacao_nacional"`

	// MunicipioPrestacao is the IBGE code of where the service was provided,
	// which is not always where the DPS is issued.
	MunicipioPrestacao string `yaml:"municipio_prestacao"`

	Descricao string `yaml:"descricao"`
}

// Valores holds the monetary amounts.
type Valores struct {
	ValorServico           float64 `yaml:"valor_servico"`
	DescontoIncondicionado float64 `yaml:"desconto_incondicionado"`
	DescontoCondicionado   float64 `yaml:"desconto_condicionado"`
	Deducoes               float64 `yaml:"deducoes"`

	// ISSAliquota is a pointer because 0 is a meaningful rate: an MEI pays ISS
	// through the DAS, so pAliq is legitimately zero. Without the pointer an
	// explicit 0 would be indistinguishable from "not set" during merging.
	ISSAliquota *float64 `yaml:"iss_aliquota"`
}

// Tomador is the party taking the service.
type Tomador struct {
	// NaoIdentificado omits the taker from the DPS entirely, which is what you
	// want when selling to the general public.
	NaoIdentificado bool `yaml:"nao_identificado"`

	CNPJ  string `yaml:"cnpj"`
	CPF   string `yaml:"cpf"`
	Nome  string `yaml:"nome"`
	Email string `yaml:"email"`
}

// DefaultFileName is the config file looked up when --config is not given.
const DefaultFileName = "nfse.yaml"

// Load reads and validates a config file.
func Load(path string) (*Config, error) {
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("arquivo de configuracao %q nao encontrado; rode 'nfse config init' para criar um", path)
		}
		return nil, fmt.Errorf("nao foi possivel ler %q: %w", path, err)
	}
	defer f.Close()

	cfg, err := Decode(f)
	if err != nil {
		return nil, fmt.Errorf("erro em %q: %w", path, err)
	}
	return cfg, nil
}

// Decode parses a config from r, applying defaults.
func Decode(r io.Reader) (*Config, error) {
	var cfg Config

	dec := yaml.NewDecoder(r)
	// Reject unknown fields. In a fiscal tool a silently ignored typo such as
	// "aliquota_iss" would produce a wrong invoice rather than an error.
	dec.KnownFields(true)

	if err := dec.Decode(&cfg); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("YAML invalido: %w", err)
	}

	cfg.applyDefaults()
	return &cfg, nil
}

// DecodeNota parses an invoice file, as given to --yaml.
func DecodeNota(r io.Reader) (*Nota, error) {
	var nota Nota

	dec := yaml.NewDecoder(r)
	dec.KnownFields(true)

	if err := dec.Decode(&nota); err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("YAML invalido: %w", err)
	}
	return &nota, nil
}

// LoadNota reads an invoice file from disk.
func LoadNota(path string) (*Nota, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("nao foi possivel ler a nota %q: %w", path, err)
	}
	defer f.Close()

	nota, err := DecodeNota(f)
	if err != nil {
		return nil, fmt.Errorf("erro em %q: %w", path, err)
	}
	return nota, nil
}

func (c *Config) applyDefaults() {
	if c.Ambiente == "" {
		// Defaulting to the test environment is deliberate: the cost of an
		// accidental test invoice is zero, the cost of an accidental real one
		// is a cancellation procedure.
		c.Ambiente = EnvProducaoRestrita
	}
	if c.Saida.Diretorio == "" {
		c.Saida.Diretorio = "."
	}
	if c.DPS.Serie == "" {
		c.DPS.Serie = "00001"
	}
}

// EnvironmentCode returns the tpAmb value used in the DPS: 1 for production,
// 2 for the restricted production environment.
func (c *Config) EnvironmentCode() int {
	if c.Ambiente == EnvProducao {
		return 1
	}
	return 2
}

// Validate checks the settings that must be present regardless of which
// invoice is being issued. Per-invoice fields are checked after merging.
func (c *Config) Validate() error {
	var problems []string

	switch c.Ambiente {
	case EnvProducao, EnvProducaoRestrita:
	default:
		problems = append(problems, fmt.Sprintf("ambiente: %q e invalido (use %q ou %q)",
			c.Ambiente, EnvProducao, EnvProducaoRestrita))
	}

	switch {
	case c.Prestador.CNPJ == "":
		problems = append(problems, "prestador.cnpj: obrigatorio")
	case !cnpjcpf.ValidateCNPJ(c.Prestador.CNPJ):
		// The government rejects an invalid CNPJ anyway; catching the check
		// digits here costs nothing and saves a round-trip.
		problems = append(problems, fmt.Sprintf("prestador.cnpj: %q tem digitos verificadores invalidos", c.Prestador.CNPJ))
	}
	if c.Prestador.Nome == "" {
		problems = append(problems, "prestador.nome: obrigatorio")
	}
	if c.Prestador.Municipio == "" {
		problems = append(problems, "prestador.municipio: obrigatorio (codigo IBGE de 7 digitos)")
	} else if len(c.Prestador.Municipio) != 7 {
		problems = append(problems, fmt.Sprintf("prestador.municipio: %q nao tem 7 digitos", c.Prestador.Municipio))
	}

	switch c.Prestador.RegimeTributario {
	case RegimeMEI, RegimeMEEPP:
	case "":
		problems = append(problems, fmt.Sprintf("prestador.regime_tributario: obrigatorio (%q ou %q)", RegimeMEI, RegimeMEEPP))
	default:
		problems = append(problems, fmt.Sprintf("prestador.regime_tributario: %q e invalido (use %q ou %q)",
			c.Prestador.RegimeTributario, RegimeMEI, RegimeMEEPP))
	}

	if len(c.DPS.Serie) != 5 {
		problems = append(problems, fmt.Sprintf("dps.serie: %q deve ter exatamente 5 digitos", c.DPS.Serie))
	}

	if len(problems) > 0 {
		return fmt.Errorf("configuracao invalida:\n  - %s", strings.Join(problems, "\n  - "))
	}
	return nil
}
