package brasilapi

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const cnpjValido = "19131243000197"

func TestConsultarCNPJ(t *testing.T) {
	var gotPath string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"cnpj": "19131243000197",
			"razao_social": "OPEN KNOWLEDGE BRASIL",
			"nome_fantasia": "REDE PELO CONHECIMENTO LIVRE",
			"municipio": "SAO PAULO",
			"uf": "SP",
			"codigo_municipio_ibge": 3550308,
			"descricao_situacao_cadastral": "ATIVA",
			"opcao_pelo_simples": true,
			"opcao_pelo_mei": false,
			"cnae_fiscal": 9430800,
			"cnae_fiscal_descricao": "Atividades de associacoes de defesa de direitos sociais"
		}`))
	}))
	defer srv.Close()

	empresa, err := New(Config{BaseURL: srv.URL}).ConsultarCNPJ(context.Background(), "19.131.243/0001-97")
	if err != nil {
		t.Fatalf("consulta falhou: %v", err)
	}

	if want := "/cnpj/v1/" + cnpjValido; gotPath != want {
		t.Errorf("caminho = %q, esperava %q", gotPath, want)
	}
	if empresa.RazaoSocial != "OPEN KNOWLEDGE BRASIL" {
		t.Errorf("razao social = %q", empresa.RazaoSocial)
	}
	if got := empresa.CodigoMunicipioIBGE.String(); got != "3550308" {
		t.Errorf("codigo IBGE = %q, esperava 3550308", got)
	}
	if !empresa.Ativa() {
		t.Error("esperava situacao cadastral ativa")
	}
	if empresa.OpcaoPeloMEI == nil || *empresa.OpcaoPeloMEI {
		t.Errorf("opcao_pelo_mei = %v, esperava false explicito", empresa.OpcaoPeloMEI)
	}
	if empresa.OpcaoPeloSimples == nil || !*empresa.OpcaoPeloSimples {
		t.Errorf("opcao_pelo_simples = %v, esperava true", empresa.OpcaoPeloSimples)
	}
}

// The registry is served by more than one implementation, and they disagree on
// whether numeric codes travel as numbers or as strings. A municipality code
// read as 0 would send every invoice to the wrong place.
func TestConsultarCNPJAceitaCodigosComoTextoOuNumero(t *testing.T) {
	tests := map[string]string{
		"numero":  `{"razao_social":"X","codigo_municipio_ibge":3550308}`,
		"texto":   `{"razao_social":"X","codigo_municipio_ibge":"3550308"}`,
		"ausente": `{"razao_social":"X"}`,
	}

	want := map[string]string{"numero": "3550308", "texto": "3550308", "ausente": ""}

	for name, body := range tests {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(body))
			}))
			defer srv.Close()

			empresa, err := New(Config{BaseURL: srv.URL}).ConsultarCNPJ(context.Background(), cnpjValido)
			if err != nil {
				t.Fatalf("consulta falhou: %v", err)
			}
			if got := empresa.CodigoMunicipioIBGE.String(); got != want[name] {
				t.Errorf("codigo IBGE = %q, esperava %q", got, want[name])
			}
		})
	}
}

func TestConsultarCNPJErros(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		body     string
		wantText string
	}{
		{"nao encontrado", http.StatusNotFound, `{"message":"CNPJ nao encontrado"}`, "nao encontrado na base"},
		{"limite de consultas", http.StatusTooManyRequests, "", "limitou as consultas"},
		{"indisponivel", http.StatusBadGateway, "", "indisponivel"},
		{"resposta vazia", http.StatusOK, `{}`, "nao trouxe razao social"},
		{"resposta nao e json", http.StatusOK, `<html>proxy</html>`, "nao e o JSON esperado"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			_, err := New(Config{BaseURL: srv.URL}).ConsultarCNPJ(context.Background(), cnpjValido)
			if err == nil {
				t.Fatal("esperava erro")
			}
			if !strings.Contains(err.Error(), tt.wantText) {
				t.Errorf("erro = %q, esperava conter %q", err, tt.wantText)
			}
		})
	}
}

// A CNPJ with wrong check digits never leaves the machine: the registry would
// answer 404 and the user would be left guessing which of the two is at fault.
func TestConsultarCNPJInvalidoNaoVaiParaRede(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("nao deveria ter feito requisicao")
	}))
	defer srv.Close()

	_, err := New(Config{BaseURL: srv.URL}).ConsultarCNPJ(context.Background(), "12345678000100")
	if err == nil || !strings.Contains(err.Error(), "invalido") {
		t.Fatalf("erro = %v, esperava recusa por CNPJ invalido", err)
	}
}

func TestHost(t *testing.T) {
	tests := map[string]string{
		"":                             "brasilapi.com.br",
		"https://brasilapi.com.br/api": "brasilapi.com.br",
		"http://127.0.0.1:8080":        "127.0.0.1:8080",
	}

	for baseURL, want := range tests {
		if got := New(Config{BaseURL: baseURL}).Host(); got != want {
			t.Errorf("Host(%q) = %q, esperava %q", baseURL, got, want)
		}
	}
}
