package validation

import (
	"strings"
	"testing"
)

// The cases below are taken from the "RN DPS_NFS-e" sheet of ANEXO_I. Catching
// them locally turns a network round-trip and a numeric rejection code into an
// immediate message that says what to do.
func TestValidateISSRate(t *testing.T) {
	ativo, inativo := true, false

	cases := []struct {
		name    string
		ctx     ISSRateContext
		wantErr string
	}{
		{
			name: "MEI sem aliquota",
			ctx:  ISSRateContext{OpSimpNac: opSimpMEI, TpRetISSQN: 1},
		},
		{
			// E0600: the ISS is already inside the MEI's DAS.
			name:    "MEI informando aliquota",
			ctx:     ISSRateContext{OpSimpNac: opSimpMEI, TpRetISSQN: 1, Rate: 2},
			wantErr: "E0600",
		},
		{
			// E0625/E0631 agree regardless of the convênio: forbidden.
			name:    "ME/EPP no SN, sem retencao, informando aliquota",
			ctx:     ISSRateContext{OpSimpNac: opSimpMEEPP, RegApTribSN: 1, TpRetISSQN: 1, Rate: 3},
			wantErr: "E0625",
		},
		{
			name: "ME/EPP no SN, sem retencao, sem aliquota",
			ctx:  ISSRateContext{OpSimpNac: opSimpMEEPP, RegApTribSN: 1, TpRetISSQN: 1},
		},
		{
			// E0621/E0628 agree regardless of the convênio: required.
			name:    "ME/EPP no SN, com retencao, sem aliquota",
			ctx:     ISSRateContext{OpSimpNac: opSimpMEEPP, RegApTribSN: 1, TpRetISSQN: 2},
			wantErr: "E0621",
		},
		{
			name: "ME/EPP no SN, com retencao, aliquota valida",
			ctx:  ISSRateContext{OpSimpNac: opSimpMEEPP, RegApTribSN: 1, TpRetISSQN: 2, Rate: 2.5},
		},
		{
			name:    "ME/EPP com retencao abaixo do minimo",
			ctx:     ISSRateContext{OpSimpNac: opSimpMEEPP, RegApTribSN: 1, TpRetISSQN: 2, Rate: 1.5},
			wantErr: "minimo",
		},
		{
			name: "ME/EPP com retencao exatamente no minimo",
			ctx:  ISSRateContext{OpSimpNac: opSimpMEEPP, RegApTribSN: 1, TpRetISSQN: 3, Rate: 1.8},
		},
		{
			// E0595 applies to everyone.
			name:    "acima de 5%",
			ctx:     ISSRateContext{OpSimpNac: opSimpMEEPP, RegApTribSN: 1, TpRetISSQN: 2, Rate: 5.01},
			wantErr: "E0595",
		},
		{
			name: "exatamente 5%",
			ctx:  ISSRateContext{OpSimpNac: opSimpMEEPP, RegApTribSN: 1, TpRetISSQN: 2, Rate: 5},
		},
		{
			// Rules E0635 and E0640 disagree depending on the convênio, so
			// without knowing it the check must not guess.
			name: "ME/EPP fora do SN, convenio desconhecido, com aliquota",
			ctx:  ISSRateContext{OpSimpNac: opSimpMEEPP, RegApTribSN: 2, TpRetISSQN: 1, Rate: 3},
		},
		{
			name: "ME/EPP fora do SN, convenio desconhecido, sem aliquota",
			ctx:  ISSRateContext{OpSimpNac: opSimpMEEPP, RegApTribSN: 3, TpRetISSQN: 1},
		},
		{
			// E0635: the Sefin applies the parameterised rate itself.
			name:    "ME/EPP fora do SN, convenio ativo, com aliquota",
			ctx:     ISSRateContext{OpSimpNac: opSimpMEEPP, RegApTribSN: 2, TpRetISSQN: 1, Rate: 3, ConvenioAtivo: &ativo},
			wantErr: "E0635",
		},
		{
			name: "ME/EPP fora do SN, convenio ativo, sem aliquota",
			ctx:  ISSRateContext{OpSimpNac: opSimpMEEPP, RegApTribSN: 3, TpRetISSQN: 2, ConvenioAtivo: &ativo},
		},
		{
			// E0640: there is no parameterised rate to apply.
			name:    "ME/EPP fora do SN, convenio inativo, sem aliquota",
			ctx:     ISSRateContext{OpSimpNac: opSimpMEEPP, RegApTribSN: 3, TpRetISSQN: 1, ConvenioAtivo: &inativo},
			wantErr: "E0640",
		},
		{
			name: "ME/EPP fora do SN, convenio inativo, com aliquota",
			ctx:  ISSRateContext{OpSimpNac: opSimpMEEPP, RegApTribSN: 2, TpRetISSQN: 1, Rate: 2, ConvenioAtivo: &inativo},
		},
		{
			// E0595 still comes first.
			name:    "ME/EPP fora do SN, convenio inativo, acima de 5%",
			ctx:     ISSRateContext{OpSimpNac: opSimpMEEPP, RegApTribSN: 2, TpRetISSQN: 1, Rate: 6, ConvenioAtivo: &inativo},
			wantErr: "E0595",
		},
		{
			// The convênio decides nothing inside the Simples: E0625 holds
			// either way.
			name:    "ME/EPP no SN, convenio inativo, sem retencao, com aliquota",
			ctx:     ISSRateContext{OpSimpNac: opSimpMEEPP, RegApTribSN: 1, TpRetISSQN: 1, Rate: 3, ConvenioAtivo: &inativo},
			wantErr: "E0625",
		},
		{
			name: "MEI ignora o convenio",
			ctx:  ISSRateContext{OpSimpNac: opSimpMEI, TpRetISSQN: 1, ConvenioAtivo: &inativo},
		},
		{
			name: "nao optante do Simples nao e barrado aqui",
			ctx:  ISSRateContext{OpSimpNac: opSimpNaoOptante, TpRetISSQN: 1, Rate: 3},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateISSRate(tc.ctx)

			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("esperava aceitar, recusou: %v", err)
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

// Only an ME/EPP assessing outside the Simples is worth a round trip to the
// ADN; everyone else is decided without it.
func TestDependsOnConvenio(t *testing.T) {
	cases := []struct {
		ctx  ISSRateContext
		want bool
	}{
		{ISSRateContext{OpSimpNac: opSimpMEI}, false},
		{ISSRateContext{OpSimpNac: opSimpMEEPP, RegApTribSN: 1}, false},
		{ISSRateContext{OpSimpNac: opSimpMEEPP, RegApTribSN: 2}, true},
		{ISSRateContext{OpSimpNac: opSimpMEEPP, RegApTribSN: 3}, true},
		{ISSRateContext{OpSimpNac: opSimpNaoOptante}, false},
	}
	for _, tc := range cases {
		if got := tc.ctx.DependsOnConvenio(); got != tc.want {
			t.Errorf("%+v: DependsOnConvenio = %v, esperava %v", tc.ctx, got, tc.want)
		}
	}
}
