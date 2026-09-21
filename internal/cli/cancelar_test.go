// Copyright 2026 Eduardo Souza
// SPDX-License-Identifier: FSL-1.1-MIT

package cli

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/beevik/etree"

	"github.com/edusouza/nfse-emissor-go/internal/infrastructure/xmlsigner"
)

const justificativaValida = "Valor do servico lancado incorretamente na nota"

func runCancel(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()
	t.Setenv(envCertPassword, testCertPassword)

	var out bytes.Buffer
	root := NewRootCommand()
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs(append([]string{"cancelar", "--config", filepath.Join(dir, "nfse.yaml")}, args...))

	err := root.Execute()
	return out.String(), err
}

func TestCancelar(t *testing.T) {
	const eventoXML = `<?xml version="1.0"?><evento><infEvento Id="EVT1"/></evento>`
	chave := strings.Repeat("2", 50)

	var gotPath, recebido string
	stubQuery(t, func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path

		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		recebido = body["pedidoRegistroEventoXmlGZipB64"]

		var buf bytes.Buffer
		zw := newGzipWriter(&buf)
		zw.Write([]byte(eventoXML))
		zw.Close()

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(map[string]any{
			"tipoAmbiente":          2,
			"versaoAplicativo":      "1.0.0",
			"dataHoraProcessamento": "2026-09-18T09:57:36-03:00",
			"eventoXmlGZipB64":      encodeBase64(buf.Bytes()),
		})
	})

	dir := workspace(t)
	out, err := runCancel(t, dir, chave, "--motivo", "erro-emissao", "--justificativa", justificativaValida)
	if err != nil {
		t.Fatalf("cancelamento falhou: %v\n%s", err, out)
	}

	if want := "/nfse/" + chave + "/eventos"; gotPath != want {
		t.Errorf("caminho = %q, esperava %q", gotPath, want)
	}

	// The request must arrive signed, gzipped and base64-encoded.
	if recebido == "" {
		t.Fatal("a Sefin nao recebeu o campo pedidoRegistroEventoXmlGZipB64")
	}
	enviado := decodeForTest(t, recebido)
	if !strings.Contains(enviado, "<Signature") {
		t.Error("o pedido enviado nao esta assinado")
	}

	doc := etree.NewDocument()
	if err := doc.ReadFromString(enviado); err != nil {
		t.Fatalf("o pedido enviado nao e XML valido: %v", err)
	}
	if el := doc.FindElement("pedRegEvento/infPedReg/chNFSe"); el == nil || el.Text() != chave {
		t.Errorf("chNFSe incorreto: %v", el)
	}
	if el := doc.FindElement("pedRegEvento/infPedReg/e101101/cMotivo"); el == nil || el.Text() != "1" {
		t.Errorf("cMotivo incorreto: %v", el)
	}

	// The signature must verify: the signer had to be generalised beyond DPS,
	// and a signature over the wrong element would still look fine here.
	verification, err := xmlsigner.NewXMLVerifier().VerifySignature(enviado)
	if err != nil {
		t.Fatalf("verificacao falhou: %v", err)
	}
	if !verification.Valid {
		t.Errorf("a assinatura do pedido nao verifica: %v", verification.Errors)
	}

	saved, err := os.ReadFile(filepath.Join(dir, "notas", "PRE"+chave+"101101-evento.xml"))
	if err != nil {
		t.Fatalf("evento nao foi gravado: %v", err)
	}
	if string(saved) != eventoXML {
		t.Error("o evento gravado nao confere com o recebido")
	}
	if !strings.Contains(out, "Cancelamento registrado") {
		t.Errorf("saida inesperada:\n%s", out)
	}
}

func TestCancelar_ArgumentosInvalidos(t *testing.T) {
	dir := workspace(t)
	chave := strings.Repeat("2", 50)

	cases := []struct {
		name string
		args []string
		want string
	}{
		{
			name: "sem motivo",
			args: []string{chave, "--justificativa", justificativaValida},
			want: "--motivo",
		},
		{
			name: "motivo desconhecido",
			args: []string{chave, "--motivo", "porque-sim", "--justificativa", justificativaValida},
			want: "--motivo",
		},
		{
			// TSMotivo demands 15 characters: the justification lands on the
			// fiscal record and "erro" tells a later reader nothing.
			name: "justificativa curta",
			args: []string{chave, "--motivo", "outros", "--justificativa", "erro"},
			want: "ao menos 15",
		},
		{
			name: "chave invalida",
			args: []string{"123", "--motivo", "outros", "--justificativa", justificativaValida},
			want: "50 digitos",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := runCancel(t, dir, tc.args...); err == nil {
				t.Fatal("esperava erro")
			} else if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("a mensagem nao menciona %q: %v", tc.want, err)
			}
		})
	}
}

func TestCancelar_Rejeicao(t *testing.T) {
	stubQuery(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any{
			"tipoAmbiente":          2,
			"versaoAplicativo":      "1.0.0",
			"dataHoraProcessamento": "2026-09-18T09:57:36-03:00",
			"erro":                  map[string]string{"codigo": "E900", "descricao": "Prazo de cancelamento expirado"},
		})
	})

	dir := workspace(t)
	_, err := runCancel(t, dir, strings.Repeat("2", 50), "--motivo", "outros", "--justificativa", justificativaValida)
	if err == nil {
		t.Fatal("esperava erro para cancelamento rejeitado")
	}
	for _, want := range []string{"E900", "Prazo de cancelamento expirado"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("a mensagem nao menciona %q: %v", want, err)
		}
	}

	// A refused cancellation must not leave an event file behind.
	if m, _ := filepath.Glob(filepath.Join(dir, "notas", "*-evento.xml")); len(m) != 0 {
		t.Errorf("um cancelamento rejeitado gravou %d arquivo(s)", len(m))
	}
}
