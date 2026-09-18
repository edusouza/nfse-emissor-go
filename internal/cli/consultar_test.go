package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/edusouza/nfse-emissor-go/internal/infrastructure/sefin"
)

// stubQuery starts a fake Sefin for query endpoints and points the CLI at it.
func stubQuery(t *testing.T, handler http.HandlerFunc) {
	t.Helper()

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	original := newSefinClient
	newSefinClient = func(cfg sefin.Config) (*sefin.Client, error) {
		cfg.BaseURL = srv.URL
		cfg.HTTPClient = srv.Client()
		return sefin.New(cfg)
	}
	t.Cleanup(func() { newSefinClient = original })
}

func runConsulta(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()
	t.Setenv(envCertPassword, testCertPassword)

	var out bytes.Buffer
	root := NewRootCommand()
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(append([]string{"consultar", "--config", filepath.Join(dir, "nfse.yaml")}, args...))

	err := root.Execute()
	return out.String(), err
}

func TestConsultar_PorChave(t *testing.T) {
	const nfseXML = `<?xml version="1.0"?><NFSe><infNFSe Id="NFS1"/></NFSe>`
	chave := strings.Repeat("5", 50)

	stubQuery(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			t.Errorf("metodo = %s", r.Method)
		}
		var buf bytes.Buffer
		zw := newGzipWriter(&buf)
		zw.Write([]byte(nfseXML))
		zw.Close()

		json.NewEncoder(w).Encode(map[string]any{
			"tipoAmbiente":          2,
			"versaoAplicativo":      "1.0.0",
			"dataHoraProcessamento": "2026-09-18T09:57:36-03:00",
			"chaveAcesso":           chave,
			"nfseXmlGZipB64":        encodeBase64(buf.Bytes()),
		})
	})

	dir := workspace(t)
	out, err := runConsulta(t, dir, chave)
	if err != nil {
		t.Fatalf("consulta falhou: %v\n%s", err, out)
	}

	saved, err := os.ReadFile(filepath.Join(dir, "notas", chave+"-nfse.xml"))
	if err != nil {
		t.Fatalf("XML nao foi gravado: %v", err)
	}
	if string(saved) != nfseXML {
		t.Error("o XML gravado nao confere com o recebido")
	}
	if !strings.Contains(out, "NFS-e encontrada") {
		t.Errorf("saida inesperada:\n%s", out)
	}
}

// TestConsultar_ChaveInvalida pins that a malformed key is caught locally,
// before a pointless round-trip to the government.
func TestConsultar_ChaveInvalida(t *testing.T) {
	var chamou bool
	stubQuery(t, func(w http.ResponseWriter, r *http.Request) { chamou = true })

	dir := workspace(t)
	_, err := runConsulta(t, dir, "123")
	if err == nil {
		t.Fatal("esperava erro para chave invalida")
	}
	if chamou {
		t.Error("a Sefin foi chamada com uma chave que nao tem 50 digitos")
	}
}

// TestConsultar_SigiloFiscal checks that a 403 is explained rather than passed
// through as a bare status.
func TestConsultar_SigiloFiscal(t *testing.T) {
	stubQuery(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	})

	dir := workspace(t)
	_, err := runConsulta(t, dir, strings.Repeat("5", 50))
	if err == nil {
		t.Fatal("esperava erro")
	}
	for _, want := range []string{"sigilo fiscal", "prestador"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("a mensagem deveria explicar a negativa (%q): %v", want, err)
		}
	}
}

func TestConsultar_PorDPS(t *testing.T) {
	chave := strings.Repeat("6", 50)

	stubQuery(t, func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]any{
			"tipoAmbiente":          2,
			"versaoAplicativo":      "1.0.0",
			"dataHoraProcessamento": "2026-09-18T09:57:36-03:00",
			"idDps":                 "DPS123",
			"chaveAcesso":           chave,
		})
	})

	dir := workspace(t)
	out, err := runConsulta(t, dir, "--dps", "DPS123")
	if err != nil {
		t.Fatalf("consulta falhou: %v\n%s", err, out)
	}
	if !strings.Contains(out, chave) {
		t.Errorf("a saida nao traz a chave de acesso:\n%s", out)
	}
}

func TestConsultar_Existe(t *testing.T) {
	cases := []struct {
		name   string
		status int
		want   string
	}{
		{name: "gerada", status: http.StatusOK, want: "gerou uma NFS-e"},
		{name: "nao gerada", status: http.StatusNotFound, want: "Nenhuma NFS-e"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			stubQuery(t, func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodHead {
					t.Errorf("metodo = %s, esperava HEAD", r.Method)
				}
				w.WriteHeader(tc.status)
			})

			dir := workspace(t)
			out, err := runConsulta(t, dir, "--dps", "DPS123", "--existe")
			// "No invoice was issued" is an answer, not a failure.
			if err != nil {
				t.Fatalf("consulta falhou: %v\n%s", err, out)
			}
			if !strings.Contains(out, tc.want) {
				t.Errorf("a saida nao menciona %q:\n%s", tc.want, out)
			}
		})
	}
}

func TestConsultar_ArgumentosIncompativeis(t *testing.T) {
	dir := workspace(t)

	cases := []struct {
		name string
		args []string
		want string
	}{
		{name: "sem chave e sem dps", args: nil, want: "--dps"},
		{name: "chave e dps juntos", args: []string{strings.Repeat("5", 50), "--dps", "X"}, want: "nunca os dois"},
		{name: "existe sem dps", args: []string{strings.Repeat("5", 50), "--existe"}, want: "--existe"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := runConsulta(t, dir, tc.args...); err == nil {
				t.Fatal("esperava erro")
			} else if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("a mensagem nao menciona %q: %v", tc.want, err)
			}
		})
	}
}
