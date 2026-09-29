package ibge

import (
	"context"
	"net/http"
	"path/filepath"
	"strings"
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
	chamou := false
	client := servidor(t, func(http.ResponseWriter, *http.Request) { chamou = true })

	for _, uf := range []string{"", "XX", "P", "PR/../..", "41"} {
		if _, err := client.Municipios(context.Background(), uf); err == nil {
			t.Errorf("Municipios(%q) nao recusou", uf)
		}
	}
	if chamou {
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
	if !strings.HasSuffix(relido.caminho, NomeArquivo) {
		t.Errorf("caminho = %q", relido.caminho)
	}
}
