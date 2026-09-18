package cli

import (
	"bytes"
	"encoding/json"
	"errors"
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

// TestConsultar_403DeProxy checks that a 403 from something other than the
// government keeps its distinguishing detail. The fiscal-secrecy explanation is
// right when the Sefin refuses; it is a wild goose chase when a corporate proxy
// is what answered.
func TestConsultar_403DeProxy(t *testing.T) {
	stubQuery(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		w.Write([]byte("<html>Blocked</html>"))
	})

	dir := workspace(t)
	_, err := runConsulta(t, dir, strings.Repeat("5", 50))
	if err == nil {
		t.Fatal("esperava erro")
	}
	if !strings.Contains(err.Error(), "sigilo fiscal") {
		t.Errorf("a explicacao principal sumiu: %v", err)
	}
	if !strings.Contains(err.Error(), "proxy") {
		t.Errorf("a pista sobre o intermediario foi descartada: %v", err)
	}
}

// Consulting the same invoice twice is the obvious thing to do — the first
// query already wrote the file, and the user has no reason to think a second
// one is a problem. It used to fail, with an emission error that mentioned a
// DPS number and a --numero flag this command does not even have.
//
// A query is idempotent and the NFS-e is immutable at the government, so the
// second fetch brings back the same document.
func TestConsultar_PodeRepetir(t *testing.T) {
	const nfseXML = `<?xml version="1.0"?><NFSe><infNFSe Id="NFS1"/></NFSe>`
	chave := strings.Repeat("6", 50)

	stubQuery(t, func(w http.ResponseWriter, r *http.Request) {
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
	if out, err := runConsulta(t, dir, chave); err != nil {
		t.Fatalf("primeira consulta falhou: %v\n%s", out, err)
	}

	out, err := runConsulta(t, dir, chave)
	if err != nil {
		t.Fatalf("a segunda consulta deveria funcionar: %v\n%s", err, out)
	}

	saved, err := os.ReadFile(filepath.Join(dir, "notas", chave+"-nfse.xml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(saved) != nfseXML {
		t.Error("o XML gravado na segunda consulta nao confere")
	}
}

// The refusal that remains — for documents a repeat would genuinely destroy —
// must not borrow emission's vocabulary. Only the DPS write has a number to
// suggest another of.
func TestWriteNew_MensagemNaoAssumeEmissao(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ja-existe.xml")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	err := writeNew(path, []byte("y"), false)
	if err == nil {
		t.Fatal("esperava recusa")
	}
	if !errors.Is(err, errArquivoExistente) {
		t.Errorf("erro nao identificavel: %v", err)
	}
	for _, proibido := range []string{"--numero", "numero da DPS"} {
		if strings.Contains(err.Error(), proibido) {
			t.Errorf("a mensagem generica menciona %q: %v", proibido, err)
		}
	}
	if !strings.Contains(err.Error(), "--sobrescrever") {
		t.Errorf("a mensagem deveria dizer como prosseguir: %v", err)
	}
}

// The real service omits idDps, although DpsGetResponse marks it required. The
// first lookup against the government printed a blank line where the identifier
// belongs, and the existing test could not see it: its stub was written from
// the swagger, and here the swagger is what is wrong.
//
// The caller typed the identifier, so it is known without asking.
func TestConsultar_PorDPS_QuandoOServidorOmiteOIdentificador(t *testing.T) {
	chave := strings.Repeat("8", 50)
	const dpsID = "DPS410690221234567800019500001000000000000009"

	stubQuery(t, func(w http.ResponseWriter, r *http.Request) {
		// Faithful to what the Sefin actually answers: no idDps.
		json.NewEncoder(w).Encode(map[string]any{
			"tipoAmbiente":          2,
			"versaoAplicativo":      "1.0.0",
			"dataHoraProcessamento": "2026-09-18T09:57:36-03:00",
			"chaveAcesso":           chave,
		})
	})

	dir := workspace(t)
	out, err := runConsulta(t, dir, "--dps", dpsID)
	if err != nil {
		t.Fatalf("consulta falhou: %v\n%s", err, out)
	}

	if !strings.Contains(out, dpsID) {
		t.Errorf("a saida nao traz o identificador da DPS:\n%s", out)
	}
	if strings.Contains(out, "DPS              \n") {
		t.Errorf("a linha da DPS saiu vazia:\n%s", out)
	}
	if !strings.Contains(out, chave) {
		t.Errorf("a saida nao traz a chave de acesso:\n%s", out)
	}
}
