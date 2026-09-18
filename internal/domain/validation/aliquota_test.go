package validation

import (
	"strings"
	"testing"
)

// The cases below are taken from the "RN DPS_NFS-e" sheet of ANEXO_I. Catching
// them locally turns a network round-trip and a numeric rejection code into an
// immediate message that says what to do.
func TestValidateISSRate(t *testing.T) {
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
			// Rules E0635 and E0640 disagree depending on the convênio, which
			// cannot be known offline, so the check must not guess.
			name: "ME/EPP apurando fora do SN e deixado passar",
			ctx:  ISSRateContext{OpSimpNac: opSimpMEEPP, RegApTribSN: 2, TpRetISSQN: 1, Rate: 3},
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
