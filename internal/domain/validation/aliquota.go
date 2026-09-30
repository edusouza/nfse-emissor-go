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

	// ConvenioAtivo is whether the convênio of the municipality where the
	// ISSQN is due is active, or nil when that is not known. Only an ME/EPP
	// assessing outside the Simples depends on it (DependsOnConvenio); for
	// everyone else it is ignored.
	ConvenioAtivo *bool
}

// DependsOnConvenio reports whether the outcome of ValidateISSRate turns on
// the municipality's convênio, which is what decides whether asking the ADN is
// worth a round trip. It is true only for an ME/EPP assessing the ISSQN
// outside the Simples (rules E0635 and E0640).
func (ctx ISSRateContext) DependsOnConvenio() bool {
	return ctx.OpSimpNac == opSimpMEEPP && ctx.RegApTribSN != regApuracaoSN
}

// ValidateISSRate checks pAliq against the rules the Sefin applies on receipt,
// so that a declaration doomed to rejection never leaves the machine.
//
// The rules come from the "RN DPS_NFS-e" sheet of
// docs/anexos/ANEXO_I-SEFIN_ADN-DPS_NFSe-SNNFSe-v1.00-20251226.xlsx.
//
// Two of them, E0635 and E0640, depend on whether the municipality's convênio
// is active, which cannot be known offline. They only affect an ME/EPP
// assessing outside the Simples, and are checked when ConvenioAtivo says
// which; for everyone else the active and inactive cases agree, so the check
// is decisive without it.
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
			return validateOutsideSimples(ctx.ConvenioAtivo, declared)
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

// validateOutsideSimples applies E0635 and E0640, which the convênio decides:
// with it active the Sefin applies the municipality's own rate and refuses one
// declared; with it inactive there is no rate to apply, and one is required.
// When the convênio is not known, the rules disagree, and nothing is said.
func validateOutsideSimples(convenioAtivo *bool, declared bool) error {
	switch {
	case convenioAtivo == nil:
		return nil
	case *convenioAtivo && declared:
		return fmt.Errorf("com o convenio do municipio ativo, um ME/EPP que apura o ISSQN fora do Simples " +
			"nao informa aliquota: a Sefin aplica a que o municipio parametrizou (a Sefin rejeita: E0635). " +
			"Use iss_aliquota: 0")
	case !*convenioAtivo && !declared:
		return fmt.Errorf("sem convenio ativo no municipio, um ME/EPP que apura o ISSQN fora do Simples " +
			"precisa informar a aliquota (a Sefin rejeita: E0640). Use --iss-aliquota com a aliquota " +
			"que a prefeitura cobra")
	}
	return nil
}
