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
	ctx     context.Context
	out     io.Writer
	errOut  io.Writer
	cache   *ibge.Cache
	client  *ibge.Client // nil under --sem-rede
	semRede bool
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
			return municipioInformado{}, fmt.Errorf("--municipio: %w", err)
		}
		m := municipioInformado{codigo: valor, uf: codmun.UF(valor), fonte: "--municipio"}
		if conhecido, ok := b.cache.Buscar(valor); ok {
			m.nome = conhecido.Nome
		}
		return m, nil
	}

	nome, uf, ok := separarUF(valor)
	if !ok {
		return municipioInformado{}, fmt.Errorf("--municipio %q: informe a UF junto do nome, como \"Curitiba/PR\" — "+
			"ha municipios com o mesmo nome em estados diferentes; ou use o codigo IBGE de 7 digitos", valor)
	}
	if texto.Chave(nome) == "" {
		return municipioInformado{}, fmt.Errorf("--municipio %q: falta o nome do municipio", valor)
	}

	if m, ok := b.cache.BuscarPorNome(nome, uf); ok {
		return municipioInformado{codigo: m.Codigo, nome: m.Nome, uf: m.UF, fonte: "--municipio"}, nil
	}

	if b.semRede || b.client == nil {
		return municipioInformado{}, fmt.Errorf("--municipio %q: sem rede, so da para achar um nome que ja esteja "+
			"no cache de municipios, e este nao esta; informe o codigo IBGE de 7 digitos, "+
			"ou rode uma vez sem --sem-rede", valor)
	}

	fmt.Fprintf(b.out, "Consultando os municipios de %s em %s para encontrar %q...\n", uf, b.client.Host(), nome)
	lista, err := b.client.Municipios(b.ctx, uf)
	if err != nil {
		return municipioInformado{}, fmt.Errorf("--municipio %q: a consulta falhou (%w); informe o codigo IBGE de 7 digitos",
			valor, err)
	}

	for _, m := range lista {
		b.cache.Guardar(m)
	}
	if err := b.cache.Gravar(); err != nil {
		fmt.Fprintf(b.errOut, "aviso: nao consegui gravar o cache de municipios: %v\n", err)
	}

	chave := texto.Chave(nome)
	for _, m := range lista {
		if texto.Chave(m.Nome) == chave {
			return municipioInformado{codigo: m.Codigo, nome: m.Nome, uf: m.UF, fonte: "--municipio, consultado em " + b.client.Host()}, nil
		}
	}

	msg := fmt.Sprintf("--municipio %q: %s nao tem municipio com esse nome", valor, uf)
	if parecidos := parecidos(lista, chave, 5); len(parecidos) > 0 {
		msg += "; voce quis dizer:\n  " + strings.Join(parecidos, "\n  ")
	}
	return municipioInformado{}, errors.New(msg)
}

// separarUF splits "Nome/UF", "Nome - UF" or "Nome, UF". The state is what
// comes after the last separator, and has to be one of the 27.
func separarUF(valor string) (nome, uf string, ok bool) {
	for _, sep := range []string{"/", " - ", ","} {
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
// typed — what a person who misspelled the end or the start of a name needs.
func parecidos(lista []ibge.Municipio, chave string, limite int) []string {
	var comeca, contem []string
	for _, m := range lista {
		k := texto.Chave(m.Nome)
		rotulo := fmt.Sprintf("%s  %s/%s", m.Codigo, m.Nome, m.UF)
		switch {
		case strings.HasPrefix(k, chave) || strings.HasPrefix(chave, k):
			comeca = append(comeca, rotulo)
		case strings.Contains(k, chave):
			contem = append(contem, rotulo)
		}
	}
	sort.Strings(comeca)
	sort.Strings(contem)

	out := append(comeca, contem...)
	if len(out) > limite {
		out = out[:limite]
	}
	return out
}
