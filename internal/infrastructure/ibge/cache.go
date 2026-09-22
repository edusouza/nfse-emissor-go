package ibge

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
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
func (c *Cache) Guardar(municipio Municipio) {
	if municipio.Codigo == "" {
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

// Gravar writes the cache out, if anything changed. The error is returned for
// callers that want to mention it; ignoring it is a valid choice.
func (c *Cache) Gravar() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.alterado || c.caminho == "" {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(c.caminho), 0o755); err != nil {
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
	if err := os.WriteFile(c.caminho, append(conteudo, '\n'), 0o644); err != nil {
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

	var lista []Municipio
	if err := json.Unmarshal(conteudo, &lista); err != nil {
		// A corrupt cache is discarded rather than repaired: it holds nothing
		// that cannot be asked for again.
		return
	}
	for _, municipio := range lista {
		if municipio.Codigo != "" {
			c.dados[municipio.Codigo] = municipio
		}
	}
}

// Consulta answers with a municipality's name, from the cache when it is known
// and from the service otherwise.
//
// It is the shape the DANFSe wants: a lookup that cannot fail. Whatever goes
// wrong, the answer is "I do not know", and the document prints the code.
type Consulta struct {
	ctx    context.Context
	client *Client
	cache  *Cache

	mu        sync.Mutex
	consultou bool
	falhas    []string
}

// NovaConsulta wires a client and a cache together. A nil client makes a
// lookup that only answers from the cache, which is what --sem-rede wants.
func NovaConsulta(ctx context.Context, client *Client, cache *Cache) *Consulta {
	return &Consulta{ctx: ctx, client: client, cache: cache}
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
	if c.client == nil {
		return "", "", false
	}

	municipio, err := c.client.Consultar(c.ctx, codigo)
	if err != nil {
		c.registrarFalha(err)
		return "", "", false
	}

	c.mu.Lock()
	c.consultou = true
	c.mu.Unlock()

	if c.cache != nil {
		c.cache.Guardar(*municipio)
	}
	return municipio.Nome, municipio.UF, true
}

// Consultou reports whether anything was actually asked of the service, so the
// caller can announce the lookup only when it happened.
func (c *Consulta) Consultou() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.consultou
}

// Falhas lists what went wrong, for a caller that wants to say so once.
func (c *Consulta) Falhas() []string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]string(nil), c.falhas...)
}

func (c *Consulta) registrarFalha(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	mensagem := err.Error()
	for _, existente := range c.falhas {
		if existente == mensagem {
			return
		}
	}
	c.falhas = append(c.falhas, mensagem)
}
