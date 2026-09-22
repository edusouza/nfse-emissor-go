package danfsepdf

import "strings"

// quebrarEm splits text into exactly n lines, making the widest line as narrow
// as it can.
//
// Greedy wrapping — fill a line until the next word does not fit, then start
// another — is what most PDF libraries do, and it fails the one place NT 008
// counts lines: the note under the QR Code has to be "disposta em 3 (três)
// linhas" at six points inside a box 4,72 cm wide. Greedy wrapping turns that
// sentence into four lines, because it packs the first lines full and leaves
// the leftovers to spill. Balancing the three lines instead keeps every one of
// them under the width of the box.
//
// The search is exhaustive over the break points, which costs nothing for a
// sentence of two dozen words and removes any doubt about the result being the
// best split rather than a lucky one.
func quebrarEm(texto string, linhas int, largura func(string) float64) []string {
	palavras := strings.Fields(texto)
	if linhas <= 1 {
		return []string{strings.Join(palavras, " ")}
	}
	if len(palavras) <= linhas {
		return apenas(palavras, linhas)
	}

	// melhor[i][k] is the narrowest possible widest-line for the words from i
	// onwards, split into k lines.
	const infinito = 1e18
	melhor := make([][]float64, len(palavras)+1)
	corte := make([][]int, len(palavras)+1)
	for i := range melhor {
		melhor[i] = make([]float64, linhas+1)
		corte[i] = make([]int, linhas+1)
		for k := range melhor[i] {
			melhor[i][k] = infinito
		}
	}
	melhor[len(palavras)][0] = 0

	for i := len(palavras) - 1; i >= 0; i-- {
		for k := 1; k <= linhas; k++ {
			for fim := i + 1; fim <= len(palavras)-(k-1); fim++ {
				resto := melhor[fim][k-1]
				if resto >= infinito {
					continue
				}
				candidato := largura(strings.Join(palavras[i:fim], " "))
				if resto > candidato {
					candidato = resto
				}
				if candidato < melhor[i][k] {
					melhor[i][k] = candidato
					corte[i][k] = fim
				}
			}
		}
	}

	resultado := make([]string, 0, linhas)
	inicio := 0
	for k := linhas; k > 0; k-- {
		fim := corte[inicio][k]
		resultado = append(resultado, strings.Join(palavras[inicio:fim], " "))
		inicio = fim
	}
	return resultado
}

// apenas puts one word per line, padding with empty lines, for the degenerate
// cases where there is no choice to make.
func apenas(palavras []string, linhas int) []string {
	resultado := make([]string, 0, linhas)
	for _, palavra := range palavras {
		resultado = append(resultado, palavra)
	}
	for len(resultado) < linhas {
		resultado = append(resultado, "")
	}
	return resultado
}
