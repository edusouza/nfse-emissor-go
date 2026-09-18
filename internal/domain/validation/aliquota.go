package validation

import "fmt"

// Regime codes as they appear in the DPS.
const (
	opSimpNaoOptante = 1
	opSimpMEI        = 2
	opSimpMEEPP      = 3

	regApuracaoSN = 1

	retencaoNenhuma = 1
)

// ISS rate bounds from the government's business rules.
const (
	// maxISSRate is the ceiling in rule E0595.
	maxISSRate = 5.0

	// minISSRateWithholding is the floor that rules E0621 and E0628 impose when
	// a Simples ME/EPP declares a rate because the ISSQN is withheld.
	minISSRateWithholding = 1.8
)

// ISSRateContext is what decides whether pAliq may, must or must not be sent.
type ISSRateContext struct {
	// OpSimpNac is the provider's Simples status: 1 not an opter, 2 MEI,
	// 3 ME/EPP.
	OpSimpNac int

	// RegApTribSN is the assessment regime, meaningful for ME/EPP.
	RegApTribSN int

	// TpRetISSQN is 1 when nobody withholds the ISSQN.
	TpRetISSQN int

	// Rate is the pAliq about to be declared. Zero means it will be omitted.
	Rate float64
}

// ValidateISSRate checks pAliq against the rules the Sefin applies on receipt,
// so that a declaration doomed to rejection never leaves the machine.
//
// The rules come from the "RN DPS_NFS-e" sheet of
// docs/anexos/ANEXO_I-SEFIN_ADN-DPS_NFSe-SNNFSe-v1.00-20251226.xlsx.
//
// Two of them depend on whether the municipality's convênio is active, which
// cannot be known offline. That only affects an ME/EPP assessing outside the
// Simples; for everyone else the active and inactive cases agree, so the check
// is decisive. See ValidateISSRate's warning return for the ambiguous case.
func ValidateISSRate(ctx ISSRateContext) error {
	declared := ctx.Rate > 0

	// E0595 — applies to everyone.
	if ctx.Rate > maxISSRate {
		return fmt.Errorf("aliquota de ISS de %.2f%% excede o maximo de %.0f%% (a Sefin rejeita: E0595)",
			ctx.Rate, maxISSRate)
	}

	switch ctx.OpSimpNac {
	case opSimpMEI:
		// E0600.
		if declared {
			return fmt.Errorf("MEI nao pode informar aliquota de ISS: o imposto ja esta no DAS " +
				"(a Sefin rejeita: E0600). Use iss_aliquota: 0")
		}

	case opSimpMEEPP:
		if ctx.RegApTribSN != regApuracaoSN {
			// Rules E0635 and E0640 disagree depending on the convênio, which
			// is not knowable here. Stay out of the way.
			return nil
		}

		if ctx.TpRetISSQN == retencaoNenhuma {
			// E0625 and E0631: forbidden whether or not the convênio is active.
			if declared {
				return fmt.Errorf("sem retencao do ISSQN, um ME/EPP do Simples nao pode informar aliquota " +
					"(a Sefin rejeita: E0625). Use iss_aliquota: 0, ou informe --retencao se o tomador retem")
			}
			return nil
		}

		// E0621 and E0628: required, whether or not the convênio is active.
		if !declared {
			return fmt.Errorf("com retencao do ISSQN, um ME/EPP do Simples precisa informar a aliquota " +
				"(a Sefin rejeita: E0621). Use --iss-aliquota")
		}
		if ctx.Rate < minISSRateWithholding {
			return fmt.Errorf("aliquota de %.2f%% e menor que o minimo de %.1f%% permitido com retencao "+
				"(a Sefin rejeita: E0621)", ctx.Rate, minISSRateWithholding)
		}
	}

	return nil
}
