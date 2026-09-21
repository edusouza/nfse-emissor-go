// Copyright 2026 Eduardo Souza
// SPDX-License-Identifier: FSL-1.1-MIT

package xmlbuilder

import (
	"strconv"
	"testing"

	"github.com/beevik/etree"
)

// From the business-rules spreadsheet, docs/anexos/ANEXO_I-...xlsx:
//
//	E0121 — "Se o emitente da DPS for o prestador de serviço (tpEmit for igual
//	         a 1), então o nome ou razão social não deve ser informado."
//	E0122 — "Se o emitente da DPS não for o prestador de serviço (tpEmit for
//	         igual a 2 ou 3), então o nome ou razão social deve ser informado."
//
// The government already knows the name from the CNPJ when the provider is the
// one emitting; sending it anyway is a rejection, not a redundancy.
func TestBuildProvider_NameFollowsTheEmitterType(t *testing.T) {
	for _, tc := range []struct {
		name     string
		emitter  int
		wantName bool
	}{
		{"prestador emite (tpEmit=1)", EmitterTypeProvider, false},
		{"tomador emite (tpEmit=2)", EmitterTypeTaker, true},
		{"intermediario emite (tpEmit=3)", EmitterTypeIntermediary, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			xml := buildMinimalDPS(t, tc.emitter)

			doc := etree.NewDocument()
			if err := doc.ReadFromString(xml); err != nil {
				t.Fatal(err)
			}

			if got := doc.FindElement("DPS/infDPS/tpEmit"); got == nil {
				t.Fatal("tpEmit ausente")
			} else if got.Text() != itoa(tc.emitter) {
				t.Errorf("tpEmit = %q, esperava %d", got.Text(), tc.emitter)
			}

			el := doc.FindElement("DPS/infDPS/prest/xNome")
			switch {
			case tc.wantName && el == nil:
				t.Error("xNome ausente: a regra E0122 o exige quando o emitente nao e o prestador")
			case !tc.wantName && el != nil:
				t.Errorf("xNome presente (%q): a regra E0121 o proibe quando o prestador emite", el.Text())
			}
		})
	}
}

// The provider is still identified — by CNPJ, which is what the rule leaves in
// place. Dropping the name must not drop the identification with it.
func TestBuildProvider_KeepsTheCNPJ(t *testing.T) {
	doc := etree.NewDocument()
	if err := doc.ReadFromString(buildMinimalDPS(t, EmitterTypeProvider)); err != nil {
		t.Fatal(err)
	}

	el := doc.FindElement("DPS/infDPS/prest/CNPJ")
	if el == nil {
		t.Fatal("CNPJ do prestador ausente")
	}
	if el.Text() != "12345678000195" {
		t.Errorf("CNPJ = %q", el.Text())
	}
}

// buildMinimalDPS builds a DPS for a given tpEmit and returns its XML.
func buildMinimalDPS(t *testing.T, emitterType int) string {
	t.Helper()

	cfg := basicDPSConfig()
	cfg.EmitterType = emitterType
	cfg.Values.ServiceValue = 100

	built, err := NewDPSBuilder(cfg).Build()
	if err != nil {
		t.Fatalf("montagem falhou: %v", err)
	}
	return built.XML
}

func itoa(n int) string {
	return strconv.Itoa(n)
}
