package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/edusouza/nfse-emissor-go/internal/domain/texto"
	"github.com/edusouza/nfse-emissor-go/internal/infrastructure/ibge"
	"github.com/edusouza/nfse-emissor-go/pkg/codmun"
)

// municipioInformado is what --municipio turned into.
type municipioInformado struct {
	codigo string
	nome   string // empty when a bare code was given and nothing knew its name
	uf     string
	fonte  string
}

func (m municipioInformado) String() string {
	if m.nome == "" {
		return "IBGE " + m.codigo
	}
	return fmt.Sprintf("%s/%s (IBGE %s)", m.nome, m.uf, m.codigo)
}

// buscaMunicipio carries what resolving a name needs: where to ask, where to
// remember, and whether asking is allowed at all.
type buscaMunicipio struct {
	ctx    context.Context
	out    io.Writer
	errOut io.Writer
	cache  *ibge.Cache
	client *ibge.Client // nil under --sem-rede: the name resolves from the cache or not at all

	// origem names where the value came from, in messages and in the file's
	// header: "--municipio" for the flag, something else for a prompt.
	origem string
}

func (b buscaMunicipio) rotulo() string {
	if b.origem == "" {
		return "--municipio"
	}
	return b.origem
}

// resolver turns --municipio into a checked IBGE code.
//
// A seven-digit code is checked offline, by its check digit and state prefix.
// A name has to say its state — 232 names repeat across states, and choosing
// one would put the invoice in the wrong city with nothing complaining — and
// is looked up in the cache first, then in the state's list at the IBGE,
// which is remembered for next time. The table itself never ships in the
// binary (ADR 0012, ADR 0013).
func (b buscaMunicipio) resolver(valor string) (municipioInformado, error) {
	valor = strings.TrimSpace(valor)

	if valor != "" && strings.Trim(valor, "0123456789") == "" {
		if err := codmun.Validar(valor); err != nil {
			return municipioInformado{}, fmt.Errorf("%s: %w", b.rotulo(), err)
		}
		m := municipioInformado{codigo: valor, uf: codmun.UF(valor), fonte: b.rotulo()}
		if conhecido, ok := b.cache.Buscar(valor); ok {
			m.nome = conhecido.Nome
		}
		return m, nil
	}

	nome, uf, ok := separarUF(valor)
	if !ok {
		return municipioInformado{}, fmt.Errorf("%s %q: nao encontrei a UF; escreva como \"Curitiba/PR\" — "+
			"ha municipios com o mesmo nome em estados diferentes; ou use o codigo IBGE de 7 digitos", b.rotulo(), valor)
	}
	chave := texto.Chave(nome)
	if chave == "" {
		return municipioInformado{}, fmt.Errorf("%s %q: falta o nome do municipio", b.rotulo(), valor)
	}

	if m, ok := b.cache.BuscarPorNome(nome, uf); ok {
		return municipioInformado{codigo: m.Codigo, nome: m.Nome, uf: m.UF, fonte: b.rotulo()}, nil
	}

	if b.client == nil {
		return municipioInformado{}, fmt.Errorf("%s %q: sem rede, so da para achar um nome que ja esteja "+
			"no cache de municipios, e este nao esta; informe o codigo IBGE de 7 digitos, "+
			"ou rode uma vez sem --sem-rede", b.rotulo(), valor)
	}

	fmt.Fprintf(b.out, "Consultando os municipios de %s em %s para encontrar %q...\n", uf, b.client.Host(), nome)
	lista, err := b.client.Municipios(b.ctx, uf)
	if err != nil {
		return municipioInformado{}, fmt.Errorf("%s %q: a consulta falhou (%w); informe o codigo IBGE de 7 digitos",
			b.rotulo(), valor, err)
	}

	for _, m := range lista {
		b.cache.Guardar(m)
	}
	if err := b.cache.Gravar(); err != nil {
		fmt.Fprintf(b.errOut, "aviso: nao consegui gravar o cache de municipios: %v\n", err)
	}

	for _, m := range lista {
		if texto.Chave(m.Nome) == chave {
			return municipioInformado{codigo: m.Codigo, nome: m.Nome, uf: m.UF, fonte: b.rotulo() + ", consultado em " + b.client.Host()}, nil
		}
	}

	msg := fmt.Sprintf("%s %q: %s nao tem municipio com esse nome", b.rotulo(), valor, uf)
	if parecidos := parecidos(lista, chave, 5); len(parecidos) > 0 {
		msg += "; voce quis dizer:\n  " + strings.Join(parecidos, "\n  ")
	}
	return municipioInformado{}, errors.New(msg)
}

// separarUF splits "Nome/UF", "Nome - UF", "Nome-UF" or "Nome, UF". The state
// is what comes after the last separator, and has to be one of the 27: a
// hyphen inside a name, as in Embu-Guaçu, leaves a word that is not a state
// and the next separator is tried. No official name ends in a hyphen or a
// slash followed by a state's abbreviation, which a test holds to ANEXO_A.
func separarUF(valor string) (nome, uf string, ok bool) {
	for _, sep := range []string{"/", " - ", ",", "-"} {
		if i := strings.LastIndex(valor, sep); i >= 0 {
			nome = strings.TrimSpace(valor[:i])
			uf = strings.ToUpper(strings.TrimSpace(valor[i+len(sep):]))
			if codmun.UFExiste(uf) {
				return nome, uf, true
			}
		}
	}
	return "", "", false
}

// parecidos lists the names in a state that start with, or contain, what was
// typed — or that what was typed starts with, for an extra letter at the end
// — in alphabetical order, prefixes first.
func parecidos(lista []ibge.Municipio, chave string, limite int) []string {
	var comeca, contem []ibge.Municipio
	for _, m := range lista {
		k := texto.Chave(m.Nome)
		switch {
		case strings.HasPrefix(k, chave) || strings.HasPrefix(chave, k):
			comeca = append(comeca, m)
		case strings.Contains(k, chave):
			contem = append(contem, m)
		}
	}
	porNome := func(s []ibge.Municipio) {
		sort.Slice(s, func(i, j int) bool { return texto.Chave(s[i].Nome) < texto.Chave(s[j].Nome) })
	}
	porNome(comeca)
	porNome(contem)

	var out []string
	for _, m := range append(comeca, contem...) {
		if len(out) == limite {
			break
		}
		out = append(out, fmt.Sprintf("%s  %s/%s", m.Codigo, m.Nome, m.UF))
	}
	return out
}
