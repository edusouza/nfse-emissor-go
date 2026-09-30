package parametrizacao

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// SimNao is the API's TipoSimNao enum. The swagger lists 0, 1 and -1 without
// naming them; 1 = sim and 0 = não is what the active municipalities show
// (their adhesions come back 1, the MAN 0). -1 never appeared and its meaning
// is not published, so it is shown as the number rather than guessed at.
type SimNao int

func (s SimNao) String() string {
	switch s {
	case 1:
		return "sim"
	case 0:
		return "nao"
	default:
		return fmt.Sprintf("%d (valor sem significado publicado)", int(s))
	}
}

// Convenio is a municipality's standing with the national system.
type Convenio struct {
	// Ativo is what the business rules call a convênio "Ativo" (E0617,
	// E0621–E0640). The service answers an active municipality with its
	// parameters and an inactive one with 404 and the reason in Mensagem,
	// which is the only signal it gives: none of the fields below says it.
	Ativo bool

	// Mensagem is the service's own words, in either case.
	Mensagem string

	AderenteAmbienteNacional              SimNao
	AderenteEmissorNacional               SimNao
	SituacaoEmissaoPadraoContribuintesRFB SimNao
	AderenteMAN                           SimNao

	// PermiteAproveitamentoDeCreditos is nil when the service leaves it null.
	PermiteAproveitamentoDeCreditos *bool
}

// Aliquota is one period of a service's ISSQN rate in a municipality.
type Aliquota struct {
	// Incidencia is the service's own word for whether the ISSQN applies,
	// "SIM" in every answer seen so far.
	Incidencia string

	// Percentual is the rate in percent (5.00 is 5%), the unit iss_aliquota
	// uses. Nil when the service leaves it null.
	Percentual *float64

	// Inicio is the first day the rate applies.
	Inicio time.Time

	// Fim is the last day, or nil while the period is open.
	Fim *time.Time
}

// envelope is what every answer shares: the field the route is about, and a
// message. Refusals (400 and 404) set the field to null and explain themselves
// in mensagem.
type envelope struct {
	Mensagem string `json:"mensagem"`
}

type respostaConvenio struct {
	envelope
	ParametrosConvenio *struct {
		AderenteAmbienteNacional              SimNao `json:"aderenteAmbienteNacional"`
		AderenteEmissorNacional               SimNao `json:"aderenteEmissorNacional"`
		SituacaoEmissaoPadraoContribuintesRFB SimNao `json:"situacaoEmissaoPadraoContribuintesRFB"`
		AderenteMAN                           SimNao `json:"aderenteMAN"`
		// The misspelling ("Aproveitameto") is the API's.
		PermiteAproveitametoDeCreditos *bool `json:"permiteAproveitametoDeCreditos"`
		// The swagger also declares tipoConvenioDeserializationSetter; the
		// live service does not send it, so it is not read.
	} `json:"parametrosConvenio"`
}

// respostaAliquotas is keyed by the full service code ("01.07.01.000"). Only
// the fields of each rate come in PascalCase; the rest of the API is camelCase.
type respostaAliquotas struct {
	envelope
	Aliquotas map[string][]struct {
		Incidencia string   `json:"Incidencia"`
		Aliq       *float64 `json:"Aliq"`
		DtIni      data     `json:"DtIni"`
		DtFim      *data    `json:"DtFim"`
	} `json:"aliquotas"`
}

// problema is ASP.NET's standard validation error, which the service sends
// instead of its own envelope when a path value cannot even be read — an
// invalid competence date, for one.
type problema struct {
	Title  string              `json:"title"`
	Errors map[string][]string `json:"errors"`
}

// data reads the service's dates, which carry a time and no zone
// ("2023-01-02T00:00:00"). They are calendar dates, kept at midnight UTC so
// that comparing two of them never depends on the machine's zone.
type data struct{ time.Time }

var layoutsData = []string{
	"2006-01-02T15:04:05",
	"2006-01-02T15:04:05.999999999",
	time.RFC3339Nano,
	"2006-01-02",
}

func (d *data) UnmarshalJSON(b []byte) error {
	// By encoding/json's convention, null leaves the value alone; a nullable
	// date is decoded into a *data, which null leaves nil.
	if string(b) == "null" {
		return nil
	}
	var texto string
	if err := json.Unmarshal(b, &texto); err != nil {
		return fmt.Errorf("data %s nao e texto: %w", b, err)
	}
	texto = strings.TrimSpace(texto)
	for _, layout := range layoutsData {
		if t, err := time.Parse(layout, texto); err == nil {
			d.Time = time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
			return nil
		}
	}
	return fmt.Errorf("data %q fora dos formatos conhecidos", texto)
}
