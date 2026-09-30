package config

import (
	"strings"
	"testing"
)

// The Sefin accepts any valid municipality code, so a mistyped digit is the
// one mistake it never reports: the invoice simply belongs to another city.
// The IBGE check digit catches it before anything is signed.

func TestValidate_ConfereOCodigoDoMunicipio(t *testing.T) {
	for _, tc := range []struct {
		name, municipio, wantErr string
	}{
		{"codigo valido", "4106902", ""},
		{"digitos transpostos", "4160902", "digito verificador"},
		{"codigo TOM da Receita", "7535", "7 digitos"},
		{"prefixo que nao e UF", "3400009", "codigo de uma UF"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &Config{
				Ambiente: EnvProducaoRestrita,
				Prestador: Prestador{
					CNPJ:             "12345678000195",
					Nome:             "EMPRESA EXEMPLO LTDA",
					RegimeTributario: RegimeMEI,
					Municipio:        tc.municipio,
				},
				DPS: DPS{Serie: "00001"},
			}

			err := cfg.Validate()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("esperava configuracao valida: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), "prestador.municipio") || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("esperava erro de prestador.municipio mencionando %q: %v", tc.wantErr, err)
			}
		})
	}
}

func TestNotaValidate_ConfereOMunicipioDaPrestacao(t *testing.T) {
	for _, tc := range []struct {
		name, municipio, wantErr string
	}{
		{"vazio usa o do prestador", "", ""},
		{"codigo valido", "3550308", ""},
		{"digito verificador errado", "3550309", "servico.municipio_prestacao"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			nota := Nota{
				Numero:  "1",
				Servico: Servico{CodigoTributacaoNacional: "010101", Descricao: "Servico", MunicipioPrestacao: tc.municipio},
				Valores: Valores{ValorServico: 100},
			}

			err := nota.Validate()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("esperava nota valida: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("esperava erro mencionando %q: %v", tc.wantErr, err)
			}
		})
	}
}
