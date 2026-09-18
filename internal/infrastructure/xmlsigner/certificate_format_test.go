package xmlsigner

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"testing"
	"time"

	pkcs12 "software.sslmate.com/src/go-pkcs12"
)

// TestParsePFX_Encodings guards against a regression that made the emitter
// unusable: golang.org/x/crypto/pkcs12 could only read PKCS#12 files built with
// SHA-1 and 3DES. Files produced by OpenSSL 3 (AES-256-CBC with a SHA-256 MAC)
// failed with "unknown digest algorithm", and recent ICP-Brasil A1 certificates
// are commonly in that format.
func TestParsePFX_Encodings(t *testing.T) {
	const password = "senha-de-teste"

	key, leaf, ca := newTestChain(t)

	encoders := map[string]*pkcs12.Encoder{
		"modern": pkcs12.Modern, // AES-256-CBC, SHA-256 MAC — OpenSSL 3 default
		"legacy": pkcs12.Legacy, // 3DES, SHA-1 MAC — older files still in the wild
	}

	for name, encoder := range encoders {
		t.Run(name, func(t *testing.T) {
			pfx, err := encoder.Encode(key, leaf, []*x509.Certificate{ca}, password)
			if err != nil {
				t.Fatalf("encoding the %s fixture: %v", name, err)
			}

			info, err := ParsePFX(pfx, password)
			if err != nil {
				t.Fatalf("ParsePFX rejected a valid %s PKCS#12 file: %v", name, err)
			}

			if got, want := info.GetSubjectCN(), leaf.Subject.CommonName; got != want {
				t.Errorf("subject CN = %q, want %q", got, want)
			}
			if info.PrivateKey == nil {
				t.Error("private key was not extracted")
			}
			// The intermediate must survive parsing: ICP-Brasil A1 files ship the
			// chain, and dropping it silently used to be the documented behaviour.
			if len(info.Chain) != 1 {
				t.Errorf("chain length = %d, want 1 (the intermediate CA)", len(info.Chain))
			}
		})
	}

	t.Run("senha incorreta", func(t *testing.T) {
		pfx, err := pkcs12.Modern.Encode(key, leaf, nil, password)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := ParsePFX(pfx, "senha-errada"); err == nil {
			t.Fatal("ParsePFX accepted a wrong password")
		}
	})
}

// newTestChain builds a throwaway CA and a leaf certificate signed by it.
func newTestChain(t *testing.T) (*rsa.PrivateKey, *x509.Certificate, *x509.Certificate) {
	t.Helper()

	caKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	caTmpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "AC TESTE"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(365 * 24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign,
		BasicConstraintsValid: true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTmpl, caTmpl, &caKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	ca, err := x509.ParseCertificate(caDER)
	if err != nil {
		t.Fatal(err)
	}

	leafKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	leafTmpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "EMPRESA TESTE LTDA:12345678000199"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
	}
	leafDER, err := x509.CreateCertificate(rand.Reader, leafTmpl, ca, &leafKey.PublicKey, caKey)
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := x509.ParseCertificate(leafDER)
	if err != nil {
		t.Fatal(err)
	}

	return leafKey, leaf, ca
}
