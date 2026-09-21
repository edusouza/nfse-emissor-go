package servico

import (
	"math"
	"sort"
	"strings"

	"github.com/edusouza/nfse-emissor-go/internal/domain/texto"
)

// Resultado is one candidate of a search, with the score that ranked it.
//
// The score has no unit and is only comparable inside one search: it orders
// candidates, it does not measure how right any of them is.
type Resultado struct {
	Servico Servico
	Escore  float64
}

// Buscar ranks the list against free text and returns at most limite results.
//
// The ranking weighs a term by how rare it is in the list — "serviços",
// "congêneres" and "natureza" appear almost everywhere and separate nothing,
// while "veterinário" or "dragagem" picks out a handful. Terms also match by
// prefix, so "contabil" finds "contabilidade" without a stemmer.
//
// A zero-score match is never returned: an empty result is a better answer
// than an arbitrary one.
func Buscar(consulta string, limite int) []Resultado {
	termos := dedup(normalizar(consulta))
	if len(termos) == 0 {
		return nil
	}

	escores := make([]float64, len(lista))
	atingidos := make([]int, len(lista))
	for _, termo := range termos {
		for doc, peso := range pesosDoTermo(termo) {
			escores[doc] += peso
			atingidos[doc]++
		}
	}

	// How much of the query an entry answers matters as much as how rare the
	// words it answered are. Without this, one rare word carries an entry over
	// another that matched almost everything: "tecnologia da informação" is
	// written verbatim inside a code about vehicle tracking, and that alone
	// outranked the code that matches suporte, técnico and manutenção.
	for i := range escores {
		escores[i] *= float64(atingidos[i]) / float64(len(termos))
	}

	// The whole query appearing verbatim in a description is stronger evidence
	// than the same words scattered across it.
	if frase := strings.Join(termos, " "); len(termos) > 1 {
		for i, doc := range documentos {
			if strings.Contains(doc.texto, frase) {
				escores[i] *= 1.5
			}
		}
	}

	var out []Resultado
	for i, escore := range escores {
		if escore > 0 {
			out = append(out, Resultado{Servico: lista[i], Escore: escore})
		}
	}

	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Escore != out[j].Escore {
			return out[i].Escore > out[j].Escore
		}
		return out[i].Servico.Codigo < out[j].Servico.Codigo
	})

	// Everything far behind the leader is noise carried in by one common word.
	if len(out) > 0 {
		corte := out[0].Escore * 0.4
		for i, r := range out {
			if r.Escore < corte {
				out = out[:i]
				break
			}
		}
	}

	if limite > 0 && len(out) > limite {
		out = out[:limite]
	}
	return out
}

// SugerirPorCNAE ranks the list against the CNAE description from the public
// registry.
//
// This is a heuristic on words, not a mapping. The CNAE is the Receita
// Federal's economic activity classification and cTribNac is the service list
// of LC 116/2003: two taxonomies written for different purposes, with no
// official correspondence between them. Nothing published by the government
// converts one into the other, which is why this ranks candidates for a person
// to choose from and never fills the field.
func SugerirPorCNAE(descricaoCNAE string, limite int) []Resultado {
	return Buscar(descricaoCNAE, limite)
}

// documento is one indexed entry. The code's own description and the heading
// of the item it sits under are kept apart: a word in the description is what
// the code says, while a word in the heading is shared by every code of the
// item and only says which family it belongs to.
type documento struct {
	texto  string
	termos map[string]float64
}

// pesoGrupo discounts a term that appears only in the item heading. Without
// it, searching for "fotografia" ties every code of item 13, because the
// heading names fonografia, fotografia, cinematografia and reprografia at once.
const pesoGrupo = 0.5

var (
	documentos []documento

	// df counts how many entries contain each term, which is what makes a
	// common word weigh less than a rare one.
	df map[string]int
)

func indexar(servicos []Servico) {
	documentos = make([]documento, len(servicos))
	df = make(map[string]int)

	for i, s := range servicos {
		termos := make(map[string]float64)
		for _, t := range normalizar(s.Grupo) {
			termos[t] = pesoGrupo
		}
		for _, t := range normalizar(s.Descricao) {
			termos[t] = 1
		}
		for t := range termos {
			df[t]++
		}

		documentos[i] = documento{
			texto:  strings.Join(normalizar(s.Descricao+" "+s.Grupo), " "),
			termos: termos,
		}
	}
}

// pesosDoTermo scores every entry against one query term.
func pesosDoTermo(termo string) map[int]float64 {
	pesos := make(map[int]float64)

	for i, doc := range documentos {
		melhor := 0.0
		for t, campo := range doc.termos {
			forca := semelhanca(t, termo)
			if forca == 0 {
				continue
			}
			if peso := idf(t) * forca * campo; peso > melhor {
				melhor = peso
			}
		}
		if melhor > 0 {
			pesos[i] = melhor
		}
	}
	return pesos
}

// semelhanca grades how much two terms look like the same word, from 1 for an
// identical pair down to 0 for unrelated ones.
//
// This stands in for a stemmer. Portuguese inflects at the end of the word, so
// the shared head is most of the signal: "contabil" and "contabilidade" are
// one prefix apart, "cabelo" and "cabeleireiros" diverge only at the sixth
// letter. The thresholds are what keep "casa" away from "cassino".
func semelhanca(a, b string) float64 {
	const (
		minPrefixo = 4
		minComum   = 5
	)

	if a == b {
		return 1
	}
	if (len(a) >= minPrefixo && strings.HasPrefix(b, a)) ||
		(len(b) >= minPrefixo && strings.HasPrefix(a, b)) {
		// One word contains the other: "program" hits "programacao".
		return 0.8
	}
	if prefixoComum(a, b) >= minComum {
		// Neither contains the other, but they share a long head. Weaker,
		// because it is also how unrelated words collide.
		return 0.55
	}
	return 0
}

// prefixoComum returns how many leading bytes two terms share. Both have been
// through normalizar, which leaves only ASCII letters and digits, so bytes and
// characters are the same thing here.
func prefixoComum(a, b string) int {
	n := len(a)
	if len(b) < n {
		n = len(b)
	}
	for i := 0; i < n; i++ {
		if a[i] != b[i] {
			return i
		}
	}
	return n
}

func idf(termo string) float64 {
	n := len(documentos)
	freq := df[termo]
	if freq == 0 {
		freq = 1
	}
	return math.Log(1 + float64(n)/float64(freq))
}

// normalizar folds the text down to comparable terms: lowercase, without
// accents, without punctuation, and without the words that carry no meaning
// on their own.
func normalizar(entrada string) []string {
	var termos []string

	for _, campo := range strings.FieldsFunc(texto.Dobrar(entrada), func(r rune) bool {
		return !(r >= 'a' && r <= 'z') && !(r >= '0' && r <= '9')
	}) {
		if len(campo) < 3 || vazios[campo] || soDigitos(campo) {
			continue
		}
		termos = append(termos, campo)
	}
	return termos
}

func soDigitos(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func dedup(termos []string) []string {
	visto := make(map[string]bool, len(termos))
	out := termos[:0:0]
	for _, t := range termos {
		if !visto[t] {
			visto[t] = true
			out = append(out, t)
		}
	}
	return out
}

// vazios are the function words that would otherwise match everything. Words
// that are merely frequent — "serviços", "congêneres" — are not listed: the
// weighting already sinks them, and doing it by frequency keeps the list from
// having to be maintained.
var vazios = map[string]bool{
	"ate": true, "aos": true, "aquele": true, "aquela": true,
	"como": true, "com": true, "das": true, "dos": true, "essa": true,
	"esse": true, "esta": true, "este": true, "entre": true,
	"mais": true, "nao": true, "nas": true, "nos": true,
	"ou": true, "para": true, "pela": true, "pelo": true, "pelas": true, "pelos": true,
	"por": true, "qual": true, "quais": true, "quando": true, "que": true,
	"sem": true, "ser": true, "seu": true, "sua": true, "seus": true, "suas": true,
	"sob": true, "sobre": true, "toda": true, "todo": true, "todas": true, "todos": true,
	"uma": true, "uns": true, "umas": true,
}
