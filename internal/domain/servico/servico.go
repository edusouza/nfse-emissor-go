// Package servico carries the national service list of LC 116/2003 — the
// table behind cTribNac, the six-digit code every DPS has to declare.
//
// The list is embedded because nothing answers it remotely. The municipality
// code comes from the public CNPJ registry and the razão social comes from the
// certificate, but no service, public or otherwise, knows what a given
// provider actually does. The table is 335 rows and changes at the pace of
// federal law, so carrying it costs 66 KB and saves the one lookup the emitter
// could not otherwise help with.
//
// Nothing here guesses. A search ranks candidates and a code is validated;
// which service was provided is the user's answer to give.
package servico

import (
	_ "embed"
	"encoding/csv"
	"fmt"
	"strings"
)

//go:generate go run gerar_lista.go

// Anexo names the government spreadsheet lista.csv was generated from. It is
// quoted in the CLI so that a code refused by the Sefin can be checked against
// the version this binary carries.
const Anexo = "anexo_b-nbs2-lista_servico_nacional-snnfse-v1-01-20260122.xlsx"

//go:embed lista.csv
var listaCSV string

// Servico is one leaf of the national service list: the codes a DPS may carry.
//
// The headings above it — the LC 116 item and subitem — are not codes and
// cannot be declared; only Grupo survives, as the item's name, because
// recognizing "Serviços de Informática e congêneres." is how someone finds
// their own trade among 335 entries.
type Servico struct {
	// Codigo is cTribNac: six digits, item + subitem + desdobro, two each.
	Codigo string

	// Grupo is the LC 116 item heading this code sits under.
	Grupo string

	// Descricao is the official text of the code.
	Descricao string
}

// Item is the LC 116 item number, without leading zeros: "1", "17".
func (s Servico) Item() string { return semZeros(s.Codigo[0:2]) }

// Subitem is the code in the notation the law uses: "1.01", "17.05".
func (s Servico) Subitem() string { return semZeros(s.Codigo[0:2]) + "." + s.Codigo[2:4] }

// Desdobro is the national breakdown of the subitem, without leading zeros.
// It exists only in the national list: the law stops at the subitem.
func (s Servico) Desdobro() string { return semZeros(s.Codigo[4:6]) }

func semZeros(digitos string) string {
	trimmed := strings.TrimLeft(digitos, "0")
	if trimmed == "" {
		return "0"
	}
	return trimmed
}

var (
	lista   []Servico
	porCode map[string]Servico
)

func init() {
	var err error
	if lista, err = parse(listaCSV); err != nil {
		// The file is embedded and generated: a failure here is a build that
		// should never have shipped, not a runtime condition to report.
		panic("servico: lista.csv invalida: " + err.Error())
	}

	porCode = make(map[string]Servico, len(lista))
	for _, s := range lista {
		porCode[s.Codigo] = s
	}

	indexar(lista)
}

func parse(data string) ([]Servico, error) {
	r := csv.NewReader(strings.NewReader(data))
	r.Comment = '#'
	r.FieldsPerRecord = 3

	records, err := r.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) < 2 {
		return nil, fmt.Errorf("esperava cabecalho e pelo menos um codigo, li %d linhas", len(records))
	}

	out := make([]Servico, 0, len(records)-1)
	for _, rec := range records[1:] {
		if len(rec[0]) != 6 {
			return nil, fmt.Errorf("codigo %q nao tem 6 digitos", rec[0])
		}
		out = append(out, Servico{Codigo: rec[0], Grupo: rec[1], Descricao: rec[2]})
	}
	return out, nil
}

// Todos returns the whole list, in code order.
func Todos() []Servico {
	out := make([]Servico, len(lista))
	copy(out, lista)
	return out
}

// PorCodigo looks a cTribNac up. It accepts the code with or without its
// leading zero, because that is how a spreadsheet shows it.
func PorCodigo(codigo string) (Servico, bool) {
	limpo := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, codigo)

	if len(limpo) == 5 {
		limpo = "0" + limpo
	}
	s, ok := porCode[limpo]
	return s, ok
}

// Grupo is one LC 116 item: the heading a set of codes sits under.
type Grupo struct {
	Item       string
	Nome       string
	Quantidade int
}

// Grupos lists the 41 items of the law, in order, so that someone whose words
// do not match anything can find their trade by reading the headings instead.
func Grupos() []Grupo {
	var out []Grupo
	for _, s := range lista {
		if n := len(out); n > 0 && out[n-1].Item == s.Item() {
			out[n-1].Quantidade++
			continue
		}
		out = append(out, Grupo{Item: s.Item(), Nome: s.Grupo, Quantidade: 1})
	}
	return out
}

// PorItem returns every code of one LC 116 item, accepting "8" or "08".
func PorItem(item string) []Servico {
	var out []Servico
	for _, s := range lista {
		if s.Item() == semZeros(item) {
			out = append(out, s)
		}
	}
	return out
}
