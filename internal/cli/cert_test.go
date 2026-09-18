package cli

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	pkcs12 "software.sslmate.com/src/go-pkcs12"
)

func TestCertInfo(t *testing.T) {
	const password = "senha-de-teste"

	tests := []struct {
		name        string
		notAfter    time.Time
		wantErr     bool
		wantOutputs []string
	}{
		{
			name:        "certificado valido",
			notAfter:    time.Now().Add(300 * 24 * time.Hour),
			wantOutputs: []string{"EMPRESA TESTE", "2048 bits", "apto a assinar"},
		},
		{
			name:        "certificado proximo do vencimento",
			notAfter:    time.Now().Add(5 * 24 * time.Hour),
			wantOutputs: []string{"aviso:", "expira em"},
		},
		{
			name:     "certificado expirado",
			notAfter: time.Now().Add(-24 * time.Hour),
			wantErr:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := writeTestPFX(t, password, tc.notAfter)

			var out bytes.Buffer
			root := NewRootCommand()
			root.SetOut(&out)
			root.SetErr(&out)
			root.SetArgs([]string{"cert", "info", "--arquivo", path, "--senha", password})

			err := root.Execute()
			if tc.wantErr && err == nil {
				t.Fatal("esperava erro para certificado invalido, obteve nil")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("erro inesperado: %v", err)
			}

			for _, want := range tc.wantOutputs {
				if !strings.Contains(out.String(), want) {
					t.Errorf("saida nao contem %q\n--- saida ---\n%s", want, out.String())
				}
			}
		})
	}
}

func TestCertInfoWrongPassword(t *testing.T) {
	path := writeTestPFX(t, "senha-certa", time.Now().Add(300*24*time.Hour))

	var out bytes.Buffer
	root := NewRootCommand()
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"cert", "info", "--arquivo", path, "--senha", "senha-errada"})

	err := root.Execute()
	if err == nil {
		t.Fatal("esperava erro com senha incorreta")
	}
	// The message must point at the likely cause; a raw ASN.1 error helps nobody.
	if !strings.Contains(err.Error(), "senha incorreta") {
		t.Errorf("mensagem de erro pouco util: %v", err)
	}
}

func TestCertInfoMissingFile(t *testing.T) {
	var out bytes.Buffer
	root := NewRootCommand()
	root.SetOut(&out)
	root.SetErr(&out)
	root.SetArgs([]string{"cert", "info", "--arquivo", filepath.Join(t.TempDir(), "nao-existe.pfx"), "--senha", "x"})

	if err := root.Execute(); err == nil {
		t.Fatal("esperava erro para arquivo inexistente")
	}
}

// writeTestPFX builds a throwaway self-signed PKCS#12 file and returns its path.
func writeTestPFX(t *testing.T, password string, notAfter time.Time) string {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(42),
		Subject:      pkix.Name{CommonName: "EMPRESA TESTE LTDA:12345678000199"},
		Issuer:       pkix.Name{CommonName: "AC TESTE"},
		NotBefore:    time.Now().Add(-365 * 24 * time.Hour),
		NotAfter:     notAfter,
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}

	pfx, err := pkcs12.Modern.Encode(key, cert, nil, password)
	if err != nil {
		t.Fatal(err)
	}

	path := filepath.Join(t.TempDir(), "teste.pfx")
	if err := os.WriteFile(path, pfx, 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}
