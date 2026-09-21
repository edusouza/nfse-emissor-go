package adn

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const chaveValida = "41069022212345678000195000000000000126081234567890"

// pdfFalso is the smallest thing that is recognizably a PDF.
var pdfFalso = []byte("%PDF-1.4\n1 0 obj\n<<>>\nendobj\ntrailer\n%%EOF\n")

func servidor(t *testing.T, h http.HandlerFunc) *Client {
	t.Helper()

	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)

	return New(Config{BaseURL: srv.URL, HTTPClient: srv.Client(), UserAgent: "teste"})
}

func TestBaixarDANFSe(t *testing.T) {
	var caminho string
	c := servidor(t, func(w http.ResponseWriter, r *http.Request) {
		caminho = r.URL.Path
		w.Header().Set("Content-Type", "application/pdf")
		_, _ = w.Write(pdfFalso)
	})

	pdf, err := c.BaixarDANFSe(context.Background(), chaveValida)
	if err != nil {
		t.Fatalf("falhou: %v", err)
	}
	if string(pdf) != string(pdfFalso) {
		t.Errorf("o conteudo voltou alterado")
	}
	// O manual diz GET /danfse/{chaveAcesso}; a chave vai no caminho, nao numa
	// query string.
	if caminho != "/"+chaveValida {
		t.Errorf("caminho = %q, esperava /%s", caminho, chaveValida)
	}
}

// Um 200 com corpo que nao e PDF e o caso perigoso: gravar isso entregaria ao
// usuario um arquivo que leitor nenhum abre e mensagem nenhuma explica.
func TestBaixarDANFSe_RecusaCorpoQueNaoEPDF(t *testing.T) {
	c := servidor(t, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"erro":{"codigo":"E999","descricao":"servico indisponivel"}}`))
	})

	_, err := c.BaixarDANFSe(context.Background(), chaveValida)
	if err == nil {
		t.Fatal("esperava erro para um corpo que nao e PDF")
	}
	for _, querido := range []string{"nao e um PDF", "application/json", "E999"} {
		if !strings.Contains(err.Error(), querido) {
			t.Errorf("o erro nao menciona %q: %v", querido, err)
		}
	}
}

func TestBaixarDANFSe_Status(t *testing.T) {
	casos := []struct {
		status   int
		corpo    string
		wantText string
	}{
		{http.StatusNotFound, `{"erro":"nao encontrada"}`, "nao encontrada no ADN"},
		{http.StatusForbidden, `acesso negado`, "manual dos municipios"},
		{http.StatusUnauthorized, ``, "manual dos municipios"},
		{http.StatusNotImplemented, `movido`, "informe o novo com --url"},
		{http.StatusBadGateway, ``, "indisponivel no momento"},
		{http.StatusBadRequest, `chave invalida`, "respondeu 400"},
	}

	for _, caso := range casos {
		c := servidor(t, func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(caso.status)
			_, _ = w.Write([]byte(caso.corpo))
		})

		_, err := c.BaixarDANFSe(context.Background(), chaveValida)
		if err == nil {
			t.Errorf("status %d: esperava erro", caso.status)
			continue
		}
		if !strings.Contains(err.Error(), caso.wantText) {
			t.Errorf("status %d: erro = %q, esperava conter %q", caso.status, err, caso.wantText)
		}
	}
}

// Uma chave malformada nao pode custar uma ida a rede.
func TestBaixarDANFSe_ValidaAChaveAntesDeSair(t *testing.T) {
	var chamou bool
	c := servidor(t, func(w http.ResponseWriter, _ *http.Request) {
		chamou = true
		_, _ = w.Write(pdfFalso)
	})

	for _, chave := range []string{"", "123", strings.Repeat("9", 51), "4106902221234567800019500000000000012608123456789X"} {
		if _, err := c.BaixarDANFSe(context.Background(), chave); err == nil {
			t.Errorf("a chave %q deveria ser recusada", chave)
		}
	}
	if chamou {
		t.Error("o cliente foi a rede com uma chave invalida")
	}
}

// A URL do ambiente e escolhida pelo tipoAmbiente, como no cliente da Sefin.
func TestBaseURLPorAmbiente(t *testing.T) {
	if got := New(Config{Environment: EnvCodeProduction}).BaseURL(); got != ProductionBaseURL {
		t.Errorf("producao = %q", got)
	}
	if got := New(Config{Environment: EnvCodeRestrictedProduction}).BaseURL(); got != RestrictedProductionBaseURL {
		t.Errorf("producao restrita = %q", got)
	}
	// Sem ambiente informado, o padrao seguro e o que nao tem valor fiscal.
	if got := New(Config{}).BaseURL(); got != RestrictedProductionBaseURL {
		t.Errorf("padrao = %q, esperava producao restrita", got)
	}
	if got := New(Config{BaseURL: "https://exemplo/danfse/"}).BaseURL(); got != "https://exemplo/danfse" {
		t.Errorf("--url = %q; a barra final deveria ser removida", got)
	}
}
