package config

import "testing"

// An unset regime de apuração has to mean "assessed under the Simples". It is
// the case of anyone within the Simples limits, and it is what decides whether
// an ISS rate may be declared at all — defaulting to zero would silently move
// the provider out of the Simples.
func TestRegimeApuracaoCode_DefaultsToSimples(t *testing.T) {
	for _, tc := range []struct {
		regime string
		want   int
	}{
		{"", 1},
		{ApuracaoSN, 1},
		{ApuracaoISSMunicipio, 2},
		{ApuracaoFora, 3},
	} {
		cfg := &Config{}
		cfg.Prestador.RegimeApuracao = tc.regime
		if got := cfg.RegimeApuracaoCode(); got != tc.want {
			t.Errorf("regime %q: regApTribSN = %d, esperava %d", tc.regime, got, tc.want)
		}
	}
}

func TestRetencaoCode_DefaultsToNenhuma(t *testing.T) {
	for _, tc := range []struct {
		retencao string
		want     int
		withheld bool
	}{
		{"", 1, false},
		{RetencaoNenhuma, 1, false},
		{RetencaoTomador, 2, true},
		{RetencaoIntermediario, 3, true},
	} {
		var n Nota
		n.Valores.RetencaoISSQN = tc.retencao
		if got := n.RetencaoCode(); got != tc.want {
			t.Errorf("retencao %q: tpRetISSQN = %d, esperava %d", tc.retencao, got, tc.want)
		}
		if got := n.HasRetencao(); got != tc.withheld {
			t.Errorf("retencao %q: HasRetencao = %v, esperava %v", tc.retencao, got, tc.withheld)
		}
	}
}
