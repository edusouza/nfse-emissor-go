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

	"github.com/beevik/etree"

	"github.com/edusouza/nfse-emissor-go/internal/infrastructure/sefin"
)

// stubSefin starts a fake Sefin Nacional and points the CLI at it for the
// duration of the test. The handler receives the decoded request body.
func stubSefin(t *testing.T, handler func(w http.ResponseWriter, body map[string]string)) {
	t.Helper()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("corpo invalido: %v", err)
		}
		handler(w, body)
	}))
	t.Cleanup(srv.Close)

	original := newSefinClient
	newSefinClient = func(cfg sefin.Config) (*sefin.Client, error) {
		cfg.BaseURL = srv.URL
		cfg.HTTPClient = srv.Client()
		return sefin.New(cfg)
	}
	t.Cleanup(func() { newSefinClient = original })
}

// respondSuccess writes a NFSePostResponseSucesso carrying the given invoice.
func respondSuccess(t *testing.T, w http.ResponseWriter, accessKey, nfseXML string) {
	t.Helper()

	var buf bytes.Buffer
	zw := newGzipWriter(&buf)
	if _, err := zw.Write([]byte(nfseXML)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]any{
		"tipoAmbiente":          2,
		"versaoAplicativo":      "1.0.0",
		"dataHoraProcessamento": "2026-09-18T09:57:36-03:00",
		"idDps":                 "DPS123",
		"chaveAcesso":           accessKey,
		"nfseXmlGZipB64":        encodeBase64(buf.Bytes()),
	})
}

func TestEmitir_Enviar(t *testing.T) {
	const nfseXML = `<?xml version="1.0"?><NFSe><infNFSe Id="NFS1"/></NFSe>`
	accessKey := strings.Repeat("7", 50)

	var receivedDPS string
	stubSefin(t, func(w http.ResponseWriter, body map[string]string) {
		receivedDPS = body["dpsXmlGZipB64"]
		respondSuccess(t, w, accessKey, nfseXML)
	})

	dir := workspace(t)
	out, err := runEmit(t, dir, "--numero", "50", "--valor", "1200",
		"--descricao", "Servico enviado", "--enviar")
	if err != nil {
		t.Fatalf("emissao falhou: %v\n%s", err, out)
	}

	// The Sefin must have received the signed declaration, not the draft.
	if receivedDPS == "" {
		t.Fatal("a Sefin nao recebeu o campo dpsXmlGZipB64")
	}
	sent := decodeForTest(t, receivedDPS)
	if !strings.Contains(sent, "<Signature") {
		t.Error("a DPS enviada nao esta assinada")
	}

	doc := etree.NewDocument()
	if err := doc.ReadFromString(sent); err != nil {
		t.Fatalf("a DPS enviada nao e XML valido: %v", err)
	}
	if el := doc.FindElement("DPS/infDPS/valores/vServPrest/vServ"); el == nil || el.Text() != "1200.00" {
		t.Errorf("valor enviado incorreto: %v", el)
	}

	// The authorised invoice must land on disk, named by its access key.
	nfsePath := filepath.Join(dir, "notas", accessKey+"-nfse.xml")
	saved, err := os.ReadFile(nfsePath)
	if err != nil {
		t.Fatalf("NFS-e nao foi gravada: %v", err)
	}
	if string(saved) != nfseXML {
		t.Error("a NFS-e gravada nao confere com a recebida")
	}

	for _, want := range []string{"NFS-e emitida", accessKey, "producao-restrita", "NAO tem valor fiscal"} {
		if !strings.Contains(out, want) {
			t.Errorf("a saida nao menciona %q:\n%s", want, out)
		}
	}
}

func TestEmitir_EnviarRejeicao(t *testing.T) {
	stubSefin(t, func(w http.ResponseWriter, body map[string]string) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any{
			"tipoAmbiente":          2,
			"versaoAplicativo":      "1.0.0",
			"dataHoraProcessamento": "2026-09-18T09:57:36-03:00",
			"erros": []map[string]string{
				{"codigo": "E001", "descricao": "Municipio nao conveniado"},
				{"codigo": "E042", "descricao": "cTribNac invalido", "complemento": "010101"},
			},
		})
	})

	dir := workspace(t)
	_, err := runEmit(t, dir, "--numero", "51", "--valor", "100", "--descricao", "x", "--enviar")
	if err == nil {
		t.Fatal("esperava erro para DPS rejeitada")
	}

	// Every rejection reason must reach the user in one go.
	for _, want := range []string{"E001", "Municipio nao conveniado", "E042", "010101"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("a mensagem nao menciona %q:\n%v", want, err)
		}
	}

	// A rejected emission must not leave an invoice file behind.
	if matches, _ := filepath.Glob(filepath.Join(dir, "notas", "*-nfse.xml")); len(matches) != 0 {
		t.Errorf("uma emissao rejeitada gravou %d arquivo(s) de NFS-e", len(matches))
	}
}

// TestEmitir_EnviarComSemAssinar pins that the two flags are refused together:
// the Sefin only accepts a signed declaration, so the combination is a mistake
// worth catching before anything is written.
func TestEmitir_EnviarComSemAssinar(t *testing.T) {
	dir := workspace(t)
	_, err := runEmit(t, dir, "--numero", "52", "--valor", "100",
		"--descricao", "x", "--enviar", "--sem-assinar")
	if err == nil {
		t.Fatal("esperava erro ao combinar --enviar com --sem-assinar")
	}
	if !strings.Contains(err.Error(), "assinada") {
		t.Errorf("a mensagem deveria explicar o motivo: %v", err)
	}
}
