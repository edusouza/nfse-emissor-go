package sefin

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The fixtures below mirror the schemas in docs/api/sefin-nacional-swagger.json:
// NFSePostRequest, NFSePostResponseSucesso, NFSePostResponseErro and
// ResponseErro. Keeping the test bodies faithful to the published shapes is the
// point — the previous client passed a large test suite while speaking a
// protocol the government does not implement.

const testSignedDPS = `<?xml version="1.0"?><DPS><infDPS Id="DPS123"/></DPS>`

// serve starts a stub Sefin and returns a client pointed at it.
func serve(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()

	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	client, err := New(Config{BaseURL: srv.URL, HTTPClient: srv.Client()})
	if err != nil {
		t.Fatal(err)
	}
	return client
}

func TestEmit_Success(t *testing.T) {
	const nfseXML = `<?xml version="1.0"?><NFSe><infNFSe Id="NFS123"/></NFSe>`

	var gotPath, gotContentType string
	var gotBody map[string]string

	client := serve(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotContentType = r.Header.Get("Content-Type")
		if err := json.NewDecoder(r.Body).Decode(&gotBody); err != nil {
			t.Errorf("corpo da requisicao invalido: %v", err)
		}

		encoded, _ := encodeGzipBase64([]byte(nfseXML))
		w.Header().Set("Content-Type", "application/json")
		// The specification answers 201 on success, not 200.
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{
			"tipoAmbiente":          EnvCodeRestrictedProduction,
			"versaoAplicativo":      "1.0.0",
			"dataHoraProcessamento": "2026-09-18T09:57:36.0593192-03:00",
			"idDps":                 "DPS123",
			"chaveAcesso":           strings.Repeat("1", 50),
			"nfseXmlGZipB64":        encoded,
			"alertas": []map[string]string{
				{"codigo": "A001", "descricao": "Aliquota presumida"},
			},
		})
	})

	result, err := client.Emit(context.Background(), []byte(testSignedDPS))
	if err != nil {
		t.Fatalf("Emit falhou: %v", err)
	}

	if gotPath != "/nfse" {
		t.Errorf("caminho = %q, esperava /nfse", gotPath)
	}
	if gotContentType != "application/json" {
		t.Errorf("Content-Type = %q", gotContentType)
	}

	// The DPS must travel gzipped and base64-encoded under the exact field name
	// the specification requires.
	encoded, ok := gotBody[fieldSignedDPS]
	if !ok {
		t.Fatalf("corpo nao contem %q; contem %v", fieldSignedDPS, keys(gotBody))
	}
	sent, err := decodeGzipBase64(encoded)
	if err != nil {
		t.Fatalf("o campo enviado nao decodifica: %v", err)
	}
	if string(sent) != testSignedDPS {
		t.Error("a DPS enviada nao confere com a original")
	}

	if string(result.NFSeXML) != nfseXML {
		t.Error("a NFS-e recebida nao foi descompactada corretamente")
	}
	if result.AccessKey != strings.Repeat("1", 50) {
		t.Errorf("chave de acesso = %q", result.AccessKey)
	}
	if result.DPSID != "DPS123" {
		t.Errorf("idDps = %q", result.DPSID)
	}
	if result.HasFiscalValue() {
		t.Error("producao restrita nao deveria ter valor fiscal")
	}
	if len(result.Warnings) != 1 {
		t.Errorf("esperava 1 alerta, obtive %d", len(result.Warnings))
	}
	if result.ProcessedAt.IsZero() {
		t.Error("dataHoraProcessamento nao foi lida")
	}
}

func TestEmit_RejectionListsEveryReason(t *testing.T) {
	client := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any{
			"tipoAmbiente":          EnvCodeRestrictedProduction,
			"versaoAplicativo":      "1.0.0",
			"dataHoraProcessamento": "2026-09-18T09:57:36-03:00",
			"idDPS":                 "DPS123",
			"erros": []map[string]string{
				{"codigo": "E001", "descricao": "cTribNac invalido", "complemento": "serv/cServ/cTribNac"},
				{"codigo": "E002", "descricao": "Aliquota nao informada"},
			},
		})
	})

	_, err := client.Emit(context.Background(), []byte(testSignedDPS))
	if err == nil {
		t.Fatal("esperava erro para DPS rejeitada")
	}
	if !errors.Is(err, ErrRejected) {
		t.Fatalf("erro nao casa com ErrRejected: %v", err)
	}

	var rejection *RejectionError
	if !errors.As(err, &rejection) {
		t.Fatalf("erro nao e *RejectionError: %T", err)
	}

	// Every reason must come back at once. Reporting one at a time would cost a
	// network round-trip per problem.
	if len(rejection.Rejections) != 2 {
		t.Fatalf("esperava 2 rejeicoes, obtive %d", len(rejection.Rejections))
	}
	msg := err.Error()
	for _, want := range []string{"E001", "cTribNac invalido", "serv/cServ/cTribNac", "E002"} {
		if !strings.Contains(msg, want) {
			t.Errorf("a mensagem nao menciona %q:\n%s", want, msg)
		}
	}
	if got := rejection.Codes(); len(got) != 2 || got[0] != "E001" {
		t.Errorf("Codes() = %v", got)
	}
}

// TestEmit_SingularErrorForm covers ResponseErro, which carries one "erro"
// object rather than an "erros" array. Reading only the plural form would lose
// the reason entirely on the endpoints that use it.
func TestEmit_SingularErrorForm(t *testing.T) {
	client := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnprocessableEntity)
		json.NewEncoder(w).Encode(map[string]any{
			"tipoAmbiente":          EnvCodeRestrictedProduction,
			"versaoAplicativo":      "1.0.0",
			"dataHoraProcessamento": "2026-09-18T09:57:36-03:00",
			"erro": map[string]string{
				"codigo": "E999", "descricao": "Regra de negocio violada",
			},
		})
	})

	_, err := client.Emit(context.Background(), []byte(testSignedDPS))
	if !errors.Is(err, ErrRejected) {
		t.Fatalf("erro nao casa com ErrRejected: %v", err)
	}
	if !strings.Contains(err.Error(), "E999") {
		t.Errorf("a mensagem nao traz o codigo: %v", err)
	}
}

func TestEmit_StatusMapping(t *testing.T) {
	cases := []struct {
		status int
		want   error
	}{
		{http.StatusForbidden, ErrForbidden},
		{http.StatusUnauthorized, ErrForbidden},
		{http.StatusNotFound, ErrNotFound},
		{http.StatusServiceUnavailable, ErrUnavailable},
	}

	for _, tc := range cases {
		t.Run(http.StatusText(tc.status), func(t *testing.T) {
			client := serve(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
			})
			_, err := client.Emit(context.Background(), []byte(testSignedDPS))
			if !errors.Is(err, tc.want) {
				t.Errorf("erro = %v, esperava %v", err, tc.want)
			}
		})
	}
}

func TestEmit_UnparseableBody(t *testing.T) {
	client := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("<html>Gateway error</html>"))
	})

	_, err := client.Emit(context.Background(), []byte(testSignedDPS))
	var httpErr *HTTPError
	if !errors.As(err, &httpErr) {
		t.Fatalf("esperava *HTTPError, obtive %T: %v", err, err)
	}
	if httpErr.StatusCode != http.StatusInternalServerError {
		t.Errorf("status = %d", httpErr.StatusCode)
	}
}

// TestEmit_DoesNotRetry pins a deliberate choice: emission is not idempotent,
// so a retry could issue a second invoice for the same service.
func TestEmit_DoesNotRetry(t *testing.T) {
	var calls int
	client := serve(t, func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.WriteHeader(http.StatusServiceUnavailable)
	})

	if _, err := client.Emit(context.Background(), []byte(testSignedDPS)); err == nil {
		t.Fatal("esperava erro")
	}
	if calls != 1 {
		t.Errorf("a Sefin foi chamada %d vezes; emissao nao pode ser repetida automaticamente", calls)
	}
}

func TestNew(t *testing.T) {
	t.Run("exige certificado", func(t *testing.T) {
		// Without a certificate there is no way to identify the issuer: the
		// national system has no API keys.
		if _, err := New(Config{Environment: EnvRestrictedProduction}); err == nil {
			t.Fatal("esperava erro sem certificado")
		}
	})

	t.Run("ambiente padrao e o restrito", func(t *testing.T) {
		client, err := New(Config{BaseURL: "http://exemplo", HTTPClient: http.DefaultClient})
		if err != nil {
			t.Fatal(err)
		}
		if client.Environment() != EnvRestrictedProduction {
			t.Errorf("ambiente padrao = %q, esperava %q", client.Environment(), EnvRestrictedProduction)
		}
	})

	t.Run("ambiente invalido", func(t *testing.T) {
		if _, err := New(Config{Environment: "caseiro", HTTPClient: http.DefaultClient}); err == nil {
			t.Fatal("esperava erro para ambiente invalido")
		}
	})

	t.Run("URLs por ambiente", func(t *testing.T) {
		for env, want := range map[string]string{
			EnvProduction:           ProductionBaseURL,
			EnvRestrictedProduction: RestrictedProductionBaseURL,
		} {
			client, err := New(Config{Environment: env, HTTPClient: http.DefaultClient})
			if err != nil {
				t.Fatal(err)
			}
			if client.BaseURL() != want {
				t.Errorf("%s: URL = %q, esperava %q", env, client.BaseURL(), want)
			}
		}
	})
}

// The Sefin requests the client certificate through TLS renegotiation rather
// than in the initial handshake, and Go refuses renegotiation unless asked. The
// default cost the first real emission a "local error: tls: no renegotiation".
//
// This asserts the setting rather than the handshake on purpose: Go's own TLS
// server cannot renegotiate, so no httptest server can reproduce what the
// government does. The assertion exists so that the setting is not tidied away
// by someone who has never seen that error.
func TestNew_AllowsTLSRenegotiation(t *testing.T) {
	cert := &tls.Certificate{}

	client, err := New(Config{Environment: EnvRestrictedProduction, Certificate: cert})
	if err != nil {
		t.Fatal(err)
	}

	transport, ok := client.httpClient.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("transporte = %T, esperava *http.Transport", client.httpClient.Transport)
	}
	if got := transport.TLSClientConfig.Renegotiation; got != tls.RenegotiateFreelyAsClient {
		t.Errorf("Renegotiation = %v, esperava RenegotiateFreelyAsClient", got)
	}
	if got := transport.TLSClientConfig.MinVersion; got != tls.VersionTLS12 {
		t.Errorf("MinVersion = %v, esperava TLS 1.2", got)
	}
}

func keys(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func TestFetchNFSe(t *testing.T) {
	const nfseXML = `<?xml version="1.0"?><NFSe><infNFSe Id="NFS1"/></NFSe>`
	accessKey := strings.Repeat("3", 50)

	var gotPath string
	client := serve(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		encoded, _ := encodeGzipBase64([]byte(nfseXML))
		json.NewEncoder(w).Encode(map[string]any{
			"tipoAmbiente":          EnvCodeProduction,
			"versaoAplicativo":      "1.0.0",
			"dataHoraProcessamento": "2026-09-18T09:57:36-03:00",
			"chaveAcesso":           accessKey,
			"nfseXmlGZipB64":        encoded,
		})
	})

	result, err := client.FetchNFSe(context.Background(), accessKey)
	if err != nil {
		t.Fatalf("FetchNFSe falhou: %v", err)
	}
	if want := "/nfse/" + accessKey; gotPath != want {
		t.Errorf("caminho = %q, esperava %q", gotPath, want)
	}
	if string(result.NFSeXML) != nfseXML {
		t.Error("a NFS-e nao foi descompactada corretamente")
	}
	if result.EnvironmentCode != EnvCodeProduction {
		t.Errorf("tipoAmbiente = %d", result.EnvironmentCode)
	}
}

// TestFetchNFSe_Forbidden covers fiscal secrecy: the government answers only
// for a certificate belonging to a party named on the invoice.
func TestFetchNFSe_Forbidden(t *testing.T) {
	client := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	})

	_, err := client.FetchNFSe(context.Background(), strings.Repeat("3", 50))
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("erro = %v, esperava ErrForbidden", err)
	}
}

func TestLookupDPS(t *testing.T) {
	accessKey := strings.Repeat("4", 50)

	var gotPath string
	client := serve(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		json.NewEncoder(w).Encode(map[string]any{
			"tipoAmbiente":          EnvCodeRestrictedProduction,
			"versaoAplicativo":      "1.0.0",
			"dataHoraProcessamento": "2026-09-18T09:57:36-03:00",
			"idDps":                 "DPS123",
			"chaveAcesso":           accessKey,
		})
	})

	result, err := client.LookupDPS(context.Background(), "DPS123")
	if err != nil {
		t.Fatalf("LookupDPS falhou: %v", err)
	}
	if gotPath != "/dps/DPS123" {
		t.Errorf("caminho = %q", gotPath)
	}
	if result.AccessKey != accessKey {
		t.Errorf("chave = %q", result.AccessKey)
	}
}

// TestLookupDPS_SingularErrorForm exercises ResponseErro on a lookup, which is
// where the single "erro" object appears rather than the "erros" array.
func TestLookupDPS_SingularErrorForm(t *testing.T) {
	client := serve(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any{
			"tipoAmbiente":          EnvCodeRestrictedProduction,
			"versaoAplicativo":      "1.0.0",
			"dataHoraProcessamento": "2026-09-18T09:57:36-03:00",
			"erro":                  map[string]string{"codigo": "E100", "descricao": "Identificador invalido"},
		})
	})

	_, err := client.LookupDPS(context.Background(), "invalido")
	if !errors.Is(err, ErrRejected) {
		t.Fatalf("erro = %v, esperava ErrRejected", err)
	}
	if !strings.Contains(err.Error(), "E100") {
		t.Errorf("a mensagem nao traz o codigo: %v", err)
	}
}

func TestDPSExists(t *testing.T) {
	cases := []struct {
		name    string
		status  int
		want    bool
		wantErr bool
	}{
		{name: "existe", status: http.StatusOK, want: true},
		{name: "nao existe", status: http.StatusNotFound, want: false},
		{name: "identificador invalido", status: http.StatusBadRequest, wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotMethod string
			client := serve(t, func(w http.ResponseWriter, r *http.Request) {
				gotMethod = r.Method
				w.WriteHeader(tc.status)
			})

			got, err := client.DPSExists(context.Background(), "DPS123")
			if tc.wantErr {
				if err == nil {
					t.Fatal("esperava erro")
				}
				return
			}
			if err != nil {
				t.Fatalf("DPSExists falhou: %v", err)
			}
			if gotMethod != http.MethodHead {
				t.Errorf("metodo = %q, esperava HEAD", gotMethod)
			}
			if got != tc.want {
				t.Errorf("existe = %v, esperava %v", got, tc.want)
			}
		})
	}
}

// TestClassify_ProxyVersusSefin403 separates a fiscal-secrecy refusal from a
// 403 produced by something between the client and the government.
//
// Both look identical at the status line, but they send the user to completely
// different places: one means "this certificate is not a party to the invoice",
// the other means "your network is blocking the request". Every documented
// Sefin response carries versaoAplicativo and dataHoraProcessamento, so their
// absence is the tell.
func TestClassify_ProxyVersusSefin403(t *testing.T) {
	t.Run("403 da Sefin", func(t *testing.T) {
		client := serve(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]any{
				"tipoAmbiente":          EnvCodeRestrictedProduction,
				"versaoAplicativo":      "1.0.0",
				"dataHoraProcessamento": "2026-09-18T09:57:36-03:00",
			})
		})

		err := mustFetchErr(t, client)
		if !errors.Is(err, ErrForbidden) {
			t.Fatalf("erro = %v, esperava ErrForbidden", err)
		}
		if strings.Contains(err.Error(), "proxy") {
			t.Errorf("uma recusa legitima da Sefin nao deveria sugerir proxy: %v", err)
		}
	})

	t.Run("403 de um intermediario", func(t *testing.T) {
		client := serve(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			w.Write([]byte("<html><body>Blocked by corporate proxy</body></html>"))
		})

		err := mustFetchErr(t, client)
		if !errors.Is(err, ErrForbidden) {
			t.Fatalf("erro = %v, esperava ErrForbidden", err)
		}
		if !strings.Contains(err.Error(), "proxy") {
			t.Errorf("a mensagem deveria levantar a hipotese de proxy: %v", err)
		}
	})

	t.Run("403 com erro detalhado vence o status", func(t *testing.T) {
		client := serve(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusForbidden)
			json.NewEncoder(w).Encode(map[string]any{
				"tipoAmbiente":          EnvCodeRestrictedProduction,
				"versaoAplicativo":      "1.0.0",
				"dataHoraProcessamento": "2026-09-18T09:57:36-03:00",
				"erro":                  map[string]string{"codigo": "E403", "descricao": "Ator nao consta na NFS-e"},
			})
		})

		err := mustFetchErr(t, client)
		// The government's own explanation is more useful than the sentinel.
		if !strings.Contains(err.Error(), "Ator nao consta") {
			t.Errorf("a explicacao da Sefin deveria aparecer: %v", err)
		}
	})
}

func mustFetchErr(t *testing.T, client *Client) error {
	t.Helper()

	_, err := client.FetchNFSe(context.Background(), strings.Repeat("1", 50))
	if err == nil {
		t.Fatal("esperava erro")
	}
	return err
}
