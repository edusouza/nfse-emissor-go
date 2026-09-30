package ibge

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

// respostaParana is the list shape the service documents for a state, cut
// down to three municipalities.
const respostaParana = `[
  {"id": 4100103, "nome": "Abatiá", "microrregiao": {"mesorregiao": {"UF": {"sigla": "PR"}}}},
  {"id": 4106902, "nome": "Curitiba", "microrregiao": {"mesorregiao": {"UF": {"sigla": "PR"}}}},
  {"id": 4128633, "nome": "Rio Branco do Ivaí", "microrregiao": {"mesorregiao": {"UF": {"sigla": "PR"}}}}
]`

func TestMunicipios_DaUF(t *testing.T) {
	var caminho string
	client := servidor(t, func(w http.ResponseWriter, r *http.Request) {
		caminho = r.URL.Path
		w.Write([]byte(respostaParana))
	})

	municipios, err := client.Municipios(context.Background(), "pr")
	if err != nil {
		t.Fatalf("Municipios devolveu erro: %v", err)
	}
	if caminho != "/estados/PR/municipios" {
		t.Errorf("o caminho consultado foi %q", caminho)
	}
	if len(municipios) != 3 || municipios[1] != (Municipio{Codigo: "4106902", Nome: "Curitiba", UF: "PR"}) {
		t.Errorf("veio %+v", municipios)
	}
}

// A list that might put a name on the wrong code is worse than no list.
func TestMunicipios_RecusaRespostaIncoerente(t *testing.T) {
	casos := map[string]string{
		"codigo de outra UF":  `[{"id": 3550308, "nome": "São Paulo"}]`,
		"digito verificador":  `[{"id": 4106903, "nome": "Curitiba"}]`,
		"sem nome":            `[{"id": 4106902, "nome": " "}]`,
		"lista vazia":         `[]`,
		"objeto em vez lista": `{"id": 4106902, "nome": "Curitiba"}`,
	}
	for nome, corpo := range casos {
		t.Run(nome, func(t *testing.T) {
			client := servidor(t, func(w http.ResponseWriter, _ *http.Request) {
				w.Write([]byte(corpo))
			})
			if municipios, err := client.Municipios(context.Background(), "PR"); err == nil {
				t.Errorf("aceitou %+v", municipios)
			}
		})
	}
}

func TestMunicipios_UFInvalidaNaoSaiDaMaquina(t *testing.T) {
	var chamou atomic.Bool
	client := servidor(t, func(http.ResponseWriter, *http.Request) { chamou.Store(true) })

	for _, uf := range []string{"", "XX", "P", "PR/../..", "41"} {
		if _, err := client.Municipios(context.Background(), uf); err == nil {
			t.Errorf("Municipios(%q) nao recusou", uf)
		}
	}
	if chamou.Load() {
		t.Error("uma UF invalida chegou ao servico")
	}
}

func TestCache_BuscarPorNome(t *testing.T) {
	cache := NovoCache(filepath.Join(t.TempDir(), NomeArquivo))
	cache.Guardar(Municipio{Codigo: "4106902", Nome: "Curitiba", UF: "PR"})
	cache.Guardar(Municipio{Codigo: "1100015", Nome: "Alta Floresta D'Oeste", UF: "RO"})
	cache.Guardar(Municipio{Codigo: "2502201", Nome: "Bom Jesus", UF: "PB"})
	cache.Guardar(Municipio{Codigo: "4302303", Nome: "Bom Jesus", UF: "RS"})

	casos := []struct {
		nome, uf, quer string
	}{
		{"curitiba", "PR", "4106902"},
		{"CURITIBA", "pr", "4106902"},
		{"alta floresta doeste", "RO", "1100015"},
		{"Bom Jesus", "RS", "4302303"},
		{"Bom Jesus", "PB", "2502201"},
		{"Curitiba", "SP", ""},
		{"Curitib", "PR", ""},
		{"", "PR", ""},
		{"...", "PR", ""},
	}
	for _, c := range casos {
		m, ok := cache.BuscarPorNome(c.nome, c.uf)
		if ok != (c.quer != "") || m.Codigo != c.quer {
			t.Errorf("BuscarPorNome(%q, %q) = %+v, %v; esperava %q", c.nome, c.uf, m, ok, c.quer)
		}
	}

	// The lookup must survive a round-trip through the file, which is what
	// makes a name resolved once available to --sem-rede later.
	if err := cache.Gravar(); err != nil {
		t.Fatal(err)
	}
	relido := NovoCache(cache.caminho)
	if m, ok := relido.BuscarPorNome("Curitiba", "PR"); !ok || m.Codigo != "4106902" {
		t.Errorf("depois de gravar e reler: %+v, %v", m, ok)
	}
}

// The cache file is plain JSON in the user's cache directory. An entry that
// pairs a name with another state's code, a code that fails its check digit,
// or a name carrying escape sequences must never come back out of it.
func TestCache_DescartaEntradasQueNaoConferem(t *testing.T) {
	caminho := filepath.Join(t.TempDir(), NomeArquivo)
	envenenado := `[
		{"codigo": "3550308", "nome": "Curitiba", "uf": "PR"},
		{"codigo": "1234567", "nome": "Londrina", "uf": "PR"},
		{"codigo": "4105805", "nome": "Colombo\u001b[31m", "uf": "PR"},
		{"codigo": "4100103", "nome": "Abatiá", "uf": "PR"}
	]`
	if err := os.WriteFile(caminho, []byte(envenenado), 0o644); err != nil {
		t.Fatal(err)
	}

	cache := NovoCache(caminho)
	for _, nome := range []string{"Curitiba", "Londrina", "Colombo"} {
		if m, ok := cache.BuscarPorNome(nome, "PR"); ok {
			t.Errorf("BuscarPorNome(%q) devolveu uma entrada envenenada: %+v", nome, m)
		}
	}
	for _, codigo := range []string{"3550308", "1234567", "4105805"} {
		if m, ok := cache.Buscar(codigo); ok {
			t.Errorf("Buscar(%s) devolveu uma entrada envenenada: %+v", codigo, m)
		}
	}
	if m, ok := cache.BuscarPorNome("abatia", "PR"); !ok || m.Codigo != "4100103" {
		t.Errorf("a entrada valida se perdeu: %+v, %v", m, ok)
	}
}

func TestCache_GuardarRecusaEntradaIncoerente(t *testing.T) {
	cache := NovoCache(filepath.Join(t.TempDir(), NomeArquivo))
	cache.Guardar(Municipio{Codigo: "3550308", Nome: "Curitiba", UF: "PR"})
	cache.Guardar(Municipio{Codigo: "4106902", Nome: "Curitiba\a", UF: "PR"})

	if _, ok := cache.BuscarPorNome("Curitiba", "PR"); ok {
		t.Error("uma entrada incoerente entrou no cache")
	}
}

// Two entries answering one name within a state cannot both be right; the
// lookup finds nothing and the caller asks the service.
func TestCache_NomeAmbiguoNaoResolve(t *testing.T) {
	cache := NovoCache(filepath.Join(t.TempDir(), NomeArquivo))
	cache.Guardar(Municipio{Codigo: "4106902", Nome: "Curitiba", UF: "PR"})
	cache.Guardar(Municipio{Codigo: "4105805", Nome: "Curitiba", UF: "PR"})

	if m, ok := cache.BuscarPorNome("Curitiba", "PR"); ok {
		t.Errorf("resolveu um nome ambiguo para %+v", m)
	}
}

func TestMunicipios_RecusaNomeComCaracterDeControle(t *testing.T) {
	client := servidor(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(`[{"id": 4106902, "nome": "Curitiba\u001b]0;x\u0007"}]`))
	})
	if municipios, err := client.Municipios(context.Background(), "PR"); err == nil {
		t.Errorf("aceitou %+v", municipios)
	}
}

func TestMunicipios_RecusaRespostaGrandeDemais(t *testing.T) {
	client := servidor(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("[" + strings.Repeat(`{},`, maxListaSize/3+10) + "{}]"))
	})
	_, err := client.Municipios(context.Background(), "PR")
	if err == nil || !strings.Contains(err.Error(), "MB") {
		t.Errorf("esperava a recusa pelo tamanho; veio %v", err)
	}
}

// The state comes from the code; the answer only has to agree with it.
func TestConsultar_UFVemDoCodigo(t *testing.T) {
	casos := map[string]struct {
		corpo string
		quer  string // "" means an error
	}{
		"UF ausente":      {`{"id": 4106902, "nome": "Curitiba"}`, "PR"},
		"UF que discorda": {`{"id": 4106902, "nome": "Curitiba", "microrregiao": {"mesorregiao": {"UF": {"sigla": "SP"}}}}`, ""},
		"nome com escape": {`{"id": 4106902, "nome": "Curitiba\u001b[2J"}`, ""},
		"UF que concorda": {respostaCuritiba, "PR"},
	}
	for nome, c := range casos {
		t.Run(nome, func(t *testing.T) {
			client := servidor(t, func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte(c.corpo)) })
			m, err := client.Consultar(context.Background(), "4106902")
			switch {
			case c.quer == "" && err == nil:
				t.Errorf("aceitou %+v", m)
			case c.quer != "" && (err != nil || m.UF != c.quer):
				t.Errorf("Consultar = %+v, %v; esperava UF %s", m, err, c.quer)
			}
		})
	}
}
