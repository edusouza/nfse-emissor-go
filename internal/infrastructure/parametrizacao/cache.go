package parametrizacao

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/edusouza/nfse-emissor-go/internal/infrastructure/arquivo"
)

// Validade is how long a remembered answer is used without asking again.
//
// A municipality changes its parameters whenever it likes and tells no one,
// so every day an answer is kept is a day it can be wrong. A day is what the
// author chose: it spares the ADN a round trip on every emission of the same
// day, and ages out before a change can outlive one working day. The cost of
// being wrong is bounded too: a stale convênio makes the Sefin reject the DPS,
// it never makes a wrong invoice (ADR 0017).
const Validade = 24 * time.Hour

// NomeArquivo is the file the cache lives in, inside the user's cache
// directory.
const NomeArquivo = "parametros.json"

// Resposta is an answer and where it came from, so that the user can be told
// how old it is.
type Resposta[T any] struct {
	Valor        T
	ConsultadoEm time.Time
	DoCache      bool
}

// entrada is one remembered answer. The key it is stored under says which
// route it answers, and so which of the two fields it must carry.
type entrada struct {
	ConsultadoEm time.Time `json:"consultado_em"`
	Convenio     *Convenio `json:"convenio,omitempty"`
	// Not omitempty: an empty list is an answer, and has to read back as one.
	Aliquotas []Aliquota `json:"aliquotas"`
}

// Cache remembers the ADN's answers on disk, for Validade.
//
// Only answers are kept: a refusal or a failure is asked again next time. An
// inactive convênio is an answer, and is kept like any other.
//
// Nothing here fails loudly. A cache that cannot be read or written costs a
// round trip, never a result.
type Cache struct {
	caminho string
	agora   func() time.Time

	mu        sync.Mutex
	carregado bool
	alterado  bool
	dados     map[string]entrada
}

// NovoCache builds a cache backed by caminho. An empty caminho puts it in the
// default directory; if even that cannot be determined, the cache lives for
// the run and never reaches disk.
func NovoCache(caminho string) *Cache {
	if caminho == "" {
		if dir, err := arquivo.DiretorioCache(); err == nil {
			caminho = filepath.Join(dir, NomeArquivo)
		}
	}
	return &Cache{caminho: caminho, agora: time.Now, dados: map[string]entrada{}}
}

// buscar returns a remembered answer that is still fresh.
func (c *Cache) buscar(chave string) (entrada, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.carregar()
	e, ok := c.dados[chave]
	if !ok || !c.fresca(e) {
		return entrada{}, false
	}
	return e, true
}

func (c *Cache) guardar(chave string, e entrada) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.carregar()
	c.dados[chave] = e
	c.alterado = true
}

// fresca reports whether an answer may still be used. One dated in the future
// — a clock set back, a hand-edited file — cannot say how old it is, and is
// not trusted.
func (c *Cache) fresca(e entrada) bool {
	agora := c.agora()
	return !e.ConsultadoEm.After(agora) && agora.Sub(e.ConsultadoEm) < Validade
}

// Gravar writes the cache out, if anything changed, dropping what has
// expired. The error is returned for callers that want to mention it;
// ignoring it is a valid choice.
func (c *Cache) Gravar() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.alterado || c.caminho == "" {
		return nil
	}

	for chave, e := range c.dados {
		if !c.fresca(e) {
			delete(c.dados, chave)
		}
	}

	// encoding/json writes map keys sorted, so two runs that learned the same
	// answers produce the same bytes.
	conteudo, err := json.MarshalIndent(c.dados, "", "  ")
	if err != nil {
		return err
	}
	if err := arquivo.GravarAtomico(c.caminho, append(conteudo, '\n')); err != nil {
		return err
	}

	c.alterado = false
	return nil
}

// carregar reads the file once. The caller holds the lock.
func (c *Cache) carregar() {
	if c.carregado {
		return
	}
	c.carregado = true

	if c.caminho == "" {
		return
	}
	conteudo, err := os.ReadFile(c.caminho)
	if err != nil {
		return
	}

	var lidos map[string]entrada
	if err := json.Unmarshal(conteudo, &lidos); err != nil {
		// A corrupt cache is discarded rather than repaired: it holds nothing
		// that cannot be asked for again.
		return
	}
	// The file is plain JSON in the user's cache directory, and anything can
	// have written it: entries are checked on the way in, not trusted. An
	// entry that fails is simply asked for again.
	for chave, e := range lidos {
		if coerente(chave, e) {
			c.dados[chave] = e
		}
	}
}

// coerente reports whether an entry answers the route its key names, with
// dates that hold together.
func coerente(chave string, e entrada) bool {
	if e.ConsultadoEm.IsZero() {
		return false
	}

	switch {
	case strings.HasSuffix(chave, rotaConvenio):
		return e.Convenio != nil && e.Aliquotas == nil
	case strings.HasSuffix(chave, rotaAliquota), strings.HasSuffix(chave, rotaHistoricoAliquotas):
		if e.Convenio != nil || e.Aliquotas == nil {
			return false
		}
		for _, a := range e.Aliquotas {
			if a.Inicio.IsZero() || (a.Fim != nil && a.Fim.Before(a.Inicio)) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

// Consulta answers from the cache when it can and from the ADN otherwise,
// remembering what the ADN said.
type Consulta struct {
	client *Client
	cache  *Cache

	// Renovar skips the remembered answers and asks the ADN, still keeping
	// what it says for next time.
	Renovar bool
}

// NovaConsulta wires a client and a cache together. A nil cache asks the ADN
// every time.
func NovaConsulta(client *Client, cache *Cache) *Consulta {
	return &Consulta{client: client, cache: cache}
}

// BaseURL reports where the lookups go.
func (c *Consulta) BaseURL() string { return c.client.BaseURL() }

// Convenio is Client.Convenio, remembered.
func (c *Consulta) Convenio(ctx context.Context, municipio string) (Resposta[*Convenio], error) {
	chave := c.client.BaseURL() + "/" + municipio + rotaConvenio
	if e, ok := c.lembrar(chave); ok {
		return Resposta[*Convenio]{Valor: e.Convenio, ConsultadoEm: e.ConsultadoEm, DoCache: true}, nil
	}

	convenio, err := c.client.Convenio(ctx, municipio)
	if err != nil {
		return Resposta[*Convenio]{}, err
	}
	agora := c.guardar(chave, entrada{Convenio: convenio})
	return Resposta[*Convenio]{Valor: convenio, ConsultadoEm: agora}, nil
}

// Aliquota is Client.Aliquota, remembered.
func (c *Consulta) Aliquota(ctx context.Context, municipio, codigo string, competencia time.Time) (Resposta[[]Aliquota], error) {
	chave := c.client.BaseURL() + "/" + municipio + "/" + codigo + "/" + competencia.Format(LayoutCompetencia) + rotaAliquota
	return c.aliquotas(chave, func() ([]Aliquota, error) {
		return c.client.Aliquota(ctx, municipio, codigo, competencia)
	})
}

// HistoricoAliquotas is Client.HistoricoAliquotas, remembered.
func (c *Consulta) HistoricoAliquotas(ctx context.Context, municipio, codigo string) (Resposta[[]Aliquota], error) {
	chave := c.client.BaseURL() + "/" + municipio + "/" + codigo + rotaHistoricoAliquotas
	return c.aliquotas(chave, func() ([]Aliquota, error) {
		return c.client.HistoricoAliquotas(ctx, municipio, codigo)
	})
}

func (c *Consulta) aliquotas(chave string, consultar func() ([]Aliquota, error)) (Resposta[[]Aliquota], error) {
	if e, ok := c.lembrar(chave); ok {
		return Resposta[[]Aliquota]{Valor: e.Aliquotas, ConsultadoEm: e.ConsultadoEm, DoCache: true}, nil
	}

	lista, err := consultar()
	if err != nil {
		return Resposta[[]Aliquota]{}, err
	}
	if lista == nil {
		// Stored as [] so that the entry reads back as an answer.
		lista = []Aliquota{}
	}
	agora := c.guardar(chave, entrada{Aliquotas: lista})
	return Resposta[[]Aliquota]{Valor: lista, ConsultadoEm: agora}, nil
}

func (c *Consulta) lembrar(chave string) (entrada, bool) {
	if c.cache == nil || c.Renovar {
		return entrada{}, false
	}
	return c.cache.buscar(chave)
}

// guardar remembers an answer and returns when it was obtained.
func (c *Consulta) guardar(chave string, e entrada) time.Time {
	if c.cache == nil {
		return time.Now()
	}
	e.ConsultadoEm = c.cache.agora()
	c.cache.guardar(chave, e)
	return e.ConsultadoEm
}
