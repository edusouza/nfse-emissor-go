// Copyright 2026 Eduardo Souza
// SPDX-License-Identifier: FSL-1.1-MIT

package xmlbuilder

import (
	"testing"
	"time"

	"github.com/beevik/etree"
)

// The assertions below are grounded in tiposComplexos_v1.00.xsd, under
// docs/schemas. An earlier version of the builder produced a shape that looked
// plausible but that the schema does not accept: discounts nested inside
// vServPrest, xDescServ beside cServ instead of within it, totTrib as a sibling
// of trib, a tax base and an ISS amount that a DPS has no place for, and the
// municipal benefit element BM repurposed to carry them. These tests pin the
// real structure so that cannot come back.

// path reads the text of an element by its path, or reports a failure.
func path(t *testing.T, xmlStr, p string) string {
	t.Helper()

	doc := etree.NewDocument()
	if err := doc.ReadFromString(xmlStr); err != nil {
		t.Fatalf("XML gerado nao e parseavel: %v", err)
	}
	el := doc.FindElement(p)
	if el == nil {
		t.Fatalf("elemento %q nao encontrado em:\n%s", p, xmlStr)
	}
	return el.Text()
}

// absent asserts that no element exists at the given path.
func absent(t *testing.T, xmlStr, p string) {
	t.Helper()

	doc := etree.NewDocument()
	if err := doc.ReadFromString(xmlStr); err != nil {
		t.Fatalf("XML gerado nao e parseavel: %v", err)
	}
	if el := doc.FindElement(p); el != nil {
		t.Errorf("elemento %q nao deveria existir, mas contem %q", p, el.Text())
	}
}

func buildXML(t *testing.T, cfg DPSConfig) string {
	t.Helper()

	result, err := NewDPSBuilder(cfg).Build()
	if err != nil {
		t.Fatalf("Build falhou: %v", err)
	}
	return result.XML
}

func TestDPSBuilder_ServiceStructure(t *testing.T) {
	cfg := basicDPSConfig()
	cfg.Values = DPSValues{ServiceValue: 1000}
	xmlStr := buildXML(t, cfg)

	// TCServ: locPrest, cServ. TCCServ: cTribNac, cTribMun?, xDescServ, ...
	if got := path(t, xmlStr, "DPS/infDPS/serv/locPrest/cLocPrestacao"); got != "3550308" {
		t.Errorf("cLocPrestacao = %q", got)
	}
	if got := path(t, xmlStr, "DPS/infDPS/serv/cServ/cTribNac"); got != "123456" {
		t.Errorf("cTribNac = %q", got)
	}
	if got := path(t, xmlStr, "DPS/infDPS/serv/cServ/xDescServ"); got != "Test service" {
		t.Errorf("xDescServ = %q", got)
	}

	// xDescServ used to sit beside cServ, which the schema rejects.
	absent(t, xmlStr, "DPS/infDPS/serv/xDescServ")
	absent(t, xmlStr, "DPS/infDPS/serv/cLocPrest")
}

func TestDPSBuilder_ProviderRegime(t *testing.T) {
	t.Run("MEI", func(t *testing.T) {
		cfg := basicDPSConfig()
		cfg.Values = DPSValues{ServiceValue: 1000}
		xmlStr := buildXML(t, cfg)

		if got := path(t, xmlStr, "DPS/infDPS/prest/regTrib/opSimpNac"); got != "2" {
			t.Errorf("opSimpNac = %q, esperava 2 (MEI)", got)
		}
		// regEspTrib is mandatory and 0 is its valid "none" encoding, so it must
		// be present even though it is a zero value.
		if got := path(t, xmlStr, "DPS/infDPS/prest/regTrib/regEspTrib"); got != "0" {
			t.Errorf("regEspTrib = %q, esperava 0", got)
		}
		absent(t, xmlStr, "DPS/infDPS/prest/regTrib/regApTribSN")
	})

	t.Run("ME/EPP", func(t *testing.T) {
		cfg := basicDPSConfig()
		cfg.Provider.TaxRegime = "me_epp"
		cfg.Values = DPSValues{ServiceValue: 1000, ISSRate: 2}
		xmlStr := buildXML(t, cfg)

		if got := path(t, xmlStr, "DPS/infDPS/prest/regTrib/opSimpNac"); got != "3" {
			t.Errorf("opSimpNac = %q, esperava 3 (ME/EPP)", got)
		}
		if got := path(t, xmlStr, "DPS/infDPS/prest/regTrib/regApTribSN"); got != "1" {
			t.Errorf("regApTribSN = %q, esperava o padrao 1", got)
		}
	})
}

func TestDPSBuilder_Discounts(t *testing.T) {
	t.Run("sem descontos o elemento e omitido", func(t *testing.T) {
		cfg := basicDPSConfig()
		cfg.Values = DPSValues{ServiceValue: 1000}
		xmlStr := buildXML(t, cfg)

		if got := path(t, xmlStr, "DPS/infDPS/valores/vServPrest/vServ"); got != "1000.00" {
			t.Errorf("vServ = %q", got)
		}
		absent(t, xmlStr, "DPS/infDPS/valores/vDescCondIncond")
	})

	t.Run("descontos ficam em vDescCondIncond", func(t *testing.T) {
		cfg := basicDPSConfig()
		cfg.Values = DPSValues{
			ServiceValue:          1500,
			UnconditionalDiscount: 100,
			ConditionalDiscount:   50,
		}
		xmlStr := buildXML(t, cfg)

		if got := path(t, xmlStr, "DPS/infDPS/valores/vDescCondIncond/vDescIncond"); got != "100.00" {
			t.Errorf("vDescIncond = %q", got)
		}
		if got := path(t, xmlStr, "DPS/infDPS/valores/vDescCondIncond/vDescCond"); got != "50.00" {
			t.Errorf("vDescCond = %q", got)
		}
		// They used to be emitted inside vServPrest.
		absent(t, xmlStr, "DPS/infDPS/valores/vServPrest/vDescIncond")
		absent(t, xmlStr, "DPS/infDPS/valores/vServPrest/vDescCond")
	})
}

func TestDPSBuilder_Deductions(t *testing.T) {
	cfg := basicDPSConfig()
	cfg.Values = DPSValues{ServiceValue: 1000, Deductions: 200}
	xmlStr := buildXML(t, cfg)

	if got := path(t, xmlStr, "DPS/infDPS/valores/vDedRed/vDR"); got != "200.00" {
		t.Errorf("vDR = %q", got)
	}
	// 200 / 1000 = 20%
	if got := path(t, xmlStr, "DPS/infDPS/valores/vDedRed/pDR"); got != "20.00" {
		t.Errorf("pDR = %q, esperava 20.00", got)
	}
}

func TestDPSBuilder_TaxSection(t *testing.T) {
	t.Run("MEI sem aliquota", func(t *testing.T) {
		cfg := basicDPSConfig()
		cfg.Values = DPSValues{ServiceValue: 1000}
		xmlStr := buildXML(t, cfg)

		if got := path(t, xmlStr, "DPS/infDPS/valores/trib/tribMun/tribISSQN"); got != "1" {
			t.Errorf("tribISSQN = %q, esperava o padrao 1", got)
		}
		// tpRetISSQN is mandatory and used to be missing entirely.
		if got := path(t, xmlStr, "DPS/infDPS/valores/trib/tribMun/tpRetISSQN"); got != "1" {
			t.Errorf("tpRetISSQN = %q, esperava o padrao 1", got)
		}
		// An MEI pays ISS through the DAS, so no rate is declared.
		absent(t, xmlStr, "DPS/infDPS/valores/trib/tribMun/pAliq")
	})

	t.Run("aliquota informada", func(t *testing.T) {
		cfg := basicDPSConfig()
		cfg.Values = DPSValues{ServiceValue: 1000, ISSRate: 2.5, ISSRetention: 2}
		xmlStr := buildXML(t, cfg)

		if got := path(t, xmlStr, "DPS/infDPS/valores/trib/tribMun/pAliq"); got != "2.50" {
			t.Errorf("pAliq = %q", got)
		}
		if got := path(t, xmlStr, "DPS/infDPS/valores/trib/tribMun/tpRetISSQN"); got != "2" {
			t.Errorf("tpRetISSQN = %q, esperava 2 (retido pelo tomador)", got)
		}
	})

	t.Run("a DPS nao carrega base de calculo nem valor de ISS", func(t *testing.T) {
		cfg := basicDPSConfig()
		cfg.Values = DPSValues{ServiceValue: 1000, ISSRate: 2}
		xmlStr := buildXML(t, cfg)

		// The government computes these and returns them on the NFS-e.
		absent(t, xmlStr, "DPS/infDPS/valores/trib/tribMun/BM/vBCCalc")
		absent(t, xmlStr, "DPS/infDPS/valores/trib/tribMun/BM/vISS")
		absent(t, xmlStr, "DPS/infDPS/valores/trib/tribMun/BM")
	})
}

// Which child of the totTrib choice is allowed depends on the provider's
// standing in the Simples Nacional, not on what the caller knows. From the
// business-rules spreadsheet, docs/anexos/ANEXO_I-...xlsx:
//
//	E0710 — "Se a situação do emitente da DPS perante o Simples Nacional na
//	         data de competência informada for MEI, o choice pTotTribSN nunca
//	         poderá ser informado."
//	E0712 — "[...] for ME/EPP, o choice indTotTrib nunca poderá ser informado."
//
// This test used to assert the opposite for a MEI, because the builder chose by
// the configured value rather than by the regime.
func TestDPSBuilder_TotalTax(t *testing.T) {
	t.Run("MEI declara indTotTrib", func(t *testing.T) {
		cfg := basicDPSConfig() // TaxRegime: "mei"
		cfg.Values = DPSValues{ServiceValue: 1000}
		xmlStr := buildXML(t, cfg)

		if got := path(t, xmlStr, "DPS/infDPS/valores/trib/totTrib/indTotTrib"); got != "0" {
			t.Errorf("indTotTrib = %q, esperava 0", got)
		}
		absent(t, xmlStr, "DPS/infDPS/valores/trib/totTrib/pTotTribSN")
		// totTrib belongs inside trib, never one level up.
		absent(t, xmlStr, "DPS/infDPS/valores/totTrib")
	})

	t.Run("MEI nunca declara pTotTribSN, mesmo com percentual configurado", func(t *testing.T) {
		cfg := basicDPSConfig()
		cfg.Values = DPSValues{ServiceValue: 1000, TotalTaxPercentSN: 6}
		xmlStr := buildXML(t, cfg)

		absent(t, xmlStr, "DPS/infDPS/valores/trib/totTrib/pTotTribSN")
		if got := path(t, xmlStr, "DPS/infDPS/valores/trib/totTrib/indTotTrib"); got != "0" {
			t.Errorf("indTotTrib = %q, esperava 0", got)
		}
	})

	t.Run("ME/EPP declara pTotTribSN", func(t *testing.T) {
		cfg := basicDPSConfig()
		cfg.Provider.TaxRegime = "me_epp"
		cfg.Values = DPSValues{ServiceValue: 1000, TotalTaxPercentSN: 6}
		xmlStr := buildXML(t, cfg)

		if got := path(t, xmlStr, "DPS/infDPS/valores/trib/totTrib/pTotTribSN"); got != "6.00" {
			t.Errorf("pTotTribSN = %q, esperava 6.00", got)
		}
		absent(t, xmlStr, "DPS/infDPS/valores/trib/totTrib/indTotTrib")
	})

	t.Run("ME/EPP sem percentual declara pTotTribSN zero", func(t *testing.T) {
		cfg := basicDPSConfig()
		cfg.Provider.TaxRegime = "me_epp"
		cfg.Values = DPSValues{ServiceValue: 1000}
		xmlStr := buildXML(t, cfg)

		// A ME/EPP has no indTotTrib to opt out with, and the schema's pattern
		// for TSDec2V2 admits a bare zero.
		if got := path(t, xmlStr, "DPS/infDPS/valores/trib/totTrib/pTotTribSN"); got != "0.00" {
			t.Errorf("pTotTribSN = %q, esperava 0.00", got)
		}
		absent(t, xmlStr, "DPS/infDPS/valores/trib/totTrib/indTotTrib")
	})
}

func TestDPSBuilder_Substitution(t *testing.T) {
	t.Run("emissao comum omite subst", func(t *testing.T) {
		cfg := basicDPSConfig()
		cfg.Values = DPSValues{ServiceValue: 1000}
		xmlStr := buildXML(t, cfg)

		// subst used to be emitted as <subst>2</subst>, but the schema models it
		// as a structure carrying the replaced invoice's key.
		absent(t, xmlStr, "DPS/infDPS/subst")
	})

	t.Run("substituicao preenche a chave e o motivo", func(t *testing.T) {
		cfg := basicDPSConfig()
		cfg.Values = DPSValues{ServiceValue: 1000}
		cfg.Substitution = &DPSSubstitution{AccessKey: "chave-de-teste", ReasonCode: "01"}
		xmlStr := buildXML(t, cfg)

		if got := path(t, xmlStr, "DPS/infDPS/subst/chSubstda"); got != "chave-de-teste" {
			t.Errorf("chSubstda = %q", got)
		}
		if got := path(t, xmlStr, "DPS/infDPS/subst/cMotivo"); got != "01" {
			t.Errorf("cMotivo = %q", got)
		}
	})
}

func TestDPSBuilder_ElementOrder(t *testing.T) {
	cfg := basicDPSConfig()
	cfg.Values = DPSValues{ServiceValue: 1000, UnconditionalDiscount: 50, Deductions: 100}
	xmlStr := buildXML(t, cfg)

	doc := etree.NewDocument()
	if err := doc.ReadFromString(xmlStr); err != nil {
		t.Fatal(err)
	}

	// TCInfoValores declares a sequence, so order is part of validity.
	valores := doc.FindElement("DPS/infDPS/valores")
	var got []string
	for _, child := range valores.ChildElements() {
		got = append(got, child.Tag)
	}

	want := []string{"vServPrest", "vDescCondIncond", "vDedRed", "trib"}
	if len(got) != len(want) {
		t.Fatalf("filhos de valores = %v, esperava %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("filhos de valores = %v, esperava %v", got, want)
			break
		}
	}
}

// basicDPSConfig returns a minimal valid configuration for an MEI in Sao Paulo.
func basicDPSConfig() DPSConfig {
	return DPSConfig{
		Environment:        2,
		EmissionDateTime:   time.Date(2026, 1, 15, 10, 30, 0, 0, time.UTC),
		ApplicationVersion: "1.0.0",
		Series:             "00001",
		Number:             "123",
		CompetenceDate:     time.Date(2026, 1, 15, 0, 0, 0, 0, time.UTC),
		EmitterType:        1,
		MunicipalityCode:   "3550308",
		Provider: DPSProvider{
			CNPJ:      "12345678000195",
			Name:      "Test Provider Ltda",
			TaxRegime: "mei",
		},
		Service: DPSService{
			NationalCode:     "123456",
			Description:      "Test service",
			MunicipalityCode: "3550308",
		},
	}
}

func BenchmarkDPSBuilder_Build(b *testing.B) {
	cfg := basicDPSConfig()
	cfg.Values = DPSValues{
		ServiceValue:          1500.00,
		UnconditionalDiscount: 100.00,
		ConditionalDiscount:   50.00,
		Deductions:            200.00,
		ISSRate:               2.00,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = NewDPSBuilder(cfg).Build()
	}
}
