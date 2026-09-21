// Package municipio carries the IBGE municipality table — the seven digits
// that go in prestador.municipio and in the first positions of every access
// key.
//
// The table is embedded because the code has to be resolvable with no network
// at all. `nfse onboard` normally gets it from the public CNPJ registry, but
// --sem-rede exists precisely for whoever does not want to, or cannot, make
// that query, and leaving the hardest field blank there defeats the flag.
//
// Nothing here guesses. A name that matches two municipalities comes back as
// an ambiguity with the candidates, never as a pick.
package municipio

import (
	_ "embed"
	"encoding/csv"
	"fmt"
	"sort"
	"strings"

	"github.com/edusouza/nfse-emissor-go/internal/domain/texto"
)

//go:generate go run gerar_lista.go

// Anexo names the government spreadsheet municipios.csv was generated from.
const Anexo = "ANEXO_A-MUNICIPIO_IBGE-PAISES_ISO2-v1.00-SNNFSe-20251210.xlsx"

//go:embed municipios.csv
var listaCSV string

// Municipio is one row of the IBGE table.
type Municipio struct {
	// Codigo is the seven-digit IBGE code.
	Codigo string

	// Nome is the municipality's name as the annex writes it, accents and all.
	Nome string

	// UF is the two-letter state abbreviation.
	UF string
}

// String renders the municipality the way the CLI shows it: "Curitiba/PR".
func (m Municipio) String() string { return m.Nome + "/" + m.UF }

var (
	lista   []Municipio
	porCode map[string]Municipio

	// porNome indexes folded "nome" and folded "nome/uf". A bare name may hit
	// several municipalities — 232 names repeat across states — so the value is
	// a slice, and resolving without a UF is only unambiguous when it holds one.
	porNome map[string][]int
)

func init() {
	var err error
	if lista, err = parse(listaCSV); err != nil {
		// The file is embedded and generated: a failure here is a build that
		// should never have shipped, not a runtime condition to report.
		panic("municipio: municipios.csv invalida: " + err.Error())
	}

	porCode = make(map[string]Municipio, len(lista))
	porNome = make(map[string][]int, 2*len(lista))

	for i, m := range lista {
		porCode[m.Codigo] = m

		nome := chave(m.Nome)
		porNome[nome] = append(porNome[nome], i)
		porNome[nome+"/"+strings.ToLower(m.UF)] = append(porNome[nome+"/"+strings.ToLower(m.UF)], i)
	}
}

func parse(data string) ([]Municipio, error) {
	r := csv.NewReader(strings.NewReader(data))
	r.Comment = '#'
	r.FieldsPerRecord = 3

	records, err := r.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) < 2 {
		return nil, fmt.Errorf("esperava cabecalho e pelo menos um municipio, li %d linhas", len(records))
	}

	out := make([]Municipio, 0, len(records)-1)
	for _, rec := range records[1:] {
		if len(rec[0]) != 7 {
			return nil, fmt.Errorf("codigo %q nao tem 7 digitos", rec[0])
		}
		if len(rec[2]) != 2 {
			return nil, fmt.Errorf("codigo %s tem a UF %q, que nao tem 2 letras", rec[0], rec[2])
		}
		out = append(out, Municipio{Codigo: rec[0], Nome: rec[1], UF: rec[2]})
	}
	return out, nil
}

// Todos returns the whole table, in code order.
func Todos() []Municipio {
	out := make([]Municipio, len(lista))
	copy(out, lista)
	return out
}

// PorCodigo looks an IBGE code up, ignoring anything that is not a digit.
func PorCodigo(codigo string) (Municipio, bool) {
	m, ok := porCode[somenteDigitos(codigo)]
	return m, ok
}

// ErroAmbiguo reports a name that belongs to more than one municipality.
//
// It carries the candidates rather than a message: which of them to show, and
// how, is the caller's business.
type ErroAmbiguo struct {
	Consulta   string
	Candidatos []Municipio
}

func (e *ErroAmbiguo) Error() string {
	return fmt.Sprintf("%q existe em %d UFs", e.Consulta, len(e.Candidatos))
}

// ErroNaoEncontrado reports a name that matched nothing, with whatever near
// misses the search could offer.
type ErroNaoEncontrado struct {
	Consulta  string
	Sugestoes []Municipio
}

func (e *ErroNaoEncontrado) Error() string {
	return fmt.Sprintf("%q nao esta na tabela do IBGE", e.Consulta)
}

// Resolver turns what someone types into exactly one municipality.
//
// It accepts the seven-digit code, "Curitiba/PR", "Curitiba - PR",
// "Curitiba, PR" and a bare "Curitiba", with or without accents and in any
// case. A bare name is only accepted when it belongs to a single municipality;
// otherwise the ambiguity comes back with the candidates, because picking one
// would put the note in the wrong city and the Sefin would accept it.
func Resolver(consulta string) (Municipio, error) {
	limpa := strings.TrimSpace(consulta)
	if limpa == "" {
		return Municipio{}, &ErroNaoEncontrado{Consulta: consulta}
	}

	// A code is unambiguous and cheap to recognize, and it is what someone
	// pastes back out of a previous nfse.yaml.
	if digitos := somenteDigitos(limpa); len(digitos) == 7 && digitos == strings.Join(strings.Fields(limpa), "") {
		if m, ok := porCode[digitos]; ok {
			return m, nil
		}
		return Municipio{}, &ErroNaoEncontrado{Consulta: consulta}
	}

	nome, uf := separarUF(limpa)

	busca := chave(nome)
	if uf != "" {
		busca += "/" + strings.ToLower(uf)
	}

	switch indices := porNome[busca]; len(indices) {
	case 1:
		return lista[indices[0]], nil
	case 0:
		return Municipio{}, &ErroNaoEncontrado{Consulta: consulta, Sugestoes: Buscar(nome, 5)}
	default:
		candidatos := make([]Municipio, 0, len(indices))
		for _, i := range indices {
			candidatos = append(candidatos, lista[i])
		}
		sort.Slice(candidatos, func(i, j int) bool { return candidatos[i].UF < candidatos[j].UF })
		return Municipio{}, &ErroAmbiguo{Consulta: consulta, Candidatos: candidatos}
	}
}

// Buscar lists municipalities whose name contains the term, for when the exact
// name is not what the annex wrote.
func Buscar(termo string, limite int) []Municipio {
	nome, uf := separarUF(strings.TrimSpace(termo))

	alvo := chave(nome)
	if alvo == "" {
		return nil
	}
	uf = strings.ToUpper(uf)

	var exato, prefixo, contem []Municipio
	for _, m := range lista {
		if uf != "" && m.UF != uf {
			continue
		}
		nomeDobrado := chave(m.Nome)
		switch {
		case nomeDobrado == alvo:
			exato = append(exato, m)
		case strings.HasPrefix(nomeDobrado, alvo):
			prefixo = append(prefixo, m)
		case strings.Contains(nomeDobrado, alvo):
			contem = append(contem, m)
		}
	}

	// Three tiers, in the order someone means them: the municipality actually
	// called that, then the ones whose name starts with it, then the rest.
	// Without the first tier, "bom jesus" answers "Bom Jesus do Tocantins"
	// before the five municipalities simply named Bom Jesus.
	out := append(append(exato, prefixo...), contem...)
	if limite > 0 && len(out) > limite {
		out = out[:limite]
	}
	return out
}

// separarUF splits "Curitiba/PR" into name and state. The separator may be a
// slash, a dash or a comma, and the state may simply be absent.
func separarUF(consulta string) (nome, uf string) {
	for _, sep := range []string{"/", " - ", ","} {
		if i := strings.LastIndex(consulta, sep); i >= 0 {
			candidato := strings.TrimSpace(consulta[i+len(sep):])
			if len(candidato) == 2 && ehLetras(candidato) {
				return strings.TrimSpace(consulta[:i]), candidato
			}
		}
	}
	return consulta, ""
}

// chave folds a name into what the index is keyed by: letters and digits only,
// no accents and no case.
//
// Spaces, apostrophes and hyphens are dropped rather than normalised to a
// single separator, which is what makes "Alta Floresta D'Oeste", "Alta
// Floresta D Oeste" and "alta floresta doeste" one key instead of three.
// Keeping them would mean guessing which punctuation someone remembers, and
// the annex is full of it: apostrophes, hyphens and "do"/"d'" alike.
//
// The risk this takes is fusing two different municipalities of one state
// whose names differ only in punctuation. No such pair exists in the annex,
// and a test holds that.
func chave(nome string) string {
	var sb strings.Builder
	for _, r := range texto.Dobrar(nome) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			sb.WriteRune(r)
		}
	}
	return sb.String()
}

func ehLetras(s string) bool {
	for _, r := range s {
		if (r < 'a' || r > 'z') && (r < 'A' || r > 'Z') {
			return false
		}
	}
	return true
}

func somenteDigitos(s string) string {
	return strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, s)
}
