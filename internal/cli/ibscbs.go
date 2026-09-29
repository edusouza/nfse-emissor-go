package cli

import (
	"fmt"
	"io"
	"time"
)

// anoIBSCBSSimples is when, per ANEXO I v1.01 (leiaute, group
// infDPS/IBSCBS), the IBS/CBS group stops being optional for Simples
// Nacional opters: "os grupos IBSCBS só serão obrigatórios a partir de
// 2027". The note gives the year only; the competence date is what the
// related rule E0850 measures, so it is what is compared here (ADR 0015).
const anoIBSCBSSimples = 2027

// avisarIBSCBS warns when an invoice falls where the IBS/CBS group becomes
// mandatory. The emitter writes layout 1.00, which cannot carry the group
// (E0854), and every provider it serves is a Simples opter.
//
// It is a warning, not a refusal: no rule in ANEXO I v1.01 rejects a DPS for
// the missing group yet, and refusing what the Sefin may still accept would
// be the emitter guessing. When the rejection comes, it comes with a code.
func avisarIBSCBS(errOut io.Writer, competencia time.Time) {
	// The calendar year of the date as written in dCompet, not an instant:
	// 22:00 on 31/12/2026 in Brasília is already 2027 in UTC, and still a
	// 2026 invoice.
	if competencia.Year() < anoIBSCBSSimples {
		return
	}
	fmt.Fprintf(errOut, "aviso: competencia %s: a partir de 2027 o ANEXO I (v1.01) torna o grupo IBSCBS "+
		"obrigatorio para optantes do Simples Nacional, e este emissor ainda gera a DPS 1.00, sem ele.\n"+
		"aviso: a nota segue assim mesmo; se a Sefin a recusar por isso, acompanhe "+
		"https://github.com/edusouza/nfse-emissor-go/issues/21\n",
		competencia.Format("02/01/2006"))
}
