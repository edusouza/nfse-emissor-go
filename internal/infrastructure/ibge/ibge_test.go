package ibge

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// respostaCuritiba is the shape the IBGE service documents, cut down to what
// the emitter reads.
const respostaCuritiba = `{
  "id": 4106902,
  "nome": "Curitiba",
  "microrregiao": {
    "id": 41025,
    "nome": "Curitiba",
    "mesorregiao": {
      "id": 4110,
      "nome": "Metropolitana de Curitiba",
      "UF": { "id": 41, "sigla": "PR", "nome": "Paraná" }
    }
  }
}`

func servidor(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()

	servidor := httptest.NewServer(handler)
	t.Cleanup(servidor.Close)

	return New(Config{BaseURL: servidor.URL})
}

func TestConsultar_MunicipioConhecido(t *testing.T) {
	var caminho string
	client := servidor(t, func(w http.ResponseWriter, r *http.Request) {
		caminho = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(respostaCuritiba))
	})

	municipio, err := client.Consultar(context.Background(), "4106902")
	if err != nil {
		t.Fatalf("Consultar devolveu erro: %v", err)
	}

	if municipio.Nome != "Curitiba" || municipio.UF != "PR" {
		t.Errorf("veio %q / %q", municipio.Nome, municipio.UF)
	}
	if caminho != "/municipios/4106902" {
		t.Errorf("o caminho consultado foi %q", caminho)
	}
}

// The service has moved the state abbreviation between branches of its own
// response. Reading only one of them would lose the UF on half the answers.
func TestConsultar_UFPelaRegiaoImediata(t *testing.T) {
	const resposta = `{
	  "id": 3550308,
	  "nome": "São Paulo",
	  "regiao-imediata": {
	    "regiao-intermediaria": {
	      "UF": { "sigla": "SP" }
	    }
	  }
	}`

	client := servidor(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(resposta))
	})

	municipio, err := client.Consultar(context.Background(), "3550308")
	if err != nil {
		t.Fatalf("Consultar devolveu erro: %v", err)
	}
	if municipio.UF != "SP" {
		t.Errorf("UF veio %q", municipio.UF)
	}
}

// An unknown code comes back as an empty array with status 200, so a
// successful request is not by itself an answer.
func TestConsultar_CodigoDesconhecido(t *testing.T) {
	client := servidor(t, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("[]"))
	})

	if _, err := client.Consultar(context.Background(), "9999999"); err == nil {
		t.Fatal("esperava erro para um codigo que o servico nao conhece")
	}
}

func TestConsultar_CodigoMalformado(t *testing.T) {
	var chamou bool
	client := servidor(t, func(w http.ResponseWriter, r *http.Request) { chamou = true })

	if _, err := client.Consultar(context.Background(), "123"); err == nil {
		t.Fatal("esperava recusa de um codigo com menos de 7 digitos")
	}
	if chamou {
		t.Error("o servico foi chamado com um codigo que nem tem o tamanho certo")
	}
}

func TestConsultar_ErroDoServico(t *testing.T) {
	client := servidor(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})

	_, err := client.Consultar(context.Background(), "4106902")
	if err == nil {
		t.Fatal("esperava erro para um 500")
	}
	if !strings.Contains(err.Error(), "500") {
		t.Errorf("a mensagem nao diz o que aconteceu: %v", err)
	}
}

// The second printing of the same invoice must not depend on the network.
func TestConsulta_UsaOCacheNaSegundaVez(t *testing.T) {
	var chamadas int
	client := servidor(t, func(w http.ResponseWriter, r *http.Request) {
		chamadas++
		w.Write([]byte(respostaCuritiba))
	})

	arquivo := filepath.Join(t.TempDir(), "municipios.json")
	cache := NovoCache(arquivo)

	consulta := NovaConsulta(context.Background(), client, cache)
	if nome, uf, ok := consulta.Nome("4106902"); !ok || nome != "Curitiba" || uf != "PR" {
		t.Fatalf("primeira consulta veio %q / %q (ok=%v)", nome, uf, ok)
	}
	if err := cache.Gravar(); err != nil {
		t.Fatalf("Gravar devolveu erro: %v", err)
	}

	// A fresh cache reading the same file, and a client that would fail if it
	// were used at all.
	semRede := NovaConsulta(context.Background(), nil, NovoCache(arquivo))
	if nome, uf, ok := semRede.Nome("4106902"); !ok || nome != "Curitiba" || uf != "PR" {
		t.Fatalf("a segunda consulta nao veio do cache: %q / %q (ok=%v)", nome, uf, ok)
	}
	if chamadas != 1 {
		t.Errorf("o servico foi chamado %d vezes, esperava 1", chamadas)
	}
}

// Whatever fails, the answer is "I do not know" — never a panic and never an
// error that could stop a document from being printed.
func TestConsulta_FalhaNaoDerruba(t *testing.T) {
	client := servidor(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})

	consulta := NovaConsulta(context.Background(), client, NovoCache(filepath.Join(t.TempDir(), "c.json")))

	if _, _, ok := consulta.Nome("4106902"); ok {
		t.Error("uma consulta que falhou nao pode responder que sabe")
	}
	if len(consulta.Falhas()) != 1 {
		t.Errorf("esperava uma falha registrada, vieram %d", len(consulta.Falhas()))
	}
	if consulta.Consultou() {
		t.Error("uma consulta que falhou nao contou como consulta bem-sucedida")
	}
}

// A repeated failure is one message, not one per field on the page.
func TestConsulta_FalhasNaoSeRepetem(t *testing.T) {
	client := servidor(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	})

	consulta := NovaConsulta(context.Background(), client, nil)
	consulta.Nome("4106902")
	consulta.Nome("4106902")

	if len(consulta.Falhas()) != 1 {
		t.Errorf("esperava uma falha registrada, vieram %d: %q", len(consulta.Falhas()), consulta.Falhas())
	}
}

// A cache file that cannot be parsed is discarded, not fatal: it holds nothing
// that cannot be asked for again.
func TestCache_ArquivoCorrompido(t *testing.T) {
	arquivo := filepath.Join(t.TempDir(), "municipios.json")
	if err := os.WriteFile(arquivo, []byte("isto nao e json"), 0o644); err != nil {
		t.Fatalf("nao consegui preparar o arquivo: %v", err)
	}

	cache := NovoCache(arquivo)
	if _, ok := cache.Buscar("4106902"); ok {
		t.Error("um cache corrompido nao pode responder nada")
	}

	cache.Guardar(Municipio{Codigo: "4106902", Nome: "Curitiba", UF: "PR"})
	if err := cache.Gravar(); err != nil {
		t.Fatalf("Gravar devolveu erro: %v", err)
	}
	if _, ok := NovoCache(arquivo).Buscar("4106902"); !ok {
		t.Error("o cache deveria ter sido reescrito por cima do conteudo invalido")
	}
}

func TestConsulta_SemRedeNaoConsulta(t *testing.T) {
	consulta := NovaConsulta(context.Background(), nil, NovoCache(filepath.Join(t.TempDir(), "c.json")))

	if _, _, ok := consulta.Nome("4106902"); ok {
		t.Error("sem cliente e sem cache preenchido nao ha o que responder")
	}
	if len(consulta.Falhas()) != 0 {
		t.Errorf("nao consultar nao e falha: %q", consulta.Falhas())
	}
}
