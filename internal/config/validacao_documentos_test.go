// Copyright 2026 Eduardo Souza
// SPDX-License-Identifier: FSL-1.1-MIT

package config

import (
	"strings"
	"testing"
)

// The emitter used to accept any digits as a CNPJ. The government rejects a
// document with bad check digits, so catching it locally turns a network
// round-trip and an opaque rejection into an immediate, specific message.

func TestValidate_RejectsInvalidProviderCNPJ(t *testing.T) {
	base := func() *Config {
		return &Config{
			Ambiente: EnvProducaoRestrita,
			Prestador: Prestador{
				CNPJ:             "12345678000195", // valid check digits
				Nome:             "EMPRESA EXEMPLO LTDA",
				RegimeTributario: RegimeMEI,
				Municipio:        "4106902",
			},
			DPS: DPS{Serie: "00001"},
		}
	}

	t.Run("aceita CNPJ valido", func(t *testing.T) {
		if err := base().Validate(); err != nil {
			t.Fatalf("esperava configuracao valida: %v", err)
		}
	})

	t.Run("recusa digitos verificadores errados", func(t *testing.T) {
		cfg := base()
		cfg.Prestador.CNPJ = "12345678000199"

		err := cfg.Validate()
		if err == nil {
			t.Fatal("esperava erro para CNPJ com digito verificador invalido")
		}
		if !strings.Contains(err.Error(), "digitos verificadores") {
			t.Errorf("a mensagem deveria explicar o problema: %v", err)
		}
	})
}

func TestNotaValidate_RejectsInvalidTakerDocument(t *testing.T) {
	base := func() Nota {
		return Nota{
			Numero:  "1",
			Servico: Servico{CodigoTributacaoNacional: "010101", Descricao: "Servico"},
			Valores: Valores{ValorServico: 100},
		}
	}

	cases := []struct {
		name    string
		tomador *Tomador
		wantErr string
	}{
		{
			name:    "CNPJ valido",
			tomador: &Tomador{CNPJ: "98765432000198", Nome: "CLIENTE SA"},
		},
		{
			name:    "CNPJ invalido",
			tomador: &Tomador{CNPJ: "98765432000188", Nome: "CLIENTE SA"},
			wantErr: "tomador.cnpj",
		},
		{
			name:    "CPF invalido",
			tomador: &Tomador{CPF: "12345678900", Nome: "CLIENTE"},
			wantErr: "tomador.cpf",
		},
		{
			name:    "nao identificado dispensa documento",
			tomador: &Tomador{NaoIdentificado: true},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			nota := base()
			nota.Tomador = tc.tomador

			err := nota.Validate()
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("esperava nota valida: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("esperava erro mencionando %q", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Errorf("a mensagem nao menciona %q: %v", tc.wantErr, err)
			}
		})
	}
}
