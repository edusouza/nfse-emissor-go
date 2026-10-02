package ibge

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/edusouza/nfse-emissor-go/internal/domain/texto"
)

// Cache remembers the municipalities already looked up, on disk.
//
// It exists for two reasons, and the second is the important one:
//
//   - a document with four people blocks would otherwise make up to four
//     requests every time it is printed;
//   - a DANFSe printed once has to be reprintable later. Municipality names do
//     not change from one day to the next, and keeping the answer means the
//     second printing of the same invoice does not depend on the network being
//     up, or on the service still existing.
//
// Nothing here fails loudly. A cache that cannot be read or written is a lost
// optimisation, never a reason to stop printing a fiscal document.
type Cache struct {
	caminho string

	mu        sync.Mutex
	carregado bool
	alterado  bool
	dados     map[string]Municipio
}

// NomeArquivo is the file the cache lives in, inside the user's cache
// directory.
const NomeArquivo = "municipios.json"

// DiretorioPadrao returns the directory the cache file belongs in, following
// whatever convention the operating system has for caches.
func DiretorioPadrao() (string, error) {
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "nfse"), nil
}

// NovoCache builds a cache backed by caminho. An empty caminho puts it in the
// default directory; if even that cannot be determined, the cache still works
// for the life of the process and simply never reaches disk.
func NovoCache(caminho string) *Cache {
	if caminho == "" {
		if dir, err := DiretorioPadrao(); err == nil {
			caminho = filepath.Join(dir, NomeArquivo)
		}
	}
	return &Cache{caminho: caminho, dados: map[string]Municipio{}}
}

// Buscar returns a remembered municipality.
func (c *Cache) Buscar(codigo string) (Municipio, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.carregar()
	municipio, ok := c.dados[codigo]
	return municipio, ok
}

// Guardar records a municipality, in memory now and on disk when Gravar runs.
// An entry that does not hold together is dropped (see coerente).
func (c *Cache) Guardar(municipio Municipio) {
	if !municipio.coerente() {
		return
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.carregar()
	if atual, ok := c.dados[municipio.Codigo]; ok && atual == municipio {
		return
	}
	c.dados[municipio.Codigo] = municipio
	c.alterado = true
}

// BuscarPorNome finds a remembered municipality by name within a state.
// The comparison folds case, accents and punctuation (texto.Chave), which the
// official table shows to be unambiguous inside one state. Two entries that
// answer the same name are not: the lookup then finds nothing, and the caller
// asks the service again.
func (c *Cache) BuscarPorNome(nome, uf string) (Municipio, bool) {
	chave := texto.Chave(nome)
	if chave == "" {
		return Municipio{}, false
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.carregar()
	var achado Municipio
	for _, municipio := range c.dados {
		if !strings.EqualFold(municipio.UF, uf) || texto.Chave(municipio.Nome) != chave {
			continue
		}
		if achado.Codigo != "" && achado.Codigo != municipio.Codigo {
			return Municipio{}, false
		}
		achado = municipio
	}
	return achado, achado.Codigo != ""
}

// Gravar writes the cache out, if anything changed. The error is returned for
// callers that want to mention it; ignoring it is a valid choice.
func (c *Cache) Gravar() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.alterado || c.caminho == "" {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(c.caminho), 0o700); err != nil {
		return err
	}

	// Sorted, so that a file a human opens reads in a sensible order and two
	// runs that learned the same municipalities produce the same bytes.
	codigos := make([]string, 0, len(c.dados))
	for codigo := range c.dados {
		codigos = append(codigos, codigo)
	}
	sort.Strings(codigos)

	lista := make([]Municipio, 0, len(codigos))
	for _, codigo := range codigos {
		lista = append(lista, c.dados[codigo])
	}

	conteudo, err := json.MarshalIndent(lista, "", "  ")
	if err != nil {
		return err
	}
	if err := gravarAtomico(c.caminho, append(conteudo, '\n')); err != nil {
		return err
	}

	c.alterado = false
	return nil
}

// gravarAtomico writes through a temporary file in the same directory and
// renames it over the destination.
//
// Writing in place truncates first: an interrupted run leaves half a JSON
// document, which carregar then throws away whole, and two runs at once
// interleave their bytes. A rename is atomic on the same filesystem, so a
// reader sees the old file or the new one, never a mix. Two concurrent runs
// still race — the last one wins — but each leaves a file that parses.
func gravarAtomico(caminho string, conteudo []byte) error {
	temporario, err := os.CreateTemp(filepath.Dir(caminho), filepath.Base(caminho)+".*.tmp")
	if err != nil {
		return err
	}
	nome := temporario.Name()

	// CreateTemp opens with 0600, which is kept. The cache holds only public
	// data, but it sits in the user's own cache directory and no other user
	// has a reason to read it, so it follows the rule of everything else the
	// CLI writes. The removals below are best effort: the write already
	// failed, and that is the error worth reporting.
	if _, err := temporario.Write(conteudo); err != nil {
		_ = temporario.Close()
		_ = os.Remove(nome)
		return err
	}
	if err := temporario.Close(); err != nil {
		_ = os.Remove(nome)
		return err
	}
	if err := os.Rename(nome, caminho); err != nil {
		_ = os.Remove(nome)
		return err
	}
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

	var lista []Municipio
	if err := json.Unmarshal(conteudo, &lista); err != nil {
		// A corrupt cache is discarded rather than repaired: it holds nothing
		// that cannot be asked for again.
		return
	}
	// The file is plain JSON in the user's cache directory, and anything
	// can have written it: entries are checked on the way in, not trusted.
	for _, municipio := range lista {
		if municipio.coerente() {
			c.dados[municipio.Codigo] = municipio
		}
	}
}

// Consulta answers with a municipality's name, from the cache when it is known
// and from the service otherwise.
//
// It is the shape the DANFSe wants: a lookup that cannot fail. Whatever goes
// wrong, the answer is "I do not know", and the document prints the code.
//
// It also bounds how long "I do not know" can take. A code that failed once is
// not asked again, and once the service cannot be reached at all — no route, a
// blocked proxy, a host that never answers — the client is dropped for the rest
// of the run. Without that, a document with four people blocks waited out the
// timeout four times over, to print four codes it could have printed at once.
type Consulta struct {
	ctx    context.Context
	client *Client
	cache  *Cache

	// antes runs once, right before the first request leaves the machine.
	antes func()

	mu          sync.Mutex
	anunciou    bool
	inacessivel bool
	falharam    map[string]bool
	falhas      []string
}

// NovaConsulta wires a client and a cache together. A nil client makes a
// lookup that only answers from the cache, which is what --sem-rede wants.
func NovaConsulta(ctx context.Context, client *Client, cache *Cache) *Consulta {
	return &Consulta{ctx: ctx, client: client, cache: cache, falharam: map[string]bool{}}
}

// AntesDeConsultar registers what to do right before the first code is sent
// to the service — announcing it, in the CLI. It does not run when every code
// is answered from the cache: nothing left the machine, so there is nothing to
// announce.
func (c *Consulta) AntesDeConsultar(f func()) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.antes = f
}

// Nome returns the municipality behind a code.
func (c *Consulta) Nome(codigo string) (string, string, bool) {
	codigo = strings.TrimSpace(codigo)
	if codigo == "" {
		return "", "", false
	}

	if c.cache != nil {
		if municipio, ok := c.cache.Buscar(codigo); ok {
			return municipio.Nome, municipio.UF, true
		}
	}
	if !c.deveConsultar(codigo) {
		return "", "", false
	}

	municipio, err := c.client.Consultar(c.ctx, codigo)
	if err != nil {
		c.registrarFalha(codigo, err)
		return "", "", false
	}

	if c.cache != nil {
		c.cache.Guardar(*municipio)
	}
	return municipio.Nome, municipio.UF, true
}

// deveConsultar decides whether a code goes to the service, and announces the
// first one that does.
func (c *Consulta) deveConsultar(codigo string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.client == nil || c.inacessivel || c.falharam[codigo] {
		return false
	}
	if !c.anunciou {
		c.anunciou = true
		if c.antes != nil {
			c.antes()
		}
	}
	return true
}

// Falhas lists what went wrong, for a caller that wants to say so once.
func (c *Consulta) Falhas() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.falhas...)
}

func (c *Consulta) registrarFalha(codigo string, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	c.falharam[codigo] = true
	if errors.Is(err, ErrInacessivel) {
		c.inacessivel = true
	}

	mensagem := err.Error()
	for _, existente := range c.falhas {
		if existente == mensagem {
			return
		}
	}
	c.falhas = append(c.falhas, mensagem)
}
