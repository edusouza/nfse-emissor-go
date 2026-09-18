package xmlsigner

import (
	"strings"
	"testing"
)

// The project had no test that signed a document and then verified it. Every
// signing path was covered in isolation, and every verifier test used a
// hand-written negative fixture, so three defects coexisted unnoticed:
//
//   - the document was re-indented after signing, inserting whitespace into
//     infDPS and SignedInfo after their digests had been computed;
//   - canonicalization dropped the namespace declaration at the apex of the
//     canonicalized subtree, producing a form no conformant verifier would
//     reproduce;
//   - certificate validation demanded a private key, which a verifier reading
//     KeyInfo can never have.
//
// Any of the three alone is enough to have every emitted invoice rejected.

func TestSignThenVerify(t *testing.T) {
	signers := map[string]func(*XMLSigner, string) (string, error){
		"SignDPS":        (*XMLSigner).SignDPS,
		"SignDPSCompact": (*XMLSigner).SignDPSCompact,
		"SignDPSWithResult": func(s *XMLSigner, xml string) (string, error) {
			res, err := s.SignDPSWithResult(xml)
			if err != nil {
				return "", err
			}
			return res.SignedXML, nil
		},
	}

	for name, sign := range signers {
		t.Run(name, func(t *testing.T) {
			cert := generateTestCertificate(t)

			signed, err := sign(NewXMLSigner(cert), testUnsignedDPS)
			if err != nil {
				t.Fatalf("assinatura falhou: %v", err)
			}

			result, err := NewXMLVerifier().VerifyDPSSignature(signed)
			if err != nil {
				t.Fatalf("verificacao retornou erro: %v", err)
			}
			if !result.Valid {
				t.Fatalf("a assinatura produzida nao verifica: %v", result.Errors)
			}
			if result.SignerCN == "" {
				t.Error("o CN do signatario nao foi extraido do certificado")
			}
			if result.SignedElementID == "" {
				t.Error("o ID do elemento assinado nao foi identificado")
			}
		})
	}
}

func TestSignThenVerify_DetectsTampering(t *testing.T) {
	cert := generateTestCertificate(t)

	signed, err := NewXMLSigner(cert).SignDPS(testUnsignedDPS)
	if err != nil {
		t.Fatal(err)
	}

	// Change a value inside the signed content.
	tampered := strings.Replace(signed, "<vServ>1000.00</vServ>", "<vServ>1.00</vServ>", 1)
	if tampered == signed {
		t.Fatal("a fixture mudou: o valor esperado nao foi encontrado")
	}

	result, err := NewXMLVerifier().VerifyDPSSignature(tampered)
	if err != nil {
		t.Fatalf("verificacao retornou erro: %v", err)
	}
	if result.Valid {
		t.Error("a verificacao aceitou um documento adulterado")
	}
}
