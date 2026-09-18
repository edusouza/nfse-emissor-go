package sefin

import (
	"bytes"
	"compress/gzip"
	"encoding/base64"
	"strings"
	"testing"
)

func TestEncodeDecodeRoundtrip(t *testing.T) {
	cases := map[string]string{
		"vazio":     "",
		"pequeno":   "<DPS/>",
		"acentuado": "<xDescServ>Manutenção de software — agosto/2026</xDescServ>",
		"grande":    strings.Repeat("<infDPS>conteudo repetido</infDPS>", 5000),
	}

	for name, original := range cases {
		t.Run(name, func(t *testing.T) {
			encoded, err := encodeGzipBase64([]byte(original))
			if err != nil {
				t.Fatalf("encode falhou: %v", err)
			}

			// The wire format must be standard base64 of a gzip stream.
			raw, err := base64.StdEncoding.DecodeString(encoded)
			if err != nil {
				t.Fatalf("saida nao e base64 padrao: %v", err)
			}
			if _, err := gzip.NewReader(bytes.NewReader(raw)); err != nil {
				t.Fatalf("conteudo decodificado nao e gzip: %v", err)
			}

			decoded, err := decodeGzipBase64(encoded)
			if err != nil {
				t.Fatalf("decode falhou: %v", err)
			}
			if string(decoded) != original {
				t.Errorf("roundtrip alterou o conteudo (%d bytes -> %d bytes)", len(original), len(decoded))
			}
		})
	}
}

func TestDecodeRejectsBadInput(t *testing.T) {
	t.Run("nao e base64", func(t *testing.T) {
		if _, err := decodeGzipBase64("nao!e!base64!"); err == nil {
			t.Fatal("esperava erro para base64 invalido")
		}
	})

	t.Run("base64 valido mas nao e gzip", func(t *testing.T) {
		plain := base64.StdEncoding.EncodeToString([]byte("isto nao esta compactado"))
		_, err := decodeGzipBase64(plain)
		if err == nil {
			t.Fatal("esperava erro para conteudo nao compactado")
		}
		if !strings.Contains(err.Error(), "gzip") {
			t.Errorf("a mensagem deveria apontar o gzip: %v", err)
		}
	})
}

// TestDecodeCapsExpansion guards the decompression limit. A gzip stream can
// expand enormously; a malformed or misidentified payload must fail with a
// clear error instead of consuming all available memory.
func TestDecodeCapsExpansion(t *testing.T) {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	// Highly compressible content well past the cap.
	chunk := bytes.Repeat([]byte{'A'}, 1<<20)
	for written := 0; written <= maxDecompressedSize; written += len(chunk) {
		if _, err := zw.Write(chunk); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}

	_, err := decodeGzipBase64(base64.StdEncoding.EncodeToString(buf.Bytes()))
	if err == nil {
		t.Fatal("esperava erro ao exceder o limite de descompactacao")
	}
	if !strings.Contains(err.Error(), "limite") {
		t.Errorf("a mensagem deveria citar o limite: %v", err)
	}
}
