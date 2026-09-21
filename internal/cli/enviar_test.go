// Copyright 2026 Eduardo Souza
// SPDX-License-Identifier: FSL-1.1-MIT

package cli

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/edusouza/nfse-emissor-go/internal/config"
)

func execEnviar(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()
	t.Setenv(envCertPassword, testCertPassword)

	var out bytes.Buffer
	root := NewRootCommand()
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(append([]string{"enviar", "--config", filepath.Join(dir, "nfse.yaml")}, args...))

	err := root.Execute()
	return out.String(), err
}

// The whole point of the command: what reaches the government is the file on
// disk, byte for byte. Rebuilding it would change dhEmi and the signature, so
// the document reviewed offline would not be the document issued.
func TestEnviar_TransmitsTheFileUnchanged(t *testing.T) {
	dir := workspace(t)

	if out, err := runEmit(t, dir, "--numero", "20", "--valor", "1500",
		"--descricao", "Consultoria"); err != nil {
		t.Fatalf("emissao falhou: %v\n%s", err, out)
	}
	dpsPath := onlyXMLPath(t, dir)
	onDisk, err := os.ReadFile(dpsPath)
	if err != nil {
		t.Fatal(err)
	}

	var received string
	stubQuery(t, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
		}
		received = decodePayload(t, body["dpsXmlGZipB64"])
		respondEmission(t, w)
	})

	out, err := execEnviar(t, dir, dpsPath)
	if err != nil {
		t.Fatalf("envio falhou: %v\n%s", err, out)
	}

	if received != string(onDisk) {
		t.Errorf("a Sefin recebeu um documento diferente do arquivo em disco")
	}
	if !strings.Contains(out, "NFS-e emitida") {
		t.Errorf("saida nao confirma a emissao:\n%s", out)
	}
}

// --sem-assinar names its output the same way, so an unsigned file is the
// likely mistake. Catching it here beats a rejection from the government.
func TestEnviar_RefusesUnsignedDPS(t *testing.T) {
	dir := workspace(t)

	if out, err := runEmit(t, dir, "--numero", "21", "--valor", "100",
		"--descricao", "Servico", "--sem-assinar"); err != nil {
		t.Fatalf("emissao falhou: %v\n%s", err, out)
	}

	out, err := execEnviar(t, dir, onlyXMLPath(t, dir))
	if err == nil {
		t.Fatalf("esperava recusa de uma DPS sem assinatura\n%s", out)
	}
	if !strings.Contains(err.Error(), "assinad") {
		t.Errorf("a mensagem deveria explicar que falta assinatura: %v", err)
	}
}

func TestEnviar_RefusesSomethingThatIsNotADPS(t *testing.T) {
	dir := workspace(t)

	path := filepath.Join(dir, "nfse.xml")
	if err := os.WriteFile(path, []byte(`<?xml version="1.0"?><NFSe><infNFSe Id="X"/></NFSe>`), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := execEnviar(t, dir, path)
	if err == nil {
		t.Fatalf("esperava recusa de um XML que nao e DPS\n%s", out)
	}
	if !strings.Contains(err.Error(), "infDPS") {
		t.Errorf("a mensagem deveria dizer o que esperava encontrar: %v", err)
	}
}

func TestEnviar_ReportsMissingFile(t *testing.T) {
	dir := workspace(t)

	out, err := execEnviar(t, dir, filepath.Join(dir, "nao-existe.xml"))
	if err == nil {
		t.Fatalf("esperava erro para arquivo inexistente\n%s", out)
	}
	if !strings.Contains(err.Error(), "nao-existe.xml") {
		t.Errorf("a mensagem deveria nomear o arquivo: %v", err)
	}
}

// The counter belongs to emission. Transmitting a document that was already
// written must not burn another number.
func TestEnviar_DoesNotAdvanceTheCounter(t *testing.T) {
	dir := workspace(t)

	if out, err := runEmit(t, dir, "--valor", "300", "--descricao", "Servico"); err != nil {
		t.Fatalf("emissao falhou: %v\n%s", err, out)
	}
	before := readCounter(t, dir)

	stubQuery(t, func(w http.ResponseWriter, r *http.Request) {
		respondEmission(t, w)
	})
	if out, err := execEnviar(t, dir, onlyXMLPath(t, dir)); err != nil {
		t.Fatalf("envio falhou: %v\n%s", err, out)
	}

	if after := readCounter(t, dir); after != before {
		t.Errorf("contador foi de %q para %q; enviar nao deveria numerar nada", before, after)
	}
}

func TestEnviar_ClientSeeksSefinConfiguration(t *testing.T) {
	var gotPath string
	stubQuery(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		respondEmission(t, w)
	})

	dir := workspace(t)
	if out, err := runEmit(t, dir, "--numero", "30", "--valor", "10",
		"--descricao", "Servico"); err != nil {
		t.Fatalf("emissao falhou: %v\n%s", err, out)
	}
	if out, err := execEnviar(t, dir, onlyXMLPath(t, dir)); err != nil {
		t.Fatalf("envio falhou: %v\n%s", err, out)
	}

	if !strings.HasSuffix(gotPath, "/nfse") {
		t.Errorf("caminho = %q, esperava terminar em /nfse", gotPath)
	}
}

// respondEmission answers with the swagger's NFSePostResponseSucesso shape.
func respondEmission(t *testing.T, w http.ResponseWriter) {
	t.Helper()

	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if _, err := zw.Write([]byte(`<?xml version="1.0"?><NFSe><infNFSe Id="N1"/></NFSe>`)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	payload := base64.StdEncoding.EncodeToString(buf.Bytes())

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"idDps":                 "DPS1",
		"chaveAcesso":           strings.Repeat("7", 50),
		"nfseXmlGZipB64":        payload,
		"tipoAmbiente":          2,
		"versaoAplicativo":      "1.0",
		"dataHoraProcessamento": "2026-09-18T12:00:00Z",
	})
}

// decodePayload undoes the gzip+base64 the API wraps every XML in.
func decodePayload(t *testing.T, encoded string) string {
	t.Helper()

	compressed, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	zr, err := gzip.NewReader(bytes.NewReader(compressed))
	if err != nil {
		t.Fatal(err)
	}
	defer zr.Close()

	raw, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

// onlyXMLPath returns the path of the single XML written under the workspace.
func onlyXMLPath(t *testing.T, dir string) string {
	t.Helper()

	matches, err := filepath.Glob(filepath.Join(dir, "notas", "*.xml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 1 {
		t.Fatalf("esperava exatamente 1 XML gerado, encontrei %d", len(matches))
	}
	return matches[0]
}

// readCounter returns the raw numbering state, so a test can assert that a
// command left it untouched.
func readCounter(t *testing.T, dir string) string {
	t.Helper()

	data, err := os.ReadFile(filepath.Join(dir, config.StateFileName))
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}
