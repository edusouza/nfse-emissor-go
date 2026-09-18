package config

import (
	"strings"
	"testing"
)

// The generated file is read back by the emitter, so the only meaningful check
// is that it parses and validates — a template that renders prettily but drops
// a field would pass any string comparison.
func TestRenderOnboardedProduzConfiguracaoValida(t *testing.T) {
	rendered, err := RenderOnboarded(Onboarded{
		CertificadoArquivo: "./certificado.pfx",
		CNPJ:               "12345678000195",
		Nome:               "EMPRESA TESTE LTDA",
		RegimeTributario:   RegimeMEI,
		Municipio:          "3550308",
		Serie:              "00001",
		Origens:            []string{"prestador.cnpj: certificado ./certificado.pfx"},
	})
	if err != nil {
		t.Fatalf("render falhou: %v", err)
	}

	cfg, err := Decode(strings.NewReader(rendered))
	if err != nil {
		t.Fatalf("o arquivo gerado nao e um YAML valido: %v\n%s", err, rendered)
	}
	if err := cfg.Validate(); err != nil {
		t.Fatalf("o arquivo gerado nao passa no config check: %v\n%s", err, rendered)
	}

	if cfg.Prestador.CNPJ != "12345678000195" {
		t.Errorf("cnpj = %q", cfg.Prestador.CNPJ)
	}
	if cfg.Prestador.Municipio != "3550308" {
		t.Errorf("municipio = %q", cfg.Prestador.Municipio)
	}
	if cfg.Ambiente != EnvProducaoRestrita {
		t.Errorf("ambiente = %q, esperava o de testes por padrao", cfg.Ambiente)
	}
	if !strings.Contains(rendered, "#   prestador.cnpj: certificado ./certificado.pfx") {
		t.Errorf("a procedencia dos dados nao foi para o cabecalho:\n%s", rendered)
	}
}

// A series like "00001" must survive the round trip as text: read back as a
// number it becomes 1, and the DPS identifier needs the five digits.
func TestRenderOnboardedPreservaZerosAEsquerda(t *testing.T) {
	rendered, err := RenderOnboarded(Onboarded{CNPJ: "12345678000195", Serie: "00042", Municipio: "0000000"})
	if err != nil {
		t.Fatalf("render falhou: %v", err)
	}

	cfg, err := Decode(strings.NewReader(rendered))
	if err != nil {
		t.Fatalf("YAML invalido: %v", err)
	}
	if cfg.DPS.Serie != "00042" {
		t.Errorf("serie = %q, esperava 00042", cfg.DPS.Serie)
	}
	if cfg.Prestador.Municipio != "0000000" {
		t.Errorf("municipio = %q, esperava os zeros preservados", cfg.Prestador.Municipio)
	}
}

// Everything the lookup did not answer stays empty, and the file still parses:
// half a configuration is a head start, not an error.
func TestRenderOnboardedIncompletoAindaEValido(t *testing.T) {
	rendered, err := RenderOnboarded(Onboarded{CNPJ: "12345678000195"})
	if err != nil {
		t.Fatalf("render falhou: %v", err)
	}

	cfg, err := Decode(strings.NewReader(rendered))
	if err != nil {
		t.Fatalf("YAML invalido: %v\n%s", err, rendered)
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("esperava que o config check apontasse os campos que faltam")
	}
}

func TestRenderOnboardedEscapaAspas(t *testing.T) {
	rendered, err := RenderOnboarded(Onboarded{CNPJ: "12345678000195", Nome: `EMPRESA "X" LTDA`})
	if err != nil {
		t.Fatalf("render falhou: %v", err)
	}

	cfg, err := Decode(strings.NewReader(rendered))
	if err != nil {
		t.Fatalf("YAML invalido: %v\n%s", err, rendered)
	}
	if cfg.Prestador.Nome != `EMPRESA "X" LTDA` {
		t.Errorf("nome = %q", cfg.Prestador.Nome)
	}
}

func TestTaxRegimeFromSimples(t *testing.T) {
	sim, nao := true, false

	tests := []struct {
		name       string
		mei        *bool
		simples    *bool
		wantRegime string
		wantOK     bool
	}{
		{"mei", &sim, &sim, RegimeMEI, true},
		{"me ou epp no simples", &nao, &sim, RegimeMEEPP, true},
		{"fora do simples", &nao, &nao, "", false},
		{"cadastro nao informou", nil, nil, "", false},
		{"so o mei conhecido", &sim, nil, RegimeMEI, true},
		{"simples desconhecido", &nao, nil, "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			regime, ok := TaxRegimeFromSimples(tt.mei, tt.simples)
			if regime != tt.wantRegime || ok != tt.wantOK {
				t.Errorf("TaxRegimeFromSimples() = (%q, %v), esperava (%q, %v)", regime, ok, tt.wantRegime, tt.wantOK)
			}
		})
	}
}
