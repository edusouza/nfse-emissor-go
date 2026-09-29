package cli

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/edusouza/nfse-emissor-go/internal/infrastructure/xmlsigner"
)

// certFor builds a CertificateInfo carrying only what the check reads: the
// common name and, optionally, the subjectAltName.
func certFor(cn string, extensions ...pkix.Extension) *xmlsigner.CertificateInfo {
	return &xmlsigner.CertificateInfo{
		Certificate: &x509.Certificate{Subject: pkix.Name{CommonName: cn}, Extensions: extensions},
	}
}

func TestEnsureCertificateBelongsToProvider(t *testing.T) {
	for _, tc := range []struct {
		name     string
		cn       string
		provider string
		wantErr  bool
	}{
		{"mesmo CNPJ", "EMPRESA LTDA:12345678000195", "12345678000195", false},
		{"CNPJ do prestador mascarado", "EMPRESA LTDA:12345678000195", "12.345.678/0001-95", false},
		{"outra empresa", "EMPRESA LTDA:12345678000195", "11222333000181", true},

		// The emitter only knows the certificate's CNPJ when the certificate
		// follows the ICP-Brasil layout. Refusing anything else would lock out
		// certificates laid out differently, so silence is the safe answer.
		{"certificado sem CNPJ no CN nem no SAN", "EMPRESA LTDA", "11222333000181", false},
		{"CN com digitos invalidos", "EMPRESA LTDA:12345678000100", "11222333000181", false},
		{"prestador desconhecido", "EMPRESA LTDA:12345678000195", "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := ensureCertificateBelongsToProvider(certFor(tc.cn), tc.provider)
			if tc.wantErr && err == nil {
				t.Fatal("esperava recusa")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("nao esperava erro: %v", err)
			}
		})
	}
}

// The Sefin identifies the signer by the subjectAltName, so that is what the
// check compares — including when the common name has no CNPJ, or a different
// one.
func TestEnsureCertificateBelongsToProvider_PeloSubjectAltName(t *testing.T) {
	san := icpBrasilCNPJExtension(t, "12345678000195")

	for _, tc := range []struct {
		name     string
		cn       string
		provider string
		wantErr  bool
	}{
		{"mesmo CNPJ", "EMPRESA LTDA", "12345678000195", false},
		{"outra empresa", "EMPRESA LTDA", "11222333000181", true},
		{"CN diz outra empresa, SAN diz o prestador", "EMPRESA LTDA:11222333000181", "12345678000195", false},
		{"CN diz o prestador, SAN diz outra empresa", "EMPRESA LTDA:11222333000181", "11222333000181", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			err := ensureCertificateBelongsToProvider(certFor(tc.cn, san), tc.provider)
			if tc.wantErr && err == nil {
				t.Fatal("esperava recusa")
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("nao esperava erro: %v", err)
			}
		})
	}

	err := ensureCertificateBelongsToProvider(certFor("EMPRESA LTDA", san), "11222333000181")
	for _, want := range []string{"12.345.678/0001-95", "11.222.333/0001-81", "EMPRESA LTDA"} {
		if err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("mensagem nao menciona %q:\n%v", want, err)
		}
	}
}

// The message exists to end a specific confusion, so it has to name both
// numbers: a refusal that does not say which certificate and which provider
// leaves the user exactly where the bare 403 left them.
func TestEnsureCertificateBelongsToProvider_MessageNamesBothParties(t *testing.T) {
	err := ensureCertificateBelongsToProvider(
		certFor("EMPRESA EXEMPLO LTDA:12345678000195"), "11222333000181")
	if err == nil {
		t.Fatal("esperava recusa")
	}

	for _, want := range []string{
		"12.345.678/0001-95",
		"11.222.333/0001-81",
		"EMPRESA EXEMPLO LTDA",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("mensagem nao menciona %q:\n%s", want, err)
		}
	}
}

// Catching it at emission matters more than catching it at send: a DPS signed
// with the wrong certificate cannot be rescued by resending it with the right
// one, because the signature inside the XML is part of what is validated.
func TestEmitir_RefusesCertificateFromAnotherCompany(t *testing.T) {
	dir := workspaceCNPJ(t, "11222333000181")

	out, err := runEmit(t, dir, "--numero", "3", "--valor", "1500", "--descricao", "Consultoria")
	if err == nil {
		t.Fatalf("esperava recusa do certificado de outra empresa\n%s", out)
	}
	if !strings.Contains(err.Error(), "11.222.333/0001-81") {
		t.Errorf("a mensagem deveria nomear o prestador: %v", err)
	}

	if matches, _ := filepath.Glob(filepath.Join(dir, "notas", "*.xml")); len(matches) != 0 {
		t.Errorf("nao deveria ter gravado XML algum, encontrei %d", len(matches))
	}
}

func TestEnviar_RefusesCertificateFromAnotherCompany(t *testing.T) {
	dir := workspace(t)

	// A DPS belonging to someone else: what the user gets by pointing --cert at
	// the throwaway certificate from exemplos/.
	path := filepath.Join(dir, "alheia-dps.xml")
	alheia := `<?xml version="1.0"?><DPS><infDPS Id="DPS410690221122233300018100001000000000000003">` +
		`<prest><CNPJ>11222333000181</CNPJ></prest>` +
		`<valores><vServPrest><vServ>1500.00</vServ></vServPrest></valores>` +
		`</infDPS><Signature/></DPS>`
	if err := os.WriteFile(path, []byte(alheia), 0o644); err != nil {
		t.Fatal(err)
	}

	out, err := execEnviar(t, dir, path)
	if err == nil {
		t.Fatalf("esperava recusa antes de abrir a conexao\n%s", out)
	}
	if !strings.Contains(err.Error(), "11.222.333/0001-81") {
		t.Errorf("a mensagem deveria nomear o prestador do arquivo: %v", err)
	}
}

func TestFormatCNPJ(t *testing.T) {
	if got := formatCNPJ("12345678000195"); got != "12.345.678/0001-95" {
		t.Errorf("formatCNPJ = %q", got)
	}
	// Anything that is not a CNPJ passes through rather than being mangled.
	if got := formatCNPJ("123"); got != "123" {
		t.Errorf("formatCNPJ = %q, esperava passar direto", got)
	}
}
